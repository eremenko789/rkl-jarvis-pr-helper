package checks

import "testing"

func TestMatchFile(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{pattern: "go.sum", path: "go.sum", want: true},
		{pattern: "go.sum", path: "vendor/go.sum", want: false},
		{pattern: "deploy/production.yaml", path: "deploy/production.yaml", want: true},
		{pattern: "deploy/production.yaml", path: "deploy/production.yml", want: false},
		{pattern: "*.yaml", path: "config.yaml", want: true},
		{pattern: "*.yaml", path: "dir/config.yaml", want: false},
		{pattern: "config/*.yaml", path: "config/app.yaml", want: true},
		{pattern: "config/*.yaml", path: "config/nested/app.yaml", want: false},
		{pattern: "vendor/**", path: "vendor/a", want: true},
		{pattern: "vendor/**", path: "vendor/a/b/c.go", want: true},
		{pattern: "vendor/**", path: "vendor", want: true},
		{pattern: "vendor/**", path: "other/vendor/a", want: false},
		{pattern: "**/.env", path: ".env", want: true},
		{pattern: "**/.env", path: "a/b/.env", want: true},
		{pattern: "**/.env", path: "a/b/env", want: false},
		{pattern: "**/*.pem", path: "certs/a/server.pem", want: true},
		{pattern: "**/*.pem", path: "server.pem", want: true},
		{pattern: "**/*.pem", path: "server.pem.bak", want: false},
		{pattern: "a/**/b", path: "a/b", want: true},
		{pattern: "a/**/b", path: "a/x/y/b", want: true},
		{pattern: "a/**/b", path: "a/x/c", want: false},
		{pattern: "a?c", path: "abc", want: true},
		{pattern: "a?c", path: "aбc", want: true},
		{pattern: "a?c", path: "abbc", want: false},
		{pattern: "dir/*", path: "dir/file", want: true},
		{pattern: "dir/*", path: "dir/sub/file", want: false},
		{pattern: "Foo", path: "foo", want: false},
		{pattern: "/go.sum", path: "go.sum", want: true},
		{pattern: `secrets\key`, path: "secrets/key", want: true},
	}

	for _, tt := range tests {
		got := MatchFile(tt.pattern, tt.path)
		if got != tt.want {
			t.Errorf("MatchFile(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}
