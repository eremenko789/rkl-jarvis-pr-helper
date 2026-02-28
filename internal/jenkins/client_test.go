package jenkins_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/gitea-jenkins-webhook/internal/jenkins"
)

// errTransport возвращает ошибку при любом RoundTrip.
type errTransport struct{ err error }

func (e errTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, e.err
}

func TestWaitForJob(t *testing.T) {
	var callCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		var jobs []jenkins.Job
		if count >= 2 {
			jobs = []jenkins.Job{{Name: "job-123", URL: "http://jenkins/job-123"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jobs": jobs,
		})
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "user", "token", &http.Client{
		Timeout: time.Second,
	}, nil)

	ctx := context.Background()
	re := regexp.MustCompile(`job-123`)
	job, err := client.WaitForJob(ctx, re, "", 2*time.Second, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if job == nil || job.Name != "job-123" {
		t.Fatalf("unexpected job: %#v", job)
	}
}

func TestWaitForJobTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jobs": []jenkins.Job{},
		})
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	re := regexp.MustCompile(`job`)
	_, err := client.WaitForJob(ctx, re, "", 300*time.Millisecond, 100*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
}

// TestWaitForJob_NoMatch покрывает ветку findJob: джобы есть, но ни один не совпадает с паттерном.
func TestWaitForJob_NoMatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobs := []jenkins.Job{
			{Name: "other-job", URL: "http://j/other", FullName: "other-job"},
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jobs": jobs})
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	re := regexp.MustCompile(`target-job`)
	_, err := client.WaitForJob(ctx, re, "", 400*time.Millisecond, 100*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout when no job matches")
	}
}

func TestWaitForJobWithJobRoot(t *testing.T) {
	var requestedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		jobs := []jenkins.Job{{Name: "test-job", URL: "http://jenkins/test-job"}}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jobs": jobs,
		})
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "user", "token", &http.Client{
		Timeout: time.Second,
	}, nil)

	ctx := context.Background()
	re := regexp.MustCompile(`test-job`)
	job, err := client.WaitForJob(ctx, re, "test_webhook/test_webhooks", 2*time.Second, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if job == nil || job.Name != "test-job" {
		t.Fatalf("unexpected job: %#v", job)
	}
	expectedPath := "/job/test_webhook/job/test_webhooks/api/json"
	if requestedPath != expectedPath {
		t.Fatalf("expected path %s, got %s", expectedPath, requestedPath)
	}
}

func TestGetJobs_Success(t *testing.T) {
	jobs := []jenkins.Job{
		{Name: "job-a", URL: "http://j/job-a", FullName: "job-a"},
		{Name: "job-b", URL: "http://j/job-b", FullName: "job-b"},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/json" || r.URL.RawQuery == "" {
			t.Errorf("unexpected path or query: path=%s rawQuery=%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jobs": jobs})
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	got, err := client.GetJobs(ctx, "")
	if err != nil {
		t.Fatalf("GetJobs: %v", err)
	}
	if len(got) != 2 || got[0].Name != "job-a" || got[1].Name != "job-b" {
		t.Fatalf("unexpected jobs: %#v", got)
	}
}

func TestGetJobs_EmptyList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"jobs": []jenkins.Job{}})
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	got, err := client.GetJobs(ctx, "")
	if err != nil {
		t.Fatalf("GetJobs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty list, got %d", len(got))
	}
}

func TestGetJobs_WithJobRoot(t *testing.T) {
	var path string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{"jobs": []jenkins.Job{}})
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	_, err := client.GetJobs(ctx, "folder/sub")
	if err != nil {
		t.Fatalf("GetJobs: %v", err)
	}
	expected := "/job/folder/job/sub/api/json"
	if path != expected {
		t.Fatalf("expected path %s, got %s", expected, path)
	}
}

func TestGetJobs_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	_, err := client.GetJobs(ctx, "")
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestGetJobs_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	_, err := client.GetJobs(ctx, "")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetJobs_DoFails(t *testing.T) {
	wantErr := errors.New("connection refused")
	client := jenkins.NewClient("http://localhost", "", "", &http.Client{
		Transport: errTransport{err: wantErr},
		Timeout:   time.Second,
	}, nil)
	ctx := context.Background()
	_, err := client.GetJobs(ctx, "")
	if err == nil {
		t.Fatal("expected error when Do fails")
	}
	if !strings.Contains(err.Error(), wantErr.Error()) {
		t.Errorf("expected error containing %q, got %v", wantErr.Error(), err)
	}
}

func TestGetJobs_StatusBadRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	_, err := client.GetJobs(ctx, "")
	if err == nil {
		t.Fatal("expected error for 400")
	}
}

func TestCheckAccessibility_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "u", "t", &http.Client{Timeout: time.Second}, nil)
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

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckAccessibility(ctx); err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestCheckAccessibility_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckAccessibility(ctx); err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestCheckAccessibility_Forbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckAccessibility(ctx); err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestCheckAccessibility_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckAccessibility(ctx); err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestCheckJobRootExists_Empty(t *testing.T) {
	client := jenkins.NewClient("http://example.com", "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckJobRootExists(ctx, ""); err != nil {
		t.Fatalf("empty job root should be valid: %v", err)
	}
}

func TestCheckJobRootExists_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/job/folder/api/json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckJobRootExists(ctx, "folder"); err != nil {
		t.Fatalf("CheckJobRootExists: %v", err)
	}
}

func TestCheckJobRootExists_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckJobRootExists(ctx, "folder"); err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestCheckJobRootExists_Forbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckJobRootExists(ctx, "folder"); err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestCheckJobRootExists_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckJobRootExists(ctx, "folder"); err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestNewClient_NilHTTPClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"jobs": []jenkins.Job{}})
	}))
	defer ts.Close()

	// NewClient with nil httpClient must use default client with timeout
	client := jenkins.NewClient(ts.URL, "", "", nil, nil)
	ctx := context.Background()
	got, err := client.GetJobs(ctx, "")
	if err != nil {
		t.Fatalf("GetJobs with nil httpClient: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty jobs, got %d", len(got))
	}
}

func TestWaitForJob_MatchFullName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobs := []jenkins.Job{
			{Name: "build", URL: "http://j/build", FullName: "folder/build-42"},
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jobs": jobs})
	}))
	defer ts.Close()

	client := jenkins.NewClient(ts.URL, "", "", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	re := regexp.MustCompile(`build-42`)
	job, err := client.WaitForJob(ctx, re, "", 2*time.Second, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForJob: %v", err)
	}
	if job == nil || job.FullName != "folder/build-42" {
		t.Fatalf("unexpected job: %#v", job)
	}
}
