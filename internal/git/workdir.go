package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PublicMirror manages a separate local git repository used as the public mirror.
type PublicMirror interface {
	EnsureInitialized(publicURL string) error
	SyncFiles(privateRepoPath string, excludeFunc func(string) bool) error
	CommitAndPush(message string, force bool) (CommitHash, error)
	// Pull fetches the latest changes from origin and returns the list of
	// relative file paths that changed between the previous and new HEAD.
	// Returns (nil, nil) if already up to date or remote has no commits yet.
	Pull() ([]string, error)
	// Tag creates an annotated tag on the current HEAD and pushes it to origin.
	Tag(version, message string) error
	// Path returns the absolute path of the mirror directory on disk.
	Path() string
}

// WorkDir is a separate git repository on disk that acts as the public mirror.
// It is never registered as a remote on the private repository.
type WorkDir struct {
	path string
}

func NewWorkDir(path string) *WorkDir {
	return &WorkDir{path: path}
}

// EnsureInitialized creates and configures the work dir as a git repo if needed.
// Safe to call repeatedly — idempotent.
func (w *WorkDir) EnsureInitialized(publicURL string) error {
	if err := os.MkdirAll(w.path, 0755); err != nil {
		return fmt.Errorf("failed to create work dir: %w", err)
	}

	gitDir := filepath.Join(w.path, ".git")
	_, err := os.Stat(gitDir)
	needsInit := os.IsNotExist(err)

	if needsInit {
		cmd := exec.Command("git", "init")
		cmd.Dir = w.path
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to init repo: %s", out)
		}

		cmd = exec.Command("git", "remote", "add", "origin", publicURL)
		cmd.Dir = w.path
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to add remote: %s", out)
		}
	} else {
		// Ensure remote URL is up to date
		cmd := exec.Command("git", "remote", "set-url", "origin", publicURL)
		cmd.Dir = w.path
		cmd.Run() // ignore error — remote may not exist yet on very first run
	}

	return nil
}

// SyncFiles copies all tracked, non-excluded files from the private repo into
// the work dir, and removes any files in the work dir that are no longer in
// the filtered set.
func (w *WorkDir) SyncFiles(privateRepoPath string, excludeFunc func(string) bool) error {
	// Get list of tracked files in the private repo
	lsCmd := exec.Command("git", "ls-files")
	lsCmd.Dir = privateRepoPath
	out, err := lsCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list tracked files: %w", err)
	}

	// Build set of files to publish
	publicSet := make(map[string]bool)
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f == "" {
			continue
		}
		if excludeFunc == nil || !excludeFunc(f) {
			publicSet[f] = true
		}
	}

	// Remove files from work dir that are no longer in the public set
	if err := removeStaleFiles(w.path, publicSet); err != nil {
		return fmt.Errorf("failed to clean work dir: %w", err)
	}

	// Copy files from private repo to work dir
	for relPath := range publicSet {
		src := filepath.Join(privateRepoPath, filepath.FromSlash(relPath))
		dst := filepath.Join(w.path, filepath.FromSlash(relPath))

		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", relPath, err)
		}
	}

	return nil
}

// CommitAndPush stages all changes, commits, and pushes to origin main.
// Returns ("", nil) if there is nothing to commit.
func (w *WorkDir) CommitAndPush(message string, force bool) (CommitHash, error) {
	// Stage everything
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = w.path
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to stage changes: %s", out)
	}

	// Check if there is anything to commit
	statusOut, err := exec.Command("git", "-C", w.path, "status", "--porcelain").Output()
	if err != nil {
		return "", fmt.Errorf("failed to check status: %w", err)
	}
	if strings.TrimSpace(string(statusOut)) == "" {
		return "", nil
	}

	// Commit
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = w.path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}

	// Push
	pushArgs := []string{"push", "origin", "HEAD:main"}
	if force {
		pushArgs = append(pushArgs, "--force")
	}
	cmd = exec.Command("git", pushArgs...)
	cmd.Dir = w.path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to push: %w", err)
	}

	// Return commit hash
	hashOut, err := exec.Command("git", "-C", w.path, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}
	return CommitHash(strings.TrimSpace(string(hashOut))), nil
}

// Path returns the absolute path of the mirror directory on disk.
func (w *WorkDir) Path() string {
	return w.path
}

// Tag creates an annotated tag at the current HEAD and pushes it to origin.
func (w *WorkDir) Tag(version, message string) error {
	if message == "" {
		message = "Release " + version
	}
	cmd := exec.Command("git", "tag", "-a", version, "-m", message)
	cmd.Dir = w.path
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create tag %s: %s", version, out)
	}

	cmd = exec.Command("git", "push", "origin", version)
	cmd.Dir = w.path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push tag %s: %w", version, err)
	}
	return nil
}

// Pull fetches from origin and fast-forward merges into the local branch.
// Returns the relative paths of files that changed, or nil if already up to date.
func (w *WorkDir) Pull() ([]string, error) {
	// Fetch from origin
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = w.path
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to fetch public remote: %s", out)
	}

	// If origin/main doesn't exist yet, the remote is empty — nothing to do
	if _, err := exec.Command("git", "-C", w.path, "rev-parse", "--verify", "origin/main").Output(); err != nil {
		return nil, nil
	}

	// Record HEAD before merging (empty string if no local commits yet)
	oldHeadOut, err := exec.Command("git", "-C", w.path, "rev-parse", "--verify", "HEAD").Output()
	hasCommits := err == nil
	oldHead := strings.TrimSpace(string(oldHeadOut))

	if !hasCommits {
		// Fresh mirror: create local main branch tracking origin/main
		cmd = exec.Command("git", "checkout", "-b", "main", "origin/main")
		cmd.Dir = w.path
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to checkout public remote: %s", out)
		}
	} else {
		// Fast-forward only — never auto-merge diverged history
		cmd = exec.Command("git", "merge", "--ff-only", "origin/main")
		cmd.Dir = w.path
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to fast-forward from public remote: %s\nHint: public history may have diverged — use 'gitdual push public --force' to reset it", out)
		}
	}

	newHeadOut, err := exec.Command("git", "-C", w.path, "rev-parse", "HEAD").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}
	newHead := strings.TrimSpace(string(newHeadOut))

	if oldHead == newHead {
		return nil, nil // already up to date
	}

	// List files that changed between old and new HEAD
	var diffArgs []string
	if !hasCommits {
		// First checkout: list all files in the new HEAD
		diffArgs = []string{"-C", w.path, "diff-tree", "--no-commit-id", "-r", "--name-only", newHead}
	} else {
		diffArgs = []string{"-C", w.path, "diff", "--name-only", oldHead, newHead}
	}
	diffOut, err := exec.Command("git", diffArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list changed files: %w", err)
	}

	var files []string
	for _, f := range strings.Split(strings.TrimSpace(string(diffOut)), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// removeStaleFiles deletes files in workDir that are not in the keep set,
// then removes any empty directories left behind (excluding .git).
func removeStaleFiles(workDir string, keep map[string]bool) error {
	return filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(workDir, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)

		if relSlash == "." {
			return nil
		}
		if relSlash == ".git" || strings.HasPrefix(relSlash, ".git/") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !keep[relSlash] {
			return os.Remove(path)
		}
		return nil
	})
}

// copyFile copies src to dst, creating parent directories as needed.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}
