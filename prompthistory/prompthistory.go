package prompthistory

import (
	"context"

	"github.com/can1357/rush/db"
	"github.com/google/uuid"
)

const MaxPromptHistory = 1000

type PromptHistory struct {
	ID        string
	Prompt    string
	CreatedAt int64
}

type Service interface {
	Add(ctx context.Context, prompt string) error
	List(ctx context.Context, limit int) ([]PromptHistory, error)
	Clear(ctx context.Context) error
}

type service struct {
	q db.Querier
}

func NewService(q db.Querier) Service {
	return &service{q: q}
}

func (s *service) Add(ctx context.Context, prompt string) error {
	_, err := s.q.AddPromptHistory(ctx, db.AddPromptHistoryParams{
		ID:     uuid.New().String(),
		Prompt: prompt,
	})
	return err
}

func (s *service) List(ctx context.Context, limit int) ([]PromptHistory, error) {
	if limit <= 0 || limit > MaxPromptHistory {
		limit = MaxPromptHistory
	}

	dbItems, err := s.q.ListPromptHistory(ctx, int64(limit))
	if err != nil {
		return nil, err
	}

	items := make([]PromptHistory, len(dbItems))
	for i, item := range dbItems {
		items[i] = PromptHistory{
			ID:        item.ID,
			Prompt:    item.Prompt,
			CreatedAt: item.CreatedAt,
		}
	}
	return items, nil
}

func (s *service) Clear(ctx context.Context) error {
	return s.q.ClearPromptHistory(ctx)
}
