package editoverlay

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/can1357/rush/tui/styles"
	"github.com/can1357/rush/tui/util"
)

type EditableMessage struct {
	ID              string
	Preview         string
	SubsequentCount int
	CreatedAt       int64
}

type Model struct {
	messages []EditableMessage
	selected int
	width    int
	height   int
}

type SelectionConfirmedMsg struct {
	Message EditableMessage
}

type CancelEditMsg struct{}

func New(messages []EditableMessage) Model {
	return Model{
		messages: messages,
		selected: 0,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, util.CmdHandler(CancelEditMsg{})

		case "enter":
			if len(m.messages) > 0 {
				return m, util.CmdHandler(SelectionConfirmedMsg{
					Message: m.messages[m.selected],
				})
			}

		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}

		case "down", "j":
			if m.selected < len(m.messages)-1 {
				m.selected++
			}

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0] - '1')
			if idx < len(m.messages) {
				m.selected = idx
				return m, util.CmdHandler(SelectionConfirmedMsg{
					Message: m.messages[m.selected],
				})
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	if len(m.messages) == 0 {
		return m.renderEmpty()
	}

	t := styles.CurrentTheme()

	var content strings.Builder
	content.WriteString(t.S().Title.Render("Select message to edit (ESC to cancel)"))
	content.WriteString("\n\n")

	// Show up to 9 messages
	maxVisible := min(9, len(m.messages))
	for i := 0; i < maxVisible; i++ {
		msg := m.messages[i]

		number := fmt.Sprintf("[%d]", i+1)
		preview := truncate(msg.Preview, 60)
		subsequent := fmt.Sprintf("↳ %d message(s) after this point", msg.SubsequentCount)

		if i == m.selected {
			// Selected style
			content.WriteString(t.S().TextSelected.Render(
				fmt.Sprintf("  %s %s\n      %s", number, preview, subsequent),
			))
		} else {
			// Normal style
			content.WriteString(fmt.Sprintf("  %s %s\n      %s",
				t.S().Muted.Render(number),
				preview,
				t.S().Muted.Render(subsequent),
			))
		}
		content.WriteString("\n\n")
	}

	if len(m.messages) > 9 {
		content.WriteString(t.S().Muted.Render(fmt.Sprintf("  ... and %d more", len(m.messages)-9)))
		content.WriteString("\n\n")
	}

	content.WriteString(t.S().Muted.Render("j/k: navigate  Enter: select  ESC: cancel"))

	// Center the content
	boxWidth := min(80, m.width-4)
	boxHeight := min(30, m.height-4)

	box := lipgloss.NewStyle().
		Width(boxWidth).
		Height(boxHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(1).
		Render(content.String())

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

func (m Model) renderEmpty() string {
	t := styles.CurrentTheme()
	msg := "No messages to edit"

	box := lipgloss.NewStyle().
		Width(40).
		Height(5).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(1).
		Render(msg)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
