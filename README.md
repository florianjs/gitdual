# GitDual

A Go CLI tool for managing dual git remotes with automatic content filtering.

## How it works

GitDual maintains your **private repository** as the source of truth. When pushing to public, it syncs filtered files into a **separate local mirror directory** (e.g. `../myrepo-public/`) and pushes from there. The public remote is never attached to your private repo — no git history, no objects, no secrets can leak.

```
~/code/myrepo/          ← private repo (your working directory)
~/code/myrepo-public/   ← public mirror (managed by gitdual, never touch it)
```

## Features

- Push to private with full history, push to public with filtered clean commits
- Excluded files/folders never appear in the public mirror — not even in git history
- The public remote is isolated in its own directory, not registered as a git remote on the private repo
- Supports glob patterns, folder exclusion, and private suffix convention (e.g. `notes-p/`)
- Interactive TUI with keyboard navigation

## Installation

**One-line install:**
```bash
curl -fsSL https://raw.githubusercontent.com/florianjs/gitdual/main/install.sh | bash
```

**From source:**
```bash
git clone https://github.com/florianjs/gitdual.git
cd gitdual
make install
```

**With Go:**
```bash
go install github.com/florianjs/gitdual/cmd/gitdual@latest
```

## Usage

```bash
gitdual                           # Launch interactive TUI
gitdual init                      # Initialize GitDual
gitdual status                    # Show status

# Pull commands
gitdual pull                      # Pull from both remotes
gitdual pull private              # Pull from private remote
gitdual pull public               # Apply collaborator changes from public remote locally
gitdual pull --dry-run            # Preview without applying

# Push commands
gitdual push                              # Push to both remotes
gitdual push public "feat: new feature"   # Sync filtered files and push to public
gitdual push private "fix: bug fix"       # Push to private (full history)
gitdual push all "release v1.0"           # Push to both
gitdual push public --dry-run             # Preview what would be pushed
gitdual push public --force               # Force push to public
```

## Configuration

Create `.gitdual.yml` in your repository root:

```yaml
version: 1

remotes:
  private: git@github.com:user/repo-private.git
  public: git@github.com:user/repo-public.git
  # public_work_dir: ../repo-public  # optional, defaults to ../reponame-public

exclude:
  folders:
    - .claude/
    - notes/
    - docs-internal/
  files:
    - "*.internal.md"
    - .env*
    - opencode.json
  private_suffix: "-p"   # any file/folder ending in -p is excluded (e.g. notes-p/)

commit:
  public_message: "auto"
```

### Tip: exclude `.gitdual.yml` itself

Add it to the `files` exclude list if you don't want your config visible publicly:

```yaml
exclude:
  files:
    - .gitdual.yml
```

## Development

```bash
make build      # Build binary
make test       # Run tests
make lint       # Lint code
```

## License

MIT
