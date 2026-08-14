package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
)

const (
	maxDays = 5 * 24 * time.Hour
)

func newClient(dataDir string) (*torrent.Client, error) {
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = dataDir
	cfg.Slogger = slog.New(slog.NewTextHandler(io.Discard, nil))

	c, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return c, err
}

func cleanOldCache(dataDir string) error {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".torrent") {
			continue
		}

		entryPath := filepath.Join(dataDir, entry.Name())
		stat, err := os.Stat(entryPath)
		if err != nil {
			continue
		}

		if time.Since(stat.ModTime()) > maxDays {
			_ = os.RemoveAll(entryPath)
		}
	}

	return nil
}

func torrentPaths(dataDir, name string) []string {
	torrentPath := filepath.Join(dataDir, name)
	matches, _ := filepath.Glob(torrentPath + "*")

	return matches
}
