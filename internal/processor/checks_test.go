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

type recordedStatus struct {
	repo   string
	sha    string
	status gitea.CommitStatus
}

type recordingGitea struct {
	mu        sync.Mutex
	files     []gitea.PullRequestFile
	listErr   error
	statusErr error
	comments  []string
	statuses  []recordedStatus
	listCalls int
	wg        sync.WaitGroup
}

func (r *recordingGitea) PostComment(_ context.Context, _ string, _ int64, body string) error {
	r.mu.Lock()
	r.comments = append(r.comments, body)
	r.mu.Unlock()
	r.wg.Done()
	return nil
}

func (r *recordingGitea) ListPullRequestFiles(_ context.Context, _ string, _ int64) ([]gitea.PullRequestFile, error) {
	r.mu.Lock()
	r.listCalls++
	files := append([]gitea.PullRequestFile(nil), r.files...)
	err := r.listErr
	r.mu.Unlock()
	return files, err
}

func (r *recordingGitea) CreateCommitStatus(_ context.Context, repoFullName, sha string, status gitea.CommitStatus) error {
	r.mu.Lock()
	r.statuses = append(r.statuses, recordedStatus{repo: repoFullName, sha: sha, status: status})
	err := r.statusErr
	r.mu.Unlock()
	r.wg.Done()
	return err
}

type unexpectedJenkins struct {
	t *testing.T
}

func (u unexpectedJenkins) WaitForJob(context.Context, *regexp.Regexp, string, time.Duration, time.Duration) (*jenkins.Job, error) {
	u.t.Errorf("WaitForJob should not be called")
	return nil, errors.New("unexpected jenkins call")
}

func blacklistConfig(t *testing.T, repos []config.RepositoryRule, rules ...config.CheckRule) *config.Config {
	t.Helper()
	if len(repos) == 0 {
		repos = []config.RepositoryRule{{
			Name:       "org/repo",
			JobPattern: "^job-{{ .Number }}$",
		}}
	}
	attached := false
	for i := range repos {
		if repos[i].Name == "org/repo" {
			repos[i].Checks = rules
			attached = true
		}
	}
	if !attached {
		t.Fatal("org/repo is required to attach checks")
	}
	cfg := &config.Config{
		Server:       config.ServerConfig{WorkerPoolSize: 1, QueueSize: 10},
		Jenkins:      config.JenkinsConfig{BaseURL: "https://jenkins.example.com", PollInterval: time.Millisecond, Timeout: time.Second},
		Gitea:        config.GiteaConfig{BaseURL: "https://gitea.example.com", Token: "t"},
		Repositories: repos,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	return cfg
}

func fileRule(name, contextName string, branches, patterns []string) config.CheckRule {
	return config.CheckRule{
		Name:               name,
		Type:               config.CheckTypeFileBlacklist,
		Context:            contextName,
		TargetBranches:     branches,
		FileBlacklist:      &config.FileBlacklistCheck{Patterns: patterns},
		SuccessDescription: "ok",
		FailureDescription: "blocked",
	}
}

func prEvent(action, base, sha string) webhook.PullRequestEvent {
	return webhook.PullRequestEvent{
		Action: action,
		PullRequest: webhook.PullRequest{
			Number: 12,
			Title:  "change files",
			Base:   webhook.PRBranch{Ref: base, SHA: "base-sha"},
			Head:   webhook.PRBranch{Ref: "feature", SHA: sha},
		},
		Repository: webhook.Repository{FullName: "org/repo"},
	}
}

func TestProcessor_FileBlacklistSuccess(t *testing.T) {
	cfg := blacklistConfig(t, nil, fileRule("forbidden", "checks/forbidden", []string{"^main$"}, []string{"go.sum", "**/.env"}))
	gc := &recordingGitea{files: []gitea.PullRequestFile{{Filename: "main.go", Status: "modified"}}}
	gc.wg.Add(1)
	proc := processor.New(cfg, stubJenkins{}, gc, nil)
	proc.Start()
	defer proc.Stop()

	if err := proc.Enqueue(prEvent("synchronized", "main", "headsha")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitWithTimeout(t, &gc.wg, 2*time.Second)

	gc.mu.Lock()
	defer gc.mu.Unlock()
	if len(gc.statuses) != 1 {
		t.Fatalf("statuses = %d", len(gc.statuses))
	}
	got := gc.statuses[0]
	if got.sha != "headsha" || got.status.State != "success" || got.status.Context != "checks/forbidden" || got.status.Description != "ok" {
		t.Fatalf("status = %+v", got)
	}
}

func TestProcessor_FileBlacklistFailure(t *testing.T) {
	cfg := blacklistConfig(t, nil, fileRule("forbidden", "checks/forbidden", []string{"^main$", "^release/.*"}, []string{"secrets/**", "go.sum"}))
	gc := &recordingGitea{files: []gitea.PullRequestFile{
		{Filename: "README.md", Status: "modified"},
		{Filename: "secrets/token.txt", Status: "added"},
	}}
	gc.wg.Add(1)
	proc := processor.New(cfg, unexpectedJenkins{t: t}, gc, nil)
	proc.Start()
	defer proc.Stop()

	if err := proc.Enqueue(prEvent("reopened", "release/2", "abc")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitWithTimeout(t, &gc.wg, 2*time.Second)

	gc.mu.Lock()
	defer gc.mu.Unlock()
	if len(gc.statuses) != 1 {
		t.Fatalf("statuses = %d", len(gc.statuses))
	}
	got := gc.statuses[0].status
	if got.State != "failure" {
		t.Fatalf("state = %s", got.State)
	}
	if got.Description != "blocked: secrets/token.txt" {
		t.Fatalf("description = %q", got.Description)
	}
	if len(gc.comments) != 0 {
		t.Fatalf("jenkins comment should not be posted on reopen, got %d", len(gc.comments))
	}
}

func TestProcessor_FileBlacklistPreviousFilename(t *testing.T) {
	cfg := blacklistConfig(t, nil, fileRule("forbidden", "checks/forbidden", []string{"^main$"}, []string{"legacy/secret.txt"}))
	gc := &recordingGitea{files: []gitea.PullRequestFile{{
		Filename:         "docs/secret.txt",
		PreviousFilename: "legacy/secret.txt",
		Status:           "renamed",
	}}}
	gc.wg.Add(1)
	proc := processor.New(cfg, stubJenkins{}, gc, nil)
	proc.Start()
	defer proc.Stop()

	if err := proc.Enqueue(prEvent("synchronized", "main", "abc")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitWithTimeout(t, &gc.wg, 2*time.Second)

	gc.mu.Lock()
	defer gc.mu.Unlock()
	if len(gc.statuses) != 1 || gc.statuses[0].status.State != "failure" {
		t.Fatalf("statuses = %+v", gc.statuses)
	}
}

func TestProcessor_FileBlacklistSkipsUnmatchedBranch(t *testing.T) {
	cfg := blacklistConfig(t,
		[]config.RepositoryRule{{Name: "org/repo", JobPattern: "^job-{{ .Number }}$", SuccessCommentTemplate: "comment"}},
		fileRule("forbidden", "checks/forbidden", []string{"^main$"}, []string{"go.sum"}),
	)
	gc := &recordingGitea{}
	gc.wg.Add(1)
	proc := processor.New(cfg, stubJenkins{job: &jenkins.Job{Name: "job-12", URL: "https://j/job-12"}}, gc, nil)
	proc.Start()
	defer proc.Stop()

	if err := proc.Enqueue(prEvent("opened", "develop", "abc")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitWithTimeout(t, &gc.wg, 2*time.Second)

	gc.mu.Lock()
	defer gc.mu.Unlock()
	if gc.listCalls != 0 || len(gc.statuses) != 0 {
		t.Fatalf("check should be skipped, listCalls=%d statuses=%d", gc.listCalls, len(gc.statuses))
	}
	if len(gc.comments) != 1 {
		t.Fatalf("expected jenkins comment, got %d", len(gc.comments))
	}
}

func TestProcessor_FileBlacklistSynchronizedWithoutJenkins(t *testing.T) {
	cfg := blacklistConfig(t,
		[]config.RepositoryRule{{Name: "org/repo", JobPattern: "^job$"}},
		fileRule("forbidden", "checks/forbidden", []string{".*"}, []string{"go.sum"}),
	)
	gc := &recordingGitea{files: []gitea.PullRequestFile{{Filename: "main.go"}}}
	gc.wg.Add(1)
	proc := processor.New(cfg, unexpectedJenkins{t: t}, gc, nil)
	proc.Start()
	defer proc.Stop()

	if err := proc.Enqueue(prEvent("synchronized", "topic", "abc")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitWithTimeout(t, &gc.wg, 2*time.Second)

	gc.mu.Lock()
	defer gc.mu.Unlock()
	if len(gc.statuses) != 1 || gc.statuses[0].status.State != "success" {
		t.Fatalf("statuses = %+v", gc.statuses)
	}
	if len(gc.comments) != 0 {
		t.Fatalf("jenkins comment should not be posted, got %d", len(gc.comments))
	}
}

func TestProcessor_FileBlacklistListError(t *testing.T) {
	cfg := blacklistConfig(t, nil, fileRule("forbidden", "checks/forbidden", []string{"^main$"}, []string{"go.sum"}))
	gc := &recordingGitea{listErr: errors.New("gitea down")}
	gc.wg.Add(2)
	proc := processor.New(cfg, stubJenkins{}, gc, nil)
	proc.Start()
	defer proc.Stop()

	if err := proc.Enqueue(prEvent("opened", "main", "abc")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitWithTimeout(t, &gc.wg, 2*time.Second)

	gc.mu.Lock()
	defer gc.mu.Unlock()
	if len(gc.statuses) != 1 || gc.statuses[0].status.State != "error" {
		t.Fatalf("statuses = %+v", gc.statuses)
	}
}

func TestProcessor_FileBlacklistMissingSHA(t *testing.T) {
	cfg := blacklistConfig(t,
		[]config.RepositoryRule{{Name: "org/repo", JobPattern: "^x$", SuccessCommentTemplate: "comment"}},
		fileRule("forbidden", "checks/forbidden", []string{"^main$"}, []string{"go.sum"}),
	)
	gc := &recordingGitea{}
	gc.wg.Add(1)
	proc := processor.New(cfg, stubJenkins{job: &jenkins.Job{Name: "x", URL: "https://j/x"}}, gc, nil)
	proc.Start()
	defer proc.Stop()

	if err := proc.Enqueue(prEvent("opened", "main", "")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitWithTimeout(t, &gc.wg, 2*time.Second)

	gc.mu.Lock()
	defer gc.mu.Unlock()
	if gc.listCalls != 0 || len(gc.statuses) != 0 {
		t.Fatalf("status must not be posted without sha, listCalls=%d statuses=%d", gc.listCalls, len(gc.statuses))
	}
}

func TestProcessor_FileBlacklistIgnoresClosed(t *testing.T) {
	cfg := blacklistConfig(t, nil, fileRule("forbidden", "checks/forbidden", []string{".*"}, []string{"go.sum"}))
	gc := &recordingGitea{files: []gitea.PullRequestFile{{Filename: "go.sum"}}}
	proc := processor.New(cfg, stubJenkins{}, gc, nil)
	proc.Start()
	defer proc.Stop()

	if err := proc.Enqueue(prEvent("closed", "main", "abc")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	gc.mu.Lock()
	defer gc.mu.Unlock()
	if gc.listCalls != 0 || len(gc.statuses) != 0 {
		t.Fatalf("closed action should skip checks, listCalls=%d statuses=%d", gc.listCalls, len(gc.statuses))
	}
}

func TestProcessor_FileBlacklistWithJenkinsComment(t *testing.T) {
	cfg := blacklistConfig(t,
		[]config.RepositoryRule{{Name: "org/repo", JobPattern: "^job-{{ .Number }}$", SuccessCommentTemplate: "job ok"}},
		fileRule("forbidden", "checks/forbidden", []string{"^main$"}, []string{"go.sum"}),
		fileRule("release-only", "checks/release", []string{"^release/.*"}, []string{"VERSION"}),
	)
	gc := &recordingGitea{files: []gitea.PullRequestFile{{Filename: "go.sum", Status: "modified"}}}
	gc.wg.Add(2)
	proc := processor.New(cfg, stubJenkins{job: &jenkins.Job{Name: "job-12", URL: "https://j/job-12"}}, gc, nil)
	proc.Start()
	defer proc.Stop()

	if err := proc.Enqueue(prEvent("opened", "main", "abc")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitWithTimeout(t, &gc.wg, 2*time.Second)

	gc.mu.Lock()
	defer gc.mu.Unlock()
	if len(gc.comments) != 1 || gc.comments[0] != "job ok" {
		t.Fatalf("comments = %+v", gc.comments)
	}
	if len(gc.statuses) != 1 {
		t.Fatalf("expected only the main check, got %+v", gc.statuses)
	}
	if gc.statuses[0].status.Context != "checks/forbidden" || gc.statuses[0].status.State != "failure" {
		t.Fatalf("status = %+v", gc.statuses[0].status)
	}
}

func TestProcessor_FileBlacklistTwoMatchingRules(t *testing.T) {
	cfg := blacklistConfig(t, nil,
		fileRule("sums", "checks/sums", []string{"^main$"}, []string{"go.sum"}),
		fileRule("envs", "checks/envs", []string{"^main$"}, []string{"**/.env"}),
	)
	gc := &recordingGitea{files: []gitea.PullRequestFile{{Filename: ".env", Status: "added"}}}
	gc.wg.Add(2)
	proc := processor.New(cfg, stubJenkins{}, gc, nil)
	proc.Start()
	defer proc.Stop()

	if err := proc.Enqueue(prEvent("synchronized", "main", "abc")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitWithTimeout(t, &gc.wg, 2*time.Second)

	gc.mu.Lock()
	defer gc.mu.Unlock()
	if len(gc.statuses) != 2 {
		t.Fatalf("statuses = %+v", gc.statuses)
	}
	byContext := map[string]string{}
	for _, status := range gc.statuses {
		byContext[status.status.Context] = status.status.State
	}
	if byContext["checks/sums"] != "success" || byContext["checks/envs"] != "failure" {
		t.Fatalf("states = %+v", byContext)
	}
}

func TestProcessor_FileBlacklistStatusPostError(t *testing.T) {
	cfg := blacklistConfig(t, nil, fileRule("forbidden", "checks/forbidden", []string{"^main$"}, []string{"go.sum"}))
	gc := &recordingGitea{
		files:     []gitea.PullRequestFile{{Filename: "main.go"}},
		statusErr: errors.New("status api down"),
	}
	gc.wg.Add(1)
	proc := processor.New(cfg, stubJenkins{}, gc, nil)
	proc.Start()
	defer proc.Stop()

	if err := proc.Enqueue(prEvent("synchronized", "main", "abc")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitWithTimeout(t, &gc.wg, 2*time.Second)

	gc.mu.Lock()
	defer gc.mu.Unlock()
	if len(gc.statuses) != 1 {
		t.Fatalf("status attempt = %d", len(gc.statuses))
	}
}

func TestProcessor_FileBlacklistSkipsOtherRepository(t *testing.T) {
	cfg := blacklistConfig(t,
		[]config.RepositoryRule{
			{Name: "org/repo", JobPattern: "^job$"},
			{Name: "org/other", JobPattern: "^job$"},
		},
		fileRule("forbidden", "checks/forbidden", []string{".*"}, []string{"go.sum"}),
	)
	gc := &recordingGitea{files: []gitea.PullRequestFile{{Filename: "go.sum"}}}
	proc := processor.New(cfg, unexpectedJenkins{t: t}, gc, nil)
	proc.Start()
	defer proc.Stop()

	evt := prEvent("synchronized", "main", "abc")
	evt.Repository.FullName = "org/other"
	if err := proc.Enqueue(evt); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	gc.mu.Lock()
	defer gc.mu.Unlock()
	if gc.listCalls != 0 || len(gc.statuses) != 0 {
		t.Fatalf("checks of org/repo must not run for org/other, listCalls=%d statuses=%d", gc.listCalls, len(gc.statuses))
	}
}
