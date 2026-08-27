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
	infoStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1"))
	errStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8"))
	borderStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("#585B70"))
	activeBorderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("#CBA6F7"))
	errBorderStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("#F38BA8"))
	popupStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("#CBA6F7"))
)

const (
	escIdx         = 2
	mediaToggleIdx = 3
)

type hint struct {
	key    string
	action string
}

var baseHints = []hint{
	{
		key:    "↑/↓",
		action: "Navigate",
	},
	{
		key:    "Enter",
		action: "Select",
	},
	{
		key: "Esc",
	},
	{
		key: "Tab",
	},
	{
		key:    "Ctrl+C",
		action: "Quit",
	},
}

var settingsHints = []hint{
	{
		key:    "↑/↓",
		action: "Navigate",
	},
	{
		key:    "Enter",
		action: "Select/Toggle",
	},
	{
		key:    "Esc",
		action: "Cancel/Save",
	},
	{
		key:    "←/→",
		action: "Cycle",
	},
}

func (am appModel) View() tea.View {
	var s string

	base := lipgloss.NewStyle().Faint(true).Render(am.baseUI())
	popup := am.settingsPopup()
	x := (am.width - lipgloss.Width(popup)) / 2
	y := (am.height - lipgloss.Height(popup)) / 2

	if am.state == stateSettings {
		popupLayer := lipgloss.NewLayer(popup).X(x).Y(y).Z(1)
		baseLayer := lipgloss.NewLayer(base, popupLayer)

		comp := lipgloss.NewCompositor(baseLayer, popupLayer)

		s = comp.Render()
	} else {
		s = base
	}

	v := tea.NewView(s)
	v.AltScreen = true

	return v
}

func (am appModel) baseUI() string {
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
	if am.state == stateInput {
		if am.err != nil {
			input = errBorderStyle.Width(inputWidth).Render(am.input.View())
		} else {
			input = activeBorderStyle.Width(inputWidth).Render(am.input.View())
		}
	} else {
		input = borderStyle.Width(inputWidth).Render(am.input.View())
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
	if am.err != nil && am.state != stateSettings {
		errorMsg := errStyle.Render("! " + strings.ToTitle(am.err.Error()))
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

	s := borderStyle.Width(am.width - 2).Height(am.height - 2).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			input,
			body,
			footer,
		),
	)

	return s
}

func (am appModel) settingsPopup() string {
	popupWidth := am.width / 3
	if popupWidth < 46 {
		popupWidth = 46
	}
	labelWidth := popupWidth / 3
	valWidth := popupWidth - labelWidth

	title := "  " + headerStyle.Bold(true).Underline(true).Width(popupWidth).Render("SETTINGS")
	rows := []string{title, ""}
	for i, item := range settingsItems {
		label := item.label
		val := item.getValue(am.config)

		if item.isPlayer || item.isBool {
			val = "← " + val + " →"
		}

		if i == am.settingsCursor {
			if am.settingsEdit && !item.isBool && !item.isPlayer {
				if am.err != nil {
					val = errStyle.Render(am.settingsInput.View())
				} else {
					val = am.settingsInput.View()
				}
			}
			row := lipgloss.JoinHorizontal(
				lipgloss.Top,
				infoStyle.Width(labelWidth).Render("▶ "+label),
				selectedStyle.Width(valWidth).Render(val),
			)
			rows = append(rows, row)
		} else {
			row := lipgloss.JoinHorizontal(
				lipgloss.Top,
				normalStyle.Width(labelWidth).Render("  "+label),
				mutedStyle.Width(valWidth).Render(val),
			)
			rows = append(rows, row)
		}
	}

	var footer string
	for _, h := range settingsHints {
		footer = lipgloss.JoinHorizontal(lipgloss.Left, footer, formatHint(h), "  ")
	}

	rows = append(rows, "", footer)

	return popupStyle.Width(popupWidth).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
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
		status += infoStyle.Render("Playing on " + am.config.Player)
	} else if am.state == stateBuffer {
		status += selectedStyle.Render("Buffering " + am.spinner.View())
	} else {
		status += normalStyle.Render("Idle")
	}
	status += "\n"

	torrentInfo := lipgloss.JoinVertical(lipgloss.Left,
		headerStyle.Underline(true).Render("TORRENT INFO"),
		formatLine("Name", am.torrent.Name(), infoStyle, selectedStyle, width, am.tickCount),
		formatLine("Total Size", humanSize(am.torrent.Length()), infoStyle, normalStyle, width, am.tickCount),
		formatLine("Info Hash", am.torrent.InfoHash().HexString(), infoStyle, mutedStyle, width, am.tickCount),
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
		formatLine("Name", filename, infoStyle, selectedStyle, width, am.tickCount),
		formatLine("Directory", fileDir, infoStyle, normalStyle, width, am.tickCount),
		formatLine("Size", humanSize(fileSize), infoStyle, normalStyle, width, am.tickCount),
		formatLine("Type", fileExt, infoStyle, normalStyle, width, am.tickCount),
	)

	peersStr := fmt.Sprintf("%d/%d", am.torrent.Stats().ActivePeers, am.torrent.Stats().TotalPeers)
	downloadSpeedStr := fmt.Sprintf("%s/s", humanSize(am.downloadSpeed))

	var swarmStats string
	if am.state == stateBuffer {
		var percent float64
		if am.done {
			percent = 100
		} else {
			firstPiece := am.file.BeginPieceIndex()
			lastPiece := am.file.EndPieceIndex()
			k := max(1, (lastPiece-firstPiece)/(100/am.config.BufferPercent))
			total := 2 * k

			done := 0
			for i := firstPiece; i < firstPiece+k; i++ {
				if am.torrent.Piece(i).State().Complete {
					done++
				}
			}
			for i := lastPiece - 1; i >= lastPiece-k; i-- {
				if am.torrent.Piece(i).State().Complete {
					done++
				}
			}

			percent = float64(done) / float64(total) * 100
		}

		swarmStats = lipgloss.JoinVertical(lipgloss.Left,
			headerStyle.Underline(true).Render("SWARM INFO"), "",
			infoStyle.Render(progressBar(percent, width-10)), "",
			formatLine("Peers", peersStr, infoStyle, selectedStyle, width, am.tickCount),
			formatLine("DL Speed", downloadSpeedStr, infoStyle, normalStyle, width, am.tickCount),
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
	baseHints[escIdx].action = escStr

	hasMedia := am.torrent != nil && len(mediaFiles(am.torrent.Files())) > 0
	mediaToggleStr := "Media Only"

	if hasMedia && am.mediaOnly {
		mediaToggleStr = "All Files"
	}
	baseHints[mediaToggleIdx].action = mediaToggleStr

	footer := "  "
	for _, h := range baseHints {
		line := formatHint(h)
		footer = lipgloss.JoinHorizontal(lipgloss.Center, footer, "  ", line)
	}

	return footer
}

func formatHint(h hint) string {
	return lipgloss.JoinHorizontal(lipgloss.Center, normalStyle.Render(h.key), " ", mutedStyle.Render(h.action))
}

func formatLine(label, val string, labelStyle, valStyle lipgloss.Style, width int, step int) string {
	labelWidth := min(width, 12)
	valWidth := width - labelWidth - 6
	if valWidth < 1 {
		valWidth = 1
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Width(labelWidth).Render(label),
		": ",
		valStyle.Width(valWidth).Render(marquee(val, valWidth, step)),
	)
}
