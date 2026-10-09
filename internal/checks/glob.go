package checks

import "strings"

// MatchFile сообщает, совпадает ли путь файла с glob-шаблоном.
// Разделитель всегда «/». Символы * и ? не переходят через «/».
// Сегмент ** совпадает с любым числом сегментов пути, включая ноль.
func MatchFile(pattern, filePath string) bool {
	pattern = normalizePath(pattern)
	filePath = normalizePath(filePath)
	return matchSegments(splitPath(pattern), splitPath(filePath))
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "/")
	return p
}

func splitPath(p string) []string {
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func matchSegments(pattern, path []string) bool {
	for {
		if len(pattern) == 0 {
			return len(path) == 0
		}
		if pattern[0] == "**" {
			if matchSegments(pattern[1:], path) {
				return true
			}
			if len(path) == 0 {
				return false
			}
			path = path[1:]
			continue
		}
		if len(path) == 0 || !matchSegment(pattern[0], path[0]) {
			return false
		}
		pattern = pattern[1:]
		path = path[1:]
	}
}

func matchSegment(pattern, segment string) bool {
	return matchRunes([]rune(pattern), []rune(segment))
}

func matchRunes(pattern, segment []rune) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			for i := 0; i <= len(segment); i++ {
				if matchRunes(pattern[1:], segment[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(segment) == 0 {
				return false
			}
			pattern = pattern[1:]
			segment = segment[1:]
		default:
			if len(segment) == 0 || segment[0] != pattern[0] {
				return false
			}
			pattern = pattern[1:]
			segment = segment[1:]
		}
	}
	return len(segment) == 0
}
