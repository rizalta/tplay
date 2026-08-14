package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anacrolix/torrent"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89B4FA"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#CBA6F7"))
	normalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70"))
)

type screen int

const (
	screenInput screen = iota
	screenLoading
	screenSelect
	screenBuffer
)

const bufferPercent = 2

type appModel struct {
	screen  screen
	input   textinput.Model
	spinner spinner.Model
	cursor  int
	torrent *torrent.Torrent
	client  *torrent.Client
	file    *torrent.File
	done    bool
	err     error
}

func newAppModel(c *torrent.Client) *appModel {
	ti := textinput.New()
	ti.Focus()
	ti.Placeholder = "magnet:?xt=..."
	ti.CharLimit = 0
	ti.SetWidth(60)
	s := spinner.New()
	s.Spinner = spinner.Ellipsis
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBA6F7"))

	return &appModel{
		screen:  screenInput,
		input:   ti,
		spinner: s,
		client:  c,
	}
}

type tickMsg struct{}

func (am appModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		am.spinner.Tick,
	)
}

func (am appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return am, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		am.spinner, cmd = am.spinner.Update(msg)
		return am, cmd
	}

	switch am.screen {
	case screenInput:
		return am.updateInput(msg)
	case screenLoading:
		return am.updateLoading(msg)
	case screenSelect:
		return am.updateSelect(msg)
	case screenBuffer:
		return am.updateBuffer(msg)
	}

	return am, nil
}

func (am appModel) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "enter" {
			uri := am.input.Value()
			if !strings.HasPrefix(uri, "magnet:?xt=") {
				am.err = fmt.Errorf("invalid magnet link")
				return am, nil
			}
			am.screen = screenLoading
			return am, loadTorrentCmd(am.client, uri)
		}
	}

	var cmd tea.Cmd
	am.input, cmd = am.input.Update(msg)
	return am, cmd
}

func (am appModel) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadTorrentMsg:
		if msg.err != nil {
			am.err = msg.err
			am.screen = screenInput
			return am, nil
		}
		am.torrent = msg.torrent
		files := am.torrent.Files()
		if len(files) == 1 {
			am.file = files[0]
			am.screen = screenBuffer
		} else {
			am.screen = screenSelect
		}
		return am, nil
	}

	return am, tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (am appModel) updateSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if am.cursor > 0 {
				am.cursor--
			}
		case "down":
			if am.cursor < len(am.torrent.Files())-1 {
				am.cursor++
			}
		case "enter":
			am.file = am.torrent.Files()[am.cursor]
			am.screen = screenBuffer
		}
	}

	return am, tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (am appModel) updateBuffer(msg tea.Msg) (tea.Model, tea.Cmd) {
	firstPiece := am.file.BeginPieceIndex()
	lastPiece := am.file.EndPieceIndex()

	am.file.SetPriority(torrent.PiecePriorityNormal)
	k := max(1, (lastPiece-firstPiece)/(100/bufferPercent))
	for i := firstPiece; i < firstPiece+k; i++ {
		am.torrent.Piece(i).SetPriority(torrent.PiecePriorityNow)
	}
	for i := lastPiece - 1; i >= lastPiece-k; i-- {
		am.torrent.Piece(i).SetPriority(torrent.PiecePriorityNow)
	}

	switch msg.(type) {
	case tickMsg:
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
			am.done = true
			return am, tea.Quit
		}

		return am, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg{}
		})
	}
	return am, nil
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

func (am appModel) View() tea.View {
	var s string
	switch am.screen {
	case screenInput:
		s = am.input.View() + "\n"
		if am.err != nil {
			s += normalStyle.Render(am.err.Error()) + "\n"
		}
	case screenLoading:
		s = selectedStyle.Render("Loading") + am.spinner.View() + "\n"
	case screenSelect:
		s = titleStyle.Render("Select your file:") + "\n"
		for i, f := range am.torrent.Files() {
			if i == am.cursor {
				s += selectedStyle.Render("> "+f.DisplayPath()) + "\n"
			} else {
				s += normalStyle.Render("  "+f.DisplayPath()) + "\n"
			}
		}
	case screenBuffer:
		s = selectedStyle.Render("Buffering") + am.spinner.View() + "\n"
	}
	return tea.NewView(s)
}
