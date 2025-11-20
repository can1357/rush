package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/todo"
)

//go:embed todo.md
var todoDescription []byte

const TodoToolName = "todo_write"

type TodoParams struct {
	Todos []TodoItem `json:"todos" description:"The updated todo list with all items"`
}

type TodoItem struct {
	Content    string `json:"content" description:"Description of the task (imperative form, e.g., 'Run tests')"`
	ActiveForm string `json:"activeForm" description:"Present continuous form shown during execution (e.g., 'Running tests')"`
	Status     string `json:"status" description:"Task status: 'pending', 'in_progress', or 'completed'"`
}

type TodoPermissionsParams struct {
	Todos []TodoItem `json:"todos"`
}

type TodoResponseMetadata struct {
	SessionID string      `json:"session_id"`
	Todos     []todo.Todo `json:"todos"`
}

func NewTodoTool(todoService todo.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		TodoToolName,
		string(todoDescription),
		func(ctx context.Context, params TodoParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session ID is required for todo operations"), nil
			}

			// Validate todos
			if len(params.Todos) == 0 {
				return fantasy.NewTextErrorResponse("todos list cannot be empty"), nil
			}

			// Validate statuses and forms
			for i, item := range params.Todos {
				if item.Content == "" {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("todo at index %d: content is required", i)), nil
				}
				if item.ActiveForm == "" {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("todo at index %d: activeForm is required", i)), nil
				}
				if item.Status != "pending" && item.Status != "in_progress" && item.Status != "completed" {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("todo at index %d: status must be 'pending', 'in_progress', or 'completed'", i)), nil
				}
			}

			// Count in_progress items
			inProgressCount := 0
			completedCount := 0
			for _, item := range params.Todos {
				if item.Status == "in_progress" {
					inProgressCount++
				}
				if item.Status == "completed" {
					completedCount++
				}
			}

			// Validate at most one in_progress
			if inProgressCount > 1 {
				return fantasy.NewTextErrorResponse("at most one task must be 'in_progress' at any time"), nil
			}

			// Convert to todo inputs
			inputs := make([]todo.TodoInput, len(params.Todos))
			for i, item := range params.Todos {
				inputs[i] = todo.TodoInput{
					Content:    item.Content,
					ActiveForm: item.ActiveForm,
					Status:     item.Status,
				}
			}

			// Replace all todos for this session
			todos, err := todoService.ReplaceAll(ctx, sessionID, inputs)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to update todos: %w", err)
			}

			// Format response
			var output strings.Builder
			output.WriteString("Todos have been modified successfully. Ensure that you continue to use the todo list to track your progress. Please proceed with the current tasks if applicable")

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output.String()),
				TodoResponseMetadata{
					SessionID: sessionID,
					Todos:     todos,
				},
			), nil
		})
}

// FormatTodoList formats the todo list for display in the UI
func FormatTodoList(todos []todo.Todo) string {
	if len(todos) == 0 {
		return "No todos"
	}

	var sb strings.Builder
	for i, t := range todos {
		statusEmoji := ""
		switch t.Status {
		case "pending":
			statusEmoji = "⏸"
		case "in_progress":
			statusEmoji = "▶"
		case "completed":
			statusEmoji = "✓"
		}

		displayText := t.Content
		if t.Status == "in_progress" {
			displayText = t.ActiveForm
		}

		sb.WriteString(fmt.Sprintf("%d. [%s %s] %s\n", i+1, statusEmoji, t.Status, displayText))
	}

	return sb.String()
}

// TodoListToJSON converts todos to JSON format
func TodoListToJSON(todos []todo.Todo) string {
	data, err := json.MarshalIndent(todos, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(data)
}
