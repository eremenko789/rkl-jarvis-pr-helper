package gitea_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/gitea-jenkins-webhook/internal/gitea"
)

func TestPostComment_OK(t *testing.T) {
	var path, auth, body string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		auth = r.Header.Get("Authorization")
		if r.Body != nil {
			var buf [1024]byte
			n, _ := r.Body.Read(buf[:])
			body = string(buf[:n])
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "mytoken", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	err := client.PostComment(ctx, "owner/repo", 5, "hello")
	if err != nil {
		t.Fatalf("PostComment: %v", err)
	}
	if !strings.Contains(path, "/owner/repo/") || !strings.Contains(path, "/issues/5/") {
		t.Fatalf("unexpected path: %s", path)
	}
	if !strings.Contains(auth, "mytoken") {
		t.Fatalf("expected token in Authorization: %s", auth)
	}
	if !strings.Contains(body, "hello") {
		t.Fatalf("expected body to contain comment: %s", body)
	}
}

func TestPostComment_InvalidRepoName(t *testing.T) {
	client := gitea.NewClient("http://localhost", "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	err := client.PostComment(ctx, "single", 1, "x")
	if err == nil {
		t.Fatal("expected error for invalid repo name")
	}
	err = client.PostComment(ctx, "", 1, "x")
	if err == nil {
		t.Fatal("expected error for empty repo name")
	}
}

func TestPostComment_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	err := client.PostComment(ctx, "owner/repo", 1, "x")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestCheckAccessibility_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckAccessibility(ctx); err != nil {
		t.Fatalf("CheckAccessibility: %v", err)
	}
}

func TestCheckAccessibility_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckAccessibility(ctx); err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestGetRepository_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.GetRepository(ctx, "owner", "repo"); err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
}

func TestGetRepository_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.GetRepository(ctx, "owner", "repo"); err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestNewClient_TrimTrailingSlash(t *testing.T) {
	var receivedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	baseURL := ts.URL + "/"
	client := gitea.NewClient(baseURL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	// PostComment builds path as baseURL + "/repos/owner/repo/..."
	// If baseURL were not trimmed we could get double slash
	err := client.PostComment(ctx, "a/b", 1, "x")
	if err != nil {
		t.Fatalf("PostComment: %v", err)
	}
	if strings.Contains(receivedPath, "//") {
		t.Fatalf("path should not contain double slash: %s", receivedPath)
	}
}
