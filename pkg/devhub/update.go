package devhub

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.UI.Width = msg.Width
		m.UI.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.UI.ViewMode == LogView {
			return m.updateLogView(msg)
		}
		return m.updateServiceView(msg)

	case StatusUpdateMsg:
		for i, service := range m.Services {
			if service.Name == msg.ServiceName {
				m.Services[i].Status = msg.Status
				m.Services[i].ErrorMsg = msg.ErrorMsg
				break
			}
		}
		return m, nil

	case BatchStatusUpdateMsg:
		// No longer needed - StateManager handles updates
		return m, nil

	case TickMsg:
		// Only update UI elements, not service data
		m.UI.SpinnerFrame++
		var cmd tea.Cmd
		m.UI.Spinner, cmd = m.UI.Spinner.Update(msg)
		// Get latest state from StateManager (non-blocking)
		m.Services = m.StateManager.GetServices()
		m.SystemStats = m.StateManager.GetSystemStats()
		return m, tea.Batch(tickCmd(), cmd)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.UI.Spinner, cmd = m.UI.Spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *Model) updateServiceView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		if m.UI.SelectedIndex > 0 {
			m.UI.SelectedIndex--
		}
		return m, nil

	case "down", "j":
		if m.UI.SelectedIndex < len(m.Services)-1 {
			m.UI.SelectedIndex++
		}
		return m, nil

	case "enter":
		if m.UI.SelectedIndex >= 0 && m.UI.SelectedIndex < len(m.Services) {
			m.UI.ViewMode = LogView
			m.LogView.ServiceIndex = m.UI.SelectedIndex
			m.LogView.ScrollPos = 0
			m.LogView.AutoScroll = true // Enable auto-scroll by default
		}
		return m, nil

	case " ":
		if m.UI.SelectedIndex >= 0 && m.UI.SelectedIndex < len(m.Services) {
			go func() {
				_ = m.ServiceManager.ToggleService(m.UI.SelectedIndex)
			}()
		}
		return m, nil

	case "r", "R":
		if m.UI.SelectedIndex >= 0 && m.UI.SelectedIndex < len(m.Services) {
			go func() {
				_ = m.ServiceManager.RestartService(m.UI.SelectedIndex)
			}()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) updateLogView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search mode input
	if m.Search.Active {
		switch msg.String() {
		case "enter":
			// Exit search mode
			m.Search.Active = false
			return m, nil

		case "esc":
			// Cancel search
			m.Search.Active = false
			m.Search.Query = ""
			m.Search.Matches = nil
			m.Search.CurrentMatch = 0
			return m, nil

		case "backspace":
			if len(m.Search.Query) > 0 {
				m.Search.Query = m.Search.Query[:len(m.Search.Query)-1]
				m.updateSearchMatches()
			}
			return m, nil

		case "n":
			// Next match
			if len(m.Search.Matches) > 0 {
				m.Search.CurrentMatch = (m.Search.CurrentMatch + 1) % len(m.Search.Matches)
				m.LogView.ScrollPos = m.Search.Matches[m.Search.CurrentMatch]
				m.LogView.AutoScroll = false
			}
			return m, nil

		case "N":
			// Previous match
			if len(m.Search.Matches) > 0 {
				m.Search.CurrentMatch = (m.Search.CurrentMatch - 1 + len(m.Search.Matches)) % len(m.Search.Matches)
				m.LogView.ScrollPos = m.Search.Matches[m.Search.CurrentMatch]
				m.LogView.AutoScroll = false
			}
			return m, nil

		default:
			// Add character to search query
			if len(msg.String()) == 1 && msg.String()[0] >= 32 && msg.String()[0] < 127 {
				m.Search.Query += msg.String()
				m.updateSearchMatches()
			}
			return m, nil
		}
	}

	// Normal log view controls
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.UI.ViewMode = ServiceView
		return m, nil

	case "up", "k":
		if m.LogView.ScrollPos > 0 {
			m.LogView.ScrollPos--
			// Disable auto-scroll when manually scrolling up
			m.LogView.AutoScroll = false
		}
		return m, nil

	case "down", "j":
		// Allow scrolling down until the last line is visible
		m.LogView.ScrollPos++

		// Re-enable auto-scroll if we've scrolled to the bottom
		if len(m.LogView.Lines) > 0 {
			logContentHeight := m.UI.Height - 3
			if m.LogView.ScrollPos >= len(m.LogView.Lines)-logContentHeight {
				m.LogView.AutoScroll = true
			}
		}
		return m, nil

	case "left", "h":
		m.LogView.ServiceIndex = (m.LogView.ServiceIndex - 1 + len(m.Services)) % len(m.Services)
		m.LogView.ScrollPos = 0
		// Keep auto-scroll state when switching services
		return m, nil

	case "right", "l":
		m.LogView.ServiceIndex = (m.LogView.ServiceIndex + 1) % len(m.Services)
		m.LogView.ScrollPos = 0
		// Keep auto-scroll state when switching services
		return m, nil

	case "f":
		// Toggle auto-scroll
		m.LogView.AutoScroll = !m.LogView.AutoScroll
		return m, nil

	case "c":
		// Clear logs for the current service
		m.ServiceManager.ClearLogs(m.LogView.ServiceIndex)
		m.LogView.ScrollPos = 0
		return m, nil

	case "/":
		// Enter search mode
		m.Search.Active = true
		m.Search.Query = ""
		m.Search.Matches = nil
		m.Search.CurrentMatch = 0
		m.LogView.AutoScroll = false
		return m, nil
	}
	return m, nil
}

// Helper function to update search matches
func (m *Model) updateSearchMatches() {
	m.Search.Matches = nil
	m.Search.CurrentMatch = 0

	if m.Search.Query == "" || m.LogView.Lines == nil {
		return
	}

	lowerQuery := strings.ToLower(m.Search.Query)
	for i, line := range m.LogView.Lines {
		if strings.Contains(strings.ToLower(line), lowerQuery) {
			m.Search.Matches = append(m.Search.Matches, i)
		}
	}

	// If we have matches, scroll to the first one
	if len(m.Search.Matches) > 0 {
		m.LogView.ScrollPos = m.Search.Matches[0]
	}
}
