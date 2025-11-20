package question

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// Question represents a single question to ask the user
type Question struct {
	Question    string   `json:"question"`
	Header      string   `json:"header"`
	MultiSelect bool     `json:"multiSelect"`
	Options     []Option `json:"options"`
}

// Option represents a single answer option
type Option struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// QuestionRequest is sent to the UI for user to answer
type QuestionRequest struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`
	Questions []Question `json:"questions"`
}

// QuestionResponse contains user's answers
type QuestionResponse struct {
	ID      string            `json:"id"`
	Answers map[string]string `json:"answers"` // question index -> selected value(s)
	Canceled bool             `json:"canceled"`
}

// Service manages question/answer interactions
type Service interface {
	pubsub.Suscriber[QuestionRequest]
	Ask(ctx context.Context, sessionID string, questions []Question) (map[string]string, error)
	Answer(id string, answers map[string]string)
	Cancel(id string)
	SubscribeResponses(ctx context.Context) <-chan pubsub.Event[QuestionResponse]
}

type service struct {
	*pubsub.Broker[QuestionRequest]
	responseBroker  *pubsub.Broker[QuestionResponse]
	pendingRequests *csync.Map[string, chan QuestionResponse]
	requestMu       sync.Mutex
}

// NewService creates a new question service
func NewService() Service {
	return &service{
		Broker:          pubsub.NewBroker[QuestionRequest](),
		responseBroker:  pubsub.NewBroker[QuestionResponse](),
		pendingRequests: csync.NewMap[string, chan QuestionResponse](),
	}
}

// Ask sends questions to the user and waits for response
func (s *service) Ask(ctx context.Context, sessionID string, questions []Question) (map[string]string, error) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	// Validate questions
	if len(questions) == 0 || len(questions) > 4 {
		return nil, fmt.Errorf("must provide 1-4 questions, got %d", len(questions))
	}

	for i, q := range questions {
		if q.Question == "" {
			return nil, fmt.Errorf("question %d: question text is required", i)
		}
		if q.Header == "" {
			return nil, fmt.Errorf("question %d: header is required", i)
		}
		if len(q.Header) > 12 {
			return nil, fmt.Errorf("question %d: header must be 12 chars or less, got %d", i, len(q.Header))
		}
		if len(q.Options) < 2 || len(q.Options) > 4 {
			return nil, fmt.Errorf("question %d: must have 2-4 options, got %d", i, len(q.Options))
		}
		for j, opt := range q.Options {
			if opt.Label == "" {
				return nil, fmt.Errorf("question %d, option %d: label is required", i, j)
			}
		}
	}

	// Create request
	req := QuestionRequest{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Questions: questions,
	}

	// Create response channel
	respCh := make(chan QuestionResponse, 1)
	s.pendingRequests.Set(req.ID, respCh)
	defer s.pendingRequests.Del(req.ID)

	// Publish request to UI
	s.Publish(pubsub.CreatedEvent, req)

	// Wait for response or context cancellation
	select {
	case resp := <-respCh:
		if resp.Canceled {
			return nil, fmt.Errorf("user canceled question")
		}
		return resp.Answers, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Answer provides the user's response to a question
func (s *service) Answer(id string, answers map[string]string) {
	respCh, ok := s.pendingRequests.Get(id)
	if !ok {
		return
	}

	response := QuestionResponse{
		ID:      id,
		Answers: answers,
	}

	s.responseBroker.Publish(pubsub.CreatedEvent, response)
	respCh <- response
}

// Cancel cancels a pending question
func (s *service) Cancel(id string) {
	respCh, ok := s.pendingRequests.Get(id)
	if !ok {
		return
	}

	response := QuestionResponse{
		ID:       id,
		Canceled: true,
	}

	s.responseBroker.Publish(pubsub.CreatedEvent, response)
	respCh <- response
}

// SubscribeResponses returns a channel for response events
func (s *service) SubscribeResponses(ctx context.Context) <-chan pubsub.Event[QuestionResponse] {
	return s.responseBroker.Subscribe(ctx)
}
