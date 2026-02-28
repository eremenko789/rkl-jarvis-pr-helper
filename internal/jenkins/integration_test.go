//go:build integration

package jenkins_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/example/gitea-jenkins-webhook/internal/config"
	"github.com/example/gitea-jenkins-webhook/internal/jenkins"
)

func TestIntegration_CheckAccessibility(t *testing.T) {
	baseURL := os.Getenv("JENKINS_BASE_URL")
	if baseURL == "" {
		t.Skip("JENKINS_BASE_URL not set, skipping integration test")
	}
	client := config.NewHTTPClient(false, 10*time.Second)
	jc := jenkins.NewClient(baseURL, os.Getenv("JENKINS_USERNAME"), os.Getenv("JENKINS_API_TOKEN"), client, nil)
	ctx := context.Background()
	if err := jc.CheckAccessibility(ctx); err != nil {
		t.Fatalf("CheckAccessibility: %v", err)
	}
}
