package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/florianjs/gitdual/internal/config"
	"github.com/florianjs/gitdual/internal/exclude"
	"github.com/florianjs/gitdual/internal/git"
)

type SyncManager struct {
	git          git.GitOperator
	mirror       git.PublicMirror
	repoPath     string
	cfg          *config.Config
	excludeMatch *exclude.Matcher
}

func NewSyncManager(g git.GitOperator, mirror git.PublicMirror, repoPath string, cfg *config.Config) *SyncManager {
	return &SyncManager{
		git:          g,
		mirror:       mirror,
		repoPath:     repoPath,
		cfg:          cfg,
		excludeMatch: exclude.NewMatcherFromConfig(cfg.Exclude.PrivateSuffix, cfg.Exclude.Files, cfg.Exclude.Folders),
	}
}

func (sm *SyncManager) handleRemoteError(remoteName string, err error) error {
	var remoteErr *git.RemoteNotFoundError
	if errors.As(err, &remoteErr) {
		return fmt.Errorf("remote '%s' not configured in git. Run: git remote add %s %s",
			remoteName, remoteName, sm.cfg.Remotes.Private)
	}
	return err
}

func (sm *SyncManager) PushPrivate(dryRun bool, message string) (int, error) {
	hasChanges, err := sm.git.HasChanges()
	if err != nil {
		return 0, fmt.Errorf("failed to check for changes: %w", err)
	}

	if !hasChanges {
		fmt.Println("  No changes to commit")
		return 0, nil
	}

	if message == "" {
		message = "Update"
	}

	fmt.Printf("  Committing changes: %s\n", message)
	hash, err := sm.git.CommitAll(message)
	if err != nil {
		return 0, fmt.Errorf("failed to commit: %w", err)
	}

	if hash == "" {
		fmt.Println("  No changes committed")
		return 0, nil
	}

	fmt.Printf("  Pushing to private remote...\n")
	if err := sm.git.Push("private", &git.PushOptions{DryRun: dryRun}); err != nil {
		return 0, sm.handleRemoteError("private", fmt.Errorf("failed to push to private: %w", err))
	}

	return 1, nil
}

func (sm *SyncManager) PushPublic(dryRun bool, message string, force bool) (int, error) {
	if message == "" {
		message = sm.cfg.Commit.PublicMessage
		if message == "auto" || message == "" {
			message = "Update"
		}
	}

	workDirPath := sm.cfg.PublicWorkDirPath(sm.repoPath)

	if dryRun {
		fmt.Printf("  [dry-run] Would sync filtered files to %s and push to %s\n",
			workDirPath, sm.cfg.Remotes.Public)
		return 1, nil
	}

	fmt.Printf("  Initializing public mirror at %s...\n", workDirPath)
	if err := sm.mirror.EnsureInitialized(string(sm.cfg.Remotes.Public)); err != nil {
		return 0, fmt.Errorf("failed to initialize public mirror: %w", err)
	}

	fmt.Printf("  Syncing filtered files (excluding *%s files/folders)...\n", sm.cfg.Exclude.PrivateSuffix)
	if err := sm.mirror.SyncFiles(sm.repoPath, sm.excludeMatch.ShouldExclude); err != nil {
		return 0, fmt.Errorf("failed to sync files: %w", err)
	}

	fmt.Printf("  Committing and pushing from public mirror...\n")
	hash, err := sm.mirror.CommitAndPush(message, force)
	if err != nil {
		return 0, fmt.Errorf("failed to push to public: %w", err)
	}
	if hash == "" {
		fmt.Println("  Nothing to push to public")
		return 0, nil
	}

	return 1, nil
}

func (sm *SyncManager) Push(dryRun bool) (private, public int, err error) {
	return sm.PushWithMessage(dryRun, "")
}

func (sm *SyncManager) PushWithMessage(dryRun bool, message string) (private, public int, err error) {
	if sm.cfg.Remotes.Private != "" {
		p, err := sm.PushPrivate(dryRun, message)
		if err != nil {
			return 0, 0, fmt.Errorf("private push failed: %w", err)
		}
		private = p
	}

	if sm.cfg.Remotes.Public != "" {
		p, err := sm.PushPublic(dryRun, message, false)
		if err != nil {
			return private, 0, fmt.Errorf("public push failed: %w", err)
		}
		public = p
	}

	return private, public, nil
}

func (sm *SyncManager) Pull(dryRun bool) error {
	if sm.cfg.Remotes.Private != "" {
		fmt.Println("  Pulling from private remote...")
		if err := sm.git.Pull("private", &git.PullOptions{DryRun: dryRun}); err != nil {
			return sm.handleRemoteError("private", fmt.Errorf("failed to pull from private: %w", err))
		}
	}

	if _, err := sm.PullPublic(dryRun); err != nil {
		return err
	}

	return nil
}

// PullPublic fetches changes pushed to the public remote by collaborators and
// copies them back into the local working tree. Only public (non-excluded)
// files are touched; private files are never overwritten.
//
// Returns the list of conflicting file paths (locally modified files that also
// changed in the public remote). When conflicts are detected the local files
// are NOT overwritten — the caller must resolve them manually.
func (sm *SyncManager) PullPublic(dryRun bool) (conflicts []string, err error) {
	if sm.cfg.Remotes.Public == "" {
		return nil, nil
	}

	mirrorPath := sm.cfg.PublicWorkDirPath(sm.repoPath)

	if dryRun {
		fmt.Printf("  [dry-run] Would pull from public remote %s\n", sm.cfg.Remotes.Public)
		return nil, nil
	}

	fmt.Println("  Initializing public mirror...")
	if err := sm.mirror.EnsureInitialized(string(sm.cfg.Remotes.Public)); err != nil {
		return nil, fmt.Errorf("failed to initialize public mirror: %w", err)
	}

	fmt.Println("  Fetching changes from public remote...")
	changedFiles, err := sm.mirror.Pull()
	if err != nil {
		return nil, fmt.Errorf("failed to pull public remote: %w", err)
	}

	if len(changedFiles) == 0 {
		fmt.Println("  Public remote is already up to date")
		return nil, nil
	}

	fmt.Printf("  %d file(s) changed in public remote\n", len(changedFiles))

	// Detect conflicts: locally modified files that also changed upstream
	for _, f := range changedFiles {
		modified, err := sm.git.IsFileModified(f)
		if err != nil {
			return nil, fmt.Errorf("failed to check local status of %s: %w", f, err)
		}
		if modified {
			conflicts = append(conflicts, f)
		}
	}
	if len(conflicts) > 0 {
		return conflicts, fmt.Errorf("%d conflict(s) — resolve manually before pulling: %v", len(conflicts), conflicts)
	}

	// Copy changed files from mirror into local working tree
	for _, relPath := range changedFiles {
		src := filepath.Join(mirrorPath, filepath.FromSlash(relPath))
		dst := filepath.Join(sm.repoPath, filepath.FromSlash(relPath))

		if _, statErr := os.Stat(src); os.IsNotExist(statErr) {
			// File was deleted in the public remote
			if removeErr := os.Remove(dst); removeErr != nil && !os.IsNotExist(removeErr) {
				return nil, fmt.Errorf("failed to remove %s: %w", relPath, removeErr)
			}
			fmt.Printf("    deleted: %s\n", relPath)
		} else {
			if err := pullCopyFile(src, dst); err != nil {
				return nil, fmt.Errorf("failed to copy %s: %w", relPath, err)
			}
			fmt.Printf("    updated: %s\n", relPath)
		}
	}

	return nil, nil
}

func pullCopyFile(src, dst string) error {
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
