package config

import "testing"

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig("nonexistent.yml")
	if err != nil {
		t.Errorf("LoadConfig failed: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Commit: CommitConfig{
			PublicMessage: "auto",
			Squash:        true,
		},
	}

	if !cfg.Commit.Squash {
		t.Error("expected squash to be true by default")
	}

	if cfg.Commit.PublicMessage != "auto" {
		t.Errorf("expected public_message 'auto', got %s", cfg.Commit.PublicMessage)
	}
}
