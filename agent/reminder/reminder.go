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
	if r.shouldShowEmptyReminder(todos) {
		reminders = append(reminders, r.emptyReminderText())
	}

	// Check for stale todo list
	if r.shouldShowStaleReminder(&sess) {
		text, err := r.staleReminderText(todos)
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

func (r *Service) shouldShowEmptyReminder(todos []todo.Todo) bool {
	return len(todos) == 0
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

func (r *Service) staleReminderText(todos []todo.Todo) (string, error) {
	var b strings.Builder

	b.WriteString(`The TodoWrite tool hasn't been used recently. If you're working on tasks that would benefit from tracking progress, consider using the TodoWrite tool to track progress. Also consider cleaning up the todo list if has become stale and no longer matches what you are working on. Only use it if it's relevant to the current work. This is just a gentle reminder - ignore if not applicable. Make sure that you NEVER mention this reminder to the user`)

	if len(todos) > 0 {
		b.WriteString("\n\n\nHere are the existing contents of your todo list:\n\n[")
		for i, t := range todos {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, t.Status, t.Content))
		}
		b.WriteString("]")
	}

	return b.String(), nil
}

func (r *Service) wrapInSystemReminder(text string) string {
	return fmt.Sprintf("<system-reminder>\n%s\n</system-reminder>", text)
}
