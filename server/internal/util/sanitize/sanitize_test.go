// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package sanitize

import (
	"strings"
	"testing"
)

func TestRemoveControlChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"normal text", "hello world", "hello world"},
		{"with null byte", "hello\x00world", "helloworld"},
		{"with tab", "hello\tworld", "helloworld"}, // tab is 0x09, removed as control char
		{"with newline", "hello\nworld", "helloworld"},
		{"with DEL", "hello\x7fworld", "helloworld"},
		{"control chars", "\x01\x02\x03hello\x1f", "hello"},
		{"unicode", "hello \u00e9 world", "hello \u00e9 world"},
		{"only spaces", "   ", ""},
		{"leading/trailing spaces", "  hello  ", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemoveControlChars(tt.input); got != tt.want {
				t.Errorf("RemoveControlChars(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRedisKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "my-key", "my-key"},
		{"with dots", "cache.user.123", "cache.user.123"},
		{"with slashes", "prefix/key/sub", "prefix/key/sub"},
		{"special chars", "key@#$%", "key____"},
		{"spaces", "my key", "my_key"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedisKey(tt.input); got != tt.want {
				t.Errorf("RedisKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRedisKey_Truncation(t *testing.T) {
	long := strings.Repeat("a", 600)
	got := RedisKey(long)
	if len(got) != 512 {
		t.Errorf("RedisKey(long) length = %d, want 512", len(got))
	}
}

func TestCommitMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"normal", "fix: resolve bug", "fix: resolve bug", false},
		{"with newlines", "title\n\nbody", "title\n\nbody", false},
		{"with tabs", "fix:\tstuff", "fix:\tstuff", false},
		{"null bytes", "fix\x00bug", "fixbug", false},
		{"control chars only", "\x01\x02\x03", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CommitMessage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("CommitMessage(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CommitMessage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGlobCharacters(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no wildcards", "hello", "hello"},
		{"asterisk", "hello*", `hello\*`},
		{"question mark", "hello?", `hello\?`},
		{"brackets", "[test]", `\[test\]`},
		{"braces", "{test}", `\{test\}`},
		{"all", "*?[]{}", `\*\?\[\]\{\}`},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GlobCharacters(tt.input); got != tt.want {
				t.Errorf("GlobCharacters(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestK8sName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercase", "my-name", "my-name"},
		{"uppercase", "MY-NAME", "my-name"},
		{"mixed case", "MyName", "myname"},
		{"spaces", "my name", "my-name"},
		{"special chars", "my@name#123", "my-name-123"},
		{"multiple hyphens", "my---name", "my-name"},
		{"leading hyphen", "-my-name", "my-name"},
		{"trailing hyphen", "my-name-", "my-name"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := K8sName(tt.input); got != tt.want {
				t.Errorf("K8sName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestK8sName_Truncation(t *testing.T) {
	long := strings.Repeat("a", 50)
	got := K8sName(long)
	if len(got) != 40 {
		t.Errorf("K8sName(long) length = %d, want 40", len(got))
	}
}

func TestPathParam(t *testing.T) {
	// PathParam is an alias for RemoveControlChars
	input := "hello\x00\x01world"
	got := PathParam(input)
	want := "helloworld"
	if got != want {
		t.Errorf("PathParam(%q) = %q, want %q", input, got, want)
	}
}

func TestFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "file.txt", "file.txt"},
		{"with spaces", "my file.txt", "my_file.txt"},
		{"path traversal", "../etc/passwd", "__etc_passwd"},
		{"special chars", "file<>:name.txt", "file___name.txt"},
		{"empty", "", "export"},
		{"only invalid", "@#$%", "____"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Filename(tt.input); got != tt.want {
				t.Errorf("Filename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFilename_Truncation(t *testing.T) {
	long := strings.Repeat("a", 300)
	got := Filename(long)
	if len(got) != 200 {
		t.Errorf("Filename(long) length = %d, want 200", len(got))
	}
}

func TestPathComponent(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"simple", "my-component", "my-component", false},
		{"with dots", "v1.2.3", "v1.2.3", false},
		{"path traversal", "../etc", "etc", false},
		{"forward slash", "a/b", "a-b", false},
		{"backslash", `a\b`, "a-b", false},
		{"empty", "", "", true},
		{"only invalid", "///", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PathComponent(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("PathComponent(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("PathComponent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
