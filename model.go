package main

import (
	"context"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/anacrolix/torrent"
)

type state int

const (
	stateInput state = iota
	stateLoading
	stateSelect
	stateBuffer
)

type appModel struct {
	width  int
	height int
	state  state

	input   textinput.Model
	spinner spinner.Model

	cursor        int
	scrollOffset  int
	tickCount     int
	downloadSpeed int64
	prevBytes     int64

	torrent *torrent.Torrent
	client  *torrent.Client
	file    *torrent.File

	cancelLoad context.CancelFunc
	done       bool
	err        error
}

func newAppModel(c *torrent.Client) *appModel {
	ti := textinput.New()
	ti.Focus()
	ti.Prompt = " ❯ "
	ti.Placeholder = "magnet:?xt=..."
	ti.CharLimit = 0
	tiStyles := ti.Styles()
	tiStyles.Focused.Prompt = selectedStyle
	tiStyles.Blurred.Prompt = mutedStyle
	ti.SetStyles(tiStyles)

	s := spinner.New()
	s.Spinner = spinner.Line

	return &appModel{
		state:   stateInput,
		input:   ti,
		spinner: s,
		client:  c,
	}
}

type tickMsg struct{}

func (am appModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg{}
		}),
		textinput.Blink,
		am.spinner.Tick,
	)
}

func (am appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return am, tea.Quit
		}

	case tickMsg:
		am.tickCount++

	case tea.WindowSizeMsg:
		am.width = msg.Width
		am.height = msg.Height
		am.input.SetWidth(msg.Width - 14)

	case spinner.TickMsg:
		var cmd tea.Cmd
		am.spinner, cmd = am.spinner.Update(msg)
		return am, cmd
	}

	var cmd tea.Cmd
	switch am.state {
	case stateInput:
		am, cmd = am.updateInput(msg)
	case stateLoading:
		am, cmd = am.updateLoading(msg)
	case stateSelect:
		am, cmd = am.updateSelect(msg)
	case stateBuffer:
		am, cmd = am.updateBuffer(msg)
	}

	if _, ok := msg.(tickMsg); ok {
		cmd = tea.Batch(tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg{}
		}), cmd)
	}

	return am, cmd
}
