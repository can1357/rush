package confirmdelete

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/can1357/rush/tui/components/chat/editoverlay"
	"github.com/can1357/rush/tui/styles"
	"github.com/can1357/rush/tui/util"
)

type Model struct {
	message       editoverlay.EditableMessage
	width         int
	height        int
	focusedButton int // 0 = Edit, 1 = Cancel
}

type ConfirmEditMsg struct {
	Message editoverlay.EditableMessage
}

type CancelEditMsg struct{}

func New(message editoverlay.EditableMessage) Model {
	return Model{
		message:       message,
		focusedButton: 0,
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

		case "tab", "left", "right":
			m.focusedButton = 1 - m.focusedButton

		case "e", "enter":
			if m.focusedButton == 0 {
				return m, util.CmdHandler(ConfirmEditMsg{Message: m.message})
			}
			return m, util.CmdHandler(CancelEditMsg{})
		}
	}

	return m, nil
}

func (m Model) View() string {
	t := styles.CurrentTheme()

	var content strings.Builder

	// Title
	content.WriteString(t.S().Title.Render("⚠️  Edit Message"))
	content.WriteString("\n\n")

	// Original message preview
	content.WriteString("Original message:\n")
	preview := truncate(m.message.Preview, 70)
	content.WriteString(t.S().Subtle.Render(fmt.Sprintf("\"%s\"", preview)))
	content.WriteString("\n\n")

	// Warning about deletion
	if m.message.SubsequentCount > 0 {
		content.WriteString(t.S().Warning.Render(
			fmt.Sprintf("This will delete %d message(s) after this point", m.message.SubsequentCount),
		))
		content.WriteString("\n\n")
	}

	// Buttons
	var editBtn, cancelBtn string
	if m.focusedButton == 0 {
		editBtn = t.S().TextSelected.Render("[E] Edit & Continue")
		cancelBtn = t.S().Muted.Render("[ESC] Cancel")
	} else {
		editBtn = t.S().Muted.Render("[E] Edit & Continue")
		cancelBtn = t.S().TextSelected.Render("[ESC] Cancel")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Left,
		editBtn,
		"    ",
		cancelBtn,
	)
	content.WriteString(lipgloss.Place(60, 1, lipgloss.Center, lipgloss.Center, buttons))

	// Box
	boxWidth := min(70, m.width-4)
	boxHeight := min(15, m.height-4)

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
