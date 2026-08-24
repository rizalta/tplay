package main

import (
	"log"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
)

func main() {
	cfg := loadConfig()

	cfgPath, err := configPath()
	if err != nil {
		log.Printf("error getting config path: %v", err)
	}
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := saveConfig(cfg); err != nil {
			log.Printf("error saving the config: %v", err)
		}
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatalf("failed to get cache dir: %v", err)
	}
	dataDir := filepath.Join(cacheDir, "tplay")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}

	if err := cleanOldCache(dataDir, cfg.Cache); err != nil {
		log.Printf("error cleaning cache: %v", err)
	}

	c, err := newClient(dataDir)
	if err != nil {
		log.Fatalf("failed to initialize torrent client: %v", err)
	}
	defer c.Close()

	app := tea.NewProgram(newAppModel(c, cfg, dataDir))
	if _, err := app.Run(); err != nil {
		log.Fatalf("error running program: %v", err)
	}
}
