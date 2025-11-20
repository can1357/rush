package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/can1357/rush/agent/prompt"
	"github.com/can1357/rush/agent/tools"
	"github.com/can1357/rush/genai"
	"github.com/can1357/rush/permission"
)

//go:embed templates/agentic_fetch.md
var agenticFetchToolDescription []byte

// agenticFetchValidationResult holds the validated parameters from the tool call context.
type agenticFetchValidationResult struct {
	SessionID      string
	AgentMessageID string
}

// validateAgenticFetchParams validates the tool call parameters and extracts required context values.
func validateAgenticFetchParams(ctx context.Context, params tools.AgenticFetchParams) (agenticFetchValidationResult, error) {
	if params.URL == "" {
		return agenticFetchValidationResult{}, errors.New("url is required")
	}

	if params.Prompt == "" {
		return agenticFetchValidationResult{}, errors.New("prompt is required")
	}

	sessionID := tools.GetSessionFromContext(ctx)
	if sessionID == "" {
		return agenticFetchValidationResult{}, errors.New("session id missing from context")
	}

	agentMessageID := tools.GetMessageFromContext(ctx)
	if agentMessageID == "" {
		return agenticFetchValidationResult{}, errors.New("agent message id missing from context")
	}

	return agenticFetchValidationResult{
		SessionID:      sessionID,
		AgentMessageID: agentMessageID,
	}, nil
}

//go:embed templates/agentic_fetch_prompt.md.tpl
var agenticFetchPromptTmpl []byte

func (c *coordinator) agenticFetchTool(_ context.Context, client *http.Client) (genai.AgentTool, error) {
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}

	return genai.NewAgentTool(
		tools.AgenticFetchToolName,
		string(agenticFetchToolDescription),
		func(ctx context.Context, params tools.AgenticFetchParams, call genai.ToolCall) (genai.ToolResponse, error) {
			validationResult, err := validateAgenticFetchParams(ctx, params)
			if err != nil {
				return genai.NewTextErrorResponse(err.Error()), nil
			}

			p := c.permissions.Request(
				permission.CreatePermissionRequest{
					SessionID:   validationResult.SessionID,
					Path:        c.cfg.WorkingDir(),
					ToolCallID:  call.ID,
					ToolName:    tools.AgenticFetchToolName,
					Action:      "fetch",
					Description: fmt.Sprintf("Fetch and analyze content from URL: %s", params.URL),
					Params:      tools.AgenticFetchPermissionsParams(params),
				},
			)

			if !p {
				return genai.ToolResponse{}, permission.ErrorPermissionDenied
			}

			content, err := tools.FetchURLAndConvert(ctx, client, params.URL)
			if err != nil {
				return genai.NewTextErrorResponse(fmt.Sprintf("Failed to fetch URL: %s", err)), nil
			}

			tmpDir, err := os.MkdirTemp(c.cfg.Options.DataDirectory, "rush-fetch-*")
			if err != nil {
				return genai.NewTextErrorResponse(fmt.Sprintf("Failed to create temporary directory: %s", err)), nil
			}
			defer os.RemoveAll(tmpDir)

			hasLargeContent := len(content) > tools.LargeContentThreshold
			var fullPrompt string

			if hasLargeContent {
				tempFile, err := os.CreateTemp(tmpDir, "page-*.md")
				if err != nil {
					return genai.NewTextErrorResponse(fmt.Sprintf("Failed to create temporary file: %s", err)), nil
				}
				tempFilePath := tempFile.Name()

				if _, err := tempFile.WriteString(content); err != nil {
					tempFile.Close()
					return genai.NewTextErrorResponse(fmt.Sprintf("Failed to write content to file: %s", err)), nil
				}
				tempFile.Close()

				fullPrompt = fmt.Sprintf("%s\n\nThe web page from %s has been saved to: %s\n\nUse the view and grep tools to analyze this file and extract the requested information.", params.Prompt, params.URL, tempFilePath)
			} else {
				fullPrompt = fmt.Sprintf("%s\n\nWeb page URL: %s\n\n<webpage_content>\n%s\n</webpage_content>", params.Prompt, params.URL, content)
			}

			promptOpts := []prompt.Option{
				prompt.WithWorkingDir(tmpDir),
			}

			promptTemplate, err := prompt.NewPrompt("agentic_fetch", string(agenticFetchPromptTmpl), promptOpts...)
			if err != nil {
				return genai.ToolResponse{}, fmt.Errorf("error creating prompt: %s", err)
			}

			_, small, err := c.buildAgentModels(ctx)
			if err != nil {
				return genai.ToolResponse{}, fmt.Errorf("error building models: %s", err)
			}

			systemPrompt, err := promptTemplate.Build(ctx, small.Model.Provider(), small.Model.Model(), *c.cfg)
			if err != nil {
				return genai.ToolResponse{}, fmt.Errorf("error building system prompt: %s", err)
			}

			smallProviderCfg, ok := c.cfg.Providers.Get(small.ModelCfg.Provider)
			if !ok {
				return genai.ToolResponse{}, errors.New("small model provider not configured")
			}

			webFetchTool := tools.NewWebFetchTool(tmpDir, client)
			fetchTools := []genai.AgentTool{
				webFetchTool,
				tools.NewGlobTool(tmpDir),
				tools.NewGrepTool(tmpDir),
				tools.NewViewTool(c.lspClients, c.permissions, tmpDir),
			}

			agent := NewSessionAgent(SessionAgentOptions{
				LargeModel:           small, // Use small model for both (fetch doesn't need large)
				SmallModel:           small,
				SystemPromptPrefix:   smallProviderCfg.SystemPromptPrefix,
				SystemPrompt:         systemPrompt,
				DisableAutoSummarize: c.cfg.Options.DisableAutoSummarize,
				IsYolo:               c.permissions.SkipRequests(),
				Sessions:             c.sessions,
				Messages:             c.messages,
				Reminders:            nil, // Sub-agents don't need reminders
				Tools:                fetchTools,
			})

			agentToolSessionID := c.sessions.CreateAgentToolSessionID(validationResult.AgentMessageID, call.ID)
			session, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, validationResult.SessionID, "Fetch Analysis")
			if err != nil {
				return genai.ToolResponse{}, fmt.Errorf("error creating session: %s", err)
			}

			c.permissions.AutoApproveSession(session.ID)

			// Use small model for web content analysis (faster and cheaper)
			maxTokens := small.CatwalkCfg.DefaultMaxTokens
			if small.ModelCfg.MaxTokens != 0 {
				maxTokens = small.ModelCfg.MaxTokens
			}

			result, err := agent.Run(ctx, SessionAgentCall{
				SessionID:        session.ID,
				Prompt:           fullPrompt,
				MaxOutputTokens:  maxTokens,
				ProviderOptions:  getProviderOptions(small, smallProviderCfg),
				Temperature:      small.ModelCfg.Temperature,
				TopP:             small.ModelCfg.TopP,
				TopK:             small.ModelCfg.TopK,
				FrequencyPenalty: small.ModelCfg.FrequencyPenalty,
				PresencePenalty:  small.ModelCfg.PresencePenalty,
			})
			if err != nil {
				return genai.NewTextErrorResponse("error generating response"), nil
			}

			updatedSession, err := c.sessions.Get(ctx, session.ID)
			if err != nil {
				return genai.ToolResponse{}, fmt.Errorf("error getting session: %s", err)
			}
			parentSession, err := c.sessions.Get(ctx, validationResult.SessionID)
			if err != nil {
				return genai.ToolResponse{}, fmt.Errorf("error getting parent session: %s", err)
			}

			parentSession.Cost += updatedSession.Cost

			_, err = c.sessions.Save(ctx, parentSession)
			if err != nil {
				return genai.ToolResponse{}, fmt.Errorf("error saving parent session: %s", err)
			}

			return genai.NewTextResponse(result.Response.Content.Text()), nil
		}), nil
}
