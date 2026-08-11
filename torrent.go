package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"

	tea "charm.land/bubbletea/v2"
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

	p := tea.NewProgram(newBufferModel(t, firstPiece, lastPiece, k))
	bufferResult, err := p.Run()
	if err != nil {
		log.Printf("error buffering: %v\n", err)
	}
	bm := bufferResult.(bufferModel)
	if bm.quit {
		return nil
	}

	reader := f.NewReader()
	reader.SetReadahead(64 << 20)

	return reader
}
