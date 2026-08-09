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
	"strings"
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

	cfg := torrent.NewDefaultClientConfig()
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

	file := files[sel]

	reader := file.NewReader()
	reader.SetReadahead(64 << 20)

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Listening on:", ln.Addr())

	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, file.DisplayPath(), time.Now(), reader)
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
}
