// Copyright 2025 Charmbracelet, Inc.
// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.
//
// This is a fork of github.com/charmbracelet/fantasy modified for Rush.

// Package openai provides an implementation of the fantasy AI SDK for OpenAI's language models.
package openai

import (
	"encoding/json"

	"github.com/can1357/rush/genai"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"
)

// Global type identifiers for OpenAI-specific provider data.
const (
	TypeProviderOptions     = Name + ".options"
	TypeProviderFileOptions = Name + ".file_options"
	TypeProviderMetadata    = Name + ".metadata"
)

// Register OpenAI provider-specific types with the global registry.
func init() {
	genai.RegisterProviderType(TypeProviderOptions, func(data []byte) (genai.ProviderOptionsData, error) {
		var v ProviderOptions
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	})
	genai.RegisterProviderType(TypeProviderFileOptions, func(data []byte) (genai.ProviderOptionsData, error) {
		var v ProviderFileOptions
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	})
	genai.RegisterProviderType(TypeProviderMetadata, func(data []byte) (genai.ProviderOptionsData, error) {
		var v ProviderMetadata
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	})
}

// ProviderMetadata represents additional metadata from OpenAI provider.
type ProviderMetadata struct {
	Logprobs                 []openai.ChatCompletionTokenLogprob `json:"logprobs"`
	AcceptedPredictionTokens int64                               `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int64                               `json:"rejected_prediction_tokens"`
}

// Options implements the ProviderOptions interface.
func (*ProviderMetadata) Options() {}

// MarshalJSON implements custom JSON marshaling with type info for ProviderMetadata.
func (m ProviderMetadata) MarshalJSON() ([]byte, error) {
	type plain ProviderMetadata
	return genai.MarshalProviderType(TypeProviderMetadata, plain(m))
}

// UnmarshalJSON implements custom JSON unmarshaling with type info for ProviderMetadata.
func (m *ProviderMetadata) UnmarshalJSON(data []byte) error {
	type plain ProviderMetadata
	var p plain
	if err := genai.UnmarshalProviderType(data, &p); err != nil {
		return err
	}
	*m = ProviderMetadata(p)
	return nil
}

// ProviderOptions represents additional options for OpenAI provider.
type ProviderOptions struct {
	LogitBias           map[string]int64        `json:"logit_bias"`
	LogProbs            *bool                   `json:"log_probs"`
	TopLogProbs         *int64                  `json:"top_log_probs"`
	ParallelToolCalls   *bool                   `json:"parallel_tool_calls"`
	User                *string                 `json:"user"`
	ReasoningEffort     *shared.ReasoningEffort `json:"reasoning_effort"`
	MaxCompletionTokens *int64                  `json:"max_completion_tokens"`
	TextVerbosity       *string                 `json:"text_verbosity"`
	Prediction          map[string]any          `json:"prediction"`
	Store               *bool                   `json:"store"`
	Metadata            map[string]any          `json:"metadata"`
	PromptCacheKey      *string                 `json:"prompt_cache_key"`
	SafetyIdentifier    *string                 `json:"safety_identifier"`
	ServiceTier         *string                 `json:"service_tier"`
	StructuredOutputs   *bool                   `json:"structured_outputs"`
}

// Options implements the ProviderOptions interface.
func (*ProviderOptions) Options() {}

// MarshalJSON implements custom JSON marshaling with type info for ProviderOptions.
func (o ProviderOptions) MarshalJSON() ([]byte, error) {
	type plain ProviderOptions
	return genai.MarshalProviderType(TypeProviderOptions, plain(o))
}

// UnmarshalJSON implements custom JSON unmarshaling with type info for ProviderOptions.
func (o *ProviderOptions) UnmarshalJSON(data []byte) error {
	type plain ProviderOptions
	var p plain
	if err := genai.UnmarshalProviderType(data, &p); err != nil {
		return err
	}
	*o = ProviderOptions(p)
	return nil
}

// ProviderFileOptions represents file options for OpenAI provider.
type ProviderFileOptions struct {
	ImageDetail string `json:"image_detail"`
}

// Options implements the ProviderOptions interface.
func (*ProviderFileOptions) Options() {}

// MarshalJSON implements custom JSON marshaling with type info for ProviderFileOptions.
func (o ProviderFileOptions) MarshalJSON() ([]byte, error) {
	type plain ProviderFileOptions
	return genai.MarshalProviderType(TypeProviderFileOptions, plain(o))
}

// UnmarshalJSON implements custom JSON unmarshaling with type info for ProviderFileOptions.
func (o *ProviderFileOptions) UnmarshalJSON(data []byte) error {
	type plain ProviderFileOptions
	var p plain
	if err := genai.UnmarshalProviderType(data, &p); err != nil {
		return err
	}
	*o = ProviderFileOptions(p)
	return nil
}

// NewProviderOptions creates new provider options for OpenAI.
func NewProviderOptions(opts *ProviderOptions) genai.ProviderOptions {
	return genai.ProviderOptions{
		Name: opts,
	}
}

// NewProviderFileOptions creates new file options for OpenAI.
func NewProviderFileOptions(opts *ProviderFileOptions) genai.ProviderOptions {
	return genai.ProviderOptions{
		Name: opts,
	}
}

// ParseOptions parses provider options from a map.
func ParseOptions(data map[string]any) (*ProviderOptions, error) {
	var options ProviderOptions
	if err := genai.ParseOptions(data, &options); err != nil {
		return nil, err
	}
	return &options, nil
}
