package main

import (
	"fmt"

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
