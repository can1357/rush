// Copyright 2025 Charmbracelet, Inc.
// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.
//
// This is a fork of github.com/charmbracelet/fantasy modified for Rush.

package google

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"strings"

	"cloud.google.com/go/auth"
	"github.com/can1357/rush/genai"
	"github.com/can1357/rush/genai/object"
	"github.com/can1357/rush/genai/providers/anthropic"
	"github.com/can1357/rush/genai/schema"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/x/exp/slice"
	"github.com/google/uuid"
	gai "google.golang.org/genai"
)

// Name is the name of the Google provider.
const Name = "google"

type provider struct {
	options options
}

// ToolCallIDFunc defines a function that generates a tool call ID.
type ToolCallIDFunc = func() string

type options struct {
	apiKey         string
	name           string
	baseURL        string
	headers        map[string]string
	client         *http.Client
	backend        gai.Backend
	project        string
	location       string
	skipAuth       bool
	toolCallIDFunc ToolCallIDFunc
	objectMode     genai.ObjectMode
	models         []catwalk.Model
}

// Option defines a function that configures Google provider options.
type Option = func(*options)

func WithModels(models []catwalk.Model) Option {
	return func(o *options) {
		o.models = models
	}
}

// New creates a new Google provider with the given options.
func New(opts ...Option) (genai.Provider, error) {
	options := options{
		headers: map[string]string{},
		toolCallIDFunc: func() string {
			return uuid.NewString()
		},
	}
	for _, o := range opts {
		o(&options)
	}

	if options.models == nil {
		options.models = genai.GetKnownProviderInfo(catwalk.InferenceProviderGemini).Models
	}

	options.name = cmp.Or(options.name, Name)

	return &provider{
		options: options,
	}, nil
}

// WithBaseURL sets the base URL for the Google provider.
func WithBaseURL(baseURL string) Option {
	return func(o *options) {
		o.baseURL = baseURL
	}
}

// WithGeminiAPIKey sets the Gemini API key for the Google provider.
func WithGeminiAPIKey(apiKey string) Option {
	return func(o *options) {
		o.backend = gai.BackendGeminiAPI
		o.apiKey = apiKey
		o.project = ""
		o.location = ""
	}
}

// WithVertex configures the Google provider to use Vertex AI.
func WithVertex(project, location string) Option {
	if project == "" || location == "" {
		panic("project and location must be provided")
	}
	return func(o *options) {
		o.backend = gai.BackendVertexAI
		o.apiKey = ""
		o.project = project
		o.location = location
	}
}

// WithSkipAuth configures whether to skip authentication for the Google provider.
func WithSkipAuth(skipAuth bool) Option {
	return func(o *options) {
		o.skipAuth = skipAuth
	}
}

// WithName sets the name for the Google provider.
func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

// WithHeaders sets the headers for the Google provider.
func WithHeaders(headers map[string]string) Option {
	return func(o *options) {
		maps.Copy(o.headers, headers)
	}
}

// WithHTTPClient sets the HTTP client for the Google provider.
func WithHTTPClient(client *http.Client) Option {
	return func(o *options) {
		o.client = client
	}
}

// WithToolCallIDFunc sets the function that generates a tool call ID.
func WithToolCallIDFunc(f ToolCallIDFunc) Option {
	return func(o *options) {
		o.toolCallIDFunc = f
	}
}

// WithObjectMode sets the object generation mode for the Google provider.
func WithObjectMode(om genai.ObjectMode) Option {
	return func(o *options) {
		o.objectMode = om
	}
}

func (*provider) Name() string {
	return Name
}

func (o *provider) Models() []catwalk.Model {
	return o.options.models
}

func (o *provider) ModelDescription(modelID string) *catwalk.Model {
	for i := range o.options.models {
		if o.options.models[i].ID == modelID {
			return &o.options.models[i]
		}
	}
	return nil
}

type languageModel struct {
	provider        string
	model           *catwalk.Model
	client          *gai.Client
	providerOptions options
	objectMode      genai.ObjectMode
}

// LanguageModel implements genai.Provider.
func (a *provider) LanguageModel(ctx context.Context, modelID string) (genai.LanguageModel, error) {
	model := a.ModelDescription(modelID)
	if model == nil {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	if strings.Contains(modelID, "anthropic") || strings.Contains(modelID, "claude") {
		p, err := anthropic.New(
			anthropic.WithVertex(a.options.project, a.options.location),
			anthropic.WithHTTPClient(a.options.client),
			anthropic.WithSkipAuth(a.options.skipAuth),
		)
		if err != nil {
			return nil, err
		}
		return p.LanguageModel(ctx, modelID)
	}

	cc := &gai.ClientConfig{
		HTTPClient: a.options.client,
		Backend:    a.options.backend,
		APIKey:     a.options.apiKey,
		Project:    a.options.project,
		Location:   a.options.location,
	}
	if a.options.skipAuth {
		cc.Credentials = &auth.Credentials{TokenProvider: dummyTokenProvider{}}
	} else if cc.Backend == gai.BackendVertexAI {
		if err := cc.UseDefaultCredentials(); err != nil {
			return nil, err
		}
	}

	if a.options.baseURL != "" || len(a.options.headers) > 0 {
		headers := http.Header{}
		for k, v := range a.options.headers {
			headers.Add(k, v)
		}
		cc.HTTPOptions = gai.HTTPOptions{
			BaseURL: a.options.baseURL,
			Headers: headers,
		}
	}
	client, err := gai.NewClient(ctx, cc)
	if err != nil {
		return nil, err
	}

	objectMode := a.options.objectMode
	if objectMode == "" {
		objectMode = genai.ObjectModeAuto
	}

	return &languageModel{
		model:           model,
		provider:        a.options.name,
		providerOptions: a.options,
		client:          client,
		objectMode:      objectMode,
	}, nil
}

func (g languageModel) prepareParams(call genai.Call) (*gai.GenerateContentConfig, []*gai.Content, []genai.CallWarning, error) {
	config := &gai.GenerateContentConfig{}

	providerOptions := &ProviderOptions{}
	if v, ok := call.ProviderOptions[Name]; ok {
		providerOptions, ok = v.(*ProviderOptions)
		if !ok {
			return nil, nil, nil, &genai.Error{Title: "invalid argument", Message: "google provider options should be *google.ProviderOptions"}
		}
	}

	systemInstructions, content, warnings := toGooglePrompt(call.Prompt)

	if providerOptions.ThinkingConfig != nil {
		if providerOptions.ThinkingConfig.IncludeThoughts != nil &&
			*providerOptions.ThinkingConfig.IncludeThoughts &&
			strings.HasPrefix(g.provider, "google.vertex.") {
			warnings = append(warnings, genai.CallWarning{
				Type: genai.CallWarningTypeOther,
				Message: "The 'includeThoughts' option is only supported with the Google Vertex provider " +
					"and might not be supported or could behave unexpectedly with the current Google provider " +
					fmt.Sprintf("(%s)", g.provider),
			})
		}

		if providerOptions.ThinkingConfig.ThinkingBudget != nil &&
			*providerOptions.ThinkingConfig.ThinkingBudget < 128 {
			warnings = append(warnings, genai.CallWarning{
				Type:    genai.CallWarningTypeOther,
				Message: "The 'thinking_budget' option can not be under 128 and will be set to 128 by default",
			})
			providerOptions.ThinkingConfig.ThinkingBudget = genai.Opt(int64(128))
		}
	}

	isGemmaModel := strings.HasPrefix(strings.ToLower(g.model.ID), "gemma-")

	if isGemmaModel && systemInstructions != nil && len(systemInstructions.Parts) > 0 {
		if len(content) > 0 && content[0].Role == gai.RoleUser {
			systemParts := []string{}
			for _, sp := range systemInstructions.Parts {
				systemParts = append(systemParts, sp.Text)
			}
			systemMsg := strings.Join(systemParts, "\n")
			content[0].Parts = append([]*gai.Part{
				{
					Text: systemMsg + "\n\n",
				},
			}, content[0].Parts...)
			systemInstructions = nil
		}
	}

	config.SystemInstruction = systemInstructions

	if call.MaxOutputTokens != nil {
		config.MaxOutputTokens = int32(*call.MaxOutputTokens) //nolint: gosec
	}

	if call.Temperature != nil {
		tmp := float32(*call.Temperature)
		config.Temperature = &tmp
	}
	if call.TopK != nil {
		tmp := float32(*call.TopK)
		config.TopK = &tmp
	}
	if call.TopP != nil {
		tmp := float32(*call.TopP)
		config.TopP = &tmp
	}
	if call.FrequencyPenalty != nil {
		tmp := float32(*call.FrequencyPenalty)
		config.FrequencyPenalty = &tmp
	}
	if call.PresencePenalty != nil {
		tmp := float32(*call.PresencePenalty)
		config.PresencePenalty = &tmp
	}

	if providerOptions.ThinkingConfig != nil {
		config.ThinkingConfig = &gai.ThinkingConfig{}
		if providerOptions.ThinkingConfig.IncludeThoughts != nil {
			config.ThinkingConfig.IncludeThoughts = *providerOptions.ThinkingConfig.IncludeThoughts
		}
		if providerOptions.ThinkingConfig.ThinkingBudget != nil {
			tmp := int32(*providerOptions.ThinkingConfig.ThinkingBudget) //nolint: gosec
			config.ThinkingConfig.ThinkingBudget = &tmp
		}
	}
	for _, safetySetting := range providerOptions.SafetySettings {
		config.SafetySettings = append(config.SafetySettings, &gai.SafetySetting{
			Category:  gai.HarmCategory(safetySetting.Category),
			Threshold: gai.HarmBlockThreshold(safetySetting.Threshold),
		})
	}
	if providerOptions.CachedContent != "" {
		config.CachedContent = providerOptions.CachedContent
	}

	if len(call.Tools) > 0 {
		tools, toolChoice, toolWarnings := toGoogleTools(call.Tools, call.ToolChoice)
		config.ToolConfig = toolChoice
		config.Tools = append(config.Tools, &gai.Tool{
			FunctionDeclarations: tools,
		})
		warnings = append(warnings, toolWarnings...)
	}

	return config, content, warnings, nil
}

func toGooglePrompt(prompt genai.Prompt) (*gai.Content, []*gai.Content, []genai.CallWarning) { //nolint: unparam
	var systemInstructions *gai.Content
	var content []*gai.Content
	var warnings []genai.CallWarning

	finishedSystemBlock := false
	for _, msg := range prompt {
		switch msg.Role {
		case genai.MessageRoleSystem:
			if finishedSystemBlock {
				// skip multiple system messages that are separated by user/assistant messages
				// TODO: see if we need to send error here?
				continue
			}
			finishedSystemBlock = true

			var systemMessages []string
			for _, part := range msg.Content {
				text, ok := genai.AsMessagePart[genai.TextPart](part)
				if !ok || text.Text == "" {
					continue
				}
				systemMessages = append(systemMessages, text.Text)
			}
			if len(systemMessages) > 0 {
				systemInstructions = &gai.Content{
					Parts: []*gai.Part{
						{
							Text: strings.Join(systemMessages, "\n"),
						},
					},
				}
			}
		case genai.MessageRoleUser:
			var parts []*gai.Part
			for _, part := range msg.Content {
				switch part.GetType() {
				case genai.ContentTypeText:
					text, ok := genai.AsMessagePart[genai.TextPart](part)
					if !ok || text.Text == "" {
						continue
					}
					parts = append(parts, &gai.Part{
						Text: text.Text,
					})
				case genai.ContentTypeFile:
					file, ok := genai.AsMessagePart[genai.FilePart](part)
					if !ok {
						continue
					}
					parts = append(parts, &gai.Part{
						InlineData: &gai.Blob{
							Data:     file.Data,
							MIMEType: file.MediaType,
						},
					})
				}
			}
			if len(parts) > 0 {
				content = append(content, &gai.Content{
					Role:  gai.RoleUser,
					Parts: parts,
				})
			}
		case genai.MessageRoleAssistant:
			var parts []*gai.Part
			var currentReasoningMetadata *ReasoningMetadata
			for _, part := range msg.Content {
				switch part.GetType() {
				case genai.ContentTypeReasoning:
					reasoning, ok := genai.AsMessagePart[genai.ReasoningPart](part)
					if !ok {
						continue
					}

					metadata, ok := reasoning.ProviderOptions[Name]
					if !ok {
						continue
					}
					reasoningMetadata, ok := metadata.(*ReasoningMetadata)
					if !ok {
						continue
					}
					currentReasoningMetadata = reasoningMetadata
				case genai.ContentTypeText:
					text, ok := genai.AsMessagePart[genai.TextPart](part)
					if !ok || text.Text == "" {
						continue
					}
					geminiPart := &gai.Part{
						Text: text.Text,
					}
					if currentReasoningMetadata != nil {
						geminiPart.ThoughtSignature = []byte(currentReasoningMetadata.Signature)
						currentReasoningMetadata = nil
					}
					parts = append(parts, geminiPart)
				case genai.ContentTypeToolCall:
					toolCall, ok := genai.AsMessagePart[genai.ToolCallPart](part)
					if !ok {
						continue
					}

					var result map[string]any
					err := json.Unmarshal([]byte(toolCall.Input), &result)
					if err != nil {
						continue
					}
					geminiPart := &gai.Part{
						FunctionCall: &gai.FunctionCall{
							ID:   toolCall.ToolCallID,
							Name: toolCall.ToolName,
							Args: result,
						},
					}
					if currentReasoningMetadata != nil {
						geminiPart.ThoughtSignature = []byte(currentReasoningMetadata.Signature)
						currentReasoningMetadata = nil
					}
					parts = append(parts, geminiPart)
				}
			}
			if len(parts) > 0 {
				content = append(content, &gai.Content{
					Role:  gai.RoleModel,
					Parts: parts,
				})
			}
		case genai.MessageRoleTool:
			var parts []*gai.Part
			for _, part := range msg.Content {
				switch part.GetType() {
				case genai.ContentTypeToolResult:
					result, ok := genai.AsMessagePart[genai.ToolResultPart](part)
					if !ok {
						continue
					}
					var toolCall genai.ToolCallPart
					for _, m := range prompt {
						if m.Role == genai.MessageRoleAssistant {
							for _, content := range m.Content {
								tc, ok := genai.AsMessagePart[genai.ToolCallPart](content)
								if !ok {
									continue
								}
								if tc.ToolCallID == result.ToolCallID {
									toolCall = tc
									break
								}
							}
						}
					}
					switch result.Output.GetType() {
					case genai.ToolResultContentTypeText:
						content, ok := genai.AsToolResultOutputType[genai.ToolResultOutputContentText](result.Output)
						if !ok {
							continue
						}
						response := map[string]any{"result": content.Text}
						parts = append(parts, &gai.Part{
							FunctionResponse: &gai.FunctionResponse{
								ID:       result.ToolCallID,
								Response: response,
								Name:     toolCall.ToolName,
							},
						})

					case genai.ToolResultContentTypeError:
						content, ok := genai.AsToolResultOutputType[genai.ToolResultOutputContentError](result.Output)
						if !ok {
							continue
						}
						response := map[string]any{"result": content.Error.Error()}
						parts = append(parts, &gai.Part{
							FunctionResponse: &gai.FunctionResponse{
								ID:       result.ToolCallID,
								Response: response,
								Name:     toolCall.ToolName,
							},
						})
					}
				}
			}
			if len(parts) > 0 {
				content = append(content, &gai.Content{
					Role:  gai.RoleUser,
					Parts: parts,
				})
			}
		default:
			panic("unsupported message role: " + msg.Role)
		}
	}
	return systemInstructions, content, warnings
}

// Generate implements genai.LanguageModel.
func (g *languageModel) Generate(ctx context.Context, call genai.Call) (*genai.Response, error) {
	config, contents, warnings, err := g.prepareParams(call)
	if err != nil {
		return nil, err
	}

	lastMessage, history, ok := slice.Pop(contents)
	if !ok {
		return nil, errors.New("no messages to send")
	}

	chat, err := g.client.Chats.Create(ctx, g.model.ID, config, history)
	if err != nil {
		return nil, err
	}

	response, err := chat.SendMessage(ctx, depointerSlice(lastMessage.Parts)...)
	if err != nil {
		return nil, toProviderErr(err)
	}

	return g.mapResponse(response, warnings)
}

// Model implements genai.LanguageModel.
func (g *languageModel) Model() *catwalk.Model {
	return g.model
}

// Provider implements genai.LanguageModel.
func (g *languageModel) Provider() string {
	return g.provider
}

// Stream implements genai.LanguageModel.
func (g *languageModel) Stream(ctx context.Context, call genai.Call) (genai.StreamResponse, error) {
	config, contents, warnings, err := g.prepareParams(call)
	if err != nil {
		return nil, err
	}

	lastMessage, history, ok := slice.Pop(contents)
	if !ok {
		return nil, errors.New("no messages to send")
	}

	chat, err := g.client.Chats.Create(ctx, g.model.ID, config, history)
	if err != nil {
		return nil, err
	}

	return func(yield func(genai.StreamPart) bool) {
		if len(warnings) > 0 {
			if !yield(genai.StreamPart{
				Type:     genai.StreamPartTypeWarnings,
				Warnings: warnings,
			}) {
				return
			}
		}

		var currentContent string
		var toolCalls []genai.ToolCallContent
		var isActiveText bool
		var isActiveReasoning bool
		var blockCounter int
		var currentTextBlockID string
		var currentReasoningBlockID string
		var usage *genai.Usage
		var lastFinishReason genai.FinishReason

		for resp, err := range chat.SendMessageStream(ctx, depointerSlice(lastMessage.Parts)...) {
			if err != nil {
				yield(genai.StreamPart{
					Type:  genai.StreamPartTypeError,
					Error: toProviderErr(err),
				})
				return
			}

			if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
				for _, part := range resp.Candidates[0].Content.Parts {
					switch {
					case part.Text != "":
						delta := part.Text
						if delta != "" {
							// Check if this is a reasoning/thought part
							if part.Thought {
								// End any active text block before starting reasoning
								if isActiveText {
									isActiveText = false
									if !yield(genai.StreamPart{
										Type: genai.StreamPartTypeTextEnd,
										ID:   currentTextBlockID,
									}) {
										return
									}
								}

								// Start new reasoning block if not already active
								if !isActiveReasoning {
									isActiveReasoning = true
									currentReasoningBlockID = fmt.Sprintf("%d", blockCounter)
									blockCounter++
									if !yield(genai.StreamPart{
										Type: genai.StreamPartTypeReasoningStart,
										ID:   currentReasoningBlockID,
									}) {
										return
									}
								}

								if !yield(genai.StreamPart{
									Type:  genai.StreamPartTypeReasoningDelta,
									ID:    currentReasoningBlockID,
									Delta: delta,
								}) {
									return
								}
							} else {
								// Start new text block if not already active
								if !isActiveText {
									isActiveText = true
									currentTextBlockID = fmt.Sprintf("%d", blockCounter)
									blockCounter++
									if !yield(genai.StreamPart{
										Type: genai.StreamPartTypeTextStart,
										ID:   currentTextBlockID,
									}) {
										return
									}
								}
								// End any active reasoning block before starting text
								if isActiveReasoning {
									isActiveReasoning = false
									metadata := &ReasoningMetadata{
										Signature: string(part.ThoughtSignature),
									}
									if !yield(genai.StreamPart{
										Type: genai.StreamPartTypeReasoningEnd,
										ID:   currentReasoningBlockID,
										ProviderMetadata: genai.ProviderMetadata{
											Name: metadata,
										},
									}) {
										return
									}
								} else if part.ThoughtSignature != nil {
									metadata := &ReasoningMetadata{
										Signature: string(part.ThoughtSignature),
									}

									if !yield(genai.StreamPart{
										Type: genai.StreamPartTypeReasoningStart,
										ID:   currentReasoningBlockID,
									}) {
										return
									}
									if !yield(genai.StreamPart{
										Type: genai.StreamPartTypeReasoningEnd,
										ID:   currentReasoningBlockID,
										ProviderMetadata: genai.ProviderMetadata{
											Name: metadata,
										},
									}) {
										return
									}
								}

								if !yield(genai.StreamPart{
									Type:  genai.StreamPartTypeTextDelta,
									ID:    currentTextBlockID,
									Delta: delta,
								}) {
									return
								}
								currentContent += delta
							}
						}
					case part.FunctionCall != nil:
						// End any active text or reasoning blocks
						if isActiveText {
							isActiveText = false
							if !yield(genai.StreamPart{
								Type: genai.StreamPartTypeTextEnd,
								ID:   currentTextBlockID,
							}) {
								return
							}
						}
						toolCallID := cmp.Or(part.FunctionCall.ID, g.providerOptions.toolCallIDFunc())
						// End any active reasoning block before starting text
						if isActiveReasoning {
							isActiveReasoning = false
							metadata := &ReasoningMetadata{
								Signature: string(part.ThoughtSignature),
								ToolID:    toolCallID,
							}
							if !yield(genai.StreamPart{
								Type: genai.StreamPartTypeReasoningEnd,
								ID:   currentReasoningBlockID,
								ProviderMetadata: genai.ProviderMetadata{
									Name: metadata,
								},
							}) {
								return
							}
						} else if part.ThoughtSignature != nil {
							metadata := &ReasoningMetadata{
								Signature: string(part.ThoughtSignature),
								ToolID:    toolCallID,
							}

							if !yield(genai.StreamPart{
								Type: genai.StreamPartTypeReasoningStart,
								ID:   currentReasoningBlockID,
							}) {
								return
							}
							if !yield(genai.StreamPart{
								Type: genai.StreamPartTypeReasoningEnd,
								ID:   currentReasoningBlockID,
								ProviderMetadata: genai.ProviderMetadata{
									Name: metadata,
								},
							}) {
								return
							}
						}
						args, err := json.Marshal(part.FunctionCall.Args)
						if err != nil {
							yield(genai.StreamPart{
								Type:  genai.StreamPartTypeError,
								Error: err,
							})
							return
						}

						if !yield(genai.StreamPart{
							Type:         genai.StreamPartTypeToolInputStart,
							ID:           toolCallID,
							ToolCallName: part.FunctionCall.Name,
						}) {
							return
						}

						if !yield(genai.StreamPart{
							Type:  genai.StreamPartTypeToolInputDelta,
							ID:    toolCallID,
							Delta: string(args),
						}) {
							return
						}

						if !yield(genai.StreamPart{
							Type: genai.StreamPartTypeToolInputEnd,
							ID:   toolCallID,
						}) {
							return
						}

						if !yield(genai.StreamPart{
							Type:             genai.StreamPartTypeToolCall,
							ID:               toolCallID,
							ToolCallName:     part.FunctionCall.Name,
							ToolCallInput:    string(args),
							ProviderExecuted: false,
						}) {
							return
						}

						toolCalls = append(toolCalls, genai.ToolCallContent{
							ToolCallID:       toolCallID,
							ToolName:         part.FunctionCall.Name,
							Input:            string(args),
							ProviderExecuted: false,
						})
					}
				}
			}

			// we need to make sure that there is actual tokendata
			if resp.UsageMetadata != nil && resp.UsageMetadata.TotalTokenCount != 0 {
				currentUsage := mapUsage(resp.UsageMetadata)
				// if first usage chunk
				if usage == nil {
					usage = &currentUsage
				} else {
					usage.OutputTokens += currentUsage.OutputTokens
					usage.ReasoningTokens += currentUsage.ReasoningTokens
					usage.CacheReadTokens += currentUsage.CacheReadTokens
				}
			}

			if len(resp.Candidates) > 0 && resp.Candidates[0].FinishReason != "" {
				lastFinishReason = mapFinishReason(resp.Candidates[0].FinishReason)
			}
		}

		// Close any open blocks before finishing
		if isActiveText {
			if !yield(genai.StreamPart{
				Type: genai.StreamPartTypeTextEnd,
				ID:   currentTextBlockID,
			}) {
				return
			}
		}
		if isActiveReasoning {
			if !yield(genai.StreamPart{
				Type: genai.StreamPartTypeReasoningEnd,
				ID:   currentReasoningBlockID,
			}) {
				return
			}
		}

		finishReason := lastFinishReason
		if len(toolCalls) > 0 {
			finishReason = genai.FinishReasonToolCalls
		} else if finishReason == "" {
			finishReason = genai.FinishReasonStop
		}

		yield(genai.StreamPart{
			Type:         genai.StreamPartTypeFinish,
			Usage:        *usage,
			FinishReason: finishReason,
		})
	}, nil
}

// GenerateObject implements genai.LanguageModel.
func (g *languageModel) GenerateObject(ctx context.Context, call genai.ObjectCall) (*genai.ObjectResponse, error) {
	switch g.objectMode {
	case genai.ObjectModeText:
		return object.GenerateWithText(ctx, g, call)
	case genai.ObjectModeTool:
		return object.GenerateWithTool(ctx, g, call)
	default:
		return g.generateObjectWithJSONMode(ctx, call)
	}
}

// StreamObject implements genai.LanguageModel.
func (g *languageModel) StreamObject(ctx context.Context, call genai.ObjectCall) (genai.ObjectStreamResponse, error) {
	switch g.objectMode {
	case genai.ObjectModeTool:
		return object.StreamWithTool(ctx, g, call)
	case genai.ObjectModeText:
		return object.StreamWithText(ctx, g, call)
	default:
		return g.streamObjectWithJSONMode(ctx, call)
	}
}

func (g *languageModel) generateObjectWithJSONMode(ctx context.Context, call genai.ObjectCall) (*genai.ObjectResponse, error) {
	// Convert our Schema to Google's JSON Schema format
	jsonSchemaMap := schema.ToMap(call.Schema)

	// Build request using prepareParams
	fantasyCall := genai.Call{
		Prompt:           call.Prompt,
		MaxOutputTokens:  call.MaxOutputTokens,
		Temperature:      call.Temperature,
		TopP:             call.TopP,
		TopK:             call.TopK,
		PresencePenalty:  call.PresencePenalty,
		FrequencyPenalty: call.FrequencyPenalty,
		ProviderOptions:  call.ProviderOptions,
	}

	config, contents, warnings, err := g.prepareParams(fantasyCall)
	if err != nil {
		return nil, err
	}

	// Set ResponseMIMEType and ResponseJsonSchema for structured output
	config.ResponseMIMEType = "application/json"
	config.ResponseJsonSchema = jsonSchemaMap

	lastMessage, history, ok := slice.Pop(contents)
	if !ok {
		return nil, errors.New("no messages to send")
	}

	chat, err := g.client.Chats.Create(ctx, g.model.ID, config, history)
	if err != nil {
		return nil, err
	}

	response, err := chat.SendMessage(ctx, depointerSlice(lastMessage.Parts)...)
	if err != nil {
		return nil, toProviderErr(err)
	}

	mappedResponse, err := g.mapResponse(response, warnings)
	if err != nil {
		return nil, err
	}

	jsonText := mappedResponse.Content.Text()
	if jsonText == "" {
		return nil, &genai.NoObjectGeneratedError{
			RawText:      "",
			ParseError:   fmt.Errorf("no text content in response"),
			Usage:        mappedResponse.Usage,
			FinishReason: mappedResponse.FinishReason,
		}
	}

	// Parse and validate
	var obj any
	if call.RepairText != nil {
		obj, err = schema.ParseAndValidateWithRepair(ctx, jsonText, call.Schema, call.RepairText)
	} else {
		obj, err = schema.ParseAndValidate(jsonText, call.Schema)
	}

	if err != nil {
		// Add usage info to error
		if nogErr, ok := err.(*genai.NoObjectGeneratedError); ok {
			nogErr.Usage = mappedResponse.Usage
			nogErr.FinishReason = mappedResponse.FinishReason
		}
		return nil, err
	}

	return &genai.ObjectResponse{
		Object:           obj,
		RawText:          jsonText,
		Usage:            mappedResponse.Usage,
		FinishReason:     mappedResponse.FinishReason,
		Warnings:         warnings,
		ProviderMetadata: mappedResponse.ProviderMetadata,
	}, nil
}

func (g *languageModel) streamObjectWithJSONMode(ctx context.Context, call genai.ObjectCall) (genai.ObjectStreamResponse, error) {
	// Convert our Schema to Google's JSON Schema format
	jsonSchemaMap := schema.ToMap(call.Schema)

	// Build request using prepareParams
	fantasyCall := genai.Call{
		Prompt:           call.Prompt,
		MaxOutputTokens:  call.MaxOutputTokens,
		Temperature:      call.Temperature,
		TopP:             call.TopP,
		TopK:             call.TopK,
		PresencePenalty:  call.PresencePenalty,
		FrequencyPenalty: call.FrequencyPenalty,
		ProviderOptions:  call.ProviderOptions,
	}

	config, contents, warnings, err := g.prepareParams(fantasyCall)
	if err != nil {
		return nil, err
	}

	// Set ResponseMIMEType and ResponseJsonSchema for structured output
	config.ResponseMIMEType = "application/json"
	config.ResponseJsonSchema = jsonSchemaMap

	lastMessage, history, ok := slice.Pop(contents)
	if !ok {
		return nil, errors.New("no messages to send")
	}

	chat, err := g.client.Chats.Create(ctx, g.model.ID, config, history)
	if err != nil {
		return nil, err
	}

	return func(yield func(genai.ObjectStreamPart) bool) {
		if len(warnings) > 0 {
			if !yield(genai.ObjectStreamPart{
				Type:     genai.ObjectStreamPartTypeObject,
				Warnings: warnings,
			}) {
				return
			}
		}

		var accumulated string
		var lastParsedObject any
		var usage *genai.Usage
		var lastFinishReason genai.FinishReason
		var streamErr error

		for resp, err := range chat.SendMessageStream(ctx, depointerSlice(lastMessage.Parts)...) {
			if err != nil {
				streamErr = toProviderErr(err)
				yield(genai.ObjectStreamPart{
					Type:  genai.ObjectStreamPartTypeError,
					Error: streamErr,
				})
				return
			}

			if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
				for _, part := range resp.Candidates[0].Content.Parts {
					if part.Text != "" && !part.Thought {
						accumulated += part.Text

						// Try to parse the accumulated text
						obj, state, parseErr := schema.ParsePartialJSON(accumulated)

						// If we successfully parsed, validate and emit
						if state == schema.ParseStateSuccessful || state == schema.ParseStateRepaired {
							if err := schema.ValidateAgainstSchema(obj, call.Schema); err == nil {
								// Only emit if object is different from last
								if !reflect.DeepEqual(obj, lastParsedObject) {
									if !yield(genai.ObjectStreamPart{
										Type:   genai.ObjectStreamPartTypeObject,
										Object: obj,
									}) {
										return
									}
									lastParsedObject = obj
								}
							}
						}

						// If parsing failed and we have a repair function, try it
						if state == schema.ParseStateFailed && call.RepairText != nil {
							repairedText, repairErr := call.RepairText(ctx, accumulated, parseErr)
							if repairErr == nil {
								obj2, state2, _ := schema.ParsePartialJSON(repairedText)
								if (state2 == schema.ParseStateSuccessful || state2 == schema.ParseStateRepaired) &&
									schema.ValidateAgainstSchema(obj2, call.Schema) == nil {
									if !reflect.DeepEqual(obj2, lastParsedObject) {
										if !yield(genai.ObjectStreamPart{
											Type:   genai.ObjectStreamPartTypeObject,
											Object: obj2,
										}) {
											return
										}
										lastParsedObject = obj2
									}
								}
							}
						}
					}
				}
			}

			// we need to make sure that there is actual tokendata
			if resp.UsageMetadata != nil && resp.UsageMetadata.TotalTokenCount != 0 {
				currentUsage := mapUsage(resp.UsageMetadata)
				if usage == nil {
					usage = &currentUsage
				} else {
					usage.OutputTokens += currentUsage.OutputTokens
					usage.ReasoningTokens += currentUsage.ReasoningTokens
					usage.CacheReadTokens += currentUsage.CacheReadTokens
				}
			}

			if len(resp.Candidates) > 0 && resp.Candidates[0].FinishReason != "" {
				lastFinishReason = mapFinishReason(resp.Candidates[0].FinishReason)
			}
		}

		// Final validation and emit
		if streamErr == nil && lastParsedObject != nil {
			finishReason := lastFinishReason
			if finishReason == "" {
				finishReason = genai.FinishReasonStop
			}

			yield(genai.ObjectStreamPart{
				Type:         genai.ObjectStreamPartTypeFinish,
				Usage:        *usage,
				FinishReason: finishReason,
			})
		} else if streamErr == nil && lastParsedObject == nil {
			// No object was generated
			finalUsage := genai.Usage{}
			if usage != nil {
				finalUsage = *usage
			}
			yield(genai.ObjectStreamPart{
				Type: genai.ObjectStreamPartTypeError,
				Error: &genai.NoObjectGeneratedError{
					RawText:      accumulated,
					ParseError:   fmt.Errorf("no valid object generated in stream"),
					Usage:        finalUsage,
					FinishReason: lastFinishReason,
				},
			})
		}
	}, nil
}

func toGoogleTools(tools []genai.Tool, toolChoice *genai.ToolChoice) (googleTools []*gai.FunctionDeclaration, googleToolChoice *gai.ToolConfig, warnings []genai.CallWarning) {
	for _, tool := range tools {
		if tool.GetType() == genai.ToolTypeFunction {
			ft, ok := tool.(genai.FunctionTool)
			if !ok {
				continue
			}

			required := []string{}
			var properties map[string]any
			if props, ok := ft.InputSchema["properties"]; ok {
				properties, _ = props.(map[string]any)
			}
			if req, ok := ft.InputSchema["required"]; ok {
				if reqArr, ok := req.([]string); ok {
					required = reqArr
				}
			}
			declaration := &gai.FunctionDeclaration{
				Name:        ft.Name,
				Description: ft.Description,
				Parameters: &gai.Schema{
					Type:       gai.TypeObject,
					Properties: convertSchemaProperties(properties),
					Required:   required,
				},
			}
			googleTools = append(googleTools, declaration)
			continue
		}
		// TODO: handle provider tool calls
		warnings = append(warnings, genai.CallWarning{
			Type:    genai.CallWarningTypeUnsupportedTool,
			Tool:    tool,
			Message: "tool is not supported",
		})
	}
	if toolChoice == nil {
		return googleTools, googleToolChoice, warnings
	}
	switch *toolChoice {
	case genai.ToolChoiceAuto:
		googleToolChoice = &gai.ToolConfig{
			FunctionCallingConfig: &gai.FunctionCallingConfig{
				Mode: gai.FunctionCallingConfigModeAuto,
			},
		}
	case genai.ToolChoiceRequired:
		googleToolChoice = &gai.ToolConfig{
			FunctionCallingConfig: &gai.FunctionCallingConfig{
				Mode: gai.FunctionCallingConfigModeAny,
			},
		}
	case genai.ToolChoiceNone:
		googleToolChoice = &gai.ToolConfig{
			FunctionCallingConfig: &gai.FunctionCallingConfig{
				Mode: gai.FunctionCallingConfigModeNone,
			},
		}
	default:
		googleToolChoice = &gai.ToolConfig{
			FunctionCallingConfig: &gai.FunctionCallingConfig{
				Mode: gai.FunctionCallingConfigModeAny,
				AllowedFunctionNames: []string{
					string(*toolChoice),
				},
			},
		}
	}
	return googleTools, googleToolChoice, warnings
}

func convertSchemaProperties(parameters map[string]any) map[string]*gai.Schema {
	properties := make(map[string]*gai.Schema)

	for name, param := range parameters {
		properties[name] = convertToSchema(param)
	}

	return properties
}

func convertToSchema(param any) *gai.Schema {
	schema := &gai.Schema{Type: gai.TypeString}

	paramMap, ok := param.(map[string]any)
	if !ok {
		return schema
	}

	if desc, ok := paramMap["description"].(string); ok {
		schema.Description = desc
	}

	typeVal, hasType := paramMap["type"]
	if !hasType {
		return schema
	}

	typeStr, ok := typeVal.(string)
	if !ok {
		return schema
	}

	schema.Type = mapJSONTypeToGoogle(typeStr)

	switch typeStr {
	case "array":
		schema.Items = processArrayItems(paramMap)
	case "object":
		if props, ok := paramMap["properties"].(map[string]any); ok {
			schema.Properties = convertSchemaProperties(props)
		}
	}

	return schema
}

func processArrayItems(paramMap map[string]any) *gai.Schema {
	items, ok := paramMap["items"].(map[string]any)
	if !ok {
		return nil
	}

	return convertToSchema(items)
}

func mapJSONTypeToGoogle(jsonType string) gai.Type {
	switch jsonType {
	case "string":
		return gai.TypeString
	case "number":
		return gai.TypeNumber
	case "integer":
		return gai.TypeInteger
	case "boolean":
		return gai.TypeBoolean
	case "array":
		return gai.TypeArray
	case "object":
		return gai.TypeObject
	default:
		return gai.TypeString // Default to string for unknown types
	}
}

func (g languageModel) mapResponse(response *gai.GenerateContentResponse, warnings []genai.CallWarning) (*genai.Response, error) {
	if len(response.Candidates) == 0 || response.Candidates[0].Content == nil {
		return nil, errors.New("no response from model")
	}

	var (
		content      []genai.Content
		finishReason genai.FinishReason
		hasToolCalls bool
		candidate    = response.Candidates[0]
	)

	for _, part := range candidate.Content.Parts {
		switch {
		case part.Text != "":
			if part.Thought {
				reasoningContent := genai.ReasoningContent{Text: part.Text}
				if part.ThoughtSignature != nil {
					metadata := &ReasoningMetadata{
						Signature: string(part.ThoughtSignature),
					}
					reasoningContent.ProviderMetadata = genai.ProviderMetadata{
						Name: metadata,
					}
				}
				content = append(content, reasoningContent)
			} else {
				foundReasoning := false
				if part.ThoughtSignature != nil {
					metadata := &ReasoningMetadata{
						Signature: string(part.ThoughtSignature),
					}
					// find the last reasoning content and add the signature
					for i := len(content) - 1; i >= 0; i-- {
						c := content[i]
						if c.GetType() == genai.ContentTypeReasoning {
							reasoningContent, ok := genai.AsContentType[genai.ReasoningContent](c)
							if !ok {
								continue
							}
							reasoningContent.ProviderMetadata = genai.ProviderMetadata{
								Name: metadata,
							}
							content[i] = reasoningContent
							foundReasoning = true
							break
						}
					}
					if !foundReasoning {
						content = append(content, genai.ReasoningContent{
							ProviderMetadata: genai.ProviderMetadata{
								Name: metadata,
							},
						})
					}
				}
				content = append(content, genai.TextContent{Text: part.Text})
			}
		case part.FunctionCall != nil:
			input, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return nil, err
			}
			toolCallID := cmp.Or(part.FunctionCall.ID, g.providerOptions.toolCallIDFunc())
			foundReasoning := false
			if part.ThoughtSignature != nil {
				metadata := &ReasoningMetadata{
					Signature: string(part.ThoughtSignature),
					ToolID:    toolCallID,
				}
				// find the last reasoning content and add the signature
				for i := len(content) - 1; i >= 0; i-- {
					c := content[i]
					if c.GetType() == genai.ContentTypeReasoning {
						reasoningContent, ok := genai.AsContentType[genai.ReasoningContent](c)
						if !ok {
							continue
						}
						reasoningContent.ProviderMetadata = genai.ProviderMetadata{
							Name: metadata,
						}
						content[i] = reasoningContent
						foundReasoning = true
						break
					}
				}
				if !foundReasoning {
					content = append(content, genai.ReasoningContent{
						ProviderMetadata: genai.ProviderMetadata{
							Name: metadata,
						},
					})
				}
			}
			content = append(content, genai.ToolCallContent{
				ToolCallID:       toolCallID,
				ToolName:         part.FunctionCall.Name,
				Input:            string(input),
				ProviderExecuted: false,
			})
			hasToolCalls = true
		default:
			// Silently skip unknown part types instead of erroring
			// This allows for forward compatibility with new part types
		}
	}

	if hasToolCalls {
		finishReason = genai.FinishReasonToolCalls
	} else {
		finishReason = mapFinishReason(candidate.FinishReason)
	}

	return &genai.Response{
		Content:      content,
		Usage:        mapUsage(response.UsageMetadata),
		FinishReason: finishReason,
		Warnings:     warnings,
	}, nil
}

// GetReasoningMetadata extracts reasoning metadata from provider options for google models.
func GetReasoningMetadata(providerOptions genai.ProviderOptions) *ReasoningMetadata {
	if googleOptions, ok := providerOptions[Name]; ok {
		if reasoning, ok := googleOptions.(*ReasoningMetadata); ok {
			return reasoning
		}
	}
	return nil
}

func mapFinishReason(reason gai.FinishReason) genai.FinishReason {
	switch reason {
	case gai.FinishReasonStop:
		return genai.FinishReasonStop
	case gai.FinishReasonMaxTokens:
		return genai.FinishReasonLength
	case gai.FinishReasonSafety,
		gai.FinishReasonBlocklist,
		gai.FinishReasonProhibitedContent,
		gai.FinishReasonSPII,
		gai.FinishReasonImageSafety:
		return genai.FinishReasonContentFilter
	case gai.FinishReasonRecitation,
		gai.FinishReasonLanguage,
		gai.FinishReasonMalformedFunctionCall:
		return genai.FinishReasonError
	case gai.FinishReasonOther:
		return genai.FinishReasonOther
	default:
		return genai.FinishReasonUnknown
	}
}

func mapUsage(usage *gai.GenerateContentResponseUsageMetadata) genai.Usage {
	return genai.Usage{
		InputTokens:         int64(usage.PromptTokenCount),
		OutputTokens:        int64(usage.CandidatesTokenCount),
		TotalTokens:         int64(usage.TotalTokenCount),
		ReasoningTokens:     int64(usage.ThoughtsTokenCount),
		CacheCreationTokens: 0,
		CacheReadTokens:     int64(usage.CachedContentTokenCount),
	}
}
