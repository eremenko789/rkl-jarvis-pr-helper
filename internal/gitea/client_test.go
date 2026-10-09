package gitea_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/example/gitea-jenkins-webhook/internal/gitea"
)

// errTransport возвращает ошибку при любом RoundTrip.
type errTransport struct{ err error }

func (e errTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, e.err
}

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

func TestPostComment_DoFails(t *testing.T) {
	wantErr := errors.New("network failure")
	client := gitea.NewClient("http://localhost", "t", &http.Client{
		Transport: errTransport{err: wantErr},
		Timeout:   time.Second,
	}, nil)
	ctx := context.Background()
	err := client.PostComment(ctx, "owner/repo", 1, "x")
	if err == nil {
		t.Fatal("expected error when Do fails")
	}
	if !errors.Is(err, wantErr) && !strings.Contains(err.Error(), wantErr.Error()) {
		t.Errorf("expected error containing %q, got %v", wantErr.Error(), err)
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

func TestCheckAccessibility_Forbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckAccessibility(ctx); err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestCheckAccessibility_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckAccessibility(ctx); err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestCheckAccessibility_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckAccessibility(ctx); err == nil {
		t.Fatal("expected error for 500")
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

func TestGetRepository_Forbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.GetRepository(ctx, "owner", "repo"); err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestGetRepository_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.GetRepository(ctx, "owner", "repo"); err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestGetRepository_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.GetRepository(ctx, "owner", "repo"); err == nil {
		t.Fatal("expected error for 500")
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

func TestNewClient_NilLogger(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// NewClient with nil logger must not panic and must use slog.Default()
	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	if err := client.CheckAccessibility(ctx); err != nil {
		t.Fatalf("CheckAccessibility with nil logger: %v", err)
	}
}

func TestNewClient_NilHTTPClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// NewClient with nil httpClient must use default client with timeout
	client := gitea.NewClient(ts.URL, "t", nil, nil)
	ctx := context.Background()
	if err := client.CheckAccessibility(ctx); err != nil {
		t.Fatalf("CheckAccessibility with nil httpClient: %v", err)
	}
}

func TestListPullRequestFiles_Pagination(t *testing.T) {
	var pages []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/pulls/7/files" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		page := r.URL.Query().Get("page")
		pages = append(pages, page)
		if r.URL.Query().Get("limit") != "50" {
			t.Errorf("limit = %s", r.URL.Query().Get("limit"))
		}
		if page == "1" {
			setPullFilesPageHeaders(w, true, 1, 2)
			files := make([]map[string]string, 50)
			for i := range files {
				files[i] = map[string]string{"filename": fmt.Sprintf("f%d.txt", i), "status": "modified"}
			}
			_ = json.NewEncoder(w).Encode(files)
			return
		}
		setPullFilesPageHeaders(w, false, 2, 2)
		_ = json.NewEncoder(w).Encode([]map[string]string{{
			"filename":          "renamed.txt",
			"previous_filename": "old.txt",
			"status":            "renamed",
		}})
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "token", &http.Client{Timeout: time.Second}, nil)
	files, err := client.ListPullRequestFiles(context.Background(), "owner/repo", 7)
	if err != nil {
		t.Fatalf("ListPullRequestFiles: %v", err)
	}
	if len(files) != 51 {
		t.Fatalf("files = %d, want 51", len(files))
	}
	if files[0].Filename != "f0.txt" || files[50].PreviousFilename != "old.txt" {
		t.Fatalf("unexpected files: %+v ... %+v", files[0], files[50])
	}
	if len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
		t.Fatalf("pages = %v", pages)
	}
}

func TestListPullRequestFiles_FullPageWithoutMore(t *testing.T) {
	var requests int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		setPullFilesPageHeaders(w, false, 1, 1)
		files := make([]map[string]string, 50)
		for i := range files {
			files[i] = map[string]string{"filename": fmt.Sprintf("f%d.txt", i), "status": "modified"}
		}
		_ = json.NewEncoder(w).Encode(files)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	files, err := client.ListPullRequestFiles(context.Background(), "owner/repo", 1)
	if err != nil {
		t.Fatalf("ListPullRequestFiles: %v", err)
	}
	if requests != 1 || len(files) != 50 {
		t.Fatalf("requests = %d, files = %d", requests, len(files))
	}
}

func TestListPullRequestFiles_ShortPageWithMore(t *testing.T) {
	var pages []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		pages = append(pages, page)
		if page == "1" {
			setPullFilesPageHeaders(w, true, 1, 2)
			_ = json.NewEncoder(w).Encode([]map[string]string{{"filename": "a.txt", "status": "added"}})
			return
		}
		setPullFilesPageHeaders(w, false, 2, 2)
		_ = json.NewEncoder(w).Encode([]map[string]string{{"filename": "b.txt", "status": "added"}})
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	files, err := client.ListPullRequestFiles(context.Background(), "owner/repo", 1)
	if err != nil {
		t.Fatalf("ListPullRequestFiles: %v", err)
	}
	if len(files) != 2 || files[0].Filename != "a.txt" || files[1].Filename != "b.txt" {
		t.Fatalf("files = %+v", files)
	}
	if len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
		t.Fatalf("pages = %v", pages)
	}
}

func TestListPullRequestFiles_Empty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setPullFilesPageHeaders(w, false, 1, 0)
		_, _ = io.WriteString(w, "[]")
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	files, err := client.ListPullRequestFiles(context.Background(), "owner/repo", 1)
	if err != nil {
		t.Fatalf("ListPullRequestFiles: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("files = %+v", files)
	}
}

func TestListPullRequestFiles_MissingPaginationHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "[]")
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	_, err := client.ListPullRequestFiles(context.Background(), "owner/repo", 1)
	if err == nil || !strings.Contains(err.Error(), "X-HasMore") {
		t.Fatalf("expected missing pagination headers, got %v", err)
	}
}

func TestListPullRequestFiles_PageMismatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setPullFilesPageHeaders(w, false, 3, 3)
		_, _ = io.WriteString(w, "[]")
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	_, err := client.ListPullRequestFiles(context.Background(), "owner/repo", 1)
	if err == nil || !strings.Contains(err.Error(), "X-Page") {
		t.Fatalf("expected page mismatch, got %v", err)
	}
}

func TestListPullRequestFiles_TooManyPages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		setPullFilesPageHeaders(w, true, page, 1000)
		files := make([]map[string]string, 50)
		for i := range files {
			files[i] = map[string]string{"filename": fmt.Sprintf("f%d.txt", i), "status": "modified"}
		}
		_ = json.NewEncoder(w).Encode(files)
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, logger)
	_, err := client.ListPullRequestFiles(context.Background(), "owner/repo", 1)
	if err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("expected page limit error, got %v", err)
	}
}

func setPullFilesPageHeaders(w http.ResponseWriter, hasMore bool, page, pageCount int) {
	w.Header().Set("X-HasMore", strconv.FormatBool(hasMore))
	w.Header().Set("X-Page", strconv.Itoa(page))
	w.Header().Set("X-PageCount", strconv.Itoa(pageCount))
}

func TestListPullRequestFiles_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	_, err := client.ListPullRequestFiles(context.Background(), "owner/repo", 1)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestListPullRequestFiles_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"filename":`)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	_, err := client.ListPullRequestFiles(context.Background(), "owner/repo", 1)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestListPullRequestFiles_InvalidRepoName(t *testing.T) {
	client := gitea.NewClient("http://localhost", "t", &http.Client{Timeout: time.Second}, nil)
	_, err := client.ListPullRequestFiles(context.Background(), "single", 1)
	if err == nil {
		t.Fatal("expected error for invalid repo name")
	}
}

func TestListPullRequestFiles_DoFails(t *testing.T) {
	wantErr := errors.New("network failure")
	client := gitea.NewClient("http://localhost", "t", &http.Client{
		Transport: errTransport{err: wantErr},
		Timeout:   time.Second,
	}, nil)
	_, err := client.ListPullRequestFiles(context.Background(), "owner/repo", 1)
	if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected network error, got %v", err)
	}
}

func TestCreateCommitStatus_OK(t *testing.T) {
	var path, auth, body string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		auth = r.Header.Get("Authorization")
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "mytoken", &http.Client{Timeout: time.Second}, nil)
	err := client.CreateCommitStatus(context.Background(), "owner/repo", "abc123", gitea.CommitStatus{
		State:       "failure",
		Context:     "checks/forbidden-files",
		Description: "Изменены файлы из чёрного списка: go.sum",
	})
	if err != nil {
		t.Fatalf("CreateCommitStatus: %v", err)
	}
	if path != "/repos/owner/repo/statuses/abc123" {
		t.Fatalf("path = %s", path)
	}
	if !strings.Contains(auth, "mytoken") {
		t.Fatalf("auth = %s", auth)
	}
	if !strings.Contains(body, `"state":"failure"`) || !strings.Contains(body, `"context":"checks/forbidden-files"`) || !strings.Contains(body, "go.sum") {
		t.Fatalf("body = %s", body)
	}
}

func TestCreateCommitStatus_Validation(t *testing.T) {
	client := gitea.NewClient("http://localhost", "t", &http.Client{Timeout: time.Second}, nil)
	ctx := context.Background()
	status := gitea.CommitStatus{State: "success", Context: "checks/x", Description: "ok"}

	if err := client.CreateCommitStatus(ctx, "owner/repo", "", status); err == nil {
		t.Fatal("expected error for empty sha")
	}
	if err := client.CreateCommitStatus(ctx, "owner/repo", "abc", gitea.CommitStatus{Context: "checks/x"}); err == nil {
		t.Fatal("expected error for empty state")
	}
	if err := client.CreateCommitStatus(ctx, "owner/repo", "abc", gitea.CommitStatus{State: "nope", Context: "checks/x"}); err == nil {
		t.Fatal("expected error for invalid state")
	}
	if err := client.CreateCommitStatus(ctx, "owner/repo", "abc", gitea.CommitStatus{State: "success"}); err == nil {
		t.Fatal("expected error for empty context")
	}
	if err := client.CreateCommitStatus(ctx, "single", "abc", status); err == nil {
		t.Fatal("expected error for invalid repo name")
	}
}

func TestCreateCommitStatus_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := gitea.NewClient(ts.URL, "t", &http.Client{Timeout: time.Second}, nil)
	err := client.CreateCommitStatus(context.Background(), "owner/repo", "abc", gitea.CommitStatus{
		State:   "success",
		Context: "checks/x",
	})
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestCreateCommitStatus_DoFails(t *testing.T) {
	wantErr := errors.New("network failure")
	client := gitea.NewClient("http://localhost", "t", &http.Client{
		Transport: errTransport{err: wantErr},
		Timeout:   time.Second,
	}, nil)
	err := client.CreateCommitStatus(context.Background(), "owner/repo", "abc", gitea.CommitStatus{
		State:   "success",
		Context: "checks/x",
	})
	if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected network error, got %v", err)
	}
}
