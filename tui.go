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
	logoStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBA6F7"))
	headerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#89B4FA"))
	selectedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#B4BEFE"))
	normalStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#CDD6F4"))
	mutedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#A6ADC8"))
	borderStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("#585B70"))
	activeBorderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("#CBA6F7"))
	doneStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1"))
	errStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8"))
	errBorderStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("#F38BA8"))
)

type state int

const (
	stateInput state = iota
	stateLoading
	stateSelect
	stateBuffer
)

const bufferPercent = 2

type appModel struct {
	width  int
	height int
	state  state

	input   textinput.Model
	spinner spinner.Model

	cursor  int
	torrent *torrent.Torrent
	client  *torrent.Client
	file    *torrent.File

	done bool
	err  error
}

func newAppModel(c *torrent.Client) *appModel {
	ti := textinput.New()
	ti.Focus()
	ti.Prompt = " ▶ "
	ti.Placeholder = "magnet:?xt=..."
	ti.CharLimit = 0
	tiStyles := ti.Styles()
	tiStyles.Focused.Prompt = selectedStyle
	tiStyles.Blurred.Prompt = mutedStyle
	ti.SetStyles(tiStyles)

	s := spinner.New()
	s.Spinner = spinner.Ellipsis

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

	case tea.WindowSizeMsg:
		am.width = msg.Width
		am.height = msg.Height
		am.input.SetWidth(msg.Width - 14)

	case spinner.TickMsg:
		var cmd tea.Cmd
		am.spinner, cmd = am.spinner.Update(msg)
		return am, cmd
	}

	switch am.state {
	case stateInput:
		return am.updateInput(msg)
	case stateLoading:
		return am.updateLoading(msg)
	case stateSelect:
		return am.updateSelect(msg)
	case stateBuffer:
		return am.updateBuffer(msg)
	}

	return am, nil
}

func (am appModel) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (am appModel) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			am.state = stateBuffer
		} else {
			am.state = stateSelect
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
			am.state = stateBuffer
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
	logo := logoStyle.Render("▀█▀ █▀█ █   ▄▀█ █▄█\n █  █▀▀ █▄▄ █▀█  █ ")
	title := headerStyle.Render("Play ur torrents")
	header := lipgloss.JoinHorizontal(lipgloss.Bottom, logo, "  ", title)

	inputWidth := am.width - 6
	var input string
	if am.err != nil {
		input = errBorderStyle.Width(inputWidth).Render(am.input.View())
	} else if am.state == stateInput {
		input = activeBorderStyle.Width(inputWidth).Render(am.input.View())
	} else {
		input = borderStyle.Width(inputWidth).Render(am.input.View())
	}

	var errorMsg string
	if am.err != nil {
		errorMsg = errStyle.Render(am.err.Error())
	}

	leftWitdth := (am.width - 6) / 2
	rightWidth := am.width - leftWitdth - 6
	panelHeight := am.height - 7

	leftPanel := borderStyle.Width(leftWitdth).Height(panelHeight).Render()
	rightPanel := borderStyle.Width(rightWidth).Height(panelHeight).Render()

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	v := tea.NewView(borderStyle.Width(am.width - 2).Height(am.height - 2).Render(lipgloss.JoinVertical(lipgloss.Left, header, input, errorMsg, body)))
	v.AltScreen = true
	return v
}
