package question

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const QuestionDialogID dialogs.DialogID = "question"

// QuestionResponseMsg represents the user's response to questions
type QuestionResponseMsg struct {
	RequestID string
	Answers   map[string]string
	Canceled  bool
}

// QuestionDialogCmp interface for question dialog component
type QuestionDialogCmp interface {
	dialogs.DialogModel
}

// questionDialogCmp is the implementation
type questionDialogCmp struct {
	wWidth  int
	wHeight int
	width   int
	height  int

	request         question.QuestionRequest
	currentQuestion int                  // Current question being shown (0-indexed)
	selections      map[int]map[int]bool // question index -> option index -> selected
	otherInputs     map[int]string       // question index -> custom text input
	otherActive     map[int]bool         // question index -> is "Other" option active
	focusedInput    bool                 // True when editing "Other" text input
	cursorPos       int                  // Cursor position for text input
	focusedOption   map[int]int          // question index -> focused option index (for multi-select)
	keyMap          KeyMap

	positionRow int // Row position for dialog
	positionCol int // Column position for dialog
}

func NewQuestionDialogCmp(req question.QuestionRequest) QuestionDialogCmp {
	selections := make(map[int]map[int]bool)
	focusedOption := make(map[int]int)
	for i, q := range req.Questions {
		selections[i] = make(map[int]bool)
		// Pre-select first option for single-select questions
		if !q.MultiSelect {
			selections[i][0] = true
		}
		focusedOption[i] = 0 // Start with first option focused
	}

	return &questionDialogCmp{
		request:       req,
		selections:    selections,
		otherInputs:   make(map[int]string),
		otherActive:   make(map[int]bool),
		focusedOption: focusedOption,
		keyMap:        DefaultKeyMap(),
	}
}

func (d *questionDialogCmp) Init() tea.Cmd {
	return nil
}

func (d *questionDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.wWidth = msg.Width
		d.wHeight = msg.Height
		return d, d.SetSize()
	case tea.KeyPressMsg:
		return d.handleKeyPress(msg)
	}
	return d, nil
}

func (d *questionDialogCmp) handleKeyPress(msg tea.KeyPressMsg) (util.Model, tea.Cmd) {
	q := d.request.Questions[d.currentQuestion]

	// Handle text input mode
	if d.focusedInput {
		switch {
		case key.Matches(msg, d.keyMap.Enter):
			d.focusedInput = false
			return d, nil
		case key.Matches(msg, d.keyMap.Backspace):
			if d.cursorPos > 0 {
				input := d.otherInputs[d.currentQuestion]
				d.otherInputs[d.currentQuestion] = input[:d.cursorPos-1] + input[d.cursorPos:]
				d.cursorPos--
			}
			return d, nil
		default:
			// Add character to input
			if len(msg.Text) > 0 {
				input := d.otherInputs[d.currentQuestion]
				d.otherInputs[d.currentQuestion] = input[:d.cursorPos] + msg.Text + input[d.cursorPos:]
				d.cursorPos += len(msg.Text)
			}
			return d, nil
		}
	}

	// Handle navigation and selection
	switch {
	case key.Matches(msg, d.keyMap.Up):
		d.moveSelection(-1, q)
		return d, nil
	case key.Matches(msg, d.keyMap.Down):
		d.moveSelection(1, q)
		return d, nil
	case key.Matches(msg, d.keyMap.Space):
		d.toggleSelection(q)
		return d, nil
	case key.Matches(msg, d.keyMap.Enter):
		// If "Other" is selected and no input yet, focus the input
		if d.otherActive[d.currentQuestion] && d.otherInputs[d.currentQuestion] == "" {
			d.focusedInput = true
			d.cursorPos = 0
			return d, nil
		}

		// Move to next question or submit
		if d.currentQuestion < len(d.request.Questions)-1 {
			d.currentQuestion++
			return d, nil
		}
		// Submit answers
		return d, tea.Batch(
			util.CmdHandler(dialogs.CloseDialogMsg{}),
			util.CmdHandler(QuestionResponseMsg{
				RequestID: d.request.ID,
				Answers:   d.collectAnswers(),
			}),
		)
	case key.Matches(msg, d.keyMap.Cancel):
		return d, tea.Batch(
			util.CmdHandler(dialogs.CloseDialogMsg{}),
			util.CmdHandler(QuestionResponseMsg{
				RequestID: d.request.ID,
				Canceled:  true,
			}),
		)
	case key.Matches(msg, d.keyMap.Prev):
		if d.currentQuestion > 0 {
			d.currentQuestion--
		}
		return d, nil
	case key.Matches(msg, d.keyMap.Next):
		if d.currentQuestion < len(d.request.Questions)-1 {
			d.currentQuestion++
		}
		return d, nil
	}

	return d, nil
}

func (d *questionDialogCmp) moveSelection(delta int, q question.Question) {
	totalOptions := len(q.Options) + 1 // +1 for "Other"
	currentFocused := d.focusedOption[d.currentQuestion]
	newFocused := (currentFocused + delta + totalOptions) % totalOptions
	d.focusedOption[d.currentQuestion] = newFocused

	if q.MultiSelect {
		// In multi-select, just move focus (selection happens with Space)
		return
	}

	// Single-select: clear all and select new one
	d.selections[d.currentQuestion] = make(map[int]bool)
	if newFocused == len(q.Options) {
		// "Other" option
		d.otherActive[d.currentQuestion] = true
	} else {
		d.selections[d.currentQuestion][newFocused] = true
		d.otherActive[d.currentQuestion] = false
	}
}

func (d *questionDialogCmp) toggleSelection(q question.Question) {
	currentFocused := d.focusedOption[d.currentQuestion]

	if currentFocused == len(q.Options) {
		// Toggling "Other"
		d.otherActive[d.currentQuestion] = !d.otherActive[d.currentQuestion]
		if d.otherActive[d.currentQuestion] {
			d.focusedInput = true
			d.cursorPos = len(d.otherInputs[d.currentQuestion])
		}
		return
	}

	if q.MultiSelect {
		// Toggle the checkbox
		d.selections[d.currentQuestion][currentFocused] = !d.selections[d.currentQuestion][currentFocused]
	} else {
		// Single-select: already handled by moveSelection
	}
}



func (d *questionDialogCmp) collectAnswers() map[string]string {
	answers := make(map[string]string)

	for i, q := range d.request.Questions {
		idx := fmt.Sprintf("%d", i)

		if d.otherActive[i] && d.otherInputs[i] != "" {
			answers[idx] = "__other__:" + d.otherInputs[i]
			continue
		}

		selected := []string{}
		for j, opt := range q.Options {
			if d.selections[i][j] {
				selected = append(selected, opt.Label)
			}
		}

		if len(selected) > 0 {
			answers[idx] = strings.Join(selected, ",")
		} else {
			answers[idx] = ""
		}
	}

	return answers
}

func (d *questionDialogCmp) View() string {
	if len(d.request.Questions) == 0 {
		return "No questions to display"
	}

	theme := styles.CurrentTheme()
	q := d.request.Questions[d.currentQuestion]

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Info)
	headerStyle := lipgloss.NewStyle().Foreground(theme.FgSubtle)
	labelStyle := lipgloss.NewStyle()
	descStyle := lipgloss.NewStyle().Foreground(theme.FgSubtle)
	selectedStyle := lipgloss.NewStyle().Foreground(theme.Success).Bold(true)

	var s strings.Builder

	// Title
	if len(d.request.Questions) > 1 {
		s.WriteString(titleStyle.Render(fmt.Sprintf("Question %d/%d", d.currentQuestion+1, len(d.request.Questions))))
		s.WriteString("\n\n")
	} else {
		s.WriteString(titleStyle.Render("Question"))
		s.WriteString("\n\n")
	}

	// Header and question text
	s.WriteString(headerStyle.Render(q.Header))
	s.WriteString("\n")
	s.WriteString(q.Question)
	s.WriteString("\n\n")

	// Options
	selType := "○"
	selTypeSelected := "●"
	if q.MultiSelect {
		selType = "☐"
		selTypeSelected = "☑"
	}

	focusStyle := lipgloss.NewStyle().Foreground(theme.Info)
	focused := d.focusedOption[d.currentQuestion]

	for i, opt := range q.Options {
		selected := d.selections[d.currentQuestion][i]
		isFocused := focused == i
		icon := selType
		if selected {
			icon = selTypeSelected
		}

		prefix := "  "
		if q.MultiSelect && isFocused {
			prefix = "> "
		}

		line := fmt.Sprintf("%s%s %s", prefix, icon, opt.Label)
		if selected {
			line = selectedStyle.Render(line)
		} else if isFocused && q.MultiSelect {
			line = focusStyle.Render(line)
		} else {
			line = labelStyle.Render(line)
		}
		s.WriteString(line)
		s.WriteString("\n")

		if opt.Description != "" {
			desc := fmt.Sprintf("    %s", opt.Description)
			s.WriteString(descStyle.Render(desc))
			s.WriteString("\n")
		}
	}

	// "Other" option
	otherSelected := d.otherActive[d.currentQuestion]
	otherFocused := focused == len(q.Options)
	otherIcon := selType
	if otherSelected {
		otherIcon = selTypeSelected
	}

	otherPrefix := "  "
	if q.MultiSelect && otherFocused {
		otherPrefix = "> "
	}

	otherLine := fmt.Sprintf("%s%s Other", otherPrefix, otherIcon)
	if otherSelected {
		otherLine = selectedStyle.Render(otherLine)
	} else if otherFocused && q.MultiSelect {
		otherLine = focusStyle.Render(otherLine)
	} else {
		otherLine = labelStyle.Render(otherLine)
	}
	s.WriteString(otherLine)

	if otherSelected {
		input := d.otherInputs[d.currentQuestion]
		if d.focusedInput {
			input = input[:d.cursorPos] + "█" + input[d.cursorPos:]
		}
		s.WriteString(fmt.Sprintf(": %s", input))
	}
	s.WriteString("\n\n")

	// Instructions
	instructions := []string{}
	if q.MultiSelect {
		instructions = append(instructions, "Space: toggle")
	}
	instructions = append(instructions, "Enter: next/submit", "Esc: cancel")
	if len(d.request.Questions) > 1 {
		instructions = append(instructions, "←/→: prev/next question")
	}

	s.WriteString(descStyle.Render(strings.Join(instructions, "  •  ")))

	// Wrap in border
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(1, 2).
		Width(d.width)

	return dialogStyle.Render(s.String())
}

func (d *questionDialogCmp) SetSize() tea.Cmd {
	// Calculate dialog dimensions (60% of window width)
	d.width = int(float64(d.wWidth) * 0.6)
	d.width = min(d.width, 80)

	// Calculate position (center, slightly higher than middle)
	d.positionRow = d.wHeight/2 - 10 // Approximate height, will center vertically
	d.positionCol = d.wWidth/2 - d.width/2

	return nil
}

func (d *questionDialogCmp) ID() dialogs.DialogID {
	return QuestionDialogID
}

func (d *questionDialogCmp) Position() (int, int) {
	return d.positionRow, d.positionCol
}

func (d *questionDialogCmp) Help() help.Model {
	return help.New()
}

func (d *questionDialogCmp) ShortHelp() []key.Binding {
	return []key.Binding{
		d.keyMap.Up,
		d.keyMap.Down,
		d.keyMap.Space,
		d.keyMap.Enter,
		d.keyMap.Cancel,
	}
}

func (d *questionDialogCmp) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{d.keyMap.Up, d.keyMap.Down, d.keyMap.Space},
		{d.keyMap.Enter, d.keyMap.Cancel},
		{d.keyMap.Prev, d.keyMap.Next},
	}
}
