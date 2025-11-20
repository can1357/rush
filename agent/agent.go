// Package agent is the core orchestration layer for Rush AI agents.
//
// It provides session-based AI agent functionality for managing
// conversations, tool execution, and message handling. It coordinates
// interactions between language models, messages, sessions, and tools while
// handling features like automatic summarization, queuing, and token
// management.
package agent

import (
	"cmp"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/can1357/rush/agent/reminder"
	"github.com/can1357/rush/agent/tools"
	"github.com/can1357/rush/config"
	"github.com/can1357/rush/csync"
	"github.com/can1357/rush/genai"
	"github.com/can1357/rush/genai/providers/anthropic"
	"github.com/can1357/rush/genai/providers/bedrock"
	"github.com/can1357/rush/genai/providers/google"
	"github.com/can1357/rush/genai/providers/openai"
	"github.com/can1357/rush/genai/providers/openrouter"
	"github.com/can1357/rush/message"
	"github.com/can1357/rush/permission"
	"github.com/can1357/rush/session"
	"github.com/can1357/rush/stringext"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
)

//go:embed templates/title.md
var titlePrompt []byte

//go:embed templates/summary.md
var summaryPrompt []byte

type SessionAgentCall struct {
	SessionID        string
	Prompt           string
	ProviderOptions  genai.ProviderOptions
	Attachments      []message.Attachment
	MaxOutputTokens  int64
	Temperature      *float64
	TopP             *float64
	TopK             *int64
	FrequencyPenalty *float64
	PresencePenalty  *float64
}

type SessionAgent interface {
	Run(context.Context, SessionAgentCall) (*genai.AgentResult, error)
	SetModel(model Model)
	SetTools(tools []genai.AgentTool)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	QueuedPrompts(sessionID string) int
	ClearQueue(sessionID string)
	Summarize(context.Context, string, genai.ProviderOptions) error
	Model() Model
}

type Model struct {
	Model     genai.LanguageModel
	Props     *catwalk.Model
	Selection config.ModelSelection
}

type sessionAgent struct {
	model                Model
	smallModel           Model
	systemPromptPrefix   string
	systemPrompt         string
	tools                []genai.AgentTool
	sessions             session.Service
	messages             message.Service
	reminders            *reminder.Service
	disableAutoSummarize bool
	isYolo               bool

	messageQueue   *csync.Map[string, []SessionAgentCall]
	activeRequests *csync.Map[string, context.CancelFunc]
}

type SessionAgentOptions struct {
	Model                Model
	SmallModel           Model
	SystemPromptPrefix   string
	SystemPrompt         string
	DisableAutoSummarize bool
	IsYolo               bool
	Sessions             session.Service
	Messages             message.Service
	Reminders            *reminder.Service
	Tools                []genai.AgentTool
}

func NewSessionAgent(
	opts SessionAgentOptions,
) SessionAgent {
	return &sessionAgent{
		model:                opts.Model,
		smallModel:           opts.SmallModel,
		systemPromptPrefix:   opts.SystemPromptPrefix,
		systemPrompt:         opts.SystemPrompt,
		sessions:             opts.Sessions,
		messages:             opts.Messages,
		reminders:            opts.Reminders,
		disableAutoSummarize: opts.DisableAutoSummarize,
		tools:                opts.Tools,
		isYolo:               opts.IsYolo,
		messageQueue:         csync.NewMap[string, []SessionAgentCall](),
		activeRequests:       csync.NewMap[string, context.CancelFunc](),
	}
}

func (a *sessionAgent) Run(ctx context.Context, call SessionAgentCall) (*genai.AgentResult, error) {
	if call.Prompt == "" {
		return nil, ErrEmptyPrompt
	}
	if call.SessionID == "" {
		return nil, ErrSessionMissing
	}

	// Queue the message if busy
	if a.IsSessionBusy(call.SessionID) {
		existing, ok := a.messageQueue.Get(call.SessionID)
		if !ok {
			existing = []SessionAgentCall{}
		}
		existing = append(existing, call)
		a.messageQueue.Set(call.SessionID, existing)
		return nil, nil
	}

	if len(a.tools) > 0 {
		// Add Anthropic caching to the last tool.
		a.tools[len(a.tools)-1].SetProviderOptions(a.getCacheControlOptions())
	}

	agent := genai.NewAgent(
		a.model.Model,
		genai.WithSystemPrompt(a.systemPrompt),
		genai.WithTools(a.tools...),
		genai.WithStopConditions(
			genai.StepCountIs(50),        // Max 50 agentic turns to prevent infinite loops.
			genai.MaxTokensUsed(100_000), // Max 100k tokens per agent call.
		),
	)

	sessionLock := sync.Mutex{}
	currentSession, err := a.sessions.Get(ctx, call.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return nil, fmt.Errorf("failed to get session messages: %w", err)
	}

	var wg sync.WaitGroup
	// Generate title if first message.
	if len(msgs) == 0 {
		wg.Go(func() {
			sessionLock.Lock()
			a.generateTitle(ctx, &currentSession, call.Prompt)
			sessionLock.Unlock()
		})
	}

	// Add the user message to the session.
	_, err = a.createUserMessage(ctx, call)
	if err != nil {
		return nil, err
	}

	// Add the session to the context.
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, call.SessionID)

	genCtx, cancel := context.WithCancel(ctx)
	a.activeRequests.Set(call.SessionID, cancel)

	defer cancel()
	defer a.activeRequests.Del(call.SessionID)

	history, files := a.preparePrompt(msgs, call.Attachments...)

	var currentAssistant *message.Message
	var shouldSummarize bool
	result, err := agent.Stream(genCtx, genai.AgentStreamCall{
		Prompt:           call.Prompt,
		Files:            files,
		Messages:         history,
		ProviderOptions:  call.ProviderOptions,
		MaxOutputTokens:  &call.MaxOutputTokens,
		TopP:             call.TopP,
		Temperature:      call.Temperature,
		PresencePenalty:  call.PresencePenalty,
		TopK:             call.TopK,
		FrequencyPenalty: call.FrequencyPenalty,
		// Before each step create a new assistant message.
		PrepareStep: func(callContext context.Context, options genai.PrepareStepFunctionOptions) (_ context.Context, prepared genai.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			// Reset all cached items.
			for i := range prepared.Messages {
				prepared.Messages[i].ProviderOptions = nil
			}

			queuedCalls, _ := a.messageQueue.Get(call.SessionID)
			a.messageQueue.Del(call.SessionID)
			for _, queued := range queuedCalls {
				userMessage, createErr := a.createUserMessage(callContext, queued)
				if createErr != nil {
					return callContext, prepared, createErr
				}
				prepared.Messages = append(prepared.Messages, userMessage.ToAIMessage()...)
			}

			lastSystemRoleInx := 0
			systemMessageUpdated := false
			for i, msg := range prepared.Messages {
				// Only add cache control to the last message.
				if msg.Role == genai.MessageRoleSystem {
					lastSystemRoleInx = i
				} else if !systemMessageUpdated {
					prepared.Messages[lastSystemRoleInx].ProviderOptions = a.getCacheControlOptions()
					systemMessageUpdated = true
				}
				// Than add cache control to the last 2 messages.
				if i > len(prepared.Messages)-3 {
					prepared.Messages[i].ProviderOptions = a.getCacheControlOptions()
				}
			}

			// Inject todo reminders if available
			combinedPrefix := a.systemPromptPrefix
			if a.reminders != nil {
				reminderText, reminderErr := a.reminders.BuildReminders(callContext, call.SessionID)
				if reminderErr == nil && reminderText != "" {
					if combinedPrefix != "" {
						combinedPrefix = combinedPrefix + "\n\n" + reminderText
					} else {
						combinedPrefix = reminderText
					}
				}
			}

			// Inject plan mode instructions if active
			sess, sessErr := a.sessions.Get(callContext, call.SessionID)
			if sessErr == nil && sess.PlanMode {
				planModeInstructions := `<system-reminder>
Plan mode is active. The user indicated that they do not want you to execute yet -- you MUST NOT make any edits, run any non-readonly tools (including changing configs or making commits), or otherwise make any changes to the system. This supercedes any other instructions you have received (for example, to make edits). Instead, you should:
1. Answer the user's query comprehensively, using the AskUserQuestion tool if you need to ask the user clarifying questions. If you do use the AskUserQuestion, make sure to ask all clarifying questions you need to fully understand the user's intent before proceeding. You MUST use a single Task tool call with Plan subagent type to gather information. Even if you have already started researching directly, you must immediately switch to using an agent instead.
2. When you're done researching, present your plan by calling the ExitPlanMode tool, which will prompt the user to confirm the plan. Do NOT make any file changes or run any tools that modify the system state in any way until the user has confirmed the plan.
</system-reminder>`
				if combinedPrefix != "" {
					combinedPrefix = combinedPrefix + "\n\n" + planModeInstructions
				} else {
					combinedPrefix = planModeInstructions
				}
			}

			if combinedPrefix != "" {
				prepared.Messages = append([]genai.Message{genai.NewSystemMessage(combinedPrefix)}, prepared.Messages...)
			}

			var assistantMsg message.Message
			assistantMsg, err = a.messages.Create(callContext, call.SessionID, message.CreateMessageParams{
				Role:     message.Assistant,
				Parts:    []message.ContentPart{},
				Model:    a.model.Selection.Model,
				Provider: a.model.Selection.Provider,
			})
			if err != nil {
				return callContext, prepared, err
			}
			callContext = context.WithValue(callContext, tools.MessageIDContextKey, assistantMsg.ID)
			currentAssistant = &assistantMsg
			return callContext, prepared, err
		},
		OnReasoningStart: func(id string, reasoning genai.ReasoningContent) error {
			currentAssistant.AppendReasoningContent(reasoning.Text)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnReasoningDelta: func(id string, text string) error {
			currentAssistant.AppendReasoningContent(text)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnReasoningEnd: func(id string, reasoning genai.ReasoningContent) error {
			// handle anthropic signature
			if anthropicData, ok := reasoning.ProviderMetadata[anthropic.Name]; ok {
				if reasoning, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok {
					currentAssistant.AppendReasoningSignature(reasoning.Signature)
				}
			}
			if googleData, ok := reasoning.ProviderMetadata[google.Name]; ok {
				if reasoning, ok := googleData.(*google.ReasoningMetadata); ok {
					currentAssistant.AppendThoughtSignature(reasoning.Signature, reasoning.ToolID)
				}
			}
			if openaiData, ok := reasoning.ProviderMetadata[openai.Name]; ok {
				if reasoning, ok := openaiData.(*openai.ResponsesReasoningMetadata); ok {
					currentAssistant.SetReasoningResponsesData(reasoning)
				}
			}
			currentAssistant.FinishThinking()
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnTextDelta: func(id string, text string) error {
			// Strip leading newline from initial text content. This is is
			// particularly important in non-interactive mode where leading
			// newlines are very visible.
			if len(currentAssistant.Parts) == 0 {
				text = strings.TrimPrefix(text, "\n")
			}

			currentAssistant.AppendContent(text)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnToolInputStart: func(id string, toolName string) error {
			toolCall := message.ToolCall{
				ID:               id,
				Name:             toolName,
				ProviderExecuted: false,
				Finished:         false,
			}
			currentAssistant.AddToolCall(toolCall)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnRetry: func(err *genai.ProviderError, delay time.Duration) {
			// TODO: implement
		},
		OnToolCall: func(tc genai.ToolCallContent) error {
			toolCall := message.ToolCall{
				ID:               tc.ToolCallID,
				Name:             tc.ToolName,
				Input:            tc.Input,
				ProviderExecuted: false,
				Finished:         true,
			}
			currentAssistant.AddToolCall(toolCall)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnToolResult: func(result genai.ToolResultContent) error {
			var resultContent string
			isError := false
			switch result.Result.GetType() {
			case genai.ToolResultContentTypeText:
				r, ok := genai.AsToolResultOutputType[genai.ToolResultOutputContentText](result.Result)
				if ok {
					resultContent = r.Text
				}
			case genai.ToolResultContentTypeError:
				r, ok := genai.AsToolResultOutputType[genai.ToolResultOutputContentError](result.Result)
				if ok {
					isError = true
					resultContent = r.Error.Error()
				}
			case genai.ToolResultContentTypeMedia:
				// TODO: handle this message type
			}
			toolResult := message.ToolResult{
				ToolCallID: result.ToolCallID,
				Name:       result.ToolName,
				Content:    resultContent,
				IsError:    isError,
				Metadata:   result.ClientMetadata,
			}
			_, createMsgErr := a.messages.Create(genCtx, currentAssistant.SessionID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					toolResult,
				},
			})
			if createMsgErr != nil {
				return createMsgErr
			}
			return nil
		},
		OnStepFinish: func(stepResult genai.StepResult) error {
			finishReason := message.FinishReasonUnknown
			switch stepResult.FinishReason {
			case genai.FinishReasonLength:
				finishReason = message.FinishReasonMaxTokens
			case genai.FinishReasonStop:
				finishReason = message.FinishReasonEndTurn
			case genai.FinishReasonToolCalls:
				finishReason = message.FinishReasonToolUse
			}
			currentAssistant.AddFinish(finishReason, "", "")
			a.updateSessionUsage(a.model, &currentSession, stepResult.Usage, a.openrouterCost(stepResult.ProviderMetadata))

			// Increment turn count if message has actual content (not thinking-only)
			if !currentAssistant.IsThinkingOnly() {
				currentSession.AssistantTurnCount++
			}

			sessionLock.Lock()
			_, sessionErr := a.sessions.Save(genCtx, currentSession)
			sessionLock.Unlock()
			if sessionErr != nil {
				return sessionErr
			}
			return a.messages.Update(genCtx, *currentAssistant)
		},
		StopWhen: []genai.StopCondition{
			func(_ []genai.StepResult) bool {
				cw := int64(a.model.Props.ContextWindow)
				tokens := currentSession.CompletionTokens + currentSession.PromptTokens
				remaining := cw - tokens
				var threshold int64
				if cw > 200_000 {
					threshold = 20_000
				} else {
					threshold = int64(float64(cw) * 0.2)
				}
				if (remaining <= threshold) && !a.disableAutoSummarize {
					slog.Info("auto-summarization triggered",
						"reason", "token budget threshold exceeded",
						"context_window", cw,
						"tokens_used", tokens,
						"tokens_remaining", remaining,
						"threshold", threshold,
						"session_id", call.SessionID,
					)
					shouldSummarize = true
					return true
				}
				return false
			},
		},
	})

	if err != nil {
		isCancelErr := errors.Is(err, context.Canceled)
		isPermissionErr := errors.Is(err, permission.ErrorPermissionDenied)
		if currentAssistant == nil {
			return result, err
		}
		// Ensure we finish thinking on error to close the reasoning state.
		currentAssistant.FinishThinking()
		toolCalls := currentAssistant.ToolCalls()
		// INFO: we use the parent context here because the genCtx has been cancelled.
		msgs, createErr := a.messages.List(ctx, currentAssistant.SessionID)
		if createErr != nil {
			return nil, createErr
		}
		for _, tc := range toolCalls {
			if !tc.Finished {
				tc.Finished = true
				tc.Input = "{}"
				currentAssistant.AddToolCall(tc)
				updateErr := a.messages.Update(ctx, *currentAssistant)
				if updateErr != nil {
					return nil, updateErr
				}
			}

			found := false
			for _, msg := range msgs {
				if msg.Role == message.Tool {
					for _, tr := range msg.ToolResults() {
						if tr.ToolCallID == tc.ID {
							found = true
							break
						}
					}
				}
				if found {
					break
				}
			}
			if found {
				continue
			}
			content := "There was an error while executing the tool"
			if isCancelErr {
				content = "Tool execution canceled by user"
			} else if isPermissionErr {
				content = "User denied permission"
			}
			toolResult := message.ToolResult{
				ToolCallID: tc.ID,
				Name:       tc.Name,
				Content:    content,
				IsError:    true,
			}
			_, createErr = a.messages.Create(context.Background(), currentAssistant.SessionID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					toolResult,
				},
			})
			if createErr != nil {
				return nil, createErr
			}
		}
		var fantasyErr *genai.Error
		var providerErr *genai.ProviderError
		const defaultTitle = "Provider Error"
		if isCancelErr {
			currentAssistant.AddFinish(message.FinishReasonCanceled, "User canceled request", "")
		} else if isPermissionErr {
			currentAssistant.AddFinish(message.FinishReasonPermissionDenied, "User denied permission", "")
		} else if errors.As(err, &providerErr) {
			currentAssistant.AddFinish(message.FinishReasonError, cmp.Or(stringext.Capitalize(providerErr.Title), defaultTitle), providerErr.Message)
		} else if errors.As(err, &fantasyErr) {
			currentAssistant.AddFinish(message.FinishReasonError, cmp.Or(stringext.Capitalize(fantasyErr.Title), defaultTitle), fantasyErr.Message)
		} else {
			currentAssistant.AddFinish(message.FinishReasonError, defaultTitle, err.Error())
		}
		// Note: we use the parent context here because the genCtx has been
		// cancelled.
		updateErr := a.messages.Update(ctx, *currentAssistant)
		if updateErr != nil {
			return nil, updateErr
		}
		return nil, err
	}
	wg.Wait()

	if shouldSummarize {
		// Must release active request before Summarize, which checks IsSessionBusy
		a.activeRequests.Del(call.SessionID)
		if summarizeErr := a.Summarize(genCtx, call.SessionID, call.ProviderOptions); summarizeErr != nil {
			return nil, summarizeErr
		}
		// Always requeue to continue work after summarization
		existing, ok := a.messageQueue.Get(call.SessionID)
		if !ok {
			existing = []SessionAgentCall{}
		}
		// Preserve the original intent but indicate this is a continuation
		call.Prompt = "Continue with the task described in the conversation above. Complete any remaining work."
		existing = append(existing, call)
		a.messageQueue.Set(call.SessionID, existing)
	}

	// Release active request before processing queued messages.
	a.activeRequests.Del(call.SessionID)
	cancel()

	queuedMessages, ok := a.messageQueue.Get(call.SessionID)
	if !ok || len(queuedMessages) == 0 {
		return result, err
	}
	// There are queued messages restart the loop.
	firstQueuedMessage := queuedMessages[0]
	a.messageQueue.Set(call.SessionID, queuedMessages[1:])
	return a.Run(ctx, firstQueuedMessage)
}

func (a *sessionAgent) Summarize(ctx context.Context, sessionID string, opts genai.ProviderOptions) error {
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}

	currentSession, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		// Nothing to summarize.
		return nil
	}

	aiMsgs, _ := a.preparePrompt(msgs)

	genCtx, cancel := context.WithCancel(ctx)
	a.activeRequests.Set(sessionID, cancel)
	defer a.activeRequests.Del(sessionID)
	defer cancel()

	agent := genai.NewAgent(a.smallModel.Model,
		genai.WithSystemPrompt("You are a helpful AI assistant tasked with summarizing conversations."),
		genai.WithStopConditions(
			genai.StepCountIs(5),        // Max 5 steps for summarization.
			genai.MaxTokensUsed(50_000), // Max 50k tokens for summarization.
		),
	)
	summaryMessage, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:             message.Assistant,
		Model:            a.smallModel.Model.Model().ID,
		Provider:         a.smallModel.Model.Provider(),
		IsSummaryMessage: true,
	})
	if err != nil {
		return err
	}

	resp, err := agent.Stream(genCtx, genai.AgentStreamCall{
		Prompt:          string(summaryPrompt),
		Messages:        aiMsgs,
		ProviderOptions: opts,
		PrepareStep: func(callContext context.Context, options genai.PrepareStepFunctionOptions) (_ context.Context, prepared genai.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			prepared.DisableAllTools = true
			if a.systemPromptPrefix != "" {
				prepared.Messages = append([]genai.Message{genai.NewSystemMessage(a.systemPromptPrefix)}, prepared.Messages...)
			}
			return callContext, prepared, nil
		},
		OnReasoningDelta: func(id string, text string) error {
			summaryMessage.AppendReasoningContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
		OnReasoningEnd: func(id string, reasoning genai.ReasoningContent) error {
			// Handle anthropic signature.
			if anthropicData, ok := reasoning.ProviderMetadata["anthropic"]; ok {
				if signature, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok && signature.Signature != "" {
					summaryMessage.AppendReasoningSignature(signature.Signature)
				}
			}
			summaryMessage.FinishThinking()
			return a.messages.Update(genCtx, summaryMessage)
		},
		OnTextDelta: func(id, text string) error {
			summaryMessage.AppendContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
	})
	if err != nil {
		isCancelErr := errors.Is(err, context.Canceled)
		if isCancelErr {
			// User cancelled summarize we need to remove the summary message.
			deleteErr := a.messages.Delete(ctx, summaryMessage.ID)
			return deleteErr
		}
		return err
	}

	summaryMessage.AddFinish(message.FinishReasonEndTurn, "", "")
	err = a.messages.Update(genCtx, summaryMessage)
	if err != nil {
		return err
	}

	// Add a continuation prompt after the summary to resume work
	continuationMsg, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:     message.User,
		Model:    a.smallModel.Model.Model().ID,
		Provider: a.smallModel.Model.Provider(),
	})
	if err != nil {
		return err
	}
	continuationMsg.AppendContent("<system-reminder>This session is being continued from a previous conversation that ran out of context. The conversation is summarized above. Please continue the conversation from where we left it off without asking the user any further questions. Continue with the last task that you were asked to work on.</system-reminder>")
	continuationMsg.AddFinish(message.FinishReasonEndTurn, "", "")
	err = a.messages.Update(ctx, continuationMsg)
	if err != nil {
		return err
	}

	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
	}

	a.updateSessionUsage(a.model, &currentSession, resp.TotalUsage, openrouterCost)

	// Just in case, get just the last usage info.
	usage := resp.Response.Usage
	currentSession.SummaryMessageID = summaryMessage.ID
	currentSession.CompletionTokens = usage.OutputTokens
	currentSession.PromptTokens = 0
	_, err = a.sessions.Save(genCtx, currentSession)
	return err
}

func (a *sessionAgent) getCacheControlOptions() genai.ProviderOptions {
	if t, _ := strconv.ParseBool(os.Getenv("RUSH_DISABLE_ANTHROPIC_CACHE")); t {
		return genai.ProviderOptions{}
	}
	return genai.ProviderOptions{
		anthropic.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
		bedrock.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
	}
}

func (a *sessionAgent) createUserMessage(ctx context.Context, call SessionAgentCall) (message.Message, error) {
	var attachmentParts []message.ContentPart
	for _, attachment := range call.Attachments {
		attachmentParts = append(attachmentParts, message.BinaryContent{Path: attachment.FilePath, MIMEType: attachment.MimeType, Data: attachment.Content})
	}
	parts := []message.ContentPart{message.TextContent{Text: call.Prompt}}
	parts = append(parts, attachmentParts...)
	msg, err := a.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: parts,
	})
	if err != nil {
		return message.Message{}, fmt.Errorf("failed to create user message: %w", err)
	}
	return msg, nil
}

func (a *sessionAgent) preparePrompt(msgs []message.Message, attachments ...message.Attachment) ([]genai.Message, []genai.FilePart) {
	var history []genai.Message
	for _, m := range msgs {
		if len(m.Parts) == 0 {
			continue
		}
		// Assistant message without content or tool calls (cancelled before it
		// returned anything).
		if m.Role == message.Assistant && len(m.ToolCalls()) == 0 && m.Content().Text == "" && m.ReasoningContent().String() == "" {
			continue
		}
		history = append(history, m.ToAIMessage()...)
	}

	var files []genai.FilePart
	for _, attachment := range attachments {
		files = append(files, genai.FilePart{
			Filename:  attachment.FileName,
			Data:      attachment.Content,
			MediaType: attachment.MimeType,
		})
	}

	return history, files
}

func (a *sessionAgent) getSessionMessages(ctx context.Context, session session.Session) ([]message.Message, error) {
	msgs, err := a.messages.List(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}

	if session.SummaryMessageID != "" {
		summaryMsgInex := -1
		for i, msg := range msgs {
			if msg.ID == session.SummaryMessageID {
				summaryMsgInex = i
				break
			}
		}
		if summaryMsgInex != -1 {
			msgs = msgs[summaryMsgInex:]
			msgs[0].Role = message.User
		}
	}
	return msgs, nil
}

func (a *sessionAgent) generateTitle(ctx context.Context, session *session.Session, prompt string) {
	if prompt == "" {
		return
	}

	var maxOutput int64 = 40
	if a.smallModel.Props.CanReason {
		maxOutput = a.smallModel.Props.DefaultMaxTokens
	}

	agent := genai.NewAgent(a.smallModel.Model,
		genai.WithSystemPrompt(string(titlePrompt)+"\n /no_think"),
		genai.WithMaxOutputTokens(maxOutput),
		genai.WithStopConditions(
			genai.StepCountIs(3),       // Max 3 steps for title generation.
			genai.MaxTokensUsed(5_000), // Max 5k tokens for title generation.
		),
	)

	resp, err := agent.Stream(ctx, genai.AgentStreamCall{
		Prompt: fmt.Sprintf("Generate a concise title for the following content:\n\n%s\n <think>\n\n</think>", prompt),
		PrepareStep: func(callContext context.Context, options genai.PrepareStepFunctionOptions) (_ context.Context, prepared genai.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			prepared.DisableAllTools = true
			if a.systemPromptPrefix != "" {
				prepared.Messages = append([]genai.Message{genai.NewSystemMessage(a.systemPromptPrefix)}, prepared.Messages...)
			}
			return callContext, prepared, nil
		},
	})
	if err != nil {
		slog.Error("error generating title", "err", err)
		return
	}

	title := resp.Response.Content.Text()

	title = strings.ReplaceAll(title, "\n", " ")

	// Remove thinking tags if present.
	if idx := strings.Index(title, "</think>"); idx > 0 {
		title = title[idx+len("</think>"):]
	}

	title = strings.TrimSpace(title)
	if title == "" {
		slog.Warn("failed to generate title", "warn", "empty title")
		return
	}

	session.Title = title

	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
	}

	a.updateSessionUsage(a.model, session, resp.TotalUsage, openrouterCost)
	_, saveErr := a.sessions.Save(ctx, *session)
	if saveErr != nil {
		slog.Error("failed to save session title & usage", "error", saveErr)
		return
	}
}

func (a *sessionAgent) openrouterCost(metadata genai.ProviderMetadata) *float64 {
	openrouterMetadata, ok := metadata[openrouter.Name]
	if !ok {
		return nil
	}

	opts, ok := openrouterMetadata.(*openrouter.ProviderMetadata)
	if !ok {
		return nil
	}
	return &opts.Usage.Cost
}

func (a *sessionAgent) updateSessionUsage(model Model, session *session.Session, usage genai.Usage, overrideCost *float64) {
	modelConfig := model.Props
	cost := modelConfig.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		modelConfig.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		modelConfig.CostPer1MIn/1e6*float64(usage.InputTokens) +
		modelConfig.CostPer1MOut/1e6*float64(usage.OutputTokens)

	if overrideCost != nil {
		session.Cost += *overrideCost
	} else {
		session.Cost += cost
	}

	session.CompletionTokens += usage.OutputTokens + usage.CacheReadTokens
	session.PromptTokens += usage.InputTokens + usage.CacheCreationTokens
}

func (a *sessionAgent) Cancel(sessionID string) {
	// Cancel regular requests.
	if cancel, ok := a.activeRequests.Take(sessionID); ok && cancel != nil {
		slog.Info("Request cancellation initiated", "session_id", sessionID)
		cancel()
	}

	// Also check for summarize requests.
	if cancel, ok := a.activeRequests.Take(sessionID + "-summarize"); ok && cancel != nil {
		slog.Info("Summarize cancellation initiated", "session_id", sessionID)
		cancel()
	}

	if a.QueuedPrompts(sessionID) > 0 {
		slog.Info("Clearing queued prompts", "session_id", sessionID)
		a.messageQueue.Del(sessionID)
	}
}

func (a *sessionAgent) ClearQueue(sessionID string) {
	if a.QueuedPrompts(sessionID) > 0 {
		slog.Info("Clearing queued prompts", "session_id", sessionID)
		a.messageQueue.Del(sessionID)
	}
}

func (a *sessionAgent) CancelAll() {
	if !a.IsBusy() {
		return
	}
	for key := range a.activeRequests.Seq2() {
		a.Cancel(key) // key is sessionID
	}

	timeout := time.After(5 * time.Second)
	for a.IsBusy() {
		select {
		case <-timeout:
			return
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (a *sessionAgent) IsBusy() bool {
	var busy bool
	for cancelFunc := range a.activeRequests.Seq() {
		if cancelFunc != nil {
			busy = true
			break
		}
	}
	return busy
}

func (a *sessionAgent) IsSessionBusy(sessionID string) bool {
	_, busy := a.activeRequests.Get(sessionID)
	return busy
}

func (a *sessionAgent) QueuedPrompts(sessionID string) int {
	l, ok := a.messageQueue.Get(sessionID)
	if !ok {
		return 0
	}
	return len(l)
}

func (a *sessionAgent) SetModel(model Model) {
	a.model = model
}

func (a *sessionAgent) SetTools(tools []genai.AgentTool) {
	a.tools = tools
}

func (a *sessionAgent) Model() Model {
	return a.model
}
