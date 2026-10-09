package webhook

import (
	"encoding/json"
	"testing"
)

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

func TestPullRequest_UnmarshalBaseHead(t *testing.T) {
	raw := []byte(`{"number":4,"title":"t","base":{"ref":"main","sha":"abc"},"head":{"ref":"feature","sha":"def"}}`)
	var pr PullRequest
	if err := json.Unmarshal(raw, &pr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if pr.Base.Ref != "main" || pr.Base.SHA != "abc" {
		t.Fatalf("base = %+v", pr.Base)
	}
	if pr.Head.Ref != "feature" || pr.Head.SHA != "def" {
		t.Fatalf("head = %+v", pr.Head)
	}
}
