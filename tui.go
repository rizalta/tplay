package main

import (
	"fmt"
	"time"

	"github.com/anacrolix/torrent"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	files    []*torrent.File
	cursor   int
	selector int
}

func newModel(files []*torrent.File) *model {
	return &model{
		files:    files,
		cursor:   0,
		selector: -1,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.files)-1 {
				m.cursor++
			}
		case "enter":
			m.selector = m.cursor
			return m, tea.Quit
		case "q":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m model) View() string {
	ret := ""
	for i, f := range m.files {
		if m.cursor == i {
			ret += fmt.Sprintf("> %s\n", f.DisplayPath())
		} else {
			ret += fmt.Sprintf("  %s\n", f.DisplayPath())
		}
	}

	return ret
}

type bufferModel struct {
	torrent    *torrent.Torrent
	firstPiece int
	lastPiece  int
	k          int
	done       bool
}

func newBufferModel(torrent *torrent.Torrent, firstPiece, lastPiece, k int) *bufferModel {
	return &bufferModel{
		torrent:    torrent,
		firstPiece: firstPiece,
		lastPiece:  lastPiece,
		k:          k,
	}
}

type tickMsg struct{}

func (bm bufferModel) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (bm bufferModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
		firstDone := true
		for i := bm.firstPiece; i < bm.firstPiece+bm.k; i++ {
			if !bm.torrent.Piece(i).State().Complete {
				firstDone = false
				break
			}
		}

		lastDone := true
		for i := bm.lastPiece - 1; i >= bm.lastPiece-bm.k; i-- {
			if !bm.torrent.Piece(i).State().Complete {
				lastDone = false
				break
			}
		}

		if firstDone && lastDone {
			bm.done = true
			return bm, tea.Quit
		}
	}

	return bm, tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (bm bufferModel) View() string {
	if bm.done {
		return "Ready\n"
	}

	return "Buffering...\n"
}
