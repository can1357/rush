package todolist

import (
	"fmt"
	"image/color"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/can1357/rush/internal/todo"
	"github.com/can1357/rush/internal/tui/styles"
	"github.com/can1357/rush/internal/tui/util"
)

// TodoListUpdateMsg is sent when the todo list is updated
type TodoListUpdateMsg struct {
	Todos []todo.Todo
}

// TodoList component interface
type TodoList interface {
	util.Model
	SetTodos(todos []todo.Todo)
	GetTodos() []todo.Todo
	ViewWithMaxItems(maxItems int) string
}

// todoList is the implementation
type todoList struct {
	todos  []todo.Todo
	width  int
	height int
}

// New creates a new todo list component
func New() TodoList {
	return &todoList{
		todos: []todo.Todo{},
	}
}

func (t *todoList) Init() tea.Cmd {
	return nil
}

func (t *todoList) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
	case TodoListUpdateMsg:
		t.SetTodos(msg.Todos)
	}
	return t, nil
}

func (t *todoList) SetTodos(todos []todo.Todo) {
	t.todos = todos
}

func (t *todoList) GetTodos() []todo.Todo {
	return t.todos
}

func (t *todoList) View() string {
	return t.ViewWithMaxItems(0)
}

// ViewWithMaxItems renders the todo list with a maximum number of items shown.
// If maxItems is 0 or greater than the number of todos, all todos are shown.
// If there are more todos than maxItems, a truncation indicator is shown.
func (t *todoList) ViewWithMaxItems(maxItems int) string {
	if len(t.todos) == 0 {
		return ""
	}

	theme := styles.CurrentTheme()

	var lines []string

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.FgSubtle)
	lines = append(lines, headerStyle.Render(fmt.Sprintf("Tasks (%d)", len(t.todos))))

	// Determine how many items to show
	itemsToShow := len(t.todos)
	if maxItems > 0 && maxItems < len(t.todos) {
		itemsToShow = maxItems
	}

	// Render each todo
	for i := 0; i < itemsToShow; i++ {
		item := t.todos[i]
		var statusIcon string
		var statusColor color.Color
		var text string

		switch item.Status {
		case "completed":
			statusIcon = "✓"
			statusColor = theme.Success
			text = item.Content
		case "in_progress":
			statusIcon = "⟳"
			statusColor = theme.Info
			text = item.ActiveForm
		case "pending":
			statusIcon = "○"
			statusColor = theme.FgSubtle
			text = item.Content
		}

		iconStyle := lipgloss.NewStyle().Foreground(statusColor)
		textStyle := lipgloss.NewStyle()

		if item.Status == "completed" {
			textStyle = textStyle.Foreground(theme.FgSubtle)
		} else if item.Status == "in_progress" {
			textStyle = textStyle.Foreground(statusColor)
		}

		line := fmt.Sprintf("  %s %s", iconStyle.Render(statusIcon), textStyle.Render(text))
		lines = append(lines, line)
	}

	// Add truncation indicator if needed
	if maxItems > 0 && len(t.todos) > maxItems {
		remaining := len(t.todos) - maxItems
		if remaining == 1 {
			lines = append(lines, theme.S().Base.Foreground(theme.FgMuted).Render("  …"))
		} else {
			lines = append(lines, theme.S().Base.Foreground(theme.FgSubtle).Render(fmt.Sprintf("  …and %d more", remaining)))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
