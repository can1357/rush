package todolist

import (
	"fmt"
	"image/color"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/todo"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
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
	if len(t.todos) == 0 {
		return ""
	}

	theme := styles.CurrentTheme()

	// Count by status for header
	completed := 0
	inProgress := 0
	pending := 0
	for _, todo := range t.todos {
		switch todo.Status {
		case "completed":
			completed++
		case "in_progress":
			inProgress++
		case "pending":
			pending++
		}
	}

	var s string

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.FgSubtle)
	s += headerStyle.Render(fmt.Sprintf("Tasks (%d)", len(t.todos))) + "\n"

	// Render each todo
	for i, item := range t.todos {
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
		s += line

		// Add newline unless it's the last item
		if i < len(t.todos)-1 {
			s += "\n"
		}
	}

	return s
}
