package exclude

import (
	"path/filepath"
	"strings"
)

const DefaultPrivateSuffix = "-p"

type Matcher struct {
	PrivateSuffix string
	Files         []string
	Folders       []string
}

func NewMatcher(privateSuffix string) *Matcher {
	if privateSuffix == "" {
		privateSuffix = DefaultPrivateSuffix
	}
	return &Matcher{PrivateSuffix: privateSuffix}
}

func NewMatcherFromConfig(privateSuffix string, files, folders []string) *Matcher {
	m := NewMatcher(privateSuffix)
	m.Files = files
	m.Folders = folders
	return m
}

func (m *Matcher) ShouldExclude(path string) bool {
	normalized := filepath.ToSlash(path)

	// Check explicit file list (glob patterns supported)
	base := filepath.Base(normalized)
	for _, pattern := range m.Files {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, normalized); matched {
			return true
		}
	}

	// Check explicit folder list
	for _, folder := range m.Folders {
		folderNorm := filepath.ToSlash(strings.TrimSuffix(folder, "/"))
		if strings.HasPrefix(normalized, folderNorm+"/") || normalized == folderNorm {
			return true
		}
	}

	// Check private suffix on any path component
	return HasPrivateSuffix(normalized, m.PrivateSuffix)
}

func HasPrivateSuffix(path, suffix string) bool {
	path = filepath.ToSlash(path)
	parts := strings.Split(path, "/")

	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			continue
		}
		if nameHasPrivateSuffix(part, suffix) {
			return true
		}
	}

	return false
}

func nameHasPrivateSuffix(name, suffix string) bool {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	return strings.HasSuffix(base, suffix)
}

func Matches(pattern, path string) bool {
	return HasPrivateSuffix(path, DefaultPrivateSuffix)
}

func FilterFiles(files []string, suffix string) []string {
	matcher := NewMatcher(suffix)
	result := make([]string, 0, len(files))
	for _, file := range files {
		if !matcher.ShouldExclude(file) {
			result = append(result, file)
		}
	}
	return result
}

func FindPrivateFiles(files []string, suffix string) []string {
	matcher := NewMatcher(suffix)
	result := make([]string, 0)
	for _, file := range files {
		if matcher.ShouldExclude(file) {
			result = append(result, file)
		}
	}
	return result
}
