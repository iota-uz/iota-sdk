package devhub

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Padding(0, 1)

	serviceStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Margin(0, 1)

	selectedServiceStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57")).
				Bold(true).
				Padding(0, 2).
				Margin(0, 1)

	statusRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("40")).
				Bold(true)

	statusStoppedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Bold(true)

	statusStartingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")).
				Bold(true)

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Padding(1, 0)
)

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Development Tools Manager"))
	b.WriteString("\n\n")

	for i, service := range m.Services {
		var style lipgloss.Style
		if i == m.SelectedIndex {
			style = selectedServiceStyle
		} else {
			style = serviceStyle
		}

		statusStr := renderStatus(service.Status)

		line := fmt.Sprintf("%-20s %s", service.Name, statusStr)
		if service.Port != "" {
			line += fmt.Sprintf(" (:%s)", service.Port)
		}

		if service.ErrorMsg != "" {
			line += fmt.Sprintf(" - %s", service.ErrorMsg)
		}

		b.WriteString(style.Render(line))
		b.WriteString("\n")

		if service.Description != "" {
			desc := fmt.Sprintf("  %s", service.Description)
			if i == m.SelectedIndex {
				desc = selectedServiceStyle.Copy().Faint(true).Render(desc)
			} else {
				desc = serviceStyle.Copy().Faint(true).Render(desc)
			}
			b.WriteString(desc)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: Navigate • Space/Enter: Toggle • q: Quit"))

	return b.String()
}

func renderStatus(status ServiceStatus) string {
	switch status {
	case StatusRunning:
		return statusRunningStyle.Render("● Running")
	case StatusStopped:
		return statusStoppedStyle.Render("○ Stopped")
	case StatusStarting:
		return statusStartingStyle.Render("◐ Starting")
	case StatusStopping:
		return statusStartingStyle.Render("◑ Stopping")
	case StatusError:
		return statusErrorStyle.Render("✗ Error")
	default:
		return statusStoppedStyle.Render("○ Unknown")
	}
}
