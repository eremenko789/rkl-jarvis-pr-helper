package config_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/example/gitea-jenkins-webhook/internal/config"
)

func TestLoad(t *testing.T) {
	cfgContent := `
server:
  listen_addr: ":9000"
jenkins:
  base_url: "https://jenkins.example.com"
  username: "john"
  api_token: "token"
gitea:
  base_url: "https://gitea.example.com"
  token: "secret"
repositories:
  - name: "org/repo"
    job_pattern: "^build-{{ .Number }}$"
    poll_interval: 1s
    timeout: 5s
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Server.ListenAddr != ":9000" {
		t.Fatalf("unexpected listen addr: %s", cfg.Server.ListenAddr)
	}
	if cfg.Server.WorkerPoolSize != 4 {
		t.Fatalf("default worker pool should be 4, got %d", cfg.Server.WorkerPoolSize)
	}
	if cfg.Repositories[0].PollInterval != time.Second {
		t.Fatalf("expected poll interval of 1s, got %s", cfg.Repositories[0].PollInterval)
	}
	if _, ok := cfg.GetRepositoryRule("org/repo"); !ok {
		t.Fatalf("expected repository rule to be registered")
	}
}

func TestLoad_FileNotExist(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if err.Error() == "" {
		t.Fatal("expected non-empty error")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("server:\n  listen_addr: [")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// Valid YAML but missing required fields (jenkins.base_url, gitea.base_url, gitea.token)
	if err := os.WriteFile(path, []byte("server: {}\njenkins: {}\ngitea: {}\nrepositories: []"), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := &config.Config{
		Server:  config.ServerConfig{ListenAddr: ":9000"},
		Jenkins: config.JenkinsConfig{BaseURL: "https://j.example.com"},
		Gitea:   config.GiteaConfig{BaseURL: "https://g.example.com", Token: "t"},
		Repositories: []config.RepositoryRule{
			{Name: "org/repo", JobPattern: "^job-{{ .Number }}$"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if cfg.Server.ListenAddr != ":9000" {
		t.Fatalf("listen_addr changed: %s", cfg.Server.ListenAddr)
	}
	if cfg.Server.WorkerPoolSize != 4 {
		t.Fatalf("expected default worker pool 4, got %d", cfg.Server.WorkerPoolSize)
	}
	if cfg.Server.QueueSize != 100 {
		t.Fatalf("expected default queue size 100, got %d", cfg.Server.QueueSize)
	}
}

func TestValidate_Defaults(t *testing.T) {
	cfg := &config.Config{
		Server:  config.ServerConfig{}, // empty listen_addr, zero pool/queue
		Jenkins: config.JenkinsConfig{BaseURL: "https://j.example.com", PollInterval: 0, Timeout: 0},
		Gitea:   config.GiteaConfig{BaseURL: "https://g.example.com", Token: "t"},
		Repositories: []config.RepositoryRule{
			{Name: "org/repo", JobPattern: "^x$"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if cfg.Server.ListenAddr != ":8080" {
		t.Fatalf("expected default listen_addr :8080, got %s", cfg.Server.ListenAddr)
	}
	if cfg.Server.WorkerPoolSize != 4 || cfg.Server.QueueSize != 100 {
		t.Fatalf("expected defaults 4 and 100, got %d %d", cfg.Server.WorkerPoolSize, cfg.Server.QueueSize)
	}
	if cfg.Jenkins.PollInterval != 15*time.Second || cfg.Jenkins.Timeout != 5*time.Minute {
		t.Fatalf("expected jenkins defaults 15s and 5m")
	}
}

func TestValidate_MissingJenkinsURL(t *testing.T) {
	cfg := &config.Config{
		Jenkins: config.JenkinsConfig{BaseURL: ""},
		Gitea:   config.GiteaConfig{BaseURL: "https://g.example.com", Token: "t"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing jenkins.base_url")
	}
	if err.Error() == "" {
		t.Fatal("expected non-empty error")
	}
}

func TestValidate_MissingGiteaURL(t *testing.T) {
	cfg := &config.Config{
		Jenkins: config.JenkinsConfig{BaseURL: "https://j.example.com"},
		Gitea:   config.GiteaConfig{BaseURL: "", Token: "t"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing gitea.base_url")
	}
}

func TestValidate_MissingGiteaToken(t *testing.T) {
	cfg := &config.Config{
		Jenkins: config.JenkinsConfig{BaseURL: "https://j.example.com"},
		Gitea:   config.GiteaConfig{BaseURL: "https://g.example.com", Token: ""},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing gitea.token")
	}
}

func TestValidate_RepoMissingName(t *testing.T) {
	cfg := &config.Config{
		Jenkins: config.JenkinsConfig{BaseURL: "https://j.example.com"},
		Gitea:   config.GiteaConfig{BaseURL: "https://g.example.com", Token: "t"},
		Repositories: []config.RepositoryRule{
			{Name: "", JobPattern: "^x$"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for repo missing name")
	}
}

func TestValidate_RepoMissingJobPattern(t *testing.T) {
	cfg := &config.Config{
		Jenkins: config.JenkinsConfig{BaseURL: "https://j.example.com"},
		Gitea:   config.GiteaConfig{BaseURL: "https://g.example.com", Token: "t"},
		Repositories: []config.RepositoryRule{
			{Name: "org/repo", JobPattern: ""},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for repo missing job_pattern")
	}
}

func TestValidate_RepoInheritsIntervals(t *testing.T) {
	cfg := &config.Config{
		Jenkins: config.JenkinsConfig{
			BaseURL:      "https://j.example.com",
			PollInterval: 20 * time.Second,
			Timeout:      3 * time.Minute,
		},
		Gitea: config.GiteaConfig{BaseURL: "https://g.example.com", Token: "t"},
		Repositories: []config.RepositoryRule{
			{Name: "org/repo", JobPattern: "^x$", PollInterval: 0, Timeout: 0},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if cfg.Repositories[0].PollInterval != 20*time.Second || cfg.Repositories[0].Timeout != 3*time.Minute {
		t.Fatalf("repo should inherit jenkins intervals")
	}
}

func TestValidate_DefaultTemplates(t *testing.T) {
	cfg := &config.Config{
		Jenkins: config.JenkinsConfig{BaseURL: "https://j.example.com"},
		Gitea:   config.GiteaConfig{BaseURL: "https://g.example.com", Token: "t"},
		Repositories: []config.RepositoryRule{
			{Name: "org/repo", JobPattern: "^x$", SuccessCommentTemplate: "", FailureCommentTemplate: ""},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	r := cfg.Repositories[0]
	if r.SuccessCommentTemplate == "" || r.FailureCommentTemplate == "" {
		t.Fatalf("expected default templates to be set")
	}
}

func TestGetRepositoryRule_Found(t *testing.T) {
	cfg := &config.Config{
		Repositories: []config.RepositoryRule{
			{Name: "org/repo", JobPattern: "^build-{{ .Number }}$"},
		},
	}
	cfg.RepoIndex = map[string]config.RepoID{
		"org/repo": {Rule: config.RepositoryRule{Name: "org/repo", JobPattern: "^build-{{ .Number }}$"}},
	}
	rule, ok := cfg.GetRepositoryRule("org/repo")
	if !ok {
		t.Fatal("expected rule to be found")
	}
	if rule.Name != "org/repo" || rule.JobPattern != "^build-{{ .Number }}$" {
		t.Fatalf("unexpected rule: %+v", rule)
	}
}

func TestGetRepositoryRule_NotFound(t *testing.T) {
	cfg := &config.Config{
		RepoIndex: map[string]config.RepoID{"org/repo": {}},
	}
	_, ok := cfg.GetRepositoryRule("other/repo")
	if ok {
		t.Fatal("expected rule not to be found")
	}
}

func TestGetRepositoryRule_NilIndex(t *testing.T) {
	cfg := &config.Config{
		Repositories: []config.RepositoryRule{
			{Name: "org/repo", JobPattern: "^x$"},
		},
		RepoIndex: nil,
	}
	rule, ok := cfg.GetRepositoryRule("org/repo")
	if !ok {
		t.Fatal("expected buildIndex to run and rule to be found")
	}
	if rule.Name != "org/repo" {
		t.Fatalf("unexpected rule name: %s", rule.Name)
	}
}

func TestNewHTTPClient_InsecureSkipVerify(t *testing.T) {
	client := config.NewHTTPClient(true, 5*time.Second)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Transport == nil {
		t.Fatal("expected transport to be set")
	}
	// TLS config is inside RoundTripper; we only check client was created
	if client.Timeout != 5*time.Second {
		t.Fatalf("expected timeout 5s, got %v", client.Timeout)
	}
}

func TestNewHTTPClient_Timeout(t *testing.T) {
	client := config.NewHTTPClient(false, 30*time.Second)
	if client.Timeout != 30*time.Second {
		t.Fatalf("expected timeout 30s, got %v", client.Timeout)
	}
}

func TestNewHTTPClient_ZeroTimeout(t *testing.T) {
	client := config.NewHTTPClient(false, 0)
	if client.Timeout != 10*time.Second {
		t.Fatalf("expected default timeout 10s for zero, got %v", client.Timeout)
	}
	clientNeg := config.NewHTTPClient(false, -1)
	if clientNeg.Timeout != 10*time.Second {
		t.Fatalf("expected default timeout 10s for negative, got %v", clientNeg.Timeout)
	}
}

func TestLoad_ExampleConfig(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Fatalf("example config: %v", err)
	}
	if len(cfg.Checks) != 1 || cfg.Checks[0].Type != config.CheckTypeFileBlacklist {
		t.Fatalf("example checks = %+v", cfg.Checks)
	}
	if !cfg.Checks[0].MatchesTargetBranch("main") {
		t.Fatal("example check should match main")
	}
}

func TestLoad_Checks(t *testing.T) {
	cfgContent := `
jenkins:
  base_url: "https://jenkins.example.com"
gitea:
  base_url: "https://gitea.example.com"
  token: "secret"
checks:
  - name: forbidden-files
    type: file_blacklist
    target_branches:
      - "^main$"
      - "^release/.*"
    file_blacklist:
      patterns:
        - "go.sum"
        - "vendor/**"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Checks) != 1 {
		t.Fatalf("checks = %d", len(cfg.Checks))
	}
	ch := cfg.Checks[0]
	if ch.Type != config.CheckTypeFileBlacklist {
		t.Fatalf("type = %s", ch.Type)
	}
	if ch.Context != "checks/forbidden-files" {
		t.Fatalf("context = %s", ch.Context)
	}
	if ch.SuccessDescription == "" || ch.FailureDescription == "" {
		t.Fatal("expected default descriptions")
	}
	if !ch.MatchesTargetBranch("main") || !ch.MatchesTargetBranch("release/1.0") {
		t.Fatal("expected branch patterns to match")
	}
	if ch.MatchesTargetBranch("develop") {
		t.Fatal("develop should not match")
	}
}

func TestValidate_ChecksMissingName(t *testing.T) {
	cfg := checkConfig(config.CheckRule{
		Type:           config.CheckTypeFileBlacklist,
		TargetBranches: []string{"^main$"},
		FileBlacklist:  &config.FileBlacklistCheck{Patterns: []string{"go.sum"}},
	})
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestValidate_ChecksDuplicateName(t *testing.T) {
	rule := config.CheckRule{
		Name:           "same",
		Type:           config.CheckTypeFileBlacklist,
		TargetBranches: []string{"^main$"},
		FileBlacklist:  &config.FileBlacklistCheck{Patterns: []string{"go.sum"}},
	}
	cfg := checkConfig(rule, rule)
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestValidate_ChecksUnknownType(t *testing.T) {
	cfg := checkConfig(config.CheckRule{
		Name:           "other",
		Type:           "title_prefix",
		TargetBranches: []string{"^main$"},
	})
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestValidate_ChecksMissingTargetBranches(t *testing.T) {
	cfg := checkConfig(config.CheckRule{
		Name:          "forbidden",
		Type:          config.CheckTypeFileBlacklist,
		FileBlacklist: &config.FileBlacklistCheck{Patterns: []string{"go.sum"}},
	})
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing target_branches")
	}
}

func TestValidate_ChecksInvalidTargetBranch(t *testing.T) {
	cfg := checkConfig(config.CheckRule{
		Name:           "forbidden",
		Type:           config.CheckTypeFileBlacklist,
		TargetBranches: []string{"["},
		FileBlacklist:  &config.FileBlacklistCheck{Patterns: []string{"go.sum"}},
	})
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestValidate_ChecksDuplicateContext(t *testing.T) {
	cfg := checkConfig(
		config.CheckRule{
			Name:           "one",
			Type:           config.CheckTypeFileBlacklist,
			Context:        "checks/files",
			TargetBranches: []string{"^main$"},
			FileBlacklist:  &config.FileBlacklistCheck{Patterns: []string{"go.sum"}},
		},
		config.CheckRule{
			Name:           "two",
			Type:           config.CheckTypeFileBlacklist,
			Context:        "checks/files",
			TargetBranches: []string{"^main$"},
			FileBlacklist:  &config.FileBlacklistCheck{Patterns: []string{"a.txt"}},
		},
	)
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for duplicate context")
	}
}

func TestValidate_ChecksFileBlacklistMissing(t *testing.T) {
	cfg := checkConfig(config.CheckRule{
		Name:           "forbidden",
		Type:           config.CheckTypeFileBlacklist,
		TargetBranches: []string{"^main$"},
	})
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing file_blacklist")
	}
}

func TestValidate_ChecksFileBlacklistEmptyPatterns(t *testing.T) {
	cfg := checkConfig(config.CheckRule{
		Name:           "forbidden",
		Type:           config.CheckTypeFileBlacklist,
		TargetBranches: []string{"^main$"},
		FileBlacklist:  &config.FileBlacklistCheck{},
	})
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty patterns")
	}
}

func TestValidate_ChecksFileBlacklistInvalidPattern(t *testing.T) {
	cfg := checkConfig(config.CheckRule{
		Name:           "forbidden",
		Type:           config.CheckTypeFileBlacklist,
		TargetBranches: []string{"^main$"},
		FileBlacklist:  &config.FileBlacklistCheck{Patterns: []string{"/abs/path"}},
	})
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for absolute pattern")
	}
}

func TestValidate_ChecksForeignSpec(t *testing.T) {
	cfg := checkConfig(config.CheckRule{
		Name:           "other",
		Type:           "not-a-type",
		TargetBranches: []string{"^main$"},
		FileBlacklist:  &config.FileBlacklistCheck{Patterns: []string{"go.sum"}},
	})
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for foreign spec")
	}
	if err.Error() == "" || !strings.Contains(err.Error(), "do not belong") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckRule_MatchesTargetBranchWithoutValidate(t *testing.T) {
	rule := config.CheckRule{TargetBranches: []string{"^main$"}}
	if rule.MatchesTargetBranch("main") {
		t.Fatal("uncompiled rule must not match")
	}
}

func checkConfig(rules ...config.CheckRule) *config.Config {
	return &config.Config{
		Jenkins: config.JenkinsConfig{BaseURL: "https://j.example.com"},
		Gitea:   config.GiteaConfig{BaseURL: "https://g.example.com", Token: "t"},
		Checks:  rules,
	}
}

func TestNewHTTPClient_TransportNotNil(t *testing.T) {
	client := config.NewHTTPClient(false, time.Second)
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	// Ensure Transport is set so Do() would not use default
	if client.Transport == nil {
		t.Fatal("expected Transport to be set")
	}
	_ = req
}
