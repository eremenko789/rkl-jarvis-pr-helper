package webhook

import "testing"

func TestDisplayName_WithTitle(t *testing.T) {
	pr := PullRequest{Number: 1, Title: "Add feature"}
	if got := pr.DisplayName(); got != "Add feature" {
		t.Fatalf("DisplayName() = %q, want %q", got, "Add feature")
	}
}

func TestDisplayName_EmptyTitle(t *testing.T) {
	pr := PullRequest{Number: 1, Title: ""}
	if got := pr.DisplayName(); got != "PR" {
		t.Fatalf("DisplayName() = %q, want %q", got, "PR")
	}
}
