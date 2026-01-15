package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Tab represents a single tab with a title and content generator.
type Tab struct {
	Title   string
	Content func() string // Dynamic content generator
}

// TabsModel manages a tabbed interface.
type TabsModel struct {
	tabs      []Tab
	activeTab int
	width     int
	height    int
}

// NewTabsModel creates a new tabs model.
func NewTabsModel(tabs []Tab, width, height int) *TabsModel {
	return &TabsModel{
		tabs:      tabs,
		activeTab: 0,
		width:     width,
		height:    height,
	}
}

// SetSize updates the size of the tabs model.
func (m *TabsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// HandleKey processes key input for tab navigation.
// Returns true if the key was handled.
func (m *TabsModel) HandleKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "left", "h":
		m.PrevTab()
		return true
	case "right", "l":
		m.NextTab()
		return true
	case "1":
		m.SetActiveTab(0)
		return true
	case "2":
		if len(m.tabs) > 1 {
			m.SetActiveTab(1)
			return true
		}
	case "3":
		if len(m.tabs) > 2 {
			m.SetActiveTab(2)
			return true
		}
	case "4":
		if len(m.tabs) > 3 {
			m.SetActiveTab(3)
			return true
		}
	case "5":
		if len(m.tabs) > 4 {
			m.SetActiveTab(4)
			return true
		}
	}
	return false
}

// NextTab moves to the next tab, wrapping around.
func (m *TabsModel) NextTab() {
	m.activeTab = (m.activeTab + 1) % len(m.tabs)
}

// PrevTab moves to the previous tab, wrapping around.
func (m *TabsModel) PrevTab() {
	m.activeTab--
	if m.activeTab < 0 {
		m.activeTab = len(m.tabs) - 1
	}
}

// SetActiveTab sets the active tab by index.
func (m *TabsModel) SetActiveTab(index int) {
	if index >= 0 && index < len(m.tabs) {
		m.activeTab = index
	}
}

// GetActiveTab returns the index of the currently active tab.
func (m *TabsModel) GetActiveTab() int {
	return m.activeTab
}

// GetActiveContent returns the content of the currently active tab.
func (m *TabsModel) GetActiveContent() string {
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		return m.tabs[m.activeTab].Content()
	}
	return ""
}

// renderTabBar renders the tab bar with highlighting for the active tab.
func (m *TabsModel) renderTabBar() string {
	if len(m.tabs) == 0 {
		return ""
	}

	renderedTabs := make([]string, 0, len(m.tabs))

	for i, tab := range m.tabs {
		var style lipgloss.Style
		if i == m.activeTab {
			// Active tab: highlighted
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")). // White
				Background(ColorPrimary).
				Bold(true).
				Padding(0, 2)
		} else {
			// Inactive tab: muted
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Gray
				Padding(0, 2)
		}

		renderedTabs = append(renderedTabs, style.Render(tab.Title))
	}

	// Join tabs with separator
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("│")

	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs[0])
	for i := 1; i < len(renderedTabs); i++ {
		tabBar = lipgloss.JoinHorizontal(lipgloss.Top, tabBar, separator, renderedTabs[i])
	}

	// Add bottom border
	borderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(m.width)

	bottomBorder := borderStyle.Render(strings.Repeat("─", m.width))

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, bottomBorder)
}

// View renders the tabs model.
func (m *TabsModel) View() string {
	if len(m.tabs) == 0 {
		return "No tabs available"
	}

	// Render tab bar
	tabBar := m.renderTabBar()

	// Render active tab content
	content := m.GetActiveContent()

	// Combine tab bar and content
	return lipgloss.JoinVertical(lipgloss.Left, tabBar, content)
}

// GetTabCount returns the number of tabs.
func (m *TabsModel) GetTabCount() int {
	return len(m.tabs)
}

// GetTabTitle returns the title of a tab by index.
func (m *TabsModel) GetTabTitle(index int) string {
	if index >= 0 && index < len(m.tabs) {
		return m.tabs[index].Title
	}
	return ""
}
