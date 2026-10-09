package checks

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/example/gitea-jenkins-webhook/internal/config"
)

func TestApplicable(t *testing.T) {
	rules := []config.CheckRule{
		compiledRule("main-only", []string{"^main$"}),
		compiledRule("release", []string{"^release/.*"}),
	}

	got := Applicable(rules, "main")
	if len(got) != 1 || got[0].Name != "main-only" {
		t.Fatalf("applicable for main: %+v", names(got))
	}

	got = Applicable(rules, "release/1.2")
	if len(got) != 1 || got[0].Name != "release" {
		t.Fatalf("applicable for release: %+v", names(got))
	}

	if got := Applicable(rules, "develop"); len(got) != 0 {
		t.Fatalf("expected no rules for develop, got %+v", names(got))
	}
}

func TestEvaluateFileBlacklist_Success(t *testing.T) {
	rule := fileRule(t, []string{"go.sum", "**/.env"})
	outcome, err := Evaluate(rule, []FileChange{{Filename: "main.go"}, {Filename: "README.md"}})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if outcome.State != StateSuccess {
		t.Fatalf("state = %s, want success", outcome.State)
	}
	if outcome.Description != rule.SuccessDescription {
		t.Fatalf("description = %q", outcome.Description)
	}
}

func TestEvaluateFileBlacklist_EmptyDiff(t *testing.T) {
	rule := fileRule(t, []string{"go.sum"})
	outcome, err := Evaluate(rule, nil)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if outcome.State != StateSuccess {
		t.Fatalf("state = %s, want success", outcome.State)
	}
}

func TestEvaluateFileBlacklist_Failure(t *testing.T) {
	rule := fileRule(t, []string{"go.sum", "secrets/**"})
	outcome, err := Evaluate(rule, []FileChange{
		{Filename: "main.go"},
		{Filename: "secrets/api.key"},
		{Filename: "go.sum"},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if outcome.State != StateFailure {
		t.Fatalf("state = %s, want failure", outcome.State)
	}
	want := rule.FailureDescription + ": secrets/api.key, go.sum"
	if outcome.Description != want {
		t.Fatalf("description = %q, want %q", outcome.Description, want)
	}
}

func TestEvaluateFileBlacklist_PreviousFilename(t *testing.T) {
	rule := fileRule(t, []string{"legacy/secret.txt"})
	outcome, err := Evaluate(rule, []FileChange{{
		Filename:         "docs/secret.txt",
		PreviousFilename: "legacy/secret.txt",
	}})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if outcome.State != StateFailure {
		t.Fatalf("state = %s, want failure", outcome.State)
	}
	if !strings.Contains(outcome.Description, "legacy/secret.txt") {
		t.Fatalf("description = %q", outcome.Description)
	}
}

func TestEvaluateFileBlacklist_TruncatesDescription(t *testing.T) {
	rule := fileRule(t, []string{"**/*"})
	var files []FileChange
	for i := 0; i < 20; i++ {
		files = append(files, FileChange{Filename: fmt.Sprintf("dir/file-%02d-%s.txt", i, strings.Repeat("x", 30))})
	}
	outcome, err := Evaluate(rule, files)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if outcome.State != StateFailure {
		t.Fatalf("state = %s", outcome.State)
	}
	if len(outcome.Description) > maxStatusDescriptionLen {
		t.Fatalf("description length = %d, want <= %d", len(outcome.Description), maxStatusDescriptionLen)
	}
	if !strings.HasSuffix(outcome.Description, "…") {
		t.Fatalf("description %q should end with ellipsis", outcome.Description)
	}
	if !utf8.ValidString(outcome.Description) {
		t.Fatal("description is not valid UTF-8")
	}
}

func TestEvaluateFileBlacklist_TruncatesOnRuneBoundary(t *testing.T) {
	rule := fileRule(t, []string{"go.sum"})
	rule.FailureDescription = strings.Repeat("я", 300)
	outcome, err := Evaluate(rule, []FileChange{{Filename: "go.sum"}})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(outcome.Description) > maxStatusDescriptionLen {
		t.Fatalf("description length = %d", len(outcome.Description))
	}
	if !utf8.ValidString(outcome.Description) || !strings.HasSuffix(outcome.Description, "…") {
		t.Fatalf("description = %q", outcome.Description)
	}
}

func TestEvaluate_UnknownType(t *testing.T) {
	_, err := Evaluate(config.CheckRule{Name: "x", Type: "other"}, nil)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestEvaluate_MissingSpec(t *testing.T) {
	_, err := Evaluate(config.CheckRule{Name: "x", Type: config.CheckTypeFileBlacklist}, nil)
	if err == nil {
		t.Fatal("expected error for missing file_blacklist settings")
	}
}

func compiledRule(name string, branches []string) config.CheckRule {
	cfg := &config.Config{
		Jenkins: config.JenkinsConfig{BaseURL: "https://j.example.com"},
		Gitea:   config.GiteaConfig{BaseURL: "https://g.example.com", Token: "t"},
		Repositories: []config.RepositoryRule{{
			Name:       "org/repo",
			JobPattern: "^x$",
			Checks: []config.CheckRule{{
				Name:           name,
				Type:           config.CheckTypeFileBlacklist,
				TargetBranches: branches,
				FileBlacklist:  &config.FileBlacklistCheck{Patterns: []string{"go.sum"}},
			}},
		}},
	}
	if err := cfg.Validate(); err != nil {
		panic(err)
	}
	return cfg.Repositories[0].Checks[0]
}

func fileRule(t *testing.T, patterns []string) config.CheckRule {
	t.Helper()
	cfg := &config.Config{
		Jenkins: config.JenkinsConfig{BaseURL: "https://j.example.com"},
		Gitea:   config.GiteaConfig{BaseURL: "https://g.example.com", Token: "t"},
		Repositories: []config.RepositoryRule{{
			Name:       "org/repo",
			JobPattern: "^x$",
			Checks: []config.CheckRule{{
				Name:           "forbidden",
				Type:           config.CheckTypeFileBlacklist,
				TargetBranches: []string{".*"},
				FileBlacklist:  &config.FileBlacklistCheck{Patterns: patterns},
			}},
		}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	return cfg.Repositories[0].Checks[0]
}

func names(rules []config.CheckRule) []string {
	out := make([]string, len(rules))
	for i, rule := range rules {
		out[i] = rule.Name
	}
	return out
}
