package main

import (
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
	if len(os.Args) < 2 {
		fmt.Println("requires magnet link")
		return
	}
	uri := os.Args[1]

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

	c, err := newClient(dataDir)
	if err != nil {
		log.Fatalf("failed to initialize torrent client: %v", err)
	}
	defer c.Close()

	t, err := addMagnet(c, uri)
	if err != nil {
		log.Fatalf("failed to add torrent: %v", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		c.Close()
		_ = os.RemoveAll(filepath.Join(dataDir, t.Info().Name))
		os.Exit(0)
	}()

	f, err := selectFile(t)
	if err != nil {
		log.Fatalf("failed to select file: %v", err)
	}

	fmt.Printf("playing: %s\n", f.DisplayPath())

	reader := bufferFile(t, f)

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

	_ = os.RemoveAll(filepath.Join(dataDir, t.Info().Name))
}
