// Package config предоставляет функциональность для загрузки и валидации конфигурации приложения.
package config

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// CheckTypeFileBlacklist — проверка, что pull request не меняет файлы из чёрного списка.
	CheckTypeFileBlacklist = "file_blacklist"

	maxCommitStatusContextLen = 255
)

// ServerConfig содержит настройки HTTP-сервера.
type ServerConfig struct {
	ListenAddr     string `yaml:"listen_addr"`
	WebhookSecret  string `yaml:"webhook_secret"`
	WorkerPoolSize int    `yaml:"worker_pool_size"`
	QueueSize      int    `yaml:"queue_size"`
}

// JenkinsConfig содержит настройки подключения к Jenkins.
type JenkinsConfig struct {
	BaseURL            string        `yaml:"base_url"`
	Username           string        `yaml:"username"`
	APIToken           string        `yaml:"api_token"`
	PollInterval       time.Duration `yaml:"poll_interval"`
	Timeout            time.Duration `yaml:"timeout"`
	InsecureSkipVerify bool          `yaml:"insecure_skip_verify"` // Игнорировать некорректные SSL-сертификаты
}

// GiteaConfig содержит настройки подключения к Gitea.
type GiteaConfig struct {
	BaseURL            string `yaml:"base_url"`
	Token              string `yaml:"token"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"` // Игнорировать некорректные SSL-сертификаты
}

// RepositoryRule определяет правила обработки событий для конкретного репозитория.
type RepositoryRule struct {
	Name                   string        `yaml:"name"`
	JobRoot                string        `yaml:"job_root"`
	JobPattern             string        `yaml:"job_pattern"`
	PollInterval           time.Duration `yaml:"poll_interval"`
	Timeout                time.Duration `yaml:"timeout"`
	SuccessCommentTemplate string        `yaml:"success_comment_template"`
	FailureCommentTemplate string        `yaml:"failure_comment_template"`
	Checks                 []CheckRule   `yaml:"checks,omitempty"`
}

// FileBlacklistCheck задаёт чёрный список путей для проверки типа file_blacklist.
// Patterns — glob-шаблоны относительно корня репозитория: * и ? не переходят через /,
// ** совпадает с любым числом сегментов пути.
type FileBlacklistCheck struct {
	Patterns []string `yaml:"patterns"`
}

// CheckRule — правило проверки pull request.
// Общие поля одинаковы для всех типов и описывают, когда проверка применяется
// и какой статус коммита публикуется. Настройки конкретного типа лежат во вложенном
// объекте, ключ которого совпадает со значением Type.
//
// Чтобы добавить новый тип проверки:
//  1. Завести константу CheckType*.
//  2. Добавить структуру настроек и указатель на неё в CheckRule.
//  3. Проверить поля в validateChecks.
//  4. Реализовать ветку в checks.Evaluate.
type CheckRule struct {
	Name               string   `yaml:"name"`
	Type               string   `yaml:"type"`
	Context            string   `yaml:"context"`
	SuccessDescription string   `yaml:"success_description"`
	FailureDescription string   `yaml:"failure_description"`
	TargetBranches     []string `yaml:"target_branches"`

	FileBlacklist *FileBlacklistCheck `yaml:"file_blacklist,omitempty"`

	targetBranches []*regexp.Regexp `yaml:"-"`
}

// Config представляет полную конфигурацию приложения, включая настройки сервера,
// подключения к внешним сервисам и правила обработки репозиториев.
type Config struct {
	Server       ServerConfig      `yaml:"server"`
	Jenkins      JenkinsConfig     `yaml:"jenkins"`
	Gitea        GiteaConfig       `yaml:"gitea"`
	Repositories []RepositoryRule  `yaml:"repositories"`
	RepoIndex    map[string]RepoID `yaml:"-"`
}

// RepoID представляет идентификатор репозитория с его правилами обработки.
type RepoID struct {
	Rule RepositoryRule // Правила обработки для репозитория
}

// Load загружает конфигурацию из YAML файла по указанному пути.
// Выполняет валидацию и построение индекса репозиториев.
// Возвращает загруженную и валидированную конфигурацию или ошибку.
func Load(path string) (*Config, error) {
	slog.Info("loading configuration", "path", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	slog.Debug("configuration file parsed", "size_bytes", len(data))

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	cfg.buildIndex()
	slog.Info("configuration validated and indexed", "repositories", len(cfg.RepoIndex))
	return &cfg, nil
}

// Validate проверяет корректность конфигурации и устанавливает значения по умолчанию
// для необязательных полей. Возвращает ошибку, если конфигурация некорректна.
func (c *Config) Validate() error {
	if c.Server.ListenAddr == "" {
		c.Server.ListenAddr = ":8080"
	}
	if c.Server.WorkerPoolSize <= 0 {
		c.Server.WorkerPoolSize = 4
	}
	if c.Server.QueueSize <= 0 {
		c.Server.QueueSize = 100
	}

	if c.Jenkins.BaseURL == "" {
		return fmt.Errorf("jenkins.base_url must be provided")
	}
	if c.Jenkins.PollInterval <= 0 {
		c.Jenkins.PollInterval = 15 * time.Second
	}
	if c.Jenkins.Timeout <= 0 {
		c.Jenkins.Timeout = 5 * time.Minute
	}

	if c.Gitea.BaseURL == "" {
		return fmt.Errorf("gitea.base_url must be provided")
	}
	if c.Gitea.Token == "" {
		return fmt.Errorf("gitea.token must be provided")
	}

	for idx := range c.Repositories {
		if c.Repositories[idx].Name == "" {
			return fmt.Errorf("repository rule at index %d missing name", idx)
		}
		if c.Repositories[idx].JobPattern == "" {
			return fmt.Errorf("repository %s must define a job pattern", c.Repositories[idx].Name)
		}
		if c.Repositories[idx].PollInterval <= 0 {
			c.Repositories[idx].PollInterval = c.Jenkins.PollInterval
		}
		if c.Repositories[idx].Timeout <= 0 {
			c.Repositories[idx].Timeout = c.Jenkins.Timeout
		}
		if c.Repositories[idx].SuccessCommentTemplate == "" {
			c.Repositories[idx].SuccessCommentTemplate = "✅ Jenkins job {{ .JobName }} detected: {{ .JobURL }}"
		}
		if c.Repositories[idx].FailureCommentTemplate == "" {
			c.Repositories[idx].FailureCommentTemplate = "⚠️ Jenkins job not detected for PR {{ .Number }} within timeout ({{ .Timeout }})."
		}
	}

	if err := c.validateChecks(); err != nil {
		return err
	}

	return nil
}

// validateChecks проверяет проверки каждого репозитория и компилирует выражения целевых веток.
// Поле checks может отсутствовать.
func (c *Config) validateChecks() error {
	for repoIdx := range c.Repositories {
		repo := &c.Repositories[repoIdx]
		if err := validateRepositoryChecks(repo.Name, repo.Checks); err != nil {
			return err
		}
	}
	return nil
}

// validateRepositoryChecks проверяет список проверок одного репозитория.
func validateRepositoryChecks(repoName string, rules []CheckRule) error {
	seenName := make(map[string]struct{}, len(rules))
	seenContext := make(map[string]struct{}, len(rules))

	for idx := range rules {
		ch := &rules[idx]
		if ch.Name == "" {
			return fmt.Errorf("repository %s: check at index %d missing name", repoName, idx)
		}
		if _, ok := seenName[ch.Name]; ok {
			return fmt.Errorf("repository %s: duplicate check name %q", repoName, ch.Name)
		}
		seenName[ch.Name] = struct{}{}

		if ch.Type == "" {
			return fmt.Errorf("repository %s: check %q missing type", repoName, ch.Name)
		}
		if len(ch.TargetBranches) == 0 {
			return fmt.Errorf("repository %s: check %q must define target_branches", repoName, ch.Name)
		}

		ch.targetBranches = make([]*regexp.Regexp, 0, len(ch.TargetBranches))
		for patternIdx, pattern := range ch.TargetBranches {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("repository %s: check %q: target_branches[%d] %q: %w", repoName, ch.Name, patternIdx, pattern, err)
			}
			ch.targetBranches = append(ch.targetBranches, re)
		}

		if ch.Context == "" {
			ch.Context = "checks/" + ch.Name
		}
		if len(ch.Context) > maxCommitStatusContextLen {
			return fmt.Errorf("repository %s: check %q: context exceeds %d bytes", repoName, ch.Name, maxCommitStatusContextLen)
		}
		if _, ok := seenContext[ch.Context]; ok {
			return fmt.Errorf("repository %s: duplicate check context %q", repoName, ch.Context)
		}
		seenContext[ch.Context] = struct{}{}

		if err := ch.rejectForeignSpecs(); err != nil {
			return fmt.Errorf("repository %s: %w", repoName, err)
		}

		switch ch.Type {
		case CheckTypeFileBlacklist:
			if err := validateFileBlacklist(ch); err != nil {
				return fmt.Errorf("repository %s: %w", repoName, err)
			}
		default:
			return fmt.Errorf("repository %s: check %q: unknown type %q", repoName, ch.Name, ch.Type)
		}
	}

	return nil
}

// rejectForeignSpecs возвращает ошибку, если у правила заполнены настройки чужого типа.
func (ch *CheckRule) rejectForeignSpecs() error {
	if ch.Type != CheckTypeFileBlacklist && ch.FileBlacklist != nil {
		return fmt.Errorf("check %q: file_blacklist settings do not belong to type %q", ch.Name, ch.Type)
	}
	return nil
}

// validateFileBlacklist проверяет настройки чёрного списка файлов и подставляет описания статуса.
func validateFileBlacklist(ch *CheckRule) error {
	if ch.FileBlacklist == nil {
		return fmt.Errorf("check %q: file_blacklist settings are required", ch.Name)
	}
	if len(ch.FileBlacklist.Patterns) == 0 {
		return fmt.Errorf("check %q: file_blacklist.patterns must not be empty", ch.Name)
	}
	for idx, pattern := range ch.FileBlacklist.Patterns {
		if err := validateFilePattern(pattern); err != nil {
			return fmt.Errorf("check %q: file_blacklist.patterns[%d]: %w", ch.Name, idx, err)
		}
	}
	if ch.SuccessDescription == "" {
		ch.SuccessDescription = "Файлы из чёрного списка не изменены"
	}
	if ch.FailureDescription == "" {
		ch.FailureDescription = "Изменены файлы из чёрного списка"
	}
	return nil
}

// validateFilePattern проверяет glob-шаблон пути относительно корня репозитория.
func validateFilePattern(pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("empty pattern")
	}
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	if strings.HasPrefix(pattern, "/") {
		return fmt.Errorf("pattern %q must be relative", pattern)
	}
	for _, part := range strings.Split(pattern, "/") {
		if part == "" {
			return fmt.Errorf("pattern %q contains an empty path segment", pattern)
		}
	}
	return nil
}

// MatchesTargetBranch сообщает, подходит ли правило к целевой ветке pull request.
// Сравнение выполняется скомпилированными выражениями из target_branches.
func (r CheckRule) MatchesTargetBranch(branch string) bool {
	for _, re := range r.targetBranches {
		if re.MatchString(branch) {
			return true
		}
	}
	return false
}

// buildIndex строит индекс репозиториев для быстрого поиска правил по полному имени репозитория.
func (c *Config) buildIndex() {
	c.RepoIndex = make(map[string]RepoID, len(c.Repositories))
	for _, repo := range c.Repositories {
		c.RepoIndex[repo.Name] = RepoID{Rule: repo}
	}
}

// GetRepositoryRule возвращает правила обработки для репозитория с указанным полным именем.
// Возвращает правила и флаг наличия репозитория в конфигурации.
func (c *Config) GetRepositoryRule(fullName string) (RepositoryRule, bool) {
	if c.RepoIndex == nil {
		c.buildIndex()
	}
	repo, ok := c.RepoIndex[fullName]
	return repo.Rule, ok
}

// NewHTTPClient создает новый HTTP клиент с настройками TLS.
// Если insecureSkipVerify равен true, отключает проверку SSL-сертификатов.
func NewHTTPClient(insecureSkipVerify bool, timeout time.Duration) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify,
		},
	}

	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}
