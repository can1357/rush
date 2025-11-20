package reminder

import (
	"context"
	"fmt"
	"strings"

	"github.com/can1357/rush/session"
	"github.com/can1357/rush/todo"
)

const (
	TurnsSinceWriteThreshold = 7
	TurnsBetweenReminders    = 3
)

type Service struct {
	todos    todo.Service
	sessions session.Service
}

func NewService(todos todo.Service, sessions session.Service) *Service {
	return &Service{
		todos:    todos,
		sessions: sessions,
	}
}

// ListTodos returns the todos for the given session
func (r *Service) ListTodos(ctx context.Context, sessionID string) ([]todo.Todo, error) {
	return r.todos.List(ctx, sessionID)
}

// BuildReminders generates reminder text based on current session state
func (r *Service) BuildReminders(ctx context.Context, sessionID string) (string, error) {
	sess, err := r.sessions.Get(ctx, sessionID)
	if err != nil {
		return "", err
	}

	todos, err := r.todos.List(ctx, sessionID)
	if err != nil {
		return "", err
	}

	var reminders []string

	// Check for empty todo list
	if r.shouldShowEmptyReminder(todos, &sess) {
		reminders = append(reminders, r.emptyReminderText())
	}

	// Check for stale todo list
	if r.shouldShowStaleReminder(&sess) {
		text, err := r.staleReminderText(todos, &sess)
		if err != nil {
			return "", err
		}
		reminders = append(reminders, text)

		// Update last reminder turn
		sess.LastReminderTurn = sess.AssistantTurnCount
		if _, err := r.sessions.Save(ctx, sess); err != nil {
			return "", err
		}
	}

	if len(reminders) == 0 {
		return "", nil
	}

	return r.wrapInSystemReminder(strings.Join(reminders, "\n\n")), nil
}

func (r *Service) shouldShowEmptyReminder(todos []todo.Todo, sess *session.Session) bool {
	// Only remind after conversation has started (3+ turns)
	return len(todos) == 0 && sess.AssistantTurnCount >= 3
}

func (r *Service) shouldShowStaleReminder(sess *session.Session) bool {
	if len := sess.AssistantTurnCount; len == 0 {
		return false
	}

	turnsSinceWrite := sess.AssistantTurnCount - sess.LastTodoWriteTurn
	turnsSinceReminder := sess.AssistantTurnCount - sess.LastReminderTurn

	return turnsSinceWrite >= TurnsSinceWriteThreshold &&
		turnsSinceReminder >= TurnsBetweenReminders
}

func (r *Service) emptyReminderText() string {
	return `This is a reminder that your todo list is currently empty. DO NOT mention this to the user explicitly because they are already aware. If you are working on tasks that would benefit from a todo list please use the TodoWrite tool to create one. If not, please feel free to ignore. Again do not mention this message to the user.`
}

func (r *Service) staleReminderText(todos []todo.Todo, sess *session.Session) (string, error) {
	var b strings.Builder

	b.WriteString(`The TodoWrite tool hasn't been used recently. If you're working on tasks that would benefit from tracking progress, consider using the TodoWrite tool to track progress. Also consider cleaning up the todo list if has become stale and no longer matches what you are working on. Only use it if it's relevant to the current work. This is just a gentle reminder - ignore if not applicable. Make sure that you NEVER mention this reminder to the user`)

	// Filter: only show todos if there are active ones OR recent activity
	turnsSinceWrite := sess.AssistantTurnCount - sess.LastTodoWriteTurn
	hasActiveTodos := false
	for _, t := range todos {
		if t.Status != "completed" && t.Status != "cancelled" {
			hasActiveTodos = true
			break
		}
	}

	// Don't show old completed todos from previous conversations (>10 turns inactive)
	if len(todos) > 0 && (hasActiveTodos || turnsSinceWrite <= 10) {
		b.WriteString("\n\n\nHere are the existing contents of your todo list:\n\n")
		b.WriteString(r.FormatTodos(todos))
	}

	return b.String(), nil
}

// FormatTodos formats a list of todos for display in the context,
// filtering out old completed tasks to save tokens.
func (r *Service) FormatTodos(todos []todo.Todo) string {
	var b strings.Builder
	b.WriteString("[")

	// Filter logic: keep all pending/in_progress, keep only last 5 completed/cancelled
	var active []string
	var completed []string

	for i, t := range todos {
		line := fmt.Sprintf("%d. [%s] %s", i+1, t.Status, t.Content)
		if t.Status == "completed" || t.Status == "cancelled" {
			completed = append(completed, line)
		} else {
			active = append(active, line)
		}
	}

	// Keep only last 5 completed
	if len(completed) > 5 {
		completed = completed[len(completed)-5:]
	}

	// Combine: completed first (as history), then active (as focus)
	all := append(completed, active...)

	for i, line := range all {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
	}
	b.WriteString("]")
	return b.String()
}

func (r *Service) wrapInSystemReminder(text string) string {
	return fmt.Sprintf("<system-reminder>\n%s\n</system-reminder>", text)
}
