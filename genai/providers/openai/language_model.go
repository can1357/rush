// Copyright 2025 Charmbracelet, Inc.
// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.
//
// This is a fork of github.com/charmbracelet/fantasy modified for Rush.

package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/can1357/rush/genai"
	"github.com/can1357/rush/genai/object"
	"github.com/can1357/rush/genai/schema"
	xjson "github.com/charmbracelet/x/json"
	"github.com/google/uuid"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/shared"
)

type languageModel struct {
	provider                   string
	modelID                    string
	client                     openai.Client
	objectMode                 genai.ObjectMode
	prepareCallFunc            LanguageModelPrepareCallFunc
	mapFinishReasonFunc        LanguageModelMapFinishReasonFunc
	extraContentFunc           LanguageModelExtraContentFunc
	usageFunc                  LanguageModelUsageFunc
	streamUsageFunc            LanguageModelStreamUsageFunc
	streamExtraFunc            LanguageModelStreamExtraFunc
	streamProviderMetadataFunc LanguageModelStreamProviderMetadataFunc
	toPromptFunc               LanguageModelToPromptFunc
}

// LanguageModelOption is a function that configures a languageModel.
type LanguageModelOption = func(*languageModel)

// WithLanguageModelPrepareCallFunc sets the prepare call function for the language model.
func WithLanguageModelPrepareCallFunc(fn LanguageModelPrepareCallFunc) LanguageModelOption {
	return func(l *languageModel) {
		l.prepareCallFunc = fn
	}
}

// WithLanguageModelMapFinishReasonFunc sets the map finish reason function for the language model.
func WithLanguageModelMapFinishReasonFunc(fn LanguageModelMapFinishReasonFunc) LanguageModelOption {
	return func(l *languageModel) {
		l.mapFinishReasonFunc = fn
	}
}

// WithLanguageModelExtraContentFunc sets the extra content function for the language model.
func WithLanguageModelExtraContentFunc(fn LanguageModelExtraContentFunc) LanguageModelOption {
	return func(l *languageModel) {
		l.extraContentFunc = fn
	}
}

// WithLanguageModelStreamExtraFunc sets the stream extra function for the language model.
func WithLanguageModelStreamExtraFunc(fn LanguageModelStreamExtraFunc) LanguageModelOption {
	return func(l *languageModel) {
		l.streamExtraFunc = fn
	}
}

// WithLanguageModelUsageFunc sets the usage function for the language model.
func WithLanguageModelUsageFunc(fn LanguageModelUsageFunc) LanguageModelOption {
	return func(l *languageModel) {
		l.usageFunc = fn
	}
}

// WithLanguageModelStreamUsageFunc sets the stream usage function for the language model.
func WithLanguageModelStreamUsageFunc(fn LanguageModelStreamUsageFunc) LanguageModelOption {
	return func(l *languageModel) {
		l.streamUsageFunc = fn
	}
}

// WithLanguageModelToPromptFunc sets the to prompt function for the language model.
func WithLanguageModelToPromptFunc(fn LanguageModelToPromptFunc) LanguageModelOption {
	return func(l *languageModel) {
		l.toPromptFunc = fn
	}
}

// WithLanguageModelObjectMode sets the object generation mode.
func WithLanguageModelObjectMode(om genai.ObjectMode) LanguageModelOption {
	return func(l *languageModel) {
		// not supported
		if om == genai.ObjectModeJSON {
			om = genai.ObjectModeAuto
		}
		l.objectMode = om
	}
}

func newLanguageModel(modelID string, provider string, client openai.Client, opts ...LanguageModelOption) languageModel {
	model := languageModel{
		modelID:                    modelID,
		provider:                   provider,
		client:                     client,
		objectMode:                 genai.ObjectModeAuto,
		prepareCallFunc:            DefaultPrepareCallFunc,
		mapFinishReasonFunc:        DefaultMapFinishReasonFunc,
		usageFunc:                  DefaultUsageFunc,
		streamUsageFunc:            DefaultStreamUsageFunc,
		streamProviderMetadataFunc: DefaultStreamProviderMetadataFunc,
		toPromptFunc:               DefaultToPrompt,
	}

	for _, o := range opts {
		o(&model)
	}
	return model
}

type streamToolCall struct {
	id          string
	name        string
	arguments   string
	hasFinished bool
}

// Model implements genai.LanguageModel.
func (o languageModel) Model() string {
	return o.modelID
}

// Provider implements genai.LanguageModel.
func (o languageModel) Provider() string {
	return o.provider
}

func (o languageModel) prepareParams(call genai.Call) (*openai.ChatCompletionNewParams, []genai.CallWarning, error) {
	params := &openai.ChatCompletionNewParams{}
	messages, warnings := o.toPromptFunc(call.Prompt, o.provider, o.modelID)
	if call.TopK != nil {
		warnings = append(warnings, genai.CallWarning{
			Type:    genai.CallWarningTypeUnsupportedSetting,
			Setting: "top_k",
		})
	}

	if call.MaxOutputTokens != nil {
		params.MaxTokens = param.NewOpt(*call.MaxOutputTokens)
	}
	if call.Temperature != nil {
		params.Temperature = param.NewOpt(*call.Temperature)
	}
	if call.TopP != nil {
		params.TopP = param.NewOpt(*call.TopP)
	}
	if call.FrequencyPenalty != nil {
		params.FrequencyPenalty = param.NewOpt(*call.FrequencyPenalty)
	}
	if call.PresencePenalty != nil {
		params.PresencePenalty = param.NewOpt(*call.PresencePenalty)
	}

	if isReasoningModel(o.modelID) {
		// remove unsupported settings for reasoning models
		// see https://platform.openai.com/docs/guides/reasoning#limitations
		if call.Temperature != nil {
			params.Temperature = param.Opt[float64]{}
			warnings = append(warnings, genai.CallWarning{
				Type:    genai.CallWarningTypeUnsupportedSetting,
				Setting: "temperature",
				Details: "temperature is not supported for reasoning models",
			})
		}
		if call.TopP != nil {
			params.TopP = param.Opt[float64]{}
			warnings = append(warnings, genai.CallWarning{
				Type:    genai.CallWarningTypeUnsupportedSetting,
				Setting: "TopP",
				Details: "TopP is not supported for reasoning models",
			})
		}
		if call.FrequencyPenalty != nil {
			params.FrequencyPenalty = param.Opt[float64]{}
			warnings = append(warnings, genai.CallWarning{
				Type:    genai.CallWarningTypeUnsupportedSetting,
				Setting: "FrequencyPenalty",
				Details: "FrequencyPenalty is not supported for reasoning models",
			})
		}
		if call.PresencePenalty != nil {
			params.PresencePenalty = param.Opt[float64]{}
			warnings = append(warnings, genai.CallWarning{
				Type:    genai.CallWarningTypeUnsupportedSetting,
				Setting: "PresencePenalty",
				Details: "PresencePenalty is not supported for reasoning models",
			})
		}

		// reasoning models use max_completion_tokens instead of max_tokens
		if call.MaxOutputTokens != nil {
			if !params.MaxCompletionTokens.Valid() {
				params.MaxCompletionTokens = param.NewOpt(*call.MaxOutputTokens)
			}
			params.MaxTokens = param.Opt[int64]{}
		}
	}

	// Handle search preview models
	if isSearchPreviewModel(o.modelID) {
		if call.Temperature != nil {
			params.Temperature = param.Opt[float64]{}
			warnings = append(warnings, genai.CallWarning{
				Type:    genai.CallWarningTypeUnsupportedSetting,
				Setting: "temperature",
				Details: "temperature is not supported for the search preview models and has been removed.",
			})
		}
	}

	optionsWarnings, err := o.prepareCallFunc(o, params, call)
	if err != nil {
		return nil, nil, err
	}

	if len(optionsWarnings) > 0 {
		warnings = append(warnings, optionsWarnings...)
	}

	params.Messages = messages
	params.Model = o.modelID

	if len(call.Tools) > 0 {
		tools, toolChoice, toolWarnings := toOpenAiTools(call.Tools, call.ToolChoice)
		// Only set tools and tool_choice if we have valid tools after filtering
		if len(tools) > 0 {
			params.Tools = tools
			if toolChoice != nil {
				params.ToolChoice = *toolChoice
			}
		}
		warnings = append(warnings, toolWarnings...)
	}
	return params, warnings, nil
}

// Generate implements genai.LanguageModel.
func (o languageModel) Generate(ctx context.Context, call genai.Call) (*genai.Response, error) {
	params, warnings, err := o.prepareParams(call)
	if err != nil {
		return nil, err
	}
	response, err := o.client.Chat.Completions.New(ctx, *params)
	if err != nil {
		return nil, toProviderErr(err)
	}

	if len(response.Choices) == 0 {
		return nil, &genai.Error{Title: "no response", Message: "no response generated"}
	}
	choice := response.Choices[0]
	content := make([]genai.Content, 0, 1+len(choice.Message.ToolCalls)+len(choice.Message.Annotations))
	text := choice.Message.Content
	if text != "" {
		content = append(content, genai.TextContent{
			Text: text,
		})
	}
	if o.extraContentFunc != nil {
		extraContent := o.extraContentFunc(choice)
		content = append(content, extraContent...)
	}
	for _, tc := range choice.Message.ToolCalls {
		toolCallID := tc.ID
		content = append(content, genai.ToolCallContent{
			ProviderExecuted: false,
			ToolCallID:       toolCallID,
			ToolName:         tc.Function.Name,
			Input:            tc.Function.Arguments,
		})
	}
	for _, annotation := range choice.Message.Annotations {
		if annotation.Type == "url_citation" {
			content = append(content, genai.SourceContent{
				SourceType: genai.SourceTypeURL,
				ID:         uuid.NewString(),
				URL:        annotation.URLCitation.URL,
				Title:      annotation.URLCitation.Title,
			})
		}
	}

	usage, providerMetadata := o.usageFunc(*response)

	mappedFinishReason := o.mapFinishReasonFunc(choice.FinishReason)
	if len(choice.Message.ToolCalls) > 0 {
		mappedFinishReason = genai.FinishReasonToolCalls
	}
	return &genai.Response{
		Content:      content,
		Usage:        usage,
		FinishReason: mappedFinishReason,
		ProviderMetadata: genai.ProviderMetadata{
			Name: providerMetadata,
		},
		Warnings: warnings,
	}, nil
}

// Stream implements genai.LanguageModel.
func (o languageModel) Stream(ctx context.Context, call genai.Call) (genai.StreamResponse, error) {
	params, warnings, err := o.prepareParams(call)
	if err != nil {
		return nil, err
	}

	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{
		IncludeUsage: openai.Bool(true),
	}

	stream := o.client.Chat.Completions.NewStreaming(ctx, *params)
	isActiveText := false
	toolCalls := make(map[int64]streamToolCall)

	providerMetadata := genai.ProviderMetadata{
		Name: &ProviderMetadata{},
	}
	acc := openai.ChatCompletionAccumulator{}
	extraContext := make(map[string]any)
	var usage genai.Usage
	var finishReason string
	return func(yield func(genai.StreamPart) bool) {
		if len(warnings) > 0 {
			if !yield(genai.StreamPart{
				Type:     genai.StreamPartTypeWarnings,
				Warnings: warnings,
			}) {
				return
			}
		}
		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)
			usage, providerMetadata = o.streamUsageFunc(chunk, extraContext, providerMetadata)
			if len(chunk.Choices) == 0 {
				continue
			}
			for _, choice := range chunk.Choices {
				if choice.FinishReason != "" {
					finishReason = choice.FinishReason
				}
				switch {
				case choice.Delta.Content != "":
					if !isActiveText {
						isActiveText = true
						if !yield(genai.StreamPart{
							Type: genai.StreamPartTypeTextStart,
							ID:   "0",
						}) {
							return
						}
					}
					if !yield(genai.StreamPart{
						Type:  genai.StreamPartTypeTextDelta,
						ID:    "0",
						Delta: choice.Delta.Content,
					}) {
						return
					}
				case len(choice.Delta.ToolCalls) > 0:
					if isActiveText {
						isActiveText = false
						if !yield(genai.StreamPart{
							Type: genai.StreamPartTypeTextEnd,
							ID:   "0",
						}) {
							return
						}
					}

					for _, toolCallDelta := range choice.Delta.ToolCalls {
						if existingToolCall, ok := toolCalls[toolCallDelta.Index]; ok {
							if existingToolCall.hasFinished {
								continue
							}
							if toolCallDelta.Function.Arguments != "" {
								existingToolCall.arguments += toolCallDelta.Function.Arguments
							}
							if !yield(genai.StreamPart{
								Type:  genai.StreamPartTypeToolInputDelta,
								ID:    existingToolCall.id,
								Delta: toolCallDelta.Function.Arguments,
							}) {
								return
							}
							toolCalls[toolCallDelta.Index] = existingToolCall
							if xjson.IsValid(existingToolCall.arguments) {
								if !yield(genai.StreamPart{
									Type: genai.StreamPartTypeToolInputEnd,
									ID:   existingToolCall.id,
								}) {
									return
								}

								if !yield(genai.StreamPart{
									Type:          genai.StreamPartTypeToolCall,
									ID:            existingToolCall.id,
									ToolCallName:  existingToolCall.name,
									ToolCallInput: existingToolCall.arguments,
								}) {
									return
								}
								existingToolCall.hasFinished = true
								toolCalls[toolCallDelta.Index] = existingToolCall
							}
						} else {
							var err error
							if toolCallDelta.Type != "function" {
								err = &genai.Error{Title: "invalid provider response", Message: "expected 'function' type."}
							}
							if toolCallDelta.ID == "" {
								err = &genai.Error{Title: "invalid provider response", Message: "expected 'id' to be a string."}
							}
							if toolCallDelta.Function.Name == "" {
								err = &genai.Error{Title: "invalid provider response", Message: "expected 'function.name' to be a string."}
							}
							if err != nil {
								yield(genai.StreamPart{
									Type:  genai.StreamPartTypeError,
									Error: toProviderErr(stream.Err()),
								})
								return
							}

							if !yield(genai.StreamPart{
								Type:         genai.StreamPartTypeToolInputStart,
								ID:           toolCallDelta.ID,
								ToolCallName: toolCallDelta.Function.Name,
							}) {
								return
							}
							toolCalls[toolCallDelta.Index] = streamToolCall{
								id:        toolCallDelta.ID,
								name:      toolCallDelta.Function.Name,
								arguments: toolCallDelta.Function.Arguments,
							}

							exTc := toolCalls[toolCallDelta.Index]
							if exTc.arguments != "" {
								if !yield(genai.StreamPart{
									Type:  genai.StreamPartTypeToolInputDelta,
									ID:    exTc.id,
									Delta: exTc.arguments,
								}) {
									return
								}
								if xjson.IsValid(toolCalls[toolCallDelta.Index].arguments) {
									if !yield(genai.StreamPart{
										Type: genai.StreamPartTypeToolInputEnd,
										ID:   toolCallDelta.ID,
									}) {
										return
									}

									if !yield(genai.StreamPart{
										Type:          genai.StreamPartTypeToolCall,
										ID:            exTc.id,
										ToolCallName:  exTc.name,
										ToolCallInput: exTc.arguments,
									}) {
										return
									}
									exTc.hasFinished = true
									toolCalls[toolCallDelta.Index] = exTc
								}
							}
							continue
						}
					}
				}

				if o.streamExtraFunc != nil {
					updatedContext, shouldContinue := o.streamExtraFunc(chunk, yield, extraContext)
					if !shouldContinue {
						return
					}
					extraContext = updatedContext
				}
			}

			for _, choice := range chunk.Choices {
				if annotations := parseAnnotationsFromDelta(choice.Delta); len(annotations) > 0 {
					for _, annotation := range annotations {
						if annotation.Type == "url_citation" {
							if !yield(genai.StreamPart{
								Type:       genai.StreamPartTypeSource,
								ID:         uuid.NewString(),
								SourceType: genai.SourceTypeURL,
								URL:        annotation.URLCitation.URL,
								Title:      annotation.URLCitation.Title,
							}) {
								return
							}
						}
					}
				}
			}
		}
		err := stream.Err()
		if err == nil || errors.Is(err, io.EOF) {
			if isActiveText {
				isActiveText = false
				if !yield(genai.StreamPart{
					Type: genai.StreamPartTypeTextEnd,
					ID:   "0",
				}) {
					return
				}
			}

			if len(acc.Choices) > 0 {
				choice := acc.Choices[0]
				providerMetadata = o.streamProviderMetadataFunc(choice, providerMetadata)

				for _, annotation := range choice.Message.Annotations {
					if annotation.Type == "url_citation" {
						if !yield(genai.StreamPart{
							Type:       genai.StreamPartTypeSource,
							ID:         acc.ID,
							SourceType: genai.SourceTypeURL,
							URL:        annotation.URLCitation.URL,
							Title:      annotation.URLCitation.Title,
						}) {
							return
						}
					}
				}
			}
			mappedFinishReason := o.mapFinishReasonFunc(finishReason)
			if len(acc.Choices) > 0 {
				choice := acc.Choices[0]
				if len(choice.Message.ToolCalls) > 0 {
					mappedFinishReason = genai.FinishReasonToolCalls
				}
			}
			yield(genai.StreamPart{
				Type:             genai.StreamPartTypeFinish,
				Usage:            usage,
				FinishReason:     mappedFinishReason,
				ProviderMetadata: providerMetadata,
			})
			return
		} else { //nolint: revive
			yield(genai.StreamPart{
				Type:  genai.StreamPartTypeError,
				Error: toProviderErr(err),
			})
			return
		}
	}, nil
}

func isReasoningModel(modelID string) bool {
	return strings.HasPrefix(modelID, "o1") || strings.Contains(modelID, "-o1") ||
		strings.HasPrefix(modelID, "o3") || strings.Contains(modelID, "-o3") ||
		strings.HasPrefix(modelID, "o4") || strings.Contains(modelID, "-o4") ||
		strings.HasPrefix(modelID, "oss") || strings.Contains(modelID, "-oss") ||
		strings.Contains(modelID, "gpt-5") || strings.Contains(modelID, "gpt-5-chat")
}

func isSearchPreviewModel(modelID string) bool {
	return strings.Contains(modelID, "search-preview")
}

func supportsFlexProcessing(modelID string) bool {
	return strings.HasPrefix(modelID, "o3") || strings.Contains(modelID, "-o3") ||
		strings.Contains(modelID, "o4-mini") || strings.Contains(modelID, "gpt-5")
}

func supportsPriorityProcessing(modelID string) bool {
	return strings.Contains(modelID, "gpt-4") || strings.Contains(modelID, "gpt-5") ||
		strings.Contains(modelID, "gpt-5-mini") || strings.HasPrefix(modelID, "o3") ||
		strings.Contains(modelID, "-o3") || strings.Contains(modelID, "o4-mini")
}

func toOpenAiTools(tools []genai.Tool, toolChoice *genai.ToolChoice) (openAiTools []openai.ChatCompletionToolUnionParam, openAiToolChoice *openai.ChatCompletionToolChoiceOptionUnionParam, warnings []genai.CallWarning) {
	for _, tool := range tools {
		if tool.GetType() == genai.ToolTypeFunction {
			ft, ok := tool.(genai.FunctionTool)
			if !ok {
				continue
			}
			openAiTools = append(openAiTools, openai.ChatCompletionToolUnionParam{
				OfFunction: &openai.ChatCompletionFunctionToolParam{
					Function: shared.FunctionDefinitionParam{
						Name:        ft.Name,
						Description: param.NewOpt(ft.Description),
						Parameters:  openai.FunctionParameters(ft.InputSchema),
						Strict:      param.NewOpt(false),
					},
					Type: "function",
				},
			})
			continue
		}

		warnings = append(warnings, genai.CallWarning{
			Type:    genai.CallWarningTypeUnsupportedTool,
			Tool:    tool,
			Message: "tool is not supported",
		})
	}

	// Don't set tool_choice if there are no tools - this breaks LiteLLM proxying to Anthropic
	if toolChoice == nil || len(openAiTools) == 0 {
		return openAiTools, openAiToolChoice, warnings
	}

	switch *toolChoice {
	case genai.ToolChoiceAuto:
		openAiToolChoice = &openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: param.NewOpt("auto"),
		}
	case genai.ToolChoiceNone:
		openAiToolChoice = &openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: param.NewOpt("none"),
		}
	default:
		openAiToolChoice = &openai.ChatCompletionToolChoiceOptionUnionParam{
			OfFunctionToolChoice: &openai.ChatCompletionNamedToolChoiceParam{
				Type: "function",
				Function: openai.ChatCompletionNamedToolChoiceFunctionParam{
					Name: string(*toolChoice),
				},
			},
		}
	}
	return openAiTools, openAiToolChoice, warnings
}

// parseAnnotationsFromDelta parses annotations from the raw JSON of a delta.
func parseAnnotationsFromDelta(delta openai.ChatCompletionChunkChoiceDelta) []openai.ChatCompletionMessageAnnotation {
	var annotations []openai.ChatCompletionMessageAnnotation

	// Parse the raw JSON to extract annotations
	var deltaData map[string]any
	if err := json.Unmarshal([]byte(delta.RawJSON()), &deltaData); err != nil {
		return annotations
	}

	// Check if annotations exist in the delta
	if annotationsData, ok := deltaData["annotations"].([]any); ok {
		for _, annotationData := range annotationsData {
			if annotationMap, ok := annotationData.(map[string]any); ok {
				if annotationType, ok := annotationMap["type"].(string); ok && annotationType == "url_citation" {
					if urlCitationData, ok := annotationMap["url_citation"].(map[string]any); ok {
						annotation := openai.ChatCompletionMessageAnnotation{
							Type: "url_citation",
							URLCitation: openai.ChatCompletionMessageAnnotationURLCitation{
								URL:   urlCitationData["url"].(string),
								Title: urlCitationData["title"].(string),
							},
						}
						annotations = append(annotations, annotation)
					}
				}
			}
		}
	}

	return annotations
}

// GenerateObject implements genai.LanguageModel.
func (o languageModel) GenerateObject(ctx context.Context, call genai.ObjectCall) (*genai.ObjectResponse, error) {
	switch o.objectMode {
	case genai.ObjectModeText:
		return object.GenerateWithText(ctx, o, call)
	case genai.ObjectModeTool:
		return object.GenerateWithTool(ctx, o, call)
	default:
		return o.generateObjectWithJSONMode(ctx, call)
	}
}

// StreamObject implements genai.LanguageModel.
func (o languageModel) StreamObject(ctx context.Context, call genai.ObjectCall) (genai.ObjectStreamResponse, error) {
	switch o.objectMode {
	case genai.ObjectModeTool:
		return object.StreamWithTool(ctx, o, call)
	case genai.ObjectModeText:
		return object.StreamWithText(ctx, o, call)
	default:
		return o.streamObjectWithJSONMode(ctx, call)
	}
}

func (o languageModel) generateObjectWithJSONMode(ctx context.Context, call genai.ObjectCall) (*genai.ObjectResponse, error) {
	jsonSchemaMap := schema.ToMap(call.Schema)

	addAdditionalPropertiesFalse(jsonSchemaMap)

	schemaName := call.SchemaName
	if schemaName == "" {
		schemaName = "response"
	}

	fantasyCall := genai.Call{
		Prompt:           call.Prompt,
		MaxOutputTokens:  call.MaxOutputTokens,
		Temperature:      call.Temperature,
		TopP:             call.TopP,
		PresencePenalty:  call.PresencePenalty,
		FrequencyPenalty: call.FrequencyPenalty,
		ProviderOptions:  call.ProviderOptions,
	}

	params, warnings, err := o.prepareParams(fantasyCall)
	if err != nil {
		return nil, err
	}

	params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
			JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:        schemaName,
				Description: param.NewOpt(call.SchemaDescription),
				Schema:      jsonSchemaMap,
				Strict:      param.NewOpt(true),
			},
		},
	}

	response, err := o.client.Chat.Completions.New(ctx, *params)
	if err != nil {
		return nil, toProviderErr(err)
	}

	if len(response.Choices) == 0 {
		usage, _ := o.usageFunc(*response)
		return nil, &genai.NoObjectGeneratedError{
			RawText:      "",
			ParseError:   fmt.Errorf("no choices in response"),
			Usage:        usage,
			FinishReason: genai.FinishReasonUnknown,
		}
	}

	choice := response.Choices[0]
	jsonText := choice.Message.Content

	var obj any
	if call.RepairText != nil {
		obj, err = schema.ParseAndValidateWithRepair(ctx, jsonText, call.Schema, call.RepairText)
	} else {
		obj, err = schema.ParseAndValidate(jsonText, call.Schema)
	}

	usage, _ := o.usageFunc(*response)
	finishReason := o.mapFinishReasonFunc(choice.FinishReason)

	if err != nil {
		if nogErr, ok := err.(*genai.NoObjectGeneratedError); ok {
			nogErr.Usage = usage
			nogErr.FinishReason = finishReason
		}
		return nil, err
	}

	return &genai.ObjectResponse{
		Object:       obj,
		RawText:      jsonText,
		Usage:        usage,
		FinishReason: finishReason,
		Warnings:     warnings,
	}, nil
}

func (o languageModel) streamObjectWithJSONMode(ctx context.Context, call genai.ObjectCall) (genai.ObjectStreamResponse, error) {
	jsonSchemaMap := schema.ToMap(call.Schema)

	addAdditionalPropertiesFalse(jsonSchemaMap)

	schemaName := call.SchemaName
	if schemaName == "" {
		schemaName = "response"
	}

	fantasyCall := genai.Call{
		Prompt:           call.Prompt,
		MaxOutputTokens:  call.MaxOutputTokens,
		Temperature:      call.Temperature,
		TopP:             call.TopP,
		PresencePenalty:  call.PresencePenalty,
		FrequencyPenalty: call.FrequencyPenalty,
		ProviderOptions:  call.ProviderOptions,
	}

	params, warnings, err := o.prepareParams(fantasyCall)
	if err != nil {
		return nil, err
	}

	params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
			JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:        schemaName,
				Description: param.NewOpt(call.SchemaDescription),
				Schema:      jsonSchemaMap,
				Strict:      param.NewOpt(true),
			},
		},
	}

	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{
		IncludeUsage: openai.Bool(true),
	}

	stream := o.client.Chat.Completions.NewStreaming(ctx, *params)

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
		var usage genai.Usage
		var finishReason genai.FinishReason
		var providerMetadata genai.ProviderMetadata
		var streamErr error

		for stream.Next() {
			chunk := stream.Current()

			// Update usage
			usage, providerMetadata = o.streamUsageFunc(chunk, make(map[string]any), providerMetadata)

			if len(chunk.Choices) == 0 {
				continue
			}

			choice := chunk.Choices[0]
			if choice.FinishReason != "" {
				finishReason = o.mapFinishReasonFunc(choice.FinishReason)
			}

			if choice.Delta.Content != "" {
				accumulated += choice.Delta.Content

				obj, state, parseErr := schema.ParsePartialJSON(accumulated)

				if state == schema.ParseStateSuccessful || state == schema.ParseStateRepaired {
					if err := schema.ValidateAgainstSchema(obj, call.Schema); err == nil {
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

		err := stream.Err()
		if err != nil && !errors.Is(err, io.EOF) {
			streamErr = toProviderErr(err)
			yield(genai.ObjectStreamPart{
				Type:  genai.ObjectStreamPartTypeError,
				Error: streamErr,
			})
			return
		}

		if lastParsedObject != nil {
			yield(genai.ObjectStreamPart{
				Type:             genai.ObjectStreamPartTypeFinish,
				Usage:            usage,
				FinishReason:     finishReason,
				ProviderMetadata: providerMetadata,
			})
		} else {
			yield(genai.ObjectStreamPart{
				Type: genai.ObjectStreamPartTypeError,
				Error: &genai.NoObjectGeneratedError{
					RawText:      accumulated,
					ParseError:   fmt.Errorf("no valid object generated in stream"),
					Usage:        usage,
					FinishReason: finishReason,
				},
			})
		}
	}, nil
}

// addAdditionalPropertiesFalse recursively adds "additionalProperties": false to all object schemas.
// This is required by OpenAI's strict mode for structured outputs.
func addAdditionalPropertiesFalse(schema map[string]any) {
	if schema["type"] == "object" {
		if _, hasAdditional := schema["additionalProperties"]; !hasAdditional {
			schema["additionalProperties"] = false
		}

		// Recursively process nested properties
		if properties, ok := schema["properties"].(map[string]any); ok {
			for _, propValue := range properties {
				if propSchema, ok := propValue.(map[string]any); ok {
					addAdditionalPropertiesFalse(propSchema)
				}
			}
		}
	}

	// Handle array items
	if items, ok := schema["items"].(map[string]any); ok {
		addAdditionalPropertiesFalse(items)
	}
}
