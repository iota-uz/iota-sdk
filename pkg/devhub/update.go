package devhub

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.ViewMode == LogView {
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
		for _, update := range msg.Updates {
			for i, service := range m.Services {
				if service.Name == update.ServiceName {
					m.Services[i].Status = update.Status
					m.Services[i].ErrorMsg = update.ErrorMsg
					break
				}
			}
		}
		return m, m.ServiceManager.StartMonitoring()

	case TickMsg:
		m.Services = m.ServiceManager.GetServices()
		m.SpinnerFrame++
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, tea.Batch(tickCmd(), m.ServiceManager.StartMonitoring(), cmd)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *Model) updateServiceView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.ServiceManager.Shutdown()
		return m, tea.Quit

	case "up", "k":
		if m.SelectedIndex > 0 {
			m.SelectedIndex--
		}
		return m, nil

	case "down", "j":
		if m.SelectedIndex < len(m.Services)-1 {
			m.SelectedIndex++
		}
		return m, nil

	case "enter":
		if m.SelectedIndex >= 0 && m.SelectedIndex < len(m.Services) {
			m.ViewMode = LogView
			m.LogViewService = m.SelectedIndex
			m.LogViewScrollPos = 0
			m.AutoScroll = true // Enable auto-scroll by default
		}
		return m, nil

	case " ":
		if m.SelectedIndex >= 0 && m.SelectedIndex < len(m.Services) {
			go func() {
				_ = m.ServiceManager.ToggleService(m.SelectedIndex)
			}()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) updateLogView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search mode input
	if m.SearchMode {
		switch msg.String() {
		case "enter":
			// Exit search mode
			m.SearchMode = false
			return m, nil

		case "esc":
			// Cancel search
			m.SearchMode = false
			m.SearchQuery = ""
			m.SearchMatches = nil
			m.CurrentMatch = 0
			return m, nil

		case "backspace":
			if len(m.SearchQuery) > 0 {
				m.SearchQuery = m.SearchQuery[:len(m.SearchQuery)-1]
				m.updateSearchMatches()
			}
			return m, nil

		case "n":
			// Next match
			if len(m.SearchMatches) > 0 {
				m.CurrentMatch = (m.CurrentMatch + 1) % len(m.SearchMatches)
				m.LogViewScrollPos = m.SearchMatches[m.CurrentMatch]
				m.AutoScroll = false
			}
			return m, nil

		case "N":
			// Previous match
			if len(m.SearchMatches) > 0 {
				m.CurrentMatch = (m.CurrentMatch - 1 + len(m.SearchMatches)) % len(m.SearchMatches)
				m.LogViewScrollPos = m.SearchMatches[m.CurrentMatch]
				m.AutoScroll = false
			}
			return m, nil

		default:
			// Add character to search query
			if len(msg.String()) == 1 && msg.String()[0] >= 32 && msg.String()[0] < 127 {
				m.SearchQuery += msg.String()
				m.updateSearchMatches()
			}
			return m, nil
		}
	}

	// Normal log view controls
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.ViewMode = ServiceView
		return m, nil

	case "up", "k":
		if m.LogViewScrollPos > 0 {
			m.LogViewScrollPos--
			// Disable auto-scroll when manually scrolling up
			m.AutoScroll = false
		}
		return m, nil

	case "down", "j":
		// Allow scrolling down until the last line is visible
		m.LogViewScrollPos++

		// Re-enable auto-scroll if we've scrolled to the bottom
		if len(m.LogLines) > 0 {
			logContentHeight := m.Height - 3
			if m.LogViewScrollPos >= len(m.LogLines)-logContentHeight {
				m.AutoScroll = true
			}
		}
		return m, nil

	case "left", "h":
		m.LogViewService = (m.LogViewService - 1 + len(m.Services)) % len(m.Services)
		m.LogViewScrollPos = 0
		// Keep auto-scroll state when switching services
		return m, nil

	case "right", "l":
		m.LogViewService = (m.LogViewService + 1) % len(m.Services)
		m.LogViewScrollPos = 0
		// Keep auto-scroll state when switching services
		return m, nil

	case "f":
		// Toggle auto-scroll
		m.AutoScroll = !m.AutoScroll
		return m, nil

	case "c":
		// Clear logs for the current service
		m.ServiceManager.ClearLogs(m.LogViewService)
		m.LogViewScrollPos = 0
		return m, nil

	case "/":
		// Enter search mode
		m.SearchMode = true
		m.SearchQuery = ""
		m.SearchMatches = nil
		m.CurrentMatch = 0
		m.AutoScroll = false
		return m, nil
	}
	return m, nil
}

// Helper function to update search matches
func (m *Model) updateSearchMatches() {
	m.SearchMatches = nil
	m.CurrentMatch = 0

	if m.SearchQuery == "" || m.LogLines == nil {
		return
	}

	lowerQuery := strings.ToLower(m.SearchQuery)
	for i, line := range m.LogLines {
		if strings.Contains(strings.ToLower(line), lowerQuery) {
			m.SearchMatches = append(m.SearchMatches, i)
		}
	}

	// If we have matches, scroll to the first one
	if len(m.SearchMatches) > 0 {
		m.LogViewScrollPos = m.SearchMatches[0]
	}
}
