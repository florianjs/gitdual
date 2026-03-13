package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type CommitHash string
type BranchName string

type RemoteNotFoundError struct {
	RemoteName string
}

func (e *RemoteNotFoundError) Error() string {
	return fmt.Sprintf("remote '%s' not found", e.RemoteName)
}

type GitOperator interface {
	Push(remote string, opts *PushOptions) error
	Pull(remote string, opts *PullOptions) error
	CherryPick(hash CommitHash) error
	CommitAll(message string) (CommitHash, error)
	ListTrackedFiles() ([]string, error)
	GetCurrentBranch() (BranchName, error)
	HasChanges() (bool, error)
	// IsFileModified reports whether path has uncommitted local changes
	// (staged or unstaged). Returns false for untracked files.
	IsFileModified(path string) (bool, error)
}

type PushOptions struct {
	DryRun bool
	Force  bool
}

type PullOptions struct {
	DryRun bool
}

type Repository struct {
	path string
	repo *git.Repository
}

func NewRepository(path string) (*Repository, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}
	return &Repository{path: path, repo: repo}, nil
}

func (r *Repository) Push(remoteName string, opts *PushOptions) error {
	if opts == nil {
		opts = &PushOptions{}
	}

	if opts.DryRun {
		fmt.Printf("  [dry-run] Would push to remote '%s'\n", remoteName)
		return nil
	}

	_, err := r.repo.Remote(remoteName)
	if err != nil {
		return &RemoteNotFoundError{RemoteName: remoteName}
	}

	branch, err := r.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	args := []string{"push", remoteName, string(branch)}
	if opts.Force {
		args = append(args, "--force")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = r.path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push to %s: %w", remoteName, err)
	}

	return nil
}

func (r *Repository) Pull(remoteName string, opts *PullOptions) error {
	if opts == nil {
		opts = &PullOptions{}
	}

	if opts.DryRun {
		fmt.Printf("  [dry-run] Would pull from remote '%s'\n", remoteName)
		return nil
	}

	_, err := r.repo.Remote(remoteName)
	if err != nil {
		return &RemoteNotFoundError{RemoteName: remoteName}
	}

	cmd := exec.Command("git", "fetch", remoteName)
	cmd.Dir = r.path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch from %s: %w", remoteName, err)
	}

	return nil
}

func (r *Repository) CherryPick(hash CommitHash) error {
	return fmt.Errorf("cherry-pick not yet implemented")
}

func (r *Repository) CommitAll(message string) (CommitHash, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	if status.IsClean() {
		return "", nil
	}

	for file := range status {
		_, err := wt.Add(file)
		if err != nil {
			return "", fmt.Errorf("failed to add %s: %w", file, err)
		}
	}

	commit, err := wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  os.Getenv("GIT_AUTHOR_NAME"),
			Email: os.Getenv("GIT_AUTHOR_EMAIL"),
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}

	return CommitHash(commit.String()), nil
}

func (r *Repository) ListTrackedFiles() ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = r.path
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list tracked files: %w", err)
	}
	var files []string
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

func (r *Repository) GetCurrentBranch() (BranchName, error) {
	ref, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	name := ref.Name().Short()
	return BranchName(name), nil
}

func (r *Repository) HasChanges() (bool, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("failed to get status: %w", err)
	}

	return !status.IsClean(), nil
}


func (r *Repository) IsFileModified(path string) (bool, error) {
	out, err := exec.Command("git", "-C", r.path, "status", "--porcelain", "--", path).Output()
	if err != nil {
		return false, fmt.Errorf("failed to check status of %s: %w", path, err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func FindRepoRoot(startPath string) (string, error) {
	dir, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository")
		}
		dir = parent
	}
}
