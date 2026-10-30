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

type hint struct {
	key    string
	action func(am appModel) string
	when   func(am appModel) bool
}

var baseHints = []hint{
	{
		key:    "/",
		action: func(am appModel) string { return "Filter" },
		when:   func(am appModel) bool { return am.state == stateSelect },
	},
	{
		key:    "↑/↓",
		action: func(am appModel) string { return "Navigate" },
		when:   func(am appModel) bool { return am.state == stateSelect },
	},
	{
		key:    "Enter",
		action: func(am appModel) string { return "Select" },
	},
	{
		key: "Esc",
		action: func(am appModel) string {
			if am.state == stateInput || am.filterInput.Value() != "" {
				return "Clear"
			}
			return "Back"
		},
	},
	{
		key: "Tab",
		action: func(am appModel) string {
			if am.torrent != nil && am.mediaOnly {
				return "All Files"
			}
			return "Media Only"
		},
		when: func(am appModel) bool { return am.state == stateSelect || am.state == stateBuffer },
	},
	{
		key:    "Ctrl+S",
		action: func(am appModel) string { return "Settings" },
	},
	{
		key:    "Ctrl+C",
		action: func(am appModel) string { return "Quit" },
	},
}

var settingsHints = []hint{
	{
		key:    "↑/↓",
		action: func(am appModel) string { return "Navigate" },
	},
	{
		key: "Enter",
		action: func(am appModel) string {
			if am.settingsEdit {
				return "Confirm"
			}
			return "Select"
		},
	},
	{
		key: "Esc",
		action: func(am appModel) string {
			if am.settingsEdit {
				return "Cancel"
			}
			return "Save/Back"
		},
	},
	{
		key:    "←/→",
		action: func(am appModel) string { return "Cycle" },
	},
}

func (am appModel) View() tea.View {
	var s string

	base := am.baseUI()

	if am.state == stateSettings {
		popup := am.settingsPopup()

		x := (am.width - lipgloss.Width(popup)) / 2
		y := (am.height - lipgloss.Height(popup)) / 2

		dimmedBase := lipgloss.NewStyle().Faint(true).Render(base)
		popupLayer := lipgloss.NewLayer(popup).X(x).Y(y).Z(1)
		baseLayer := lipgloss.NewLayer(dimmedBase, popupLayer)

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

	// Use prevState when settings overlay is open so the base UI stays frozen
	renderState := am.state
	if am.state == stateSettings {
		renderState = am.prevState
	}

	var leftPanel string
	if am.torrent != nil {
		leftPanel = am.selectView(leftWidth, panelHeight)
	}
	if renderState == stateSelect {
		leftPanel = activeBorderStyle.Width(leftWidth).Height(panelHeight).Render(leftPanel)
	} else {
		leftPanel = borderStyle.Width(leftWidth).Height(panelHeight).Render(mutedStyle.Render(leftPanel))
	}

	var rightPanel string
	if am.err != nil && am.state != stateSettings {
		errorMsg := errStyle.Render("! " + strings.ToTitle(am.err.Error()))
		rightPanel = errBorderStyle.Width(rightWidth).Height(panelHeight).Render(errorMsg)
	} else {
		switch renderState {
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

		if item.isBool {
			val = "← " + val + " →"
		}

		if i == am.settingsCursor {
			if am.settingsEdit && !item.isBool {
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

	return popupStyle.Width(popupWidth).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (am appModel) selectView(width, height int) string {
	visibleRows := height - 3
	if visibleRows < 1 {
		visibleRows = 1
	}
	maxLen := width - 6

	files := am.visibleFiles()

	if len(files) == 0 {
		hasMedia := len(mediaFiles(am.torrent.Files())) > 0
		if am.mediaOnly && !hasMedia {
			return mutedStyle.Render("No media files found. Press Tab to show all files") + "\n"
		} else if strings.TrimSpace(am.filterInput.Value()) != "" {
			return mutedStyle.Render("No files match the filter") + "\n"
		}
		return mutedStyle.Render("No files") + "\n"
	}

	start := min(am.scrollOffset, len(files)-1)
	end := start + visibleRows
	if end > len(files) {
		end = len(files)
	}

	renderState := am.state
	if am.state == stateSettings {
		renderState = am.prevState
	}

	var ret string
	for i, f := range files[start:end] {
		name := f.DisplayPath()
		if am.cursor == i+start {
			if renderState == stateSelect {
				ret += selectedStyle.Render("▶ "+marquee(name, maxLen, am.tickCount)) + "\n"
			} else {
				ret += mutedStyle.Render("▶ "+marquee(name, maxLen, am.tickCount)) + "\n"
			}
		} else {
			if renderState == stateSelect {
				ret += normalStyle.Render("  "+truncate(name, maxLen)) + "\n"
			} else {
				ret += mutedStyle.Render("  "+truncate(name, maxLen)) + "\n"
			}
		}
	}

	return ret
}

func (am appModel) statsView(width int) string {
	renderState := am.state
	if am.state == stateSettings {
		renderState = am.prevState
	}

	status := headerStyle.Render("Status: ")
	if am.done {
		status += infoStyle.Render("Playing on mpv")
	} else if renderState == stateBuffer {
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

	files := am.visibleFiles()
	fileInfo := selectedStyle.Render("No file selected")
	if am.cursor < len(files) {
		f := files[am.cursor]
		filename := filepath.Base(f.DisplayPath())
		fileDir := filepath.Dir(f.DisplayPath())
		fileExt := filepath.Ext(f.DisplayPath())
		if fileExt != "" {
			fileExt = fileExt[1:]
		}
		fileSize := f.Length()

		fieldLines := []string{
			headerStyle.Underline(true).Render("FILE INFO"),
			formatLine("Name", filename, infoStyle, selectedStyle, width, am.tickCount),
			formatLine("Directory", fileDir, infoStyle, normalStyle, width, am.tickCount),
			formatLine("Size", humanSize(fileSize), infoStyle, normalStyle, width, am.tickCount),
			formatLine("Type", fileExt, infoStyle, normalStyle, width, am.tickCount),
		}
		if am.config.ResumePlayback && am.resumeStore != nil {
			if entry, ok := am.resumeStore.get(am.torrent.InfoHash().HexString(), f.DisplayPath()); ok {
				progressInfo := fmt.Sprintf("%s / %s", formatDuration(entry.Position), formatDuration(entry.Duration))
				fieldLines = append(fieldLines, formatLine("Progress", progressInfo, infoStyle, normalStyle, width, am.tickCount))
			}
		}

		fileInfo = lipgloss.JoinVertical(lipgloss.Left, fieldLines...)
	}

	peersStr := fmt.Sprintf("%d/%d", am.torrent.Stats().ActivePeers, am.torrent.Stats().TotalPeers)
	downloadSpeedStr := fmt.Sprintf("%s/s", humanSize(am.downloadSpeed))

	var swarmStats string
	if renderState == stateBuffer {
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
	maxWidth := max(1, am.width-6)
	footer := "  "

	appendHint := func(footer, hintStr string) string {
		candidate := footer + "  " + hintStr
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
		return footer
	}

	if am.state == stateSettings {
		for _, h := range settingsHints {
			footer = appendHint(footer, formatHint(am, h))
		}
	} else if am.filterActive {
		fi := am.filterInput
		fi.SetWidth(max(1, maxWidth-6))
		footer = lipgloss.JoinHorizontal(lipgloss.Center, footer, "  ", fi.View())
	} else {
		query := strings.TrimSpace(am.filterInput.Value())
		if query != "" {
			filterInfo := normalStyle.Render("Filter: ") + mutedStyle.Render(truncate(query, 10))
			footer = appendHint(footer, filterInfo)
		}
		for _, h := range baseHints {
			if h.when == nil || h.when(am) {
				footer = appendHint(footer, formatHint(am, h))
			}
		}
	}

	return footer
}

func formatHint(am appModel, h hint) string {
	return lipgloss.JoinHorizontal(lipgloss.Center, normalStyle.Render(h.key), " ", mutedStyle.Render(h.action(am)))
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
