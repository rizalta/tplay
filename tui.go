package main

import (
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

type appModel struct {
	input textinput.Model
	uri   string
}

func newAppModel() *appModel {
	ti := textinput.New()
	ti.Focus()
	ti.Placeholder = "magnet:?xt=..."
	ti.CharLimit = 0
	ti.SetWidth(60)

	return &appModel{
		input: ti,
	}
}

func (am appModel) Init() tea.Cmd {
	return textinput.Blink
}

func (am appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			am.uri = am.input.Value()
			return am, tea.Quit

		case "esc", "ctrl+c":
			return am, tea.Quit

		}
	}
	var cmd tea.Cmd
	am.input, cmd = am.input.Update(msg)
	return am, cmd
}

func (am appModel) View() tea.View {
	return tea.NewView(am.input.View())
}

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

func (m model) View() tea.View {
	s := titleStyle.Render("Select the file:") + "\n"
	for i, f := range m.files {
		if m.cursor == i {
			s += selectedStyle.Render("> "+f.DisplayPath()) + "\n"
		} else {
			s += normalStyle.Render("  "+f.DisplayPath()) + "\n"
		}
	}

	return tea.NewView(s)
}

type bufferModel struct {
	spinner    spinner.Model
	torrent    *torrent.Torrent
	firstPiece int
	lastPiece  int
	k          int
	done       bool
	quit       bool
}

func newBufferModel(torrent *torrent.Torrent, firstPiece, lastPiece, k int) *bufferModel {
	s := spinner.New()
	s.Spinner = spinner.Ellipsis
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBA6F7"))
	return &bufferModel{
		spinner:    s,
		torrent:    torrent,
		firstPiece: firstPiece,
		lastPiece:  lastPiece,
		k:          k,
	}
}

type tickMsg struct{}

func (bm bufferModel) Init() tea.Cmd {
	return tea.Batch(
		bm.spinner.Tick,
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg{}
		}),
	)
}

func (bm bufferModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			bm.quit = true
			return bm, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		bm.spinner, cmd = bm.spinner.Update(msg)
		return bm, cmd
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

func (bm bufferModel) View() tea.View {
	var s string
	if bm.done {
		s = titleStyle.Render("Ready") + "\n"
	} else {
		s = selectedStyle.Render("Buffering") + bm.spinner.View() + "\n"
	}

	return tea.NewView(s)
}
