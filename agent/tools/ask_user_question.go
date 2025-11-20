package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/can1357/rush/ai"
	"github.com/can1357/rush/question"
)

//go:embed ask_user_question.md
var askUserQuestionDescription []byte

const AskUserQuestionToolName = "ask_user_question"

type AskUserQuestionParams struct {
	Questions []QuestionItem `json:"questions" description:"Array of 1-4 questions to ask the user"`
}

type QuestionItem struct {
	Question    string       `json:"question" description:"The question to ask (should be clear and end with '?')"`
	Header      string       `json:"header" description:"Short label (max 12 chars) like 'Library' or 'Approach'"`
	MultiSelect bool         `json:"multiSelect" description:"Allow multiple selections (checkboxes instead of radio buttons)"`
	Options     []OptionItem `json:"options" description:"2-4 answer options (system will auto-add 'Other' option)"`
}

type OptionItem struct {
	Label       string `json:"label" description:"Display text (1-5 words)"`
	Description string `json:"description" description:"Explanation of what this option means"`
}

type AskUserQuestionResponseMetadata struct {
	SessionID string            `json:"session_id"`
	Answers   map[string]string `json:"answers"`
}

func NewAskUserQuestionTool(questionService question.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		AskUserQuestionToolName,
		string(askUserQuestionDescription),
		func(ctx context.Context, params AskUserQuestionParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session ID is required for asking questions"), nil
			}

			// Validate params
			if len(params.Questions) == 0 || len(params.Questions) > 4 {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("must provide 1-4 questions, got %d", len(params.Questions))), nil
			}

			// Convert to internal format
			questions := make([]question.Question, len(params.Questions))
			for i, q := range params.Questions {
				// Validate question
				if q.Question == "" {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("question %d: question text is required", i)), nil
				}
				if q.Header == "" {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("question %d: header is required", i)), nil
				}
				if len(q.Header) > 12 {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("question %d: header too long (%d chars, max 12)", i, len(q.Header))), nil
				}
				if len(q.Options) < 2 || len(q.Options) > 4 {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("question %d: must have 2-4 options, got %d", i, len(q.Options))), nil
				}

				options := make([]question.Option, len(q.Options))
				for j, opt := range q.Options {
					if opt.Label == "" {
						return fantasy.NewTextErrorResponse(fmt.Sprintf("question %d, option %d: label is required", i, j)), nil
					}
					if opt.Description == "" {
						return fantasy.NewTextErrorResponse(fmt.Sprintf("question %d, option %d: description is required", i, j)), nil
					}
					options[j] = question.Option{
						Label:       opt.Label,
						Description: opt.Description,
					}
				}

				questions[i] = question.Question{
					Question:    q.Question,
					Header:      q.Header,
					MultiSelect: q.MultiSelect,
					Options:     options,
				}
			}

			// Ask the questions (blocks until user responds)
			answers, err := questionService.Ask(ctx, sessionID, questions)
			if err != nil {
				if strings.Contains(err.Error(), "canceled") {
					return fantasy.NewTextErrorResponse("User canceled the question"), nil
				}
				return fantasy.ToolResponse{}, fmt.Errorf("failed to ask question: %w", err)
			}

			// Format response
			var output strings.Builder
			output.WriteString("User provided the following answers:\n\n")

			for i, q := range params.Questions {
				idx := fmt.Sprintf("%d", i)
				answer, ok := answers[idx]
				if !ok {
					answer = "(no answer provided)"
				}

				output.WriteString(fmt.Sprintf("**%s**: %s\n", q.Header, q.Question))
				output.WriteString(fmt.Sprintf("Answer: %s\n\n", answer))
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output.String()),
				AskUserQuestionResponseMetadata{
					SessionID: sessionID,
					Answers:   answers,
				},
			), nil
		})
}
