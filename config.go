package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type config struct {
	Player        string `json:"player"`
	ReadAhead     int64  `json:"read_ahead"`
	Cache         int    `json:"cache"`
	BufferPercent int    `json:"buffer_percent"`
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
		Player:        "mpv",
		ReadAhead:     62,
		Cache:         0,
		BufferPercent: 2,
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
