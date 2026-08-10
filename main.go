package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/anacrolix/torrent"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("requires magnet link")
		return
	}
	magnetURI := os.Args[1]

	if !strings.HasPrefix(magnetURI, "magnet:?") {
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

	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = dataDir
	cfg.Slogger = slog.New(slog.NewTextHandler(io.Discard, nil))
	c, err := torrent.NewClient(cfg)
	if err != nil {
		log.Fatalf("failed to initialize torrent client: %v", err)
	}
	defer c.Close()

	t, err := c.AddMagnet(magnetURI)
	if err != nil {
		log.Fatalf("failed to add torrent: %v", err)
	}

	<-t.GotInfo()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		c.Close()
		_ = os.RemoveAll(filepath.Join(dataDir, t.Info().Name))
		os.Exit(0)
	}()

	files := t.Files()

	sel := 0

	if len(files) != 1 {
		for i, f := range files {
			fmt.Printf("[%d] %s\n", i, f.DisplayPath())
		}

		fmt.Printf("enter your selection: ")
		if _, err := fmt.Scanf("%d", &sel); err != nil {
			log.Fatalf("error reading input: %v", err)
		}

		if sel < 0 || sel >= len(files) {
			log.Fatalf("invalid selector")
		}
	}

	f := files[sel]
	fmt.Printf("playing: %s\n", f.DisplayPath())

	f.SetPriority(torrent.PiecePriorityNormal)

	firstPiece := f.BeginPieceIndex()
	lastPiece := f.EndPieceIndex()

	k := max(1, (lastPiece-firstPiece)/20)

	for i := firstPiece; i < firstPiece+k; i++ {
		t.Piece(i).SetPriority(torrent.PiecePriorityNow)
	}

	for i := lastPiece - 1; i >= lastPiece-k; i-- {
		t.Piece(i).SetPriority(torrent.PiecePriorityNow)
	}

	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		firstDone := true
		for i := firstPiece; i < firstPiece+k; i++ {
			if !t.Piece(i).State().Complete {
				firstDone = false
				break
			}
		}

		lastDone := true
		for i := lastPiece - 1; i >= lastPiece-k; i-- {
			if !t.Piece(i).State().Complete {
				lastDone = false
				break
			}
		}

		fmt.Printf("\rBuffering...")
		if firstDone && lastDone {
			ticker.Stop()
			break
		}
	}
	fmt.Println()

	reader := f.NewReader()
	reader.SetReadahead(64 << 20)

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
