package devhub

import (
	"fmt"
	"strings"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/charmbracelet/lipgloss"
	"github.com/iota-uz/iota-sdk/pkg/devhub/services"
)

var (
	serviceStyle = lipgloss.NewStyle().
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

	statusQueuedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#94A3B8")).
				Italic(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#868E96")).
			Padding(1, 1)

	logHeaderStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#333333")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	metadataStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#868E96"))

	selectorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C7CFA")).
			Bold(true)
)

func formatUptime(startTime *time.Time) string {
	if startTime == nil {
		return ""
	}

	duration := time.Since(*startTime)
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm%ds", int(duration.Minutes()), int(duration.Seconds())%60)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else {
		days := int(duration.Hours()) / 24
		hours := int(duration.Hours()) % 24
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}

func formatResourceUsage(cpu float64, memory float64) string {
	if cpu == 0 && memory == 0 {
		return ""
	}
	return fmt.Sprintf("CPU: %.1f%% | Mem: %.1fMB", cpu, memory)
}

func (m Model) View() string {
	if m.UI.ViewMode == LogView {
		return m.logView()
	}
	return m.serviceListView()
}

func (m Model) serviceListView() string {
	var b strings.Builder

	// Calculate the width for full-width rendering
	width := m.UI.Width
	if width == 0 {
		width = 80 // Default width
	}

	// Calculate stats first
	serviceCount := len(m.Services)
	runningCount := 0
	var totalCPU float64
	var totalMemory float64

	for _, s := range m.Services {
		if s.Status == services.StatusRunning {
			runningCount++
			totalCPU += s.CPUPercent
			totalMemory += s.MemoryMB
		}
	}

	// Convert large memory values to GB
	systemMemoryDisplay := fmt.Sprintf("%.1f MB", m.SystemStats.MemoryMB)
	if m.SystemStats.MemoryMB > 1024 {
		systemMemoryDisplay = fmt.Sprintf("%.1f GB", m.SystemStats.MemoryMB/1024)
	}

	// Build the header box
	boxWidth := 50
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5C7CFA"))

	// Title section
	titleLine1 := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B6B")).
		Bold(true).
		Render("🚀 DevHub")
	titleLine2 := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#868E96")).
		Render("Development Tools Manager")

	// Status section
	statusLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5C7CFA")).
		Render(fmt.Sprintf("📦 %d services  •  ✅ %d running", serviceCount, runningCount))

	// Resource section
	resourceStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#868E96"))

	var servicesLine string
	if runningCount > 0 {
		servicesLine = fmt.Sprintf("Services:  ⚡ %.1f%%  💾 %.1f MB", totalCPU, totalMemory)
	} else {
		servicesLine = "Services:  ⚡ 0.0%  💾 0.0 MB"
	}

	systemLine := fmt.Sprintf("System:    ⚡ %.1f%%  💾 %s (%.0f%%)",
		m.SystemStats.CPUPercent, systemMemoryDisplay, m.SystemStats.MemoryPercent)

	// Create the box
	var box strings.Builder

	// Top border
	box.WriteString(borderStyle.Render("╭" + strings.Repeat("─", boxWidth-2) + "╮"))
	box.WriteString("\n")

	// Title section
	box.WriteString(borderStyle.Render("│"))
	box.WriteString(lipgloss.PlaceHorizontal(boxWidth-2, lipgloss.Center, titleLine1))
	box.WriteString(borderStyle.Render("│"))
	box.WriteString("\n")

	box.WriteString(borderStyle.Render("│"))
	box.WriteString(lipgloss.PlaceHorizontal(boxWidth-2, lipgloss.Center, titleLine2))
	box.WriteString(borderStyle.Render("│"))
	box.WriteString("\n")

	// Divider
	box.WriteString(borderStyle.Render("├" + strings.Repeat("─", boxWidth-2) + "┤"))
	box.WriteString("\n")

	// Status section
	box.WriteString(borderStyle.Render("│"))
	box.WriteString(lipgloss.PlaceHorizontal(boxWidth-2, lipgloss.Center, statusLine))
	box.WriteString(borderStyle.Render("│"))
	box.WriteString("\n")

	// Divider
	box.WriteString(borderStyle.Render("├" + strings.Repeat("─", boxWidth-2) + "┤"))
	box.WriteString("\n")

	// Resource section
	box.WriteString(borderStyle.Render("│"))
	box.WriteString(lipgloss.PlaceHorizontal(boxWidth-2, lipgloss.Left, "  "+resourceStyle.Render(servicesLine)))
	box.WriteString(borderStyle.Render("│"))
	box.WriteString("\n")

	box.WriteString(borderStyle.Render("│"))
	box.WriteString(lipgloss.PlaceHorizontal(boxWidth-2, lipgloss.Left, "  "+resourceStyle.Render(systemLine)))
	box.WriteString(borderStyle.Render("│"))
	box.WriteString("\n")

	// Bottom border
	box.WriteString(borderStyle.Render("╰" + strings.Repeat("─", boxWidth-2) + "╯"))

	// Center the entire box
	headerContainer := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		MarginTop(1).
		MarginBottom(2)

	b.WriteString(headerContainer.Render(box.String()))
	b.WriteString("\n")

	for i, service := range m.Services {
		// Build the service line
		statusStr := m.renderStatusWithSpinner(service.Status, service.HealthStatus)

		serviceName := service.Name
		if service.Port != "" {
			serviceName += fmt.Sprintf(" (:%s)", service.Port)
		}

		// Add selection indicator
		var lineBuilder strings.Builder
		if i == m.UI.SelectedIndex {
			lineBuilder.WriteString(selectorStyle.Render("▶ "))
		} else {
			lineBuilder.WriteString("  ")
		}

		// Calculate spacing for right-aligned status
		baseContent := "  " + serviceName + "  " + stripansi.Strip(statusStr)
		spacing := width - len(baseContent) - 4 // Account for padding
		if spacing < 2 {
			spacing = 2
		}

		lineBuilder.WriteString(serviceName)
		lineBuilder.WriteString(strings.Repeat(" ", spacing))
		lineBuilder.WriteString(statusStr)

		if service.ErrorMsg != "" {
			lineBuilder.WriteString(fmt.Sprintf(" - %s", service.ErrorMsg))
		}

		// Apply styling - use normal style for all, just with indicator
		b.WriteString(serviceStyle.Render(lineBuilder.String()))
		b.WriteString("\n")

		// Add description if present
		if service.Description != "" {
			desc := "  " + service.Description // Indent to align with service name
			descStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#868E96")).
				Padding(0, 2)
			b.WriteString(descStyle.Render(desc))
			b.WriteString("\n")
		}

		// Add metadata line if service is running
		if service.Status == services.StatusRunning || service.Status == services.StatusStarting {
			var metadata []string

			// Add PID
			if service.PID > 0 {
				metadata = append(metadata, fmt.Sprintf("PID: %d", service.PID))
			}

			// Add uptime
			if uptime := formatUptime(service.StartTime); uptime != "" {
				metadata = append(metadata, fmt.Sprintf("Uptime: %s", uptime))
			}

			// Add resource usage
			if resources := formatResourceUsage(service.CPUPercent, service.MemoryMB); resources != "" {
				metadata = append(metadata, resources)
			}

			// Add dependencies
			if len(service.DependsOn) > 0 {
				metadata = append(metadata, fmt.Sprintf("Depends on: %s", strings.Join(service.DependsOn, ", ")))
			}

			if len(metadata) > 0 {
				metaLine := "  " + strings.Join(metadata, " | ") // Indent to align
				metaStyle := metadataStyle.Padding(0, 2)
				b.WriteString(metaStyle.Render(metaLine))
				b.WriteString("\n")
			}
		}

		// Add spacing between services
		if i < len(m.Services)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: Navigate • Space: Toggle • r: Restart • Enter: View Logs • q: Quit"))

	return b.String()
}

func (m Model) logView() string {
	var b strings.Builder

	width := m.UI.Width
	if width == 0 {
		width = 80 // Default width
	}
	height := m.UI.Height
	if height == 0 {
		height = 24 // Default height
	}

	service := m.Services[m.LogView.ServiceIndex]

	// Get logs and use cache for efficient parsing
	logs := m.ServiceManager.Logs(m.LogView.ServiceIndex)
	logLines := m.LogCache.GetLines(m.LogView.ServiceIndex, logs)

	// Update the cached log lines
	m.LogView.Lines = logLines

	headerText := fmt.Sprintf("Logs: %s (%d lines)", service.Name, len(logLines))
	if m.LogView.AutoScroll {
		headerText += " [📍 Following]"
	} else {
		headerText += " [⏸ Paused]"
	}

	b.WriteString(logHeaderStyle.Width(width).Render(headerText))
	b.WriteString("\n")

	// Show search bar if in search mode
	if m.Search.Active {
		searchStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("#333333")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)

		searchText := fmt.Sprintf("Search: %s", m.Search.Query)
		if len(m.Search.Matches) > 0 {
			searchText += fmt.Sprintf(" (%d/%d matches)", m.Search.CurrentMatch+1, len(m.Search.Matches))
		} else if m.Search.Query != "" {
			searchText += " (no matches)"
		}

		b.WriteString(searchStyle.Width(width).Render(searchText))
		b.WriteString("\n")
	}

	logContentHeight := height - 3 // Header and footer
	if m.Search.Active {
		logContentHeight = height - 4 // Header, search bar, and footer
	}

	// If auto-scroll is enabled, scroll to bottom
	if m.LogView.AutoScroll && len(logLines) > 0 {
		m.LogView.ScrollPos = len(logLines) - logContentHeight
		if m.LogView.ScrollPos < 0 {
			m.LogView.ScrollPos = 0
		}
	}

	// Get only visible lines using cache for efficiency
	visibleLines, adjustedScrollPos := m.LogCache.GetVisibleLines(
		m.LogView.ServiceIndex,
		logs,
		m.LogView.ScrollPos,
		logContentHeight,
	)
	m.LogView.ScrollPos = adjustedScrollPos

	// Highlight style for search matches
	highlightStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#FFD43B")).
		Foreground(lipgloss.Color("#000000"))

	for _, line := range visibleLines {
		// Highlight search matches if in search mode and query is not empty
		if m.Search.Query != "" && strings.Contains(strings.ToLower(line), strings.ToLower(m.Search.Query)) {
			// Simple case-insensitive highlighting
			lowerLine := strings.ToLower(line)
			lowerQuery := strings.ToLower(m.Search.Query)
			idx := strings.Index(lowerLine, lowerQuery)

			if idx >= 0 {
				before := line[:idx]
				match := line[idx : idx+len(m.Search.Query)]
				after := line[idx+len(m.Search.Query):]

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
	for i := 0; i < logContentHeight-len(visibleLines); i++ {
		b.WriteString("\n")
	}

	footer := "↑/↓: Scroll • ←/→: Switch Service • f: Follow • c: Clear • /: Search • Esc/q: Back"
	if m.Search.Active {
		footer = "Enter: Exit Search • n/N: Next/Prev Match • Esc: Cancel Search"
	}
	b.WriteString(helpStyle.Render(footer))

	return b.String()
}

func (m Model) renderStatusWithSpinner(status services.ServiceStatus, healthStatus services.HealthStatus) string {
	switch status {
	case services.StatusRunning:
		// Integrate health status into running state
		switch healthStatus {
		case services.HealthUnknown:
			return statusRunningStyle.Render("◉ Running")
		case services.HealthHealthy:
			return statusRunningStyle.Render("◉ Running")
		case services.HealthStarting:
			spinner := spinnerFrames[m.UI.SpinnerFrame%len(spinnerFrames)]
			return statusStartingStyle.Render(spinner + " Starting")
		case services.HealthUnhealthy:
			return statusErrorStyle.Render("✘ Unhealthy")
		default:
			return statusRunningStyle.Render("◉ Running")
		}
	case services.StatusStopped:
		return statusStoppedStyle.Render("◯ Stopped")
	case services.StatusQueued:
		return statusQueuedStyle.Render("⏸ Queued")
	case services.StatusStarting:
		spinner := spinnerFrames[m.UI.SpinnerFrame%len(spinnerFrames)]
		return statusStartingStyle.Render(spinner + " Starting")
	case services.StatusStopping:
		spinner := spinnerFrames[m.UI.SpinnerFrame%len(spinnerFrames)]
		return statusStartingStyle.Render(spinner + " Stopping")
	case services.StatusError:
		return statusErrorStyle.Render("✘ Error")
	default:
		return statusStoppedStyle.Render("◯ Unknown")
	}
}
