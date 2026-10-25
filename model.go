package main

import (
	"cmp"
	"context"
	"errors"
	"os/exec"
	"slices"
	"strings"
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
	stateSettings
)

type appModel struct {
	config          *config
	sessionTorrents map[string]struct{}

	width     int
	height    int
	state     state
	prevState state
	mediaOnly bool

	input   textinput.Model
	spinner spinner.Model

	cursor        int
	scrollOffset  int
	tickCount     int
	downloadSpeed int64
	prevBytes     int64

	filterInput  textinput.Model
	filterActive bool

	playerIdx      int
	settingsCursor int
	settingsEdit   bool
	settingsInput  textinput.Model

	dataDir   string
	torrent   *torrent.Torrent
	client    *torrent.Client
	file      *torrent.File
	playerCmd *exec.Cmd

	cancelLoad context.CancelFunc
	done       bool
	err        error
}

func newAppModel(c *torrent.Client, cfg *config, dataDir string) *appModel {
	ti := textinput.New()
	ti.Focus()
	ti.Prompt = " ❯ "
	ti.Placeholder = "magnet:?xt=..."
	ti.CharLimit = 0
	tiStyles := ti.Styles()
	tiStyles.Focused.Prompt = selectedStyle
	tiStyles.Blurred.Prompt = mutedStyle
	ti.SetStyles(tiStyles)

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.Placeholder = "Filter"
	fi.CharLimit = 0
	fiStyles := fi.Styles()
	fiStyles.Focused.Prompt = selectedStyle
	fiStyles.Blurred.Prompt = mutedStyle
	fi.SetStyles(fiStyles)

	si := textinput.New()
	si.Prompt = ""
	siStyles := si.Styles()
	siStyles.Focused.Text = selectedStyle
	siStyles.Blurred.Text = mutedStyle
	si.SetStyles(siStyles)

	s := spinner.New()
	s.Spinner = spinner.Line

	playerIdx := slices.IndexFunc(knownPlayers, func(p player) bool {
		return p.name == cfg.Player
	})
	if playerIdx == -1 {
		playerIdx = 0
	}

	return &appModel{
		config:          cfg,
		sessionTorrents: make(map[string]struct{}),
		state:           stateInput,
		prevState:       stateInput,
		mediaOnly:       true,
		input:           ti,
		spinner:         s,
		filterInput:     fi,
		playerIdx:       playerIdx,
		settingsInput:   si,
		dataDir:         dataDir,
		client:          c,
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

		case "tab":
			am.mediaOnly = !am.mediaOnly
			am.cursor = 0
			am.scrollOffset = 0
			return am, nil

		case "ctrl+s":
			if am.state != stateSettings {
				am.prevState = am.state
				am.state = stateSettings
			}
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
		am.sessionTorrents[am.torrent.Name()] = struct{}{}
		am.cursor = 0
		am.scrollOffset = 0
		am.filterInput.Reset()
		am.filterInput.Blur()

		files := am.visibleFiles()
		var nextState state
		if len(files) == 1 && isMedia(files[0].DisplayPath()) {
			am.file = files[0]
			setFilePriority(am.torrent, am.file, am.config.BufferPercent)
			nextState = stateBuffer
		} else {
			nextState = stateSelect
		}

		if am.state == stateSettings {
			am.prevState = nextState
		} else {
			am.state = nextState
		}
		return am, nil

	case tickMsg:
		am.tickCount++

	case tea.WindowSizeMsg:
		am.width = msg.Width
		am.height = msg.Height
		am.input.SetWidth(max(1, msg.Width-14))
		am.filterInput.SetWidth(max(1, msg.Width-14))
		var cmd tea.Cmd
		am.input, cmd = am.input.Update(msg)
		return am, cmd

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
	case stateSettings:
		am, cmd = am.updateSettings(msg)
	}

	if _, ok := msg.(tickMsg); ok {
		cmd = tea.Batch(tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg{}
		}), cmd)
	}

	return am, cmd
}

func (am appModel) visibleFiles() []*torrent.File {
	filtered := slices.Clone(am.torrent.Files())
	if am.mediaOnly {
		filtered = mediaFiles(am.torrent.Files())
	}

	filtered = filterFiles(filtered, am.filterInput.Value())

	slices.SortFunc(filtered, func(a, b *torrent.File) int {
		return cmp.Compare(strings.ToLower(a.DisplayPath()), strings.ToLower(b.DisplayPath()))
	})

	return filtered
}
