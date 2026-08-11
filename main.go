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
	"strings"
	"syscall"
	"time"
)

func main() {
	noCache := flag.Bool("no-cache", false, "delete download data after playing")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("requires magnet link")
		return
	}
	uri := args[0]

	if !strings.HasPrefix(uri, "magnet:?") {
		fmt.Println("invalid magnet link")
		return
	}

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

	t, err := addMagnet(c, uri)
	if err != nil {
		log.Fatalf("failed to add torrent: %v", err)
	}
	torrentPath := filepath.Join(dataDir, t.Info().Name)

	cleanup := func() {
		c.Close()
		_ = os.RemoveAll(torrentPath)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cleanup()
		os.Exit(0)
	}()

	f, err := selectFile(t)
	if err != nil {
		log.Fatalf("failed to select file: %v", err)
	}

	reader := bufferFile(t, f)
	if reader == nil {
		cleanup()
		return
	}

	now := time.Now()
	if err := os.Chtimes(torrentPath, now, now); err != nil {
		log.Printf("failed to change mtime: %v", err)
	}

	fmt.Println(selectedStyle.Render("playing: " + f.DisplayPath()))

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, f.DisplayPath(), time.Time{}, reader)
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
		_ = os.RemoveAll(torrentPath)
	}
}
