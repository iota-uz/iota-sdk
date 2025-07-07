package devhub

import (
	"fmt"
	"strings"

	"github.com/acarl005/stripansi"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true).
			Padding(0, 1).
			MarginBottom(1)

	serviceStyle = lipgloss.NewStyle().
			Padding(0, 2)

	selectedServiceStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#5C7CFA")).
				Padding(0, 2)

	statusRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#51CF66")).
				Bold(true)

	statusStoppedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#868E96"))

	statusStartingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFD43B")).
				Bold(true)

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6B6B")).
				Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#868E96")).
			Padding(1, 1)

	logHeaderStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#333333")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

func (m Model) View() string {
	if m.ViewMode == LogView {
		return m.logView()
	}
	return m.serviceListView()
}

func (m Model) serviceListView() string {
	var b strings.Builder

	// Calculate the width for full-width rendering
	width := m.Width
	if width == 0 {
		width = 80 // Default width
	}

	b.WriteString(titleStyle.Render("Development Tools Manager"))
	b.WriteString("\n\n")

	for i, service := range m.Services {
		// Build the service line
		statusStr := m.renderStatusWithSpinner(service.Status)

		serviceName := service.Name
		if service.Port != "" {
			serviceName += fmt.Sprintf(" (:%s)", service.Port)
		}

		// Calculate spacing for right-aligned status
		baseContent := serviceName + "  " + stripansi.Strip(statusStr)
		spacing := width - len(baseContent) - 4 // Account for padding
		if spacing < 2 {
			spacing = 2
		}

		line := serviceName + strings.Repeat(" ", spacing) + statusStr

		if service.ErrorMsg != "" {
			line += fmt.Sprintf(" - %s", service.ErrorMsg)
		}

		// Apply styling
		if i == m.SelectedIndex {
			// Full width highlighting
			renderedLine := selectedServiceStyle.Width(width).Render(line)
			b.WriteString(renderedLine)
		} else {
			b.WriteString(serviceStyle.Render(line))
		}
		b.WriteString("\n")

		// Add description if present
		if service.Description != "" {
			desc := "  " + service.Description
			if i == m.SelectedIndex {
				descStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("#E9ECEF")).
					Background(lipgloss.Color("#5C7CFA")).
					Padding(0, 2).
					Width(width)
				b.WriteString(descStyle.Render(desc))
			} else {
				descStyle := serviceStyle.
					Foreground(lipgloss.Color("#868E96"))
				b.WriteString(descStyle.Render(desc))
			}
			b.WriteString("\n")
		}

		// Add spacing between services
		if i < len(m.Services)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: Navigate • Space: Toggle • Enter: View Logs • q: Quit"))

	return b.String()
}

func (m Model) logView() string {
	var b strings.Builder

	width := m.Width
	if width == 0 {
		width = 80 // Default width
	}
	height := m.Height
	if height == 0 {
		height = 24 // Default height
	}

	service := m.Services[m.LogViewService]

	// Build header with auto-scroll indicator and line count
	logs := m.ServiceManager.Logs(m.LogViewService)
	logLines := strings.Split(string(logs), "\n")

	headerText := fmt.Sprintf("Logs: %s (%d lines)", service.Name, len(logLines))
	if m.AutoScroll {
		headerText += " [📍 Following]"
	} else {
		headerText += " [⏸ Paused]"
	}

	b.WriteString(logHeaderStyle.Width(width).Render(headerText))
	b.WriteString("\n")

	// Show search bar if in search mode
	if m.SearchMode {
		searchStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("#333333")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)

		searchText := fmt.Sprintf("Search: %s", m.SearchQuery)
		if len(m.SearchMatches) > 0 {
			searchText += fmt.Sprintf(" (%d/%d matches)", m.CurrentMatch+1, len(m.SearchMatches))
		} else if m.SearchQuery != "" {
			searchText += " (no matches)"
		}

		b.WriteString(searchStyle.Width(width).Render(searchText))
		b.WriteString("\n")
	}

	logContentHeight := height - 3 // Header and footer
	if m.SearchMode {
		logContentHeight = height - 4 // Header, search bar, and footer
	}

	// If auto-scroll is enabled, scroll to bottom
	if m.AutoScroll && len(logLines) > 0 {
		m.LogViewScrollPos = len(logLines) - logContentHeight
		if m.LogViewScrollPos < 0 {
			m.LogViewScrollPos = 0
		}
	}

	// Ensure scroll position is valid
	if m.LogViewScrollPos >= len(logLines) {
		m.LogViewScrollPos = len(logLines) - 1
	}
	if m.LogViewScrollPos < 0 {
		m.LogViewScrollPos = 0
	}

	start := m.LogViewScrollPos
	end := start + logContentHeight
	if end > len(logLines) {
		end = len(logLines)
	}

	// Highlight style for search matches
	highlightStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#FFD43B")).
		Foreground(lipgloss.Color("#000000"))

	for i := start; i < end; i++ {
		line := logLines[i]

		// Highlight search matches if in search mode and query is not empty
		if m.SearchQuery != "" && strings.Contains(strings.ToLower(line), strings.ToLower(m.SearchQuery)) {
			// Simple case-insensitive highlighting
			lowerLine := strings.ToLower(line)
			lowerQuery := strings.ToLower(m.SearchQuery)
			idx := strings.Index(lowerLine, lowerQuery)

			if idx >= 0 {
				before := line[:idx]
				match := line[idx : idx+len(m.SearchQuery)]
				after := line[idx+len(m.SearchQuery):]

				b.WriteString(before)
				b.WriteString(highlightStyle.Render(match))
				b.WriteString(after)
			} else {
				b.WriteString(line)
			}
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	// Fill remaining space
	for i := 0; i < logContentHeight-(end-start); i++ {
		b.WriteString("\n")
	}

	footer := "↑/↓: Scroll • ←/→: Switch Service • f: Follow • c: Clear • /: Search • Esc/q: Back"
	if m.SearchMode {
		footer = "Enter: Exit Search • n/N: Next/Prev Match • Esc: Cancel Search"
	}
	b.WriteString(helpStyle.Render(footer))

	return b.String()
}

func (m Model) renderStatusWithSpinner(status ServiceStatus) string {
	switch status {
	case StatusRunning:
		return statusRunningStyle.Render("◉ Running")
	case StatusStopped:
		return statusStoppedStyle.Render("◯ Stopped")
	case StatusStarting:
		spinner := spinnerFrames[m.SpinnerFrame%len(spinnerFrames)]
		return statusStartingStyle.Render(spinner + " Starting")
	case StatusStopping:
		spinner := spinnerFrames[m.SpinnerFrame%len(spinnerFrames)]
		return statusStartingStyle.Render(spinner + " Stopping")
	case StatusError:
		return statusErrorStyle.Render("✘ Error")
	default:
		return statusStoppedStyle.Render("◯ Unknown")
	}
}
