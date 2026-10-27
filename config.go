package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type config struct {
	ReadAhead     int64 `json:"read_ahead"`
	Cache         int   `json:"cache"`
	NoCache       bool  `json:"no_cache"`
	BufferPercent int   `json:"buffer_percent"`
}

func resolveMPV() (string, error) {
	cmds := []string{
		"mpv",
		"/opt/homebrew/bin/mpv",
		"/usr/local/bin/mpv",
	}

	for _, cmd := range cmds {
		if path, err := exec.LookPath(cmd); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("mpv not found: please install mpv (https://mpv.io)")
}

func configPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, "tplay", "config.json"), nil
}

func defaultConfig() *config {
	return &config{
		ReadAhead:     62,
		Cache:         0,
		NoCache:       false,
		BufferPercent: 1,
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
