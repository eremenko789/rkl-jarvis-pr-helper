package processor_test

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/example/gitea-jenkins-webhook/internal/config"
	"github.com/example/gitea-jenkins-webhook/internal/gitea"
	"github.com/example/gitea-jenkins-webhook/internal/jenkins"
	"github.com/example/gitea-jenkins-webhook/internal/processor"
	"github.com/example/gitea-jenkins-webhook/pkg/webhook"
)

type stubJenkins struct {
	job *jenkins.Job
	err error
}

func (s stubJenkins) WaitForJob(ctx context.Context, _ *regexp.Regexp, _ string, timeout, interval time.Duration) (*jenkins.Job, error) {
	return s.job, s.err
}

type stubGitea struct {
	t        *testing.T
	mu       sync.Mutex
	comments []string
	wg       sync.WaitGroup
}

func newStubGitea(t *testing.T) *stubGitea {
	return &stubGitea{t: t}
}

func (s *stubGitea) PostComment(ctx context.Context, repoFullName string, issueIndex int64, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.comments = append(s.comments, body)
	s.wg.Done()
	return nil
}

func (s *stubGitea) ListPullRequestFiles(context.Context, string, int64) ([]gitea.PullRequestFile, error) {
	return nil, nil
}

func (s *stubGitea) CreateCommitStatus(context.Context, string, string, gitea.CommitStatus) error {
	return nil
}

func TestProcessor_PostsSuccessComment(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			WorkerPoolSize: 1,
			QueueSize:      10,
		},
		Jenkins: config.JenkinsConfig{
			BaseURL:      "https://jenkins.example.com",
			PollInterval: time.Millisecond,
			Timeout:      time.Second,
		},
		Gitea: config.GiteaConfig{
			BaseURL: "https://gitea.example.com",
			Token:   "token",
		},
		Repositories: []config.RepositoryRule{
			{
				Name:       "org/repo",
				JobPattern: `^job-{{ .Number }}$`,
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	job := &jenkins.Job{Name: "job-42", URL: "https://jenkins/job-42"}
	jClient := stubJenkins{job: job}
	gClient := newStubGitea(t)
	gClient.wg.Add(1)

	proc := processor.New(cfg, jClient, gClient, nil)
	proc.Start()
	defer proc.Stop()

	event := webhook.PullRequestEvent{
		Action: "opened",
		PullRequest: webhook.PullRequest{
			Number: 42,
			Title:  "test",
		},
		Repository: webhook.Repository{
			FullName: "org/repo",
		},
	}

	if err := proc.Enqueue(event); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	waitWithTimeout(t, &gClient.wg, 2*time.Second)

	gClient.mu.Lock()
	defer gClient.mu.Unlock()
	if len(gClient.comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(gClient.comments))
	}
	if got := gClient.comments[0]; got != "✅ Jenkins job job-42 detected: https://jenkins/job-42" {
		t.Fatalf("unexpected comment: %s", got)
	}
}

func TestProcessor_PostsFailureCommentWhenNoJobFound(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			WorkerPoolSize: 1,
			QueueSize:      10,
		},
		Jenkins: config.JenkinsConfig{
			BaseURL:      "https://jenkins.example.com",
			PollInterval: time.Millisecond,
			Timeout:      time.Second,
		},
		Gitea: config.GiteaConfig{
			BaseURL: "https://gitea.example.com",
			Token:   "token",
		},
		Repositories: []config.RepositoryRule{
			{
				Name:                   "org/repo",
				JobPattern:             `^job-{{ .Number }}$`,
				FailureCommentTemplate: "failure for {{ .Number }}",
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	jClient := stubJenkins{job: nil, err: context.DeadlineExceeded}
	gClient := newStubGitea(t)
	gClient.wg.Add(1)

	proc := processor.New(cfg, jClient, gClient, nil)
	proc.Start()
	defer proc.Stop()

	event := webhook.PullRequestEvent{
		Action: "opened",
		PullRequest: webhook.PullRequest{
			Number: 7,
			Title:  "test",
		},
		Repository: webhook.Repository{
			FullName: "org/repo",
		},
	}

	if err := proc.Enqueue(event); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	waitWithTimeout(t, &gClient.wg, 2*time.Second)

	gClient.mu.Lock()
	defer gClient.mu.Unlock()
	if len(gClient.comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(gClient.comments))
	}
	if got := gClient.comments[0]; got != "failure for 7" {
		t.Fatalf("unexpected comment: %s", got)
	}
}

func waitWithTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for waitgroup")
	}
}

func TestProcessor_EnqueueNotStarted(t *testing.T) {
	cfg := &config.Config{
		Server:       config.ServerConfig{WorkerPoolSize: 1, QueueSize: 10},
		Jenkins:      config.JenkinsConfig{BaseURL: "https://j"},
		Gitea:        config.GiteaConfig{BaseURL: "https://g", Token: "t"},
		Repositories: []config.RepositoryRule{{Name: "org/repo", JobPattern: "^x$"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, stubJenkins{}, newStubGitea(t), nil)
	// Do not Start
	err := proc.Enqueue(webhook.PullRequestEvent{
		Action:      "opened",
		Repository:  webhook.Repository{FullName: "org/repo"},
		PullRequest: webhook.PullRequest{Number: 1},
	})
	if err == nil {
		t.Fatal("expected error when enqueue without start")
	}
}

func TestProcessor_EnqueueQueueFull(t *testing.T) {
	// QueueSize 2, 1 worker: 1-й забирает воркер и блокирует в WaitForJob; 2-й и 3-й помещаются в буфер; 4-й — очередь полная.
	// Синхронизация через entered: ждём входа воркера в WaitForJob (значит событие #1 уже вынуто из канала), затем заполняем буфер 2 и 3.
	cfg := &config.Config{
		Server:       config.ServerConfig{WorkerPoolSize: 1, QueueSize: 2},
		Jenkins:      config.JenkinsConfig{BaseURL: "https://j", Timeout: time.Hour},
		Gitea:        config.GiteaConfig{BaseURL: "https://g", Token: "t"},
		Repositories: []config.RepositoryRule{{Name: "org/repo", JobPattern: "^x$"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	blockChan := make(chan struct{})
	entered := make(chan struct{})
	blockingStub := &blockingStubJenkins{unblock: blockChan, entered: entered}
	gClient := &noopGitea{}
	proc := processor.New(cfg, blockingStub, gClient, nil)
	proc.Start()
	defer proc.Stop()

	evt := webhook.PullRequestEvent{
		Action:      "opened",
		Repository:  webhook.Repository{FullName: "org/repo"},
		PullRequest: webhook.PullRequest{Number: 1},
	}
	if err := proc.Enqueue(evt); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	// Ждём, пока воркер заберёт событие #1 и войдёт в WaitForJob (буфер очереди пуст).
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(blockChan)
		t.Fatal("worker did not enter WaitForJob in time")
	}
	if err := proc.Enqueue(evt); err != nil {
		t.Fatalf("second enqueue: %v", err)
	}
	if err := proc.Enqueue(evt); err != nil {
		t.Fatalf("third enqueue: %v", err)
	}
	err := proc.Enqueue(evt)
	if err == nil {
		close(blockChan)
		t.Fatal("expected error when queue full")
	}
	close(blockChan)
	time.Sleep(500 * time.Millisecond)
}

type noopGitea struct{}

func (noopGitea) PostComment(context.Context, string, int64, string) error {
	return nil
}

func (noopGitea) ListPullRequestFiles(context.Context, string, int64) ([]gitea.PullRequestFile, error) {
	return nil, nil
}

func (noopGitea) CreateCommitStatus(context.Context, string, string, gitea.CommitStatus) error {
	return nil
}

// errorGitea возвращает ошибку из PostComment (для покрытия ветки обработки ошибки).
type errorGitea struct{ err error }

func (e errorGitea) PostComment(context.Context, string, int64, string) error {
	return e.err
}

func (e errorGitea) ListPullRequestFiles(context.Context, string, int64) ([]gitea.PullRequestFile, error) {
	return nil, nil
}

func (e errorGitea) CreateCommitStatus(context.Context, string, string, gitea.CommitStatus) error {
	return nil
}

type blockingStubJenkins struct {
	unblock     chan struct{}
	entered     chan struct{} // закрывается один раз при первом входе в WaitForJob
	enteredOnce sync.Once
}

func (b *blockingStubJenkins) WaitForJob(ctx context.Context, _ *regexp.Regexp, _ string, _, _ time.Duration) (*jenkins.Job, error) {
	b.enteredOnce.Do(func() { close(b.entered) }) // одноразовый сигнал тесту: воркер забрал событие и вошёл сюда
	select {
	case <-b.unblock:
	case <-ctx.Done():
	}
	return nil, context.DeadlineExceeded
}

func TestProcessor_StartIdempotent(t *testing.T) {
	cfg := &config.Config{
		Server:       config.ServerConfig{WorkerPoolSize: 1, QueueSize: 10},
		Jenkins:      config.JenkinsConfig{BaseURL: "https://j"},
		Gitea:        config.GiteaConfig{BaseURL: "https://g", Token: "t"},
		Repositories: []config.RepositoryRule{{Name: "org/repo", JobPattern: "^x$"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, stubJenkins{}, newStubGitea(t), nil)
	proc.Start()
	proc.Start() // second start should be no-op
	proc.Stop()
}

func TestProcessor_StopWithoutStart(t *testing.T) {
	cfg := &config.Config{
		Server:       config.ServerConfig{WorkerPoolSize: 1, QueueSize: 10},
		Jenkins:      config.JenkinsConfig{BaseURL: "https://j"},
		Gitea:        config.GiteaConfig{BaseURL: "https://g", Token: "t"},
		Repositories: []config.RepositoryRule{{Name: "org/repo", JobPattern: "^x$"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, stubJenkins{}, newStubGitea(t), nil)
	proc.Stop() // should not block
}

func TestProcessor_ProcessEvent_RepoNotConfigured(t *testing.T) {
	cfg := &config.Config{
		Server:       config.ServerConfig{WorkerPoolSize: 1, QueueSize: 10},
		Jenkins:      config.JenkinsConfig{BaseURL: "https://j"},
		Gitea:        config.GiteaConfig{BaseURL: "https://g", Token: "t"},
		Repositories: []config.RepositoryRule{{Name: "org/repo", JobPattern: "^x$"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	gClient := newStubGitea(t)
	proc := processor.New(cfg, stubJenkins{}, gClient, nil)
	proc.Start()
	defer proc.Stop()

	evt := webhook.PullRequestEvent{
		Action:      "opened",
		Repository:  webhook.Repository{FullName: "other/repo"},
		PullRequest: webhook.PullRequest{Number: 1},
	}
	if err := proc.Enqueue(evt); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	gClient.mu.Lock()
	n := len(gClient.comments)
	gClient.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected no comments for unconfigured repo, got %d", n)
	}
}

func TestProcessor_ProcessEvent_IgnoredAction(t *testing.T) {
	cfg := &config.Config{
		Server:       config.ServerConfig{WorkerPoolSize: 1, QueueSize: 10},
		Jenkins:      config.JenkinsConfig{BaseURL: "https://j"},
		Gitea:        config.GiteaConfig{BaseURL: "https://g", Token: "t"},
		Repositories: []config.RepositoryRule{{Name: "org/repo", JobPattern: "^x$"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	gClient := newStubGitea(t)
	proc := processor.New(cfg, stubJenkins{}, gClient, nil)
	proc.Start()
	defer proc.Stop()

	for _, action := range []string{"synchronized", "closed"} {
		gClient.mu.Lock()
		gClient.comments = nil
		gClient.mu.Unlock()
		evt := webhook.PullRequestEvent{
			Action:      action,
			Repository:  webhook.Repository{FullName: "org/repo"},
			PullRequest: webhook.PullRequest{Number: 1},
		}
		if err := proc.Enqueue(evt); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
		gClient.mu.Lock()
		n := len(gClient.comments)
		gClient.mu.Unlock()
		if n != 0 {
			t.Fatalf("action %q: expected no comments, got %d", action, n)
		}
	}
}

func TestProcessor_ProcessEvent_EmptyRepoName(t *testing.T) {
	cfg := &config.Config{
		Server:       config.ServerConfig{WorkerPoolSize: 1, QueueSize: 10},
		Jenkins:      config.JenkinsConfig{BaseURL: "https://j"},
		Gitea:        config.GiteaConfig{BaseURL: "https://g", Token: "t"},
		Repositories: []config.RepositoryRule{{Name: "org/repo", JobPattern: "^x$"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	gClient := newStubGitea(t)
	proc := processor.New(cfg, stubJenkins{}, gClient, nil)
	proc.Start()
	defer proc.Stop()

	evt := webhook.PullRequestEvent{
		Action:      "opened",
		Repository:  webhook.Repository{FullName: ""},
		PullRequest: webhook.PullRequest{Number: 1},
	}
	if err := proc.Enqueue(evt); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	gClient.mu.Lock()
	n := len(gClient.comments)
	gClient.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected no comments for empty repo name, got %d", n)
	}
}

func TestProcessor_WaitForJobReturnsError(t *testing.T) {
	// Покрывает ветку "else if err != nil" — jobFound != nil и err != nil (редкий кейс)
	cfg := &config.Config{
		Server: config.ServerConfig{WorkerPoolSize: 1, QueueSize: 10},
		Jenkins: config.JenkinsConfig{
			BaseURL:      "https://j",
			PollInterval: time.Millisecond,
			Timeout:      time.Second,
		},
		Gitea:        config.GiteaConfig{BaseURL: "https://g", Token: "t"},
		Repositories: []config.RepositoryRule{{Name: "org/repo", JobPattern: `^x$`}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	// Возвращаем и джобу, и ошибку — процессор залогирует "error waiting for jenkins job", затем постит success-комментарий
	jClient := stubJenkins{job: &jenkins.Job{Name: "x", URL: "https://j/x"}, err: errors.New("jenkins unreachable")}
	gClient := newStubGitea(t)
	gClient.wg.Add(1)
	proc := processor.New(cfg, jClient, gClient, nil)
	proc.Start()
	defer proc.Stop()

	evt := webhook.PullRequestEvent{
		Action:      "opened",
		Repository:  webhook.Repository{FullName: "org/repo"},
		PullRequest: webhook.PullRequest{Number: 1},
	}
	if err := proc.Enqueue(evt); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitWithTimeout(t, &gClient.wg, 2*time.Second)
	gClient.mu.Lock()
	n := len(gClient.comments)
	gClient.mu.Unlock()
	if n != 1 {
		t.Fatalf("expected 1 comment when WaitForJob returns job+error, got %d", n)
	}
}

func TestProcessor_PostCommentFails(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{WorkerPoolSize: 1, QueueSize: 10},
		Jenkins: config.JenkinsConfig{
			BaseURL:      "https://jenkins.example.com",
			PollInterval: time.Millisecond,
			Timeout:      time.Second,
		},
		Gitea: config.GiteaConfig{BaseURL: "https://gitea.example.com", Token: "t"},
		Repositories: []config.RepositoryRule{
			{Name: "org/repo", JobPattern: `^job-{{ .Number }}$`},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	jClient := stubJenkins{job: &jenkins.Job{Name: "job-7", URL: "https://j/job-7"}}
	gClient := errorGitea{err: errors.New("gitea api down")}
	proc := processor.New(cfg, jClient, gClient, nil)
	proc.Start()
	defer proc.Stop()

	evt := webhook.PullRequestEvent{
		Action:      "opened",
		Repository:  webhook.Repository{FullName: "org/repo"},
		PullRequest: webhook.PullRequest{Number: 7},
	}
	if err := proc.Enqueue(evt); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// Дам воркеру время обработать; процессор логирует ошибку и не паникует
	time.Sleep(500 * time.Millisecond)
}

func TestProcessor_InvalidCommentTemplate(t *testing.T) {
	// Шаблон с синтаксической ошибкой — executeTemplate возвращает ошибку, PostComment не вызывается
	cfg := &config.Config{
		Server: config.ServerConfig{WorkerPoolSize: 1, QueueSize: 10},
		Jenkins: config.JenkinsConfig{
			BaseURL:      "https://j",
			PollInterval: time.Millisecond,
			Timeout:      time.Second,
		},
		Gitea: config.GiteaConfig{BaseURL: "https://g", Token: "t"},
		Repositories: []config.RepositoryRule{
			{Name: "org/repo", JobPattern: `^x$`, SuccessCommentTemplate: "{{ .Number "}, // unclosed
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	gClient := newStubGitea(t)
	jClient := stubJenkins{job: &jenkins.Job{Name: "x", URL: "https://j/x"}}
	proc := processor.New(cfg, jClient, gClient, nil)
	proc.Start()
	defer proc.Stop()

	evt := webhook.PullRequestEvent{
		Action:      "opened",
		Repository:  webhook.Repository{FullName: "org/repo"},
		PullRequest: webhook.PullRequest{Number: 1},
	}
	if err := proc.Enqueue(evt); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	gClient.mu.Lock()
	n := len(gClient.comments)
	gClient.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected no comment when template invalid, got %d", n)
	}
}

func TestProcessor_ProcessEvent_InvalidJobPattern(t *testing.T) {
	// Template output that is invalid regex: e.g. "[invalid" or just "["
	cfg := &config.Config{
		Server:  config.ServerConfig{WorkerPoolSize: 1, QueueSize: 10},
		Jenkins: config.JenkinsConfig{BaseURL: "https://j"},
		Gitea:   config.GiteaConfig{BaseURL: "https://g", Token: "t"},
		Repositories: []config.RepositoryRule{
			{Name: "org/repo", JobPattern: "["}, // "[" is invalid regex
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	gClient := newStubGitea(t)
	proc := processor.New(cfg, stubJenkins{}, gClient, nil)
	proc.Start()
	defer proc.Stop()

	evt := webhook.PullRequestEvent{
		Action:      "opened",
		Repository:  webhook.Repository{FullName: "org/repo"},
		PullRequest: webhook.PullRequest{Number: 1},
	}
	if err := proc.Enqueue(evt); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	gClient.mu.Lock()
	n := len(gClient.comments)
	gClient.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected no comments when job pattern invalid, got %d", n)
	}
}
