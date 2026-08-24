package main

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
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

func cleanOldCache(dataDir string, maxDays int) error {
	if maxDays <= 0 {
		return nil
	}

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

		if time.Since(stat.ModTime()) > time.Duration(maxDays)*24*time.Hour {
			_ = os.RemoveAll(entryPath)
		}
	}

	return nil
}

func torrentPaths(dataDir, name string) []string {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil
	}

	var matches []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), name) {
			matches = append(matches, filepath.Join(dataDir, e.Name()))
		}
	}

	return matches
}

func serveToPort(name string, reader io.ReadSeeker) (net.Listener, error) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, name, time.Time{}, reader)
	})

	go func() {
		_ = http.Serve(ln, mux)
	}()

	return ln, nil
}
