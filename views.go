package main

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
		leftPanel = am.selectView()
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
		case stateSelect, stateBuffer:
			rightPanel = am.statsView()
		}
		rightPanel = borderStyle.Width(rightWidth).Height(panelHeight).Render(rightPanel)
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	footer := mutedStyle.Render(" ↑/↓ Navigate • Enter Select • Esc/Ctrl+C Quit")

	v := tea.NewView(borderStyle.Width(am.width - 2).Height(am.height - 2).Render(lipgloss.JoinVertical(lipgloss.Left, header, input, body, footer)))
	v.AltScreen = true
	return v
}

func (am appModel) selectView() string {
	panelHeight := am.height - 6
	leftWitdth := (am.width - 6) * 2 / 3

	visibleRows := panelHeight - 6
	if visibleRows < 1 {
		visibleRows = 1
	}
	maxLen := leftWitdth - 6

	files := am.torrent.Files()
	start := am.scrollOffset
	end := start + visibleRows
	if end > len(files) {
		end = len(files)
	}

	var ret string
	for i, f := range files[start:end] {
		name := f.DisplayPath()
		if am.cursor == i+start {
			if am.state == stateSelect {
				ret += selectedStyle.Render("▶ "+marquee(name, maxLen, am.tickCount)) + "\n"
			} else {
				ret += mutedStyle.Render("▶ "+marquee(name, maxLen, am.tickCount)) + "\n"
			}
		} else {
			if am.state == stateSelect {
				ret += normalStyle.Render("  "+truncate(name, maxLen)) + "\n"
			} else {
				ret += mutedStyle.Render("  "+truncate(name, maxLen)) + "\n"
			}
		}
	}

	return ret
}

func (am appModel) statsView() string {
	var status string
	if am.done {
		status = doneStyle.Render("Playing on MPV")
	} else if am.state == stateBuffer {
		status = selectedStyle.Render("Buffering " + am.spinner.View())
	} else {
		status = normalStyle.Render("Idle")
	}
	status += "\n"

	torrentInfo := headerStyle.Underline(true).Render("TORRENT INFO") + "\n" +
		doneStyle.Render("Name: ") +
		selectedStyle.Render(am.torrent.Name()) + "\n" +
		doneStyle.Render("Total Size: ") +
		normalStyle.Render(humanSize(am.torrent.Length())) + "\n" +
		doneStyle.Render("Info Hash: ") +
		mutedStyle.Render(am.torrent.InfoHash().HexString()[:8]) + "\n"

	f := am.torrent.Files()[am.cursor]
	filename := filepath.Base(f.DisplayPath())
	fileExt := filepath.Ext(f.DisplayPath())
	if fileExt != "" {
		fileExt = fileExt[1:]
	}

	fileInfo := headerStyle.Underline(true).Render("FILE INFO") + "\n" +
		doneStyle.Render("Name: ") +
		selectedStyle.Render(filename) + "\n" +
		doneStyle.Render("Path: ") +
		normalStyle.Render(filepath.Dir(f.DisplayPath())) + "\n" +
		doneStyle.Render("Size: ") +
		normalStyle.Render(humanSize(f.Length())) + "\n" +
		doneStyle.Render("Type: ") +
		normalStyle.Render(fileExt) + "\n"

	var swarmStats string
	if am.state == stateBuffer {
		swarmStats = headerStyle.Underline(true).Render("SWARM INFO") + "\n" +
			doneStyle.Render("Active Peers: ") +
			selectedStyle.Render(fmt.Sprintf("%d", am.torrent.Stats().ActivePeers)) + "\n" +
			doneStyle.Render("Swarm size: ") +
			normalStyle.Render(fmt.Sprintf("%d", am.torrent.Stats().TotalPeers)) + "\n" +
			doneStyle.Render("DL Speed: ") +
			normalStyle.Render(fmt.Sprintf("%s/s", humanSize(am.downloadSpeed))) + "\n"
	}

	return lipgloss.JoinVertical(lipgloss.Left, status, torrentInfo, fileInfo, swarmStats)
}
