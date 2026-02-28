//go:build integration

package gitea_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/example/gitea-jenkins-webhook/internal/gitea"
)

func TestIntegration_CheckAccessibility(t *testing.T) {
	baseURL := os.Getenv("GITEA_BASE_URL")
	token := os.Getenv("GITEA_TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("GITEA_BASE_URL or GITEA_TOKEN not set, skipping integration test")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	gc := gitea.NewClient(baseURL, token, client, nil)
	ctx := context.Background()
	if err := gc.CheckAccessibility(ctx); err != nil {
		t.Fatalf("CheckAccessibility: %v", err)
	}
}
