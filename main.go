package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
)

func main() {
	noCache := flag.Bool("no-cache", false, "delete download data after playing")
	flag.Parse()

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatalf("failed to get cache dir: %v", err)
	}
	dataDir := filepath.Join(cacheDir, "tplay")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}

	if err := cleanOldCache(dataDir); err != nil {
		log.Printf("error cleaning cache: %v", err)
	}

	c, err := newClient(dataDir)
	if err != nil {
		log.Fatalf("failed to initialize torrent client: %v", err)
	}
	defer c.Close()

	app := tea.NewProgram(newAppModel(c))
	result, err := app.Run()
	if err != nil {
		log.Fatalf("error running program")
	}
	am := result.(appModel)
	if am.file == nil || !am.done {
		return
	}

	torrentPaths := torrentPaths(dataDir, am.torrent.Name())

	cleanup := func() {
		c.Close()
		for _, tp := range torrentPaths {
			_ = os.RemoveAll(tp)
		}
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cleanup()
		os.Exit(0)
	}()

	reader := am.file.NewReader()
	reader.SetReadahead(64 << 20)
	if reader == nil {
		cleanup()
		return
	}

	now := time.Now()
	for _, tp := range torrentPaths {
		if err := os.Chtimes(tp, now, now); err != nil {
			log.Printf("failed to change mtime: %v", err)
		}
	}

	fmt.Println(selectedStyle.Render("playing: " + am.file.DisplayPath()))

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, am.file.DisplayPath(), time.Time{}, reader)
	})

	go func() {
		if err := http.Serve(ln, nil); err != nil {
			log.Fatalf("error serving file: %v", err)
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port

	cmd := exec.Command("mpv", fmt.Sprintf("http://localhost:%d/stream", port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	if *noCache {
		for _, tp := range torrentPaths {
			_ = os.RemoveAll(tp)
		}
	}
}
