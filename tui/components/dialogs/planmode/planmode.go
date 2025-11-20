package planmode

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/can1357/rush/tui/components/dialogs"
	"github.com/can1357/rush/tui/styles"
	"github.com/can1357/rush/tui/util"
)

const PlanModeDialogID dialogs.DialogID = "plan_mode"

type PlanModeToggleMsg struct {
	SessionID string
	Enable    bool
	Confirmed bool
}

type PlanModeDialog interface {
	dialogs.DialogModel
}

type planModeDialogCmp struct {
	sessionID  string
	enable     bool // true to enter plan mode, false to exit
	wWidth     int
	wHeight    int
	selectedNo bool // true if "No" button is selected
	keymap     KeyMap
}

func New(sessionID string, enable bool) PlanModeDialog {
	return &planModeDialogCmp{
		sessionID:  sessionID,
		enable:     enable,
		selectedNo: true, // Default to "No" for safety
		keymap:     DefaultKeymap(),
	}
}

func (d *planModeDialogCmp) Init() tea.Cmd {
	return nil
}

func (d *planModeDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.wWidth = msg.Width
		d.wHeight = msg.Height
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keymap.LeftRight, d.keymap.Tab):
			d.selectedNo = !d.selectedNo
			return d, nil
		case key.Matches(msg, d.keymap.EnterSpace):
			if !d.selectedNo {
				// User confirmed
				return d, tea.Batch(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					util.CmdHandler(PlanModeToggleMsg{
						SessionID: d.sessionID,
						Enable:    d.enable,
						Confirmed: true,
					}),
				)
			}
			// User cancelled
			return d, util.CmdHandler(dialogs.CloseDialogMsg{})
		case key.Matches(msg, d.keymap.Yes):
			return d, tea.Batch(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(PlanModeToggleMsg{
					SessionID: d.sessionID,
					Enable:    d.enable,
					Confirmed: true,
				}),
			)
		case key.Matches(msg, d.keymap.No, d.keymap.Close):
			return d, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
	}
	return d, nil
}

func (d *planModeDialogCmp) View() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base
	yesStyle := t.S().Text
	noStyle := yesStyle

	if d.selectedNo {
		noStyle = noStyle.Foreground(t.White).Background(t.Secondary)
		yesStyle = yesStyle.Background(t.BgSubtle)
	} else {
		yesStyle = yesStyle.Foreground(t.White).Background(t.Secondary)
		noStyle = noStyle.Background(t.BgSubtle)
	}

	var question, details string
	if d.enable {
		question = "Enter Plan Mode?"
		details = `This enables read-only mode where you can explore
and analyze code, but cannot make changes.

Use Ctrl+/ or exit_plan_mode tool to exit later.`
	} else {
		question = "Exit Plan Mode?"
		details = "This will restore full access to write and execute tools."
	}

	const horizontalPadding = 3
	yesButton := yesStyle.PaddingLeft(horizontalPadding).Underline(true).Render("Y") +
		yesStyle.PaddingRight(horizontalPadding).Render("es")
	noButton := noStyle.PaddingLeft(horizontalPadding).Underline(true).Render("N") +
		noStyle.PaddingRight(horizontalPadding).Render("o")

	maxWidth := max(lipgloss.Width(question), lipgloss.Width(details))

	buttons := baseStyle.Width(maxWidth).Align(lipgloss.Right).Render(
		lipgloss.JoinHorizontal(lipgloss.Center, yesButton, "  ", noButton),
	)

	content := baseStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			baseStyle.Bold(true).Render(question),
			"",
			baseStyle.Width(maxWidth).Render(details),
			"",
			buttons,
		),
	)

	dialogStyle := baseStyle.
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Warning)

	return dialogStyle.Render(content)
}

func (d *planModeDialogCmp) Position() (int, int) {
	row := d.wHeight / 2
	row -= 10 / 2
	col := d.wWidth / 2
	col -= 30

	return row, col
}

func (d *planModeDialogCmp) ID() dialogs.DialogID {
	return PlanModeDialogID
}
