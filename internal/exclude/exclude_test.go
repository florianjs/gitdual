package exclude

import "testing"

func TestHasPrivateSuffix(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		suffix   string
		expected bool
	}{
		{
			name:     "folder with suffix",
			path:     "docs-p/readme.md",
			suffix:   "-p",
			expected: true,
		},
		{
			name:     "file with suffix",
			path:     "src/config-p.yaml",
			suffix:   "-p",
			expected: true,
		},
		{
			name:     "nested folder with suffix",
			path:     "src/components-p/button.tsx",
			suffix:   "-p",
			expected: true,
		},
		{
			name:     "no suffix",
			path:     "src/config.yaml",
			suffix:   "-p",
			expected: false,
		},
		{
			name:     "suffix in middle of name",
			path:     "src/config-prod.yaml",
			suffix:   "-p",
			expected: false,
		},
		{
			name:     "custom suffix",
			path:     "docs-private/readme.md",
			suffix:   "-private",
			expected: true,
		},
		{
			name:     "empty path",
			path:     "",
			suffix:   "-p",
			expected: false,
		},
		{
			name:     "dot path",
			path:     "./file.txt",
			suffix:   "-p",
			expected: false,
		},
		{
			name:     "backslash on unix (not converted)",
			path:     "docs-p\\readme.md",
			suffix:   "-p",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPrivateSuffix(tt.path, tt.suffix)
			if result != tt.expected {
				t.Errorf("HasPrivateSuffix(%q, %q) = %v, want %v", tt.path, tt.suffix, result, tt.expected)
			}
		})
	}
}

func TestMatcherShouldExclude(t *testing.T) {
	matcher := NewMatcher("-p")

	tests := []struct {
		path     string
		expected bool
	}{
		{"docs-p/file.txt", true},
		{"src/file-p.txt", true},
		{"normal/file.txt", false},
		{"src/components-p/ui/button.tsx", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := matcher.ShouldExclude(tt.path)
			if result != tt.expected {
				t.Errorf("ShouldExclude(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFilterFiles(t *testing.T) {
	files := []string{
		"src/main.go",
		"docs-p/readme.md",
		"config-p.yaml",
		"public/index.html",
	}

	filtered := FilterFiles(files, "-p")

	expected := []string{
		"src/main.go",
		"public/index.html",
	}

	if len(filtered) != len(expected) {
		t.Errorf("expected %d files, got %d", len(expected), len(filtered))
	}

	for i, f := range filtered {
		if f != expected[i] {
			t.Errorf("filtered[%d] = %q, want %q", i, f, expected[i])
		}
	}
}

func TestFindPrivateFiles(t *testing.T) {
	files := []string{
		"src/main.go",
		"docs-p/readme.md",
		"config-p.yaml",
		"public/index.html",
		"notes-p/todo.md",
	}

	private := FindPrivateFiles(files, "-p")

	expected := []string{
		"docs-p/readme.md",
		"config-p.yaml",
		"notes-p/todo.md",
	}

	if len(private) != len(expected) {
		t.Errorf("expected %d files, got %d", len(expected), len(private))
	}

	for i, f := range private {
		if f != expected[i] {
			t.Errorf("private[%d] = %q, want %q", i, f, expected[i])
		}
	}
}

func TestMatcherFromConfig(t *testing.T) {
	matcher := NewMatcherFromConfig("-p",
		[]string{"opencode.json", "AGENTS.md", "*.internal.md"},
		[]string{"docs/", "notes"},
	)

	tests := []struct {
		path     string
		expected bool
	}{
		{"opencode.json", true},
		{"AGENTS.md", true},
		{"secret.internal.md", true},
		{"docs/readme.md", true},
		{"notes/todo.txt", true},
		{"src/main.go", false},
		{"README.md", false},
		{"docs-p/file.txt", true}, // still caught by suffix
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := matcher.ShouldExclude(tt.path)
			if result != tt.expected {
				t.Errorf("ShouldExclude(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestNewMatcherDefaultSuffix(t *testing.T) {
	matcher := NewMatcher("")
	if matcher.PrivateSuffix != DefaultPrivateSuffix {
		t.Errorf("expected default suffix %q, got %q", DefaultPrivateSuffix, matcher.PrivateSuffix)
	}
}
