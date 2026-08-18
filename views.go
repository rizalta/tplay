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

const (
	escIdx         = 2
	mediaToggleIdx = 3
)

var keys = []struct {
	key    string
	action string
}{
	{
		key:    "↑/↓",
		action: "Navigate",
	},
	{
		key:    "Enter",
		action: "Select",
	},
	{
		key:    "Esc",
		action: "",
	},
	{
		key:    "Tab",
		action: "",
	},
	{
		key:    "Ctrl+C",
		action: "Quit",
	},
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

	leftWidth := (am.width - 6) * 2 / 3
	rightWidth := am.width - leftWidth - 6
	panelHeight := am.height - 9

	var leftPanel string
	if am.torrent != nil {
		leftPanel = am.selectView(leftWidth, panelHeight)
	}
	if am.state == stateSelect {
		leftPanel = activeBorderStyle.Width(leftWidth).Height(panelHeight).Render(leftPanel)
	} else {
		leftPanel = borderStyle.Width(leftWidth).Height(panelHeight).Render(mutedStyle.Render(leftPanel))
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
			rightPanel = am.statsView(rightWidth)
		}
		rightPanel = borderStyle.Width(rightWidth).Height(panelHeight).Render(rightPanel)
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	footer := am.footerView()

	v := tea.NewView(borderStyle.Width(am.width - 2).Height(am.height - 2).Render(lipgloss.JoinVertical(lipgloss.Left, header, input, body, footer)))
	v.AltScreen = true
	return v
}

func (am appModel) selectView(width, height int) string {
	visibleRows := height - 3
	if visibleRows < 1 {
		visibleRows = 1
	}
	maxLen := width - 6

	files := am.visibleFiles()
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

func (am appModel) statsView(width int) string {
	status := headerStyle.Render("Status: ")
	if am.done {
		status += doneStyle.Render("Playing on MPV")
	} else if am.state == stateBuffer {
		status += selectedStyle.Render("Buffering " + am.spinner.View())
	} else {
		status += normalStyle.Render("Idle")
	}
	status += "\n"

	torrentInfo := lipgloss.JoinVertical(lipgloss.Left,
		headerStyle.Underline(true).Render("TORRENT INFO"),
		formatLine("Name", am.torrent.Name(), doneStyle, selectedStyle, width),
		formatLine("Total Size", humanSize(am.torrent.Length()), doneStyle, normalStyle, width),
		formatLine("Info Hash", am.torrent.InfoHash().HexString(), doneStyle, mutedStyle, width),
	)

	f := am.visibleFiles()[am.cursor]
	filename := filepath.Base(f.DisplayPath())
	fileDir := filepath.Dir(f.DisplayPath())
	fileExt := filepath.Ext(f.DisplayPath())
	if fileExt != "" {
		fileExt = fileExt[1:]
	}
	fileSize := f.Length()

	fileInfo := lipgloss.JoinVertical(lipgloss.Left,
		headerStyle.Underline(true).Render("FILE INFO"),
		formatLine("Name", filename, doneStyle, selectedStyle, width),
		formatLine("Directory", fileDir, doneStyle, normalStyle, width),
		formatLine("Size", humanSize(fileSize), doneStyle, normalStyle, width),
		formatLine("Type", fileExt, doneStyle, normalStyle, width),
	)

	activePeersStr := fmt.Sprintf("%d", am.torrent.Stats().ActivePeers)
	swarmSizeStr := fmt.Sprintf("%d", am.torrent.Stats().TotalPeers)
	downloadSpeedStr := fmt.Sprintf("%s/s", humanSize(am.downloadSpeed))

	var swarmStats string
	if am.state == stateBuffer {
		swarmStats = lipgloss.JoinVertical(lipgloss.Left,
			headerStyle.Underline(true).Render("SWARM INFO"),
			formatLine("Active Peers", activePeersStr, doneStyle, selectedStyle, width),
			formatLine("Swarm Size", swarmSizeStr, doneStyle, normalStyle, width),
			formatLine("DL Speed", downloadSpeedStr, doneStyle, normalStyle, width),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		status,
		torrentInfo, "",
		fileInfo, "",
		swarmStats)
}

func (am appModel) footerView() string {
	escStr := "Back"
	if am.state == stateInput {
		escStr = "Clear"
	}
	keys[escIdx].action = escStr

	mediaToggleStr := "Media Only"
	if am.mediaOnly {
		mediaToggleStr = "All Files"
	}
	keys[mediaToggleIdx].action = mediaToggleStr

	footer := "  "
	for _, k := range keys {
		line := lipgloss.JoinHorizontal(lipgloss.Center, normalStyle.Render(k.key), " ", mutedStyle.Render(k.action))
		footer = lipgloss.JoinHorizontal(lipgloss.Center, footer, "  ", line)
	}

	return footer
}

func formatLine(label, val string, labelStyle, valStyle lipgloss.Style, width int) string {
	labelWidth := min(width, 12)
	valWidth := width - labelWidth - 6

	wrappedVal := lipgloss.Wrap(val, width, " ")
	return lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Width(labelWidth).Render(label),
		": ",
		valStyle.Width(valWidth).Render(wrappedVal),
	)
}
