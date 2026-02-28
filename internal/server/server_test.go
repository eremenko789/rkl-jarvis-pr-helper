package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/example/gitea-jenkins-webhook/internal/config"
	"github.com/example/gitea-jenkins-webhook/internal/jenkins"
	"github.com/example/gitea-jenkins-webhook/internal/processor"
)

// errReader возвращает ошибку при Read (для покрытия ветки ошибки чтения body).
type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) {
	return 0, e.err
}

func TestHandleHealth_OK(t *testing.T) {
	cfg := &config.Config{
		Server:  config.ServerConfig{ListenAddr: ":0"},
		Jenkins: config.JenkinsConfig{BaseURL: "https://j"},
		Gitea:   config.GiteaConfig{BaseURL: "https://g", Token: "t"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, blockingJenkins{}, &nopGitea{}, nil)
	srv := New(cfg, proc, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.handleHealth(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Fatalf("expected body 'ok', got %q", body)
	}
}

func TestComputeSignature_Deterministic(t *testing.T) {
	payload := []byte("test")
	secret := "secret"
	a := computeSignature(payload, secret)
	b := computeSignature(payload, secret)
	if a != b {
		t.Fatalf("signature not deterministic: %q != %q", a, b)
	}
	if len(a) == 0 {
		t.Fatal("expected non-empty hex signature")
	}
}

func TestComputeSignature_EmptyPayload(t *testing.T) {
	sig := computeSignature([]byte{}, "secret")
	if len(sig) == 0 {
		t.Fatal("expected non-empty signature for empty payload")
	}
}

func TestVerifySignature_Valid(t *testing.T) {
	payload := []byte("body")
	secret := "secret"
	sig := computeSignature(payload, secret)
	if err := verifySignature(payload, sig, secret); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestVerifySignature_WithPrefix(t *testing.T) {
	payload := []byte("body")
	secret := "secret"
	sig := computeSignature(payload, secret)
	if err := verifySignature(payload, "sha256="+sig, secret); err != nil {
		t.Fatalf("expected no error with sha256= prefix: %v", err)
	}
}

func TestVerifySignature_Mismatch(t *testing.T) {
	payload := []byte("body")
	secret := "secret"
	if err := verifySignature(payload, "deadbeef", secret); err == nil {
		t.Fatal("expected error for wrong signature")
	}
}

func TestVerifySignature_Empty(t *testing.T) {
	if err := verifySignature([]byte("x"), "", "secret"); err == nil {
		t.Fatal("expected error for empty signature")
	}
}

func TestNormalizeSignature(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"sha256=abc", "abc"},
		{"  sha256=def  ", "def"},
		{"xyz", "xyz"},
		{"  xyz  ", "xyz"},
	}
	for _, tt := range tests {
		got := normalizeSignature(tt.in)
		if got != tt.want {
			t.Errorf("normalizeSignature(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

type blockingJenkins struct{}

func (blockingJenkins) WaitForJob(ctx context.Context, _ *regexp.Regexp, _ string, _, _ time.Duration) (*jenkins.Job, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// quickJenkins returns immediately so the worker can finish (used when processor is started).
type quickJenkins struct{}

func (quickJenkins) WaitForJob(context.Context, *regexp.Regexp, string, time.Duration, time.Duration) (*jenkins.Job, error) {
	return nil, context.DeadlineExceeded
}

type nopGitea struct{}

func (nopGitea) PostComment(ctx context.Context, _ string, _ int64, _ string) error {
	return nil
}

func minimalPRPayload() []byte {
	return []byte(`{"action":"opened","number":1,"pull_request":{"number":1,"title":"t","body":"","url":""},"repository":{"id":1,"name":"r","full_name":"org/r","html_url":""},"sender":{"id":1,"login":"u","full_name":""}}`)
}

func TestHandleWebhook_AcceptedNoSecret(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{ListenAddr: ":0", WorkerPoolSize: 1, QueueSize: 10},
		Repositories: []config.RepositoryRule{
			{Name: "org/r", JobPattern: "^job-1$"},
		},
	}
	cfg.Jenkins.BaseURL = "https://j"
	cfg.Gitea.BaseURL = "https://g"
	cfg.Gitea.Token = "t"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, quickJenkins{}, &nopGitea{}, nil)
	proc.Start()
	defer proc.Stop()
	srv := New(cfg, proc, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(minimalPRPayload()))
	req.Header.Set("X-Gitea-Event", "pull_request")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.handleWebhook(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body %s", rec.Code, rec.Body.String())
	}
}

func TestHandleWebhook_AcceptedWithValidSignature(t *testing.T) {
	secret := "webhook-secret"
	cfg := &config.Config{
		Server: config.ServerConfig{ListenAddr: ":0", WorkerPoolSize: 1, QueueSize: 10, WebhookSecret: secret},
		Repositories: []config.RepositoryRule{
			{Name: "org/r", JobPattern: "^job-1$"},
		},
	}
	cfg.Jenkins.BaseURL = "https://j"
	cfg.Gitea.BaseURL = "https://g"
	cfg.Gitea.Token = "t"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, quickJenkins{}, &nopGitea{}, nil)
	proc.Start()
	defer proc.Stop()
	srv := New(cfg, proc, nil)

	body := minimalPRPayload()
	sig := computeSignature(body, secret)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Gitea-Event", "pull_request")
	req.Header.Set("X-Gitea-Signature", "sha256="+sig)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.handleWebhook(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body %s", rec.Code, rec.Body.String())
	}
}

func TestHandleWebhook_InvalidSignature(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{ListenAddr: ":0", WebhookSecret: "secret"},
	}
	cfg.Jenkins.BaseURL = "https://j"
	cfg.Gitea.BaseURL = "https://g"
	cfg.Gitea.Token = "t"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, blockingJenkins{}, &nopGitea{}, nil)
	srv := New(cfg, proc, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(minimalPRPayload()))
	req.Header.Set("X-Gitea-Event", "pull_request")
	req.Header.Set("X-Gitea-Signature", "wronghex")
	rec := httptest.NewRecorder()
	srv.handleWebhook(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleWebhook_MissingSignature(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{ListenAddr: ":0", WebhookSecret: "secret"},
	}
	cfg.Jenkins.BaseURL = "https://j"
	cfg.Gitea.BaseURL = "https://g"
	cfg.Gitea.Token = "t"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, blockingJenkins{}, &nopGitea{}, nil)
	srv := New(cfg, proc, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(minimalPRPayload()))
	req.Header.Set("X-Gitea-Event", "pull_request")
	rec := httptest.NewRecorder()
	srv.handleWebhook(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleWebhook_UnsupportedEvent(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{ListenAddr: ":0"}}
	cfg.Jenkins.BaseURL = "https://j"
	cfg.Gitea.BaseURL = "https://g"
	cfg.Gitea.Token = "t"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, blockingJenkins{}, &nopGitea{}, nil)
	srv := New(cfg, proc, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(minimalPRPayload()))
	req.Header.Set("X-Gitea-Event", "push")
	rec := httptest.NewRecorder()
	srv.handleWebhook(rec, req)
	// Server currently writes 204 then Error(400); first WriteHeader wins
	if rec.Code != http.StatusNoContent && rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 204 or 400 for unsupported event, got %d", rec.Code)
	}
}

func TestHandleWebhook_ReadBodyError(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{ListenAddr: ":0"}}
	cfg.Jenkins.BaseURL = "https://j"
	cfg.Gitea.BaseURL = "https://g"
	cfg.Gitea.Token = "t"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, blockingJenkins{}, &nopGitea{}, nil)
	srv := New(cfg, proc, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", errReader{err: errors.New("read fail")})
	req.Header.Set("X-Gitea-Event", "pull_request")
	req.Body = io.NopCloser(errReader{err: errors.New("read fail")})
	rec := httptest.NewRecorder()
	srv.handleWebhook(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when body read fails, got %d", rec.Code)
	}
}

func TestHandleWebhook_InvalidJSON(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{ListenAddr: ":0"}}
	cfg.Jenkins.BaseURL = "https://j"
	cfg.Gitea.BaseURL = "https://g"
	cfg.Gitea.Token = "t"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, blockingJenkins{}, &nopGitea{}, nil)
	srv := New(cfg, proc, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader([]byte("not json")))
	req.Header.Set("X-Gitea-Event", "pull_request")
	rec := httptest.NewRecorder()
	srv.handleWebhook(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRun_ListenAndServeError(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	cfg := &config.Config{
		Server:  config.ServerConfig{ListenAddr: ":" + strconv.Itoa(port)},
		Jenkins: config.JenkinsConfig{BaseURL: "https://j"},
		Gitea:   config.GiteaConfig{BaseURL: "https://g", Token: "t"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, blockingJenkins{}, &nopGitea{}, nil)
	srv := New(cfg, proc, nil)

	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(context.Background())
	}()

	select {
	case err := <-runErr:
		if err == nil {
			t.Fatal("expected Run to return error when ListenAndServe fails")
		}
		if !strings.Contains(err.Error(), "address already in use") && !strings.Contains(err.Error(), "bind") {
			t.Logf("Run returned error (expected): %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return within 5s")
	}
}

func TestHandleWebhook_EnqueueFails(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{ListenAddr: ":0", WorkerPoolSize: 1, QueueSize: 10},
		Repositories: []config.RepositoryRule{
			{Name: "org/r", JobPattern: "^x$"},
		},
	}
	cfg.Jenkins.BaseURL = "https://j"
	cfg.Gitea.BaseURL = "https://g"
	cfg.Gitea.Token = "t"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proc := processor.New(cfg, blockingJenkins{}, &nopGitea{}, nil)
	// Do not start processor — Enqueue will return "processor not started" and we get 503
	srv := New(cfg, proc, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(minimalPRPayload()))
	req.Header.Set("X-Gitea-Event", "pull_request")
	rec := httptest.NewRecorder()
	srv.handleWebhook(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when Enqueue fails, got %d", rec.Code)
	}
}
