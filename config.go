package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type config struct {
	ReadAhead      int64 `json:"read_ahead"`
	Cache          int   `json:"cache"`
	NoCache        bool  `json:"no_cache"`
	BufferPercent  int   `json:"buffer_percent"`
	ResumePlayback bool  `json:"resume_playback"`
}

func configDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "tplay"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir not found: %w", err)
	}

	return filepath.Join(home, ".config", "tplay"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), nil
}

func defaultConfig() *config {
	return &config{
		ReadAhead:      62,
		Cache:          0,
		NoCache:        false,
		BufferPercent:  1,
		ResumePlayback: false,
	}
}

func loadConfig() *config {
	path, err := configPath()
	if err != nil {
		return defaultConfig()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return defaultConfig()
	}

	cfg := defaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return defaultConfig()
	}

	if cfg.BufferPercent <= 0 {
		cfg.BufferPercent = 1
	}

	return cfg
}

func saveConfig(cfg *config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
