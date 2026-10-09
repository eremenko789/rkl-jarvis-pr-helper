// Package checks выполняет проверки pull request по правилам из конфигурации.
package checks

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/example/gitea-jenkins-webhook/internal/config"
)

const (
	// StateSuccess — проверка пройдена.
	StateSuccess = "success"
	// StateFailure — проверка не пройдена.
	StateFailure = "failure"
	// StateError — проверку не удалось выполнить.
	StateError = "error"

	maxStatusDescriptionLen = 255
)

// FileChange — изменённый файл pull request.
type FileChange struct {
	Filename         string
	PreviousFilename string
}

// Outcome — результат проверки, который публикуется как статус коммита.
type Outcome struct {
	State       string
	Description string
}

// Applicable возвращает правила, чьи target_branches совпадают с целевой веткой.
func Applicable(rules []config.CheckRule, baseBranch string) []config.CheckRule {
	matched := make([]config.CheckRule, 0, len(rules))
	for _, rule := range rules {
		if rule.MatchesTargetBranch(baseBranch) {
			matched = append(matched, rule)
		}
	}
	return matched
}

// Evaluate выполняет проверку указанного типа.
// Для file_blacklist успех означает, что ни один изменённый путь не входит в чёрный список.
func Evaluate(rule config.CheckRule, files []FileChange) (Outcome, error) {
	switch rule.Type {
	case config.CheckTypeFileBlacklist:
		return evaluateFileBlacklist(rule, files)
	default:
		return Outcome{}, fmt.Errorf("unsupported check type %q", rule.Type)
	}
}

func evaluateFileBlacklist(rule config.CheckRule, files []FileChange) (Outcome, error) {
	if rule.FileBlacklist == nil {
		return Outcome{}, fmt.Errorf("check %q: file_blacklist settings are missing", rule.Name)
	}

	matched := blacklistedPaths(rule.FileBlacklist.Patterns, files)
	if len(matched) == 0 {
		return Outcome{
			State:       StateSuccess,
			Description: truncateDescription(rule.SuccessDescription),
		}, nil
	}

	description := rule.FailureDescription + ": " + strings.Join(matched, ", ")
	return Outcome{
		State:       StateFailure,
		Description: truncateDescription(description),
	}, nil
}

func blacklistedPaths(patterns []string, files []FileChange) []string {
	var matched []string
	seen := make(map[string]struct{})
	add := func(path string) {
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		if pathMatches(patterns, path) {
			seen[path] = struct{}{}
			matched = append(matched, path)
		}
	}
	for _, file := range files {
		add(file.Filename)
		add(file.PreviousFilename)
	}
	return matched
}

func pathMatches(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if MatchFile(pattern, path) {
			return true
		}
	}
	return false
}

func truncateDescription(description string) string {
	if len(description) <= maxStatusDescriptionLen {
		return description
	}
	const ellipsis = "…"
	limit := maxStatusDescriptionLen - len(ellipsis)
	if limit < 1 {
		return ellipsis
	}
	cut := description[:limit]
	for !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut + ellipsis
}
