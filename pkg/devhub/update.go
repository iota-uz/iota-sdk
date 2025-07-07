package devhub

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
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

		case "enter", " ":
			if m.SelectedIndex >= 0 && m.SelectedIndex < len(m.Services) {
				go func() {
					m.ServiceManager.ToggleService(m.SelectedIndex)
				}()
			}
			return m, nil
		}

	case StatusUpdateMsg:
		for i, service := range m.Services {
			if service.Name == msg.ServiceName {
				m.Services[i].Status = msg.Status
				m.Services[i].ErrorMsg = msg.ErrorMsg
				break
			}
		}
		return m, nil

	case TickMsg:
		m.Services = m.ServiceManager.GetServices()
		return m, tea.Batch(tickCmd(), m.ServiceManager.StartMonitoring())
	}

	return m, nil
}
