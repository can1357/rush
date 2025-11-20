package todo

import (
	"context"
	"fmt"

	"github.com/can1357/rush/db"
	"github.com/can1357/rush/pubsub"
	"github.com/google/uuid"
)

// Todo represents a single todo item
type Todo struct {
	ID          string
	SessionID   string
	Content     string
	ActiveForm  string
	Status      string
	Position    int64
	CreatedAt   int64
	UpdatedAt   int64
	CompletedAt int64
}

// Service manages todos
type Service interface {
	pubsub.Suscriber[Todo]
	Create(ctx context.Context, sessionID, content, activeForm, status string) (Todo, error)
	Get(ctx context.Context, id string) (Todo, error)
	List(ctx context.Context, sessionID string) ([]Todo, error)
	UpdateStatus(ctx context.Context, id, status string) (Todo, error)
	UpdateContent(ctx context.Context, id, content, activeForm string) (Todo, error)
	Delete(ctx context.Context, id string) error
	DeleteBySession(ctx context.Context, sessionID string) error
	ReplaceAll(ctx context.Context, sessionID string, todos []TodoInput) ([]Todo, error)
}

// TodoInput is used for bulk todo operations
type TodoInput struct {
	Content    string
	ActiveForm string
	Status     string
}

type service struct {
	*pubsub.Broker[Todo]
	q db.Querier
}

// NewService creates a new todo service
func NewService(q db.Querier) Service {
	return &service{
		Broker: pubsub.NewBroker[Todo](),
		q:      q,
	}
}

func (s *service) Create(ctx context.Context, sessionID, content, activeForm, status string) (Todo, error) {
	// Get the next position
	maxPos, err := s.q.GetMaxTodoPosition(ctx, sessionID)
	if err != nil {
		return Todo{}, fmt.Errorf("failed to get max position: %w", err)
	}

	// Convert maxPos to int64
	var position int64 = 1
	if maxPos != nil {
		if pos, ok := maxPos.(int64); ok {
			position = pos + 1
		}
	}

	dbTodo, err := s.q.CreateTodo(ctx, db.CreateTodoParams{
		ID:         uuid.New().String(),
		SessionID:  sessionID,
		Content:    content,
		ActiveForm: activeForm,
		Status:     status,
		Position:   position,
	})
	if err != nil {
		return Todo{}, fmt.Errorf("failed to create todo: %w", err)
	}

	todo := s.fromDBItem(dbTodo)
	s.Publish(pubsub.CreatedEvent, todo)
	return todo, nil
}

func (s *service) Get(ctx context.Context, id string) (Todo, error) {
	dbTodo, err := s.q.GetTodoByID(ctx, id)
	if err != nil {
		return Todo{}, fmt.Errorf("failed to get todo: %w", err)
	}
	return s.fromDBItem(dbTodo), nil
}

func (s *service) List(ctx context.Context, sessionID string) ([]Todo, error) {
	dbTodos, err := s.q.ListTodosBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list todos: %w", err)
	}

	todos := make([]Todo, len(dbTodos))
	for i, dbTodo := range dbTodos {
		todos[i] = s.fromDBItem(dbTodo)
	}
	return todos, nil
}

func (s *service) UpdateStatus(ctx context.Context, id, status string) (Todo, error) {
	dbTodo, err := s.q.UpdateTodoStatus(ctx, db.UpdateTodoStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return Todo{}, fmt.Errorf("failed to update todo status: %w", err)
	}

	todo := s.fromDBItem(dbTodo)
	s.Publish(pubsub.UpdatedEvent, todo)
	return todo, nil
}

func (s *service) UpdateContent(ctx context.Context, id, content, activeForm string) (Todo, error) {
	dbTodo, err := s.q.UpdateTodoContent(ctx, db.UpdateTodoContentParams{
		ID:         id,
		Content:    content,
		ActiveForm: activeForm,
	})
	if err != nil {
		return Todo{}, fmt.Errorf("failed to update todo content: %w", err)
	}

	todo := s.fromDBItem(dbTodo)
	s.Publish(pubsub.UpdatedEvent, todo)
	return todo, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	todo, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := s.q.DeleteTodo(ctx, id); err != nil {
		return fmt.Errorf("failed to delete todo: %w", err)
	}

	s.Publish(pubsub.DeletedEvent, todo)
	return nil
}

func (s *service) DeleteBySession(ctx context.Context, sessionID string) error {
	if err := s.q.DeleteTodosBySession(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to delete todos by session: %w", err)
	}
	return nil
}

// ReplaceAll atomically replaces all todos for a session
func (s *service) ReplaceAll(ctx context.Context, sessionID string, inputs []TodoInput) ([]Todo, error) {
	// Delete all existing todos for this session
	if err := s.DeleteBySession(ctx, sessionID); err != nil {
		return nil, err
	}

	// Create new todos
	todos := make([]Todo, len(inputs))
	for i, input := range inputs {
		dbTodo, err := s.q.CreateTodo(ctx, db.CreateTodoParams{
			ID:         uuid.New().String(),
			SessionID:  sessionID,
			Content:    input.Content,
			ActiveForm: input.ActiveForm,
			Status:     input.Status,
			Position:   int64(i),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create todo at position %d: %w", i, err)
		}
		todos[i] = s.fromDBItem(dbTodo)
	}

	// Publish events for all new todos
	for _, todo := range todos {
		s.Publish(pubsub.CreatedEvent, todo)
	}

	return todos, nil
}

func (s *service) fromDBItem(item db.Todo) Todo {
	completedAt := int64(0)
	if item.CompletedAt.Valid {
		completedAt = item.CompletedAt.Int64
	}

	return Todo{
		ID:          item.ID,
		SessionID:   item.SessionID,
		Content:     item.Content,
		ActiveForm:  item.ActiveForm,
		Status:      item.Status,
		Position:    item.Position,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
		CompletedAt: completedAt,
	}
}
