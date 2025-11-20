package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/can1357/rush/db"
	"github.com/can1357/rush/message"
	"github.com/can1357/rush/session"
)

type Service interface {
	ExportSession(ctx context.Context, sessionID string, opts ExportOptions) (*ExportResult, error)
	ExportToClipboard(ctx context.Context, sessionID string, opts ExportOptions) error
	ExportToFile(ctx context.Context, sessionID string, filepath string, opts ExportOptions) error
}

type MessageService interface {
	Get(ctx context.Context, id string) (message.Message, error)
	List(ctx context.Context, sessionID string) ([]message.Message, error)
}

type service struct {
	q         db.Querier
	sessions  session.Service
	messages  MessageService
	clipboard *ClipboardManager
}

func NewService(q db.Querier, sessions session.Service, messages MessageService) Service {
	return &service{
		q:         q,
		sessions:  sessions,
		messages:  messages,
		clipboard: NewClipboardManager(),
	}
}

func (s *service) ExportSession(ctx context.Context, sessionID string, opts ExportOptions) (*ExportResult, error) {
	// Get session
	sess, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	// Get messages (using message service for proper parsing)
	msgs, err := s.messages.List(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}

	// Render based on format
	var content string
	switch opts.Format {
	case FormatMarkdown:
		renderer := NewMarkdownRenderer(opts)
		content, err = renderer.Render(sess, msgs)
		if err != nil {
			return nil, fmt.Errorf("render markdown: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported format: %v", opts.Format)
	}

	return &ExportResult{
		Content:  content,
		Size:     int64(len(content)),
		Messages: len(msgs),
	}, nil
}

func (s *service) ExportToClipboard(ctx context.Context, sessionID string, opts ExportOptions) error {
	result, err := s.ExportSession(ctx, sessionID, opts)
	if err != nil {
		return err
	}

	return s.clipboard.Copy(result.Content)
}

func (s *service) ExportToFile(ctx context.Context, sessionID string, path string, opts ExportOptions) error {
	result, err := s.ExportSession(ctx, sessionID, opts)
	if err != nil {
		return err
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Write atomically
	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(result.Content), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("rename file: %w", err)
	}

	return nil
}

func GenerateFilename(sess session.Session) string {
	title := sess.Title
	if title == "" {
		title = "untitled"
	}

	// Sanitize
	for _, c := range []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"} {
		title = strings.ReplaceAll(title, c, "_")
	}

	// Truncate
	if len(title) > 50 {
		title = title[:50]
	}

	return fmt.Sprintf("%s.md", title)
}
