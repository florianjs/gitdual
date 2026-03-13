package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type RemoteURL string
type BranchName string

type Config struct {
	Version int           `yaml:"version"`
	Remotes RemotesConfig `yaml:"remotes"`
	Exclude ExcludeConfig `yaml:"exclude"`
	Commit  CommitConfig  `yaml:"commit"`
}

type RemotesConfig struct {
	Private      RemoteURL `yaml:"private"`
	Public       RemoteURL `yaml:"public"`
	PublicWorkDir string   `yaml:"public_work_dir"`
}

type ExcludeConfig struct {
	Folders       []string `yaml:"folders"`
	Files         []string `yaml:"files"`
	Branches      []string `yaml:"branches"`
	PrivateSuffix string   `yaml:"private_suffix"`
}

type CommitConfig struct {
	PublicMessage string `yaml:"public_message"`
	Squash        bool   `yaml:"squash"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Version == 0 {
		cfg.Version = 1
	}

	if cfg.Commit.PublicMessage == "" {
		cfg.Commit.PublicMessage = "auto"
	}

	if cfg.Exclude.PrivateSuffix == "" {
		cfg.Exclude.PrivateSuffix = "-p"
	}

	return &cfg, nil
}

func FindConfig(startDir string) (string, error) {
	dir := startDir
	for {
		configPath := filepath.Join(dir, ".gitdual.yml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .gitdual.yml found")
		}
		dir = parent
	}
}

// PublicWorkDirPath returns the absolute path to the public mirror directory.
// If PublicWorkDir is set in config it is resolved relative to repoRoot.
// Otherwise it defaults to ../basename-public/ next to repoRoot.
func (c *Config) PublicWorkDirPath(repoRoot string) string {
	if c.Remotes.PublicWorkDir != "" {
		if filepath.IsAbs(c.Remotes.PublicWorkDir) {
			return c.Remotes.PublicWorkDir
		}
		return filepath.Join(repoRoot, c.Remotes.PublicWorkDir)
	}
	base := filepath.Base(repoRoot)
	return filepath.Join(filepath.Dir(repoRoot), base+"-public")
}

func (c *Config) Validate() error {
	if c.Remotes.Private == "" && c.Remotes.Public == "" {
		return fmt.Errorf("at least one remote must be configured")
	}
	return nil
}
