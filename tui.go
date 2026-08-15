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

	cursor       int
	scrollOffset int
	tickCount    int

	torrent *torrent.Torrent
	client  *torrent.Client
	file    *torrent.File

	done bool
	err  error
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

	tickCmd := tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})

	return am, tea.Batch(cmd, tickCmd)
}

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

func (am appModel) updateBuffer(msg tea.Msg) (appModel, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
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
			am.done = true
			return am, tea.Quit
		}

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

func (am appModel) View() tea.View {
	logo := logoStyle.Render("▀█▀ █▀█ █   ▄▀█ █▄█\n █  █▀▀ █▄▄ █▀█  █ ")
	title := headerStyle.Render("Play ur torrents")
	header := lipgloss.JoinHorizontal(lipgloss.Bottom, logo, "  ", title)

	inputWidth := am.width - 6
	if am.state == stateLoading {
		am.input.Prompt = " " + am.spinner.View() + " "
	} else {
		am.input.Prompt = " ❯ "
	}

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
		errorMsg = errStyle.Render("! " + strings.ToTitle(am.err.Error()))
	}

	leftWitdth := (am.width - 6) * 2 / 3
	rightWidth := am.width - leftWitdth - 6
	panelHeight := am.height - 9

	var leftPanel string
	if am.torrent != nil {
		leftPanel = selectView(am.torrent.Files(), am.cursor, leftWitdth-6, am.tickCount, panelHeight, am.scrollOffset, am.state)
	}
	if am.state == stateSelect {
		leftPanel = activeBorderStyle.Width(leftWitdth).Height(panelHeight).Render(leftPanel)
	} else {
		leftPanel = borderStyle.Width(leftWitdth).Height(panelHeight).Render(mutedStyle.Render(leftPanel))
	}

	var rightPanel string
	if errorMsg != "" {
		rightPanel = errBorderStyle.Width(rightWidth).Height(panelHeight).Render(errorMsg)
	} else {
		switch am.state {
		case stateInput:
			rightPanel = mutedStyle.Render("Paste your magnet link and press Enter")
		case stateLoading:
			rightPanel = selectedStyle.Render("Fetching Metadata ") + am.spinner.View()
		case stateSelect:
			rightPanel = mutedStyle.Render("Press Enter to begin stream...")
		case stateBuffer:
			rightPanel = selectedStyle.Render("Buffering ") + am.spinner.View()
		}
		rightPanel = borderStyle.Width(rightWidth).Height(panelHeight).Render(rightPanel)
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	footer := mutedStyle.Render(" ↑/↓ Navigate • Enter Select • Esc/Ctrl+C Quit")

	v := tea.NewView(borderStyle.Width(am.width - 2).Height(am.height - 2).Render(lipgloss.JoinVertical(lipgloss.Left, header, input, body, footer)))
	v.AltScreen = true
	return v
}

func selectView(files []*torrent.File, cursor, maxLen, step, panelHeight, scrollOffset int, state state) string {
	visibleRows := panelHeight - 3
	if visibleRows < 1 {
		visibleRows = 1
	}

	start := scrollOffset
	end := start + visibleRows
	if end > len(files) {
		end = len(files)
	}

	var ret string
	for i, f := range files[start:end] {
		name := f.DisplayPath()
		if cursor == i+start {
			if state == stateSelect {
				ret += selectedStyle.Render("▶ "+marquee(name, maxLen, step)) + "\n"
			} else {
				ret += mutedStyle.Render("▶ "+marquee(name, maxLen, step)) + "\n"
			}
		} else {
			if state == stateSelect {
				ret += normalStyle.Render("  "+truncate(name, maxLen)) + "\n"
			} else {
				ret += mutedStyle.Render("  "+truncate(name, maxLen)) + "\n"
			}
		}
	}

	return ret
}

func truncate(s string, maxLen int) string {
	if maxLen > 3 && len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func marquee(s string, maxLen, step int) string {
	if len(s) <= maxLen {
		return s
	}

	padded := s + "   " + s
	offset := step % (len(s) + 3)
	return padded[offset : offset+maxLen]
}
