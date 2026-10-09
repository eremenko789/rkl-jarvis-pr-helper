// Package gitea предоставляет клиент для взаимодействия с API Gitea.
package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	pullFilesPageSize = 50
	maxPullFilePages  = 200
)

// PullRequestFile — файл, изменённый в pull request.
type PullRequestFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
}

// CommitStatus — статус коммита, публикуемый в Gitea.
type CommitStatus struct {
	State       string `json:"state"`
	Context     string `json:"context,omitempty"`
	Description string `json:"description,omitempty"`
	TargetURL   string `json:"target_url,omitempty"`
}

// Client представляет клиент для работы с API Gitea.
type Client struct {
	baseURL string
	token   string
	client  *http.Client
	log     *slog.Logger
}

// commentRequest представляет запрос на создание комментария в Gitea.
type commentRequest struct {
	Body string `json:"body"` // Текст комментария
}

// NewClient создает новый клиент для работы с API Gitea.
// Если httpClient равен nil, создается клиент с таймаутом 10 секунд.
// Если logger равен nil, используется логгер по умолчанию.
func NewClient(baseURL, token string, httpClient *http.Client, logger *slog.Logger) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  httpClient,
		log:     logger,
	}
}

// PostComment публикует комментарий в указанном issue или pull request репозитория Gitea.
// repoFullName должен быть в формате "owner/repo", issueIndex - номер issue/PR.
func (c *Client) PostComment(ctx context.Context, repoFullName string, issueIndex int64, body string) error {
	c.log.Info("posting comment to Gitea",
		"repo", repoFullName,
		"issue_index", issueIndex,
		"comment_length", len(body))

	owner, repo, err := splitRepoFullName(repoFullName)
	if err != nil {
		c.log.Error("failed to split repo full name", "err", err, "repo", repoFullName)
		return err
	}

	path := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.baseURL, owner, repo, issueIndex)
	payload := commentRequest{Body: body}
	data, err := json.Marshal(payload)
	if err != nil {
		c.log.Error("failed to marshal comment payload", "err", err)
		return fmt.Errorf("marshal comment payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(data))
	if err != nil {
		c.log.Error("failed to create request", "err", err)
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))

	// Логирование запроса
	c.log.Info("Gitea API request",
		"method", http.MethodPost,
		"url", path,
		"base_url", c.baseURL,
		"content_type", req.Header.Get("Content-Type"),
		"request_body", string(data),
		"request_body_length", len(data))

	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Error("failed to execute Gitea request",
			"err", err,
			"url", path,
			"base_url", c.baseURL)
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Логирование ответа
	c.log.Info("Gitea API response",
		"url", path,
		"base_url", c.baseURL,
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"response_headers", resp.Header,
		"response_body", string(respBody),
		"response_body_length", len(respBody))

	if resp.StatusCode >= 400 {
		c.log.Error("Gitea API error",
			"url", path,
			"base_url", c.baseURL,
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"response_body", string(respBody))
		return fmt.Errorf("post comment failed: status %s", resp.Status)
	}

	c.log.Info("comment posted to Gitea successfully",
		"repo", repoFullName,
		"issue_index", issueIndex,
		"status_code", resp.StatusCode)
	return nil
}

// splitRepoFullName разделяет полное имя репозитория (формат "owner/repo") на владельца и имя репозитория.
func splitRepoFullName(fullName string) (string, string, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo full name: %s", fullName)
	}
	return parts[0], parts[1], nil
}

// CheckAccessibility проверяет доступность Gitea, выполняя запрос к эндпоинту /user.
// Возвращает ошибку, если Gitea недоступен или аутентификация не удалась.
func (c *Client) CheckAccessibility(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	endpoint := fmt.Sprintf("%s/user", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))

	c.log.Info("Gitea API request",
		"method", http.MethodGet,
		"url", endpoint,
		"base_url", c.baseURL)

	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Error("failed to execute Gitea request",
			"err", err,
			"url", endpoint,
			"base_url", c.baseURL)
		return fmt.Errorf("gitea api request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.log.Info("Gitea API response",
		"url", endpoint,
		"base_url", c.baseURL,
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"response_body", string(respBody))

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("authentication failed: status %s", resp.Status)
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("gitea api not found: status %s", resp.Status)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gitea api error: status %s", resp.Status)
	}

	return nil
}

// GetRepository проверяет существование репозитория в Gitea.
// Возвращает ошибку, если репозиторий не найден, доступ запрещен или произошла другая ошибка API.
func (c *Client) GetRepository(ctx context.Context, owner, repo string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	endpoint := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))

	c.log.Info("Gitea API request",
		"method", http.MethodGet,
		"url", endpoint,
		"base_url", c.baseURL,
		"owner", owner,
		"repo", repo)

	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Error("failed to execute Gitea request",
			"err", err,
			"url", endpoint,
			"base_url", c.baseURL)
		return fmt.Errorf("gitea api request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.log.Info("Gitea API response",
		"url", endpoint,
		"base_url", c.baseURL,
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"response_body", string(respBody))

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("repository not found: status %s", resp.Status)
	}
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("access denied to repository: status %s", resp.Status)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed: status %s", resp.Status)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gitea api error: status %s", resp.Status)
	}

	return nil
}

// pullFilesPage сообщает, есть ли следующая страница списка файлов.
// Заголовки Gitea: X-HasMore, X-Page, X-PageCount.
type pullFilesPage struct {
	hasMore   bool
	page      int
	pageCount int
}

// ListPullRequestFiles возвращает файлы, изменённые в pull request.
// Страницы запрашиваются, пока X-HasMore равен true и номер страницы меньше X-PageCount.
func (c *Client) ListPullRequestFiles(ctx context.Context, repoFullName string, index int64) ([]PullRequestFile, error) {
	owner, repo, err := splitRepoFullName(repoFullName)
	if err != nil {
		c.log.Error("failed to split repo full name", "err", err, "repo", repoFullName)
		return nil, err
	}

	var all []PullRequestFile
	for page := 1; ; page++ {
		if page > maxPullFilePages {
			return nil, fmt.Errorf("pull request file list exceeded %d pages", maxPullFilePages)
		}
		endpoint := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/files?page=%d&limit=%d",
			c.baseURL, owner, repo, index, page, pullFilesPageSize)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))

		c.log.Info("Gitea API request",
			"method", http.MethodGet,
			"url", endpoint,
			"base_url", c.baseURL,
			"page", page)

		resp, err := c.client.Do(req)
		if err != nil {
			c.log.Error("failed to execute Gitea request",
				"err", err,
				"url", endpoint,
				"base_url", c.baseURL)
			return nil, fmt.Errorf("list pull request files: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read pull request files: %w", readErr)
		}

		c.log.Info("Gitea API response",
			"url", endpoint,
			"base_url", c.baseURL,
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"x_has_more", resp.Header.Get("X-HasMore"),
			"x_page", resp.Header.Get("X-Page"),
			"x_page_count", resp.Header.Get("X-PageCount"),
			"response_body_length", len(respBody))

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			c.log.Error("Gitea API error",
				"url", endpoint,
				"status_code", resp.StatusCode,
				"response_body", string(respBody))
			return nil, fmt.Errorf("list pull request files failed: status %s", resp.Status)
		}

		var batch []PullRequestFile
		if err := json.Unmarshal(respBody, &batch); err != nil {
			return nil, fmt.Errorf("decode pull request files: %w", err)
		}
		info, err := parsePullFilesPage(resp.Header)
		if err != nil {
			return nil, fmt.Errorf("pull request files pagination: %w", err)
		}
		if info.page != page {
			return nil, fmt.Errorf("pull request files pagination: X-Page %d does not match requested page %d", info.page, page)
		}
		all = append(all, batch...)
		if !info.hasMore || page >= info.pageCount {
			break
		}
	}

	c.log.Info("pull request files listed",
		"repo", repoFullName,
		"index", index,
		"files", len(all))
	return all, nil
}

// CreateCommitStatus публикует статус коммита в репозитории Gitea.
// sha — SHA коммита head pull request. Статус отображается на pull request.
func (c *Client) CreateCommitStatus(ctx context.Context, repoFullName, sha string, status CommitStatus) error {
	if sha == "" {
		return fmt.Errorf("commit sha is empty")
	}
	if status.State == "" {
		return fmt.Errorf("commit status state is empty")
	}
	if !validCommitStatusState(status.State) {
		return fmt.Errorf("invalid commit status state %q", status.State)
	}
	if status.Context == "" {
		return fmt.Errorf("commit status context is empty")
	}

	owner, repo, err := splitRepoFullName(repoFullName)
	if err != nil {
		c.log.Error("failed to split repo full name", "err", err, "repo", repoFullName)
		return err
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/statuses/%s", c.baseURL, owner, repo, url.PathEscape(sha))
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal commit status: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))

	c.log.Info("Gitea API request",
		"method", http.MethodPost,
		"url", endpoint,
		"base_url", c.baseURL,
		"state", status.State,
		"context", status.Context,
		"request_body", string(data))

	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Error("failed to execute Gitea request",
			"err", err,
			"url", endpoint,
			"base_url", c.baseURL)
		return fmt.Errorf("create commit status: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.log.Info("Gitea API response",
		"url", endpoint,
		"base_url", c.baseURL,
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"response_body", string(respBody),
		"response_body_length", len(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.log.Error("Gitea API error",
			"url", endpoint,
			"status_code", resp.StatusCode,
			"response_body", string(respBody))
		return fmt.Errorf("create commit status failed: status %s", resp.Status)
	}

	c.log.Info("commit status posted",
		"repo", repoFullName,
		"sha", sha,
		"context", status.Context,
		"state", status.State)
	return nil
}

func parsePullFilesPage(header http.Header) (pullFilesPage, error) {
	hasMoreRaw := header.Get("X-HasMore")
	pageRaw := header.Get("X-Page")
	pageCountRaw := header.Get("X-PageCount")
	if hasMoreRaw == "" || pageRaw == "" || pageCountRaw == "" {
		return pullFilesPage{}, fmt.Errorf("missing X-HasMore, X-Page or X-PageCount")
	}
	hasMore, err := strconv.ParseBool(hasMoreRaw)
	if err != nil {
		return pullFilesPage{}, fmt.Errorf("parse X-HasMore %q: %w", hasMoreRaw, err)
	}
	page, err := strconv.Atoi(pageRaw)
	if err != nil || page < 1 {
		return pullFilesPage{}, fmt.Errorf("parse X-Page %q", pageRaw)
	}
	pageCount, err := strconv.Atoi(pageCountRaw)
	if err != nil || pageCount < 0 {
		return pullFilesPage{}, fmt.Errorf("parse X-PageCount %q", pageCountRaw)
	}
	return pullFilesPage{hasMore: hasMore, page: page, pageCount: pageCount}, nil
}

func validCommitStatusState(state string) bool {
	switch state {
	case "pending", "success", "error", "failure", "warning":
		return true
	default:
		return false
	}
}
