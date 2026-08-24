package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type player struct {
	name    string
	command []string
}

var knownPlayers []player = []player{
	{
		name:    "mpv",
		command: []string{"mpv"},
	},
	{
		name: "vlc",
		command: []string{
			"vlc",
			"/Applications/VLC.app/Contents/MacOS/VLC",
		},
	},
	{
		name: "iina",
		command: []string{
			"iina",
			"iina-cli",
			"/Applications/IINA.app/Contents/MacOS/iina",
			"/Applications/IINA.app/Contents/MacOS/iina-cli",
		},
	},
}

type config struct {
	Player        string `json:"player"`
	ReadAhead     int64  `json:"read_ahead"`
	Cache         int    `json:"cache"`
	NoCache       bool   `json:"no_cache"`
	BufferPercent int    `json:"buffer_percent"`
}

func resolvePlayer(name string) (string, error) {
	name = strings.ToLower(name)
	for _, p := range knownPlayers {
		if p.name == name {
			for _, cmd := range p.command {
				if path, err := exec.LookPath(cmd); err == nil {
					return path, nil
				}
			}

			return "", fmt.Errorf("player %q not found", name)
		}
	}
	return "", fmt.Errorf("unknown player %q", name)
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
