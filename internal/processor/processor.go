// Package processor предоставляет функциональность для обработки событий вебхуков от Gitea.
package processor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/example/gitea-jenkins-webhook/internal/checks"
	"github.com/example/gitea-jenkins-webhook/internal/config"
	"github.com/example/gitea-jenkins-webhook/internal/gitea"
	"github.com/example/gitea-jenkins-webhook/internal/jenkins"
	"github.com/example/gitea-jenkins-webhook/pkg/webhook"
)

// JenkinsClient определяет интерфейс для работы с задачами Jenkins.
type JenkinsClient interface {
	WaitForJob(ctx context.Context, pattern *regexp.Regexp, jobRoot string, timeout, interval time.Duration) (*jenkins.Job, error)
}

// GiteaClient определяет интерфейс для публикации комментариев и статусов в Gitea.
type GiteaClient interface {
	PostComment(ctx context.Context, repoFullName string, issueIndex int64, body string) error
	ListPullRequestFiles(ctx context.Context, repoFullName string, index int64) ([]gitea.PullRequestFile, error)
	CreateCommitStatus(ctx context.Context, repoFullName, sha string, status gitea.CommitStatus) error
}

// Processor обрабатывает события pull request из Gitea, ожидает появления соответствующих
// задач в Jenkins и публикует комментарии с результатами в Gitea.
type Processor struct {
	cfg     *config.Config
	log     *slog.Logger
	jc      JenkinsClient
	gc      GiteaClient
	queue   chan webhook.PullRequestEvent
	wg      sync.WaitGroup
	started bool
	mu      sync.Mutex
}

// New создает новый процессор событий с указанной конфигурацией и клиентами.
// Если logger равен nil, используется логгер по умолчанию.
func New(cfg *config.Config, jc JenkinsClient, gc GiteaClient, logger *slog.Logger) *Processor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Processor{
		cfg:   cfg,
		log:   logger,
		jc:    jc,
		gc:    gc,
		queue: make(chan webhook.PullRequestEvent, cfg.Server.QueueSize),
	}
}

// Start запускает процессор, создавая пул воркеров для обработки событий.
// Если процессор уже запущен, выводит предупреждение и не выполняет повторный запуск.
func (p *Processor) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		p.log.Warn("processor already started")
		return
	}

	p.log.Info("starting processor",
		"worker_pool_size", p.cfg.Server.WorkerPoolSize,
		"queue_size", p.cfg.Server.QueueSize)
	for i := 0; i < p.cfg.Server.WorkerPoolSize; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	p.started = true
	p.log.Info("processor started successfully", "workers", p.cfg.Server.WorkerPoolSize)
}

// Stop останавливает процессор, закрывая очередь и ожидая завершения всех воркеров.
func (p *Processor) Stop() {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return
	}
	p.log.Info("stopping processor, closing queue")
	close(p.queue)
	p.mu.Unlock()
	p.wg.Wait()
	p.log.Info("processor stopped, all workers finished")
}

// Enqueue добавляет событие в очередь обработки.
// Возвращает ошибку, если процессор не запущен или очередь переполнена.
func (p *Processor) Enqueue(evt webhook.PullRequestEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started {
		p.log.Error("attempted to enqueue event but processor not started")
		return errors.New("processor not started")
	}
	select {
	case p.queue <- evt:
		p.log.Debug("event enqueued",
			"repo", evt.Repository.FullName,
			"pr_number", evt.PullRequest.Number,
			"queue_length", len(p.queue))
		return nil
	default:
		p.log.Warn("processor queue is full",
			"repo", evt.Repository.FullName,
			"pr_number", evt.PullRequest.Number,
			"queue_size", p.cfg.Server.QueueSize)
		return fmt.Errorf("processor queue is full")
	}
}

// worker обрабатывает события из очереди. Запускается в отдельной горутине.
// id - уникальный идентификатор воркера для логирования.
func (p *Processor) worker(id int) {
	p.log.Debug("worker started", "worker_id", id)
	defer func() {
		p.log.Debug("worker stopped", "worker_id", id)
		p.wg.Done()
	}()
	for evt := range p.queue {
		p.log.Debug("worker processing event",
			"worker_id", id,
			"repo", evt.Repository.FullName,
			"pr_number", evt.PullRequest.Number)
		p.processEvent(context.Background(), evt)
	}
}

// processEvent обрабатывает одно событие pull request:
// - выполняет проверки из конфигурации и публикует статус коммита
// - проверяет наличие правил для репозитория
// - для Jenkins обрабатывает только события opened и reopened
// - ожидает появления задачи Jenkins по шаблону
// - публикует комментарий в Gitea с результатом
func (p *Processor) processEvent(ctx context.Context, evt webhook.PullRequestEvent) {
	p.log.Debug("processing event",
		"action", evt.Action,
		"repo", evt.Repository.FullName,
		"pr_number", evt.PullRequest.Number,
		"sender", evt.Sender.Login)

	if evt.Repository.FullName == "" {
		p.log.Warn("event missing repository", "event", evt)
		return
	}

	p.runChecks(ctx, evt)

	rule, ok := p.cfg.GetRepositoryRule(evt.Repository.FullName)
	if !ok {
		p.log.Info("repository not configured, skipping jenkins", "repo", evt.Repository.FullName)
		return
	}

	p.log.Debug("repository rule found",
		"repo", evt.Repository.FullName,
		"rule_name", rule.Name,
		"job_pattern", rule.JobPattern,
		"job_root", rule.JobRoot,
		"timeout", rule.Timeout,
		"poll_interval", rule.PollInterval)

	if evt.Action != "opened" && evt.Action != "reopened" {
		p.log.Info("ignoring pull request action for jenkins", "action", evt.Action)
		return
	}

	ctx = context.WithValue(ctx, "repository", evt.Repository.FullName)
	p.log.Info("processing pull request",
		"repo", evt.Repository.FullName,
		"pr", evt.PullRequest.Number,
		"title", evt.PullRequest.Title)

	data := map[string]any{
		"Number":  evt.PullRequest.Number,
		"Title":   evt.PullRequest.Title,
		"Repo":    evt.Repository.FullName,
		"Sender":  evt.Sender.Login,
		"Timeout": rule.Timeout,
	}

	var (
		jobFound *jenkins.Job
		pattern  string
		err      error
	)

	p.log.Debug("processing job pattern",
		"pattern_template", rule.JobPattern)
	pattern, err = executeTemplate("pattern", rule.JobPattern, data)
	if err != nil {
		p.log.Error("failed to execute pattern template",
			"err", err,
			"pattern_template", rule.JobPattern)
		return
	}
	p.log.Debug("pattern template executed",
		"compiled_pattern", pattern)
	re, err := regexp.Compile(pattern)
	if err != nil {
		p.log.Error("invalid regex pattern",
			"pattern", pattern,
			"err", err)
		return
	}

	p.log.Info("waiting for jenkins job",
		"pattern", pattern,
		"job_root", rule.JobRoot,
		"timeout", rule.Timeout,
		"poll_interval", rule.PollInterval)
	jobFound, err = p.jc.WaitForJob(ctx, re, rule.JobRoot, rule.Timeout, rule.PollInterval)
	if err == nil && jobFound != nil {
		p.log.Info("jenkins job detected",
			"job", jobFound.Name,
			"url", jobFound.URL,
			"full_name", jobFound.FullName)
	} else if errors.Is(err, context.DeadlineExceeded) || jobFound == nil {
		p.log.Warn("jenkins job not found within timeout",
			"pattern", pattern,
			"timeout", rule.Timeout)
	} else if err != nil {
		p.log.Error("error waiting for jenkins job",
			"pattern", pattern,
			"err", err)
	}

	var commentTemplate string
	if jobFound != nil {
		commentTemplate = rule.SuccessCommentTemplate
		data["JobName"] = jobFound.Name
		data["JobURL"] = jobFound.URL
		p.log.Debug("using success comment template",
			"template", commentTemplate,
			"job_name", jobFound.Name,
			"job_url", jobFound.URL)
	} else {
		commentTemplate = rule.FailureCommentTemplate
		p.log.Debug("using failure comment template",
			"template", commentTemplate)
	}

	body, err := executeTemplate("comment", commentTemplate, data)
	if err != nil {
		p.log.Error("failed to execute comment template",
			"err", err,
			"template", commentTemplate)
		return
	}

	p.log.Debug("comment template executed",
		"comment_body", body,
		"body_length", len(body))

	if err := p.gc.PostComment(ctx, evt.Repository.FullName, evt.PullRequest.Number, body); err != nil {
		p.log.Error("failed to post comment to gitea",
			"err", err,
			"repo", evt.Repository.FullName,
			"pr_number", evt.PullRequest.Number)
	} else {
		p.log.Info("comment posted to Gitea",
			"repo", evt.Repository.FullName,
			"pr", evt.PullRequest.Number,
			"comment_length", len(body))
	}
}

// runChecks выполняет проверки, привязанные к целевой ветке pull request,
// и публикует результат статусом коммита head.
func (p *Processor) runChecks(ctx context.Context, evt webhook.PullRequestEvent) {
	if len(p.cfg.Checks) == 0 {
		return
	}
	if !isCheckAction(evt.Action) {
		p.log.Debug("skipping checks for action", "action", evt.Action)
		return
	}

	base := evt.PullRequest.Base.Ref
	applicable := checks.Applicable(p.cfg.Checks, base)
	if len(applicable) == 0 {
		p.log.Info("no checks match target branch",
			"repo", evt.Repository.FullName,
			"pr", evt.PullRequest.Number,
			"base", base)
		return
	}

	sha := evt.PullRequest.Head.SHA
	if sha == "" {
		p.log.Error("pull request head sha is empty, cannot set commit status",
			"repo", evt.Repository.FullName,
			"pr", evt.PullRequest.Number)
		return
	}

	files, err := p.gc.ListPullRequestFiles(ctx, evt.Repository.FullName, pullRequestNumber(evt))
	if err != nil {
		p.log.Error("failed to list pull request files",
			"err", err,
			"repo", evt.Repository.FullName,
			"pr", evt.PullRequest.Number)
		for _, rule := range applicable {
			p.publishCheckStatus(ctx, evt, sha, rule, checks.Outcome{
				State:       checks.StateError,
				Description: "Не удалось получить список файлов pull request",
			})
		}
		return
	}

	changes := make([]checks.FileChange, len(files))
	for i, file := range files {
		changes[i] = checks.FileChange{
			Filename:         file.Filename,
			PreviousFilename: file.PreviousFilename,
		}
	}

	for _, rule := range applicable {
		outcome, evalErr := checks.Evaluate(rule, changes)
		if evalErr != nil {
			p.log.Error("check evaluation failed",
				"err", evalErr,
				"check", rule.Name,
				"type", rule.Type)
			outcome = checks.Outcome{
				State:       checks.StateError,
				Description: "Не удалось выполнить проверку",
			}
		}
		p.publishCheckStatus(ctx, evt, sha, rule, outcome)
	}
}

func (p *Processor) publishCheckStatus(ctx context.Context, evt webhook.PullRequestEvent, sha string, rule config.CheckRule, outcome checks.Outcome) {
	status := gitea.CommitStatus{
		State:       outcome.State,
		Context:     rule.Context,
		Description: outcome.Description,
	}
	if err := p.gc.CreateCommitStatus(ctx, evt.Repository.FullName, sha, status); err != nil {
		p.log.Error("failed to post commit status",
			"err", err,
			"repo", evt.Repository.FullName,
			"pr", evt.PullRequest.Number,
			"sha", sha,
			"check", rule.Name,
			"context", rule.Context,
			"state", outcome.State)
		return
	}
	p.log.Info("commit status posted",
		"repo", evt.Repository.FullName,
		"pr", evt.PullRequest.Number,
		"sha", sha,
		"check", rule.Name,
		"context", rule.Context,
		"state", outcome.State)
}

func pullRequestNumber(evt webhook.PullRequestEvent) int64 {
	if evt.PullRequest.Number != 0 {
		return evt.PullRequest.Number
	}
	return evt.Number
}

// isCheckAction сообщает, может ли действие вебхука изменить набор файлов pull request.
func isCheckAction(action string) bool {
	switch action {
	case "opened", "reopened", "synchronized":
		return true
	default:
		return false
	}
}

// executeTemplate выполняет шаблон с указанными данными и возвращает результат.
// name используется для идентификации шаблона в сообщениях об ошибках.
func executeTemplate(name, tpl string, data any) (string, error) {
	t, err := template.New(name).Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
