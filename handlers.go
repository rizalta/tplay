package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anacrolix/torrent"
)

const bufferPercent = 2

func (am appModel) updateInput(msg tea.Msg) (appModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "enter" {
			am.err = nil
			uri := am.input.Value()
			if !strings.HasPrefix(uri, "magnet:?xt=") {
				am.err = fmt.Errorf("invalid magnet link")
				return am, nil
			}
			am.state = stateLoading
			return am, loadTorrentCmd(am.client, uri)
		}
	}

	var cmd tea.Cmd
	am.input, cmd = am.input.Update(msg)
	return am, cmd
}

func (am appModel) updateLoading(msg tea.Msg) (appModel, tea.Cmd) {
	switch msg := msg.(type) {
	case loadTorrentMsg:
		if msg.err != nil {
			am.err = msg.err
			am.state = stateInput
			return am, nil
		}
		am.torrent = msg.torrent
		files := am.torrent.Files()
		if len(files) == 1 {
			am.file = files[0]
			setFilePriority(am.torrent, am.file)
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
			if am.cursor < len(am.torrent.Files())-1 {
				am.cursor++
				visibleRows := am.height - 12
				if am.cursor >= am.scrollOffset+visibleRows {
					am.scrollOffset = am.cursor - visibleRows + 1
				}
			}
		case "enter":
			am.file = am.torrent.Files()[am.cursor]
			setFilePriority(am.torrent, am.file)
			am.state = stateBuffer
		}
	}

	return am, nil
}

type mpvStoppedMsg struct{}

func (am appModel) updateBuffer(msg tea.Msg) (appModel, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
		if am.tickCount%5 != 0 {
			return am, nil
		}

		curr := am.torrent.BytesCompleted()
		am.downloadSpeed = curr - am.prevBytes
		am.prevBytes = curr

		if am.done {
			return am, nil
		}

		firstPiece := am.file.BeginPieceIndex()
		lastPiece := am.file.EndPieceIndex()
		k := max(1, (lastPiece-firstPiece)/(100/bufferPercent))

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
			reader.SetReadahead(62 << 20)

			ln, err := serveToPort(am.file.DisplayPath(), reader)
			if err != nil {
				am.err = err
				return am, nil
			}
			port := ln.Addr().(*net.TCPAddr).Port

			cmd := exec.Command("mpv", fmt.Sprintf("http://localhost:%d/stream", port))
			if err := cmd.Start(); err != nil {
				am.err = err
				return am, nil
			}

			am.done = true
			return am, func() tea.Msg {
				_ = cmd.Wait()
				_ = reader.Close()
				_ = ln.Close()

				return mpvStoppedMsg{}
			}
		}

		return am, nil

	case mpvStoppedMsg:
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

func setFilePriority(t *torrent.Torrent, f *torrent.File) {
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

func loadTorrentCmd(c *torrent.Client, uri string) tea.Cmd {
	return func() tea.Msg {
		t, err := c.AddMagnet(uri)
		if err != nil {
			return loadTorrentMsg{err: err}
		}
		<-t.GotInfo()
		return loadTorrentMsg{torrent: t}
	}
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
