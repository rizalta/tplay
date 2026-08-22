package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anacrolix/torrent"
)

func (am appModel) updateInput(msg tea.Msg) (appModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			am.err = nil
			uri := am.input.Value()
			if !strings.HasPrefix(uri, "magnet:?xt=") {
				am.err = fmt.Errorf("invalid magnet link")
				return am, nil
			}

			ctx, cancel := context.WithCancel(context.Background())
			am.cancelLoad = cancel

			am.state = stateLoading
			return am, loadTorrentCmd(ctx, am.client, uri)

		case "esc":
			am.err = nil
			am.input.SetValue("")

			return am, nil
		}
	}

	var cmd tea.Cmd
	am.input, cmd = am.input.Update(msg)
	return am, cmd
}

func (am appModel) updateLoading(msg tea.Msg) (appModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			if am.cancelLoad != nil {
				am.cancelLoad()
				am.cancelLoad = nil
			}

			am.state = stateInput
			return am, nil
		}
	case loadTorrentMsg:
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				return am, nil
			}
			am.err = msg.err
			am.state = stateInput
			return am, nil
		}
		am.torrent = msg.torrent
		files := am.visibleFiles()
		if len(files) == 1 {
			am.file = files[0]
			setFilePriority(am.torrent, am.file, am.config.BufferPercent)
			am.state = stateBuffer
		} else {
			am.state = stateSelect
		}
		return am, nil
	}

	return am, nil
}

func (am appModel) updateSelect(msg tea.Msg) (appModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if am.cursor > 0 {
				am.cursor--
				if am.cursor < am.scrollOffset {
					am.scrollOffset = am.cursor
				}
			}
		case "down":
			if am.cursor < len(am.visibleFiles())-1 {
				am.cursor++
				visibleRows := am.height - 12
				if am.cursor >= am.scrollOffset+visibleRows {
					am.scrollOffset = am.cursor - visibleRows + 1
				}
			}
		case "enter":
			am.file = am.visibleFiles()[am.cursor]
			setFilePriority(am.torrent, am.file, am.config.BufferPercent)
			am.state = stateBuffer

		case "esc":
			am.torrent = nil
			am.cursor = 0
			am.scrollOffset = 0
			am.err = nil
			am.state = stateInput
		}
	}

	return am, nil
}

type playerStoppedMsg struct{}

func (am appModel) updateBuffer(msg tea.Msg) (appModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if am.playerCmd != nil && am.playerCmd.Process != nil {
				if err := am.playerCmd.Process.Kill(); err != nil {
					am.err = err
				}
			}

			if am.file != nil {
				am.file.SetPriority(torrent.PiecePriorityNone)
			}

			am.done = false
			am.file = nil
			am.downloadSpeed = 0
			am.state = stateSelect

			return am, nil
		}
	case tickMsg:
		if am.tickCount%5 != 0 {
			return am, nil
		}

		curr := am.torrent.BytesCompleted()
		if curr >= am.prevBytes {
			am.downloadSpeed = curr - am.prevBytes
		}
		am.prevBytes = curr

		if am.done {
			return am, nil
		}

		firstPiece := am.file.BeginPieceIndex()
		lastPiece := am.file.EndPieceIndex()
		k := max(1, (lastPiece-firstPiece)/(100/am.config.BufferPercent))

		firstDone := true
		for i := firstPiece; i < firstPiece+k; i++ {
			if !am.torrent.Piece(i).State().Complete {
				firstDone = false
				break
			}
		}

		lastDone := true
		for i := lastPiece - 1; i >= lastPiece-k; i-- {
			if !am.torrent.Piece(i).State().Complete {
				lastDone = false
				break
			}
		}

		if firstDone && lastDone {
			reader := am.file.NewReader()
			reader.SetReadahead(am.config.ReadAhead << 20)

			ln, err := serveToPort(am.file.DisplayPath(), reader)
			if err != nil {
				am.err = err
				return am, nil
			}
			port := ln.Addr().(*net.TCPAddr).Port

			playerCmd := exec.Command(am.config.Player, fmt.Sprintf("http://localhost:%d/stream", port))
			if err := playerCmd.Start(); err != nil {
				am.err = err
				return am, nil
			}
			am.playerCmd = playerCmd

			am.done = true
			return am, func() tea.Msg {
				_ = playerCmd.Wait()
				_ = reader.Close()
				_ = ln.Close()

				return playerStoppedMsg{}
			}
		}

		return am, nil

	case playerStoppedMsg:
		if am.torrent != nil {
			torrentPaths := torrentPaths(am.dataDir, am.torrent.Name())
			now := time.Now()
			for _, tp := range torrentPaths {
				_ = os.Chtimes(tp, now, now)
			}
		}

		if am.file != nil {
			am.file.SetPriority(torrent.PiecePriorityNone)
		}
		am.done = false
		am.downloadSpeed = 0
		am.state = stateSelect

		return am, nil
	}

	return am, nil
}

func setFilePriority(t *torrent.Torrent, f *torrent.File, bufferPercent int) {
	firstPiece := f.BeginPieceIndex()
	lastPiece := f.EndPieceIndex()

	f.SetPriority(torrent.PiecePriorityNormal)
	k := max(1, (lastPiece-firstPiece)/(100/bufferPercent))
	for i := firstPiece; i < firstPiece+k; i++ {
		t.Piece(i).SetPriority(torrent.PiecePriorityNow)
	}
	for i := lastPiece - 1; i >= lastPiece-k; i-- {
		t.Piece(i).SetPriority(torrent.PiecePriorityNow)
	}
}

type loadTorrentMsg struct {
	torrent *torrent.Torrent
	err     error
}

func loadTorrentCmd(ctx context.Context, c *torrent.Client, uri string) tea.Cmd {
	return func() tea.Msg {
		t, err := c.AddMagnet(uri)
		if err != nil {
			return loadTorrentMsg{err: err}
		}
		select {
		case <-t.GotInfo():
			return loadTorrentMsg{torrent: t}
		case <-ctx.Done():
			t.Drop()
			return loadTorrentMsg{err: context.Canceled}
		}
	}
}
