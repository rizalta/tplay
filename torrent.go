package main

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/anacrolix/torrent"
	tea "github.com/charmbracelet/bubbletea"
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

func addMagnet(c *torrent.Client, uri string) (*torrent.Torrent, error) {
	t, err := c.AddMagnet(uri)
	if err != nil {
		return nil, err
	}

	<-t.GotInfo()

	return t, nil
}

func selectFile(t *torrent.Torrent) (*torrent.File, error) {
	files := t.Files()

	if len(files) == 1 {
		return files[0], nil
	}

	p := tea.NewProgram(newModel(files))
	result, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to select file: %v", err)
	}

	m := result.(model)

	if m.selector == -1 {
		return nil, fmt.Errorf("no file selected")
	}

	return files[m.selector], nil
}

func bufferFile(t *torrent.Torrent, f *torrent.File) torrent.Reader {
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

	return reader
}
