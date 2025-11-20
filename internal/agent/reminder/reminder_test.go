package reminder

import (
	"strings"
	"testing"

	"github.com/can1357/rush/internal/session"
	"github.com/can1357/rush/internal/todo"
)

func TestShouldShowEmptyReminder(t *testing.T) {
	svc := &Service{}

	tests := []struct {
		name  string
		todos []todo.Todo
		want  bool
	}{
		{
			name:  "empty list should trigger reminder",
			todos: []todo.Todo{},
			want:  true,
		},
		{
			name: "non-empty list should not trigger reminder",
			todos: []todo.Todo{
				{Content: "Task 1", Status: "pending"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.shouldShowEmptyReminder(tt.todos)
			if got != tt.want {
				t.Errorf("shouldShowEmptyReminder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldShowStaleReminder(t *testing.T) {
	svc := &Service{}

	tests := []struct {
		name    string
		session *session.Session
		want    bool
	}{
		{
			name: "no turns yet",
			session: &session.Session{
				AssistantTurnCount: 0,
				LastTodoWriteTurn:  0,
				LastReminderTurn:   0,
			},
			want: false,
		},
		{
			name: "just wrote todo",
			session: &session.Session{
				AssistantTurnCount: 5,
				LastTodoWriteTurn:  5,
				LastReminderTurn:   0,
			},
			want: false,
		},
		{
			name: "7 turns since write, 0 since reminder - should trigger",
			session: &session.Session{
				AssistantTurnCount: 10,
				LastTodoWriteTurn:  3,
				LastReminderTurn:   0,
			},
			want: true,
		},
		{
			name: "8 turns since write, just showed reminder - should not trigger",
			session: &session.Session{
				AssistantTurnCount: 10,
				LastTodoWriteTurn:  2,
				LastReminderTurn:   9,
			},
			want: false,
		},
		{
			name: "8 turns since write, 3 turns since reminder - should trigger",
			session: &session.Session{
				AssistantTurnCount: 10,
				LastTodoWriteTurn:  2,
				LastReminderTurn:   7,
			},
			want: true,
		},
		{
			name: "exactly 7 turns since write, 3 turns since reminder - should trigger",
			session: &session.Session{
				AssistantTurnCount: 10,
				LastTodoWriteTurn:  3,
				LastReminderTurn:   7,
			},
			want: true,
		},
		{
			name: "6 turns since write, 3 turns since reminder - should not trigger",
			session: &session.Session{
				AssistantTurnCount: 10,
				LastTodoWriteTurn:  4,
				LastReminderTurn:   7,
			},
			want: false,
		},
		{
			name: "7 turns since write, 2 turns since reminder - should not trigger",
			session: &session.Session{
				AssistantTurnCount: 10,
				LastTodoWriteTurn:  3,
				LastReminderTurn:   8,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.shouldShowStaleReminder(tt.session)
			if got != tt.want {
				t.Errorf("shouldShowStaleReminder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmptyReminderText(t *testing.T) {
	svc := &Service{}
	text := svc.emptyReminderText()

	if !strings.Contains(text, "todo list is currently empty") {
		t.Error("empty reminder should mention empty todo list")
	}

	if !strings.Contains(text, "DO NOT mention this to the user") {
		t.Error("empty reminder should include instruction not to mention to user")
	}
}

func TestStaleReminderText(t *testing.T) {
	svc := &Service{}

	t.Run("with todos", func(t *testing.T) {
		todos := []todo.Todo{
			{Content: "Task 1", Status: "pending"},
			{Content: "Task 2", Status: "in_progress"},
		}

		text, err := svc.staleReminderText(todos)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(text, "TodoWrite tool hasn't been used recently") {
			t.Error("stale reminder should mention TodoWrite not being used")
		}

		if !strings.Contains(text, "Task 1") {
			t.Error("stale reminder should include todo content")
		}

		if !strings.Contains(text, "pending") {
			t.Error("stale reminder should include todo status")
		}
	})

	t.Run("empty todos", func(t *testing.T) {
		text, err := svc.staleReminderText([]todo.Todo{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(text, "TodoWrite tool hasn't been used recently") {
			t.Error("stale reminder should mention TodoWrite not being used")
		}
	})
}

func TestWrapInSystemReminder(t *testing.T) {
	svc := &Service{}
	text := "test reminder"
	wrapped := svc.wrapInSystemReminder(text)

	if !strings.HasPrefix(wrapped, "<system-reminder>") {
		t.Error("should start with <system-reminder> tag")
	}

	if !strings.HasSuffix(wrapped, "</system-reminder>") {
		t.Error("should end with </system-reminder> tag")
	}

	if !strings.Contains(wrapped, text) {
		t.Error("should contain the original text")
	}
}
