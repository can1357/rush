package genaiopts

import (
	"maps"

	"github.com/can1357/rush/genai"
	"github.com/can1357/rush/genai/providers/anthropic"
	"github.com/can1357/rush/genai/providers/openai"
	"github.com/can1357/rush/genai/providers/openaicompat"
	"github.com/can1357/rush/genai/providers/openrouter"
	"github.com/openai/openai-go/v2/shared"
)

// ReasoningEffort is a provider-agnostic effort level for structured reasoning.
type ReasoningEffort int64

const (
	// ReasoningEffortOff disables reasoning.
	ReasoningEffortOff ReasoningEffort = 0
	// ReasoningEffortLow is a low level of reasoning effort.
	ReasoningEffortLow ReasoningEffort = 1024
	// ReasoningEffortMedium is a medium level of reasoning effort.
	ReasoningEffortMedium ReasoningEffort = 8192
	// ReasoningEffortHigh is a high level of reasoning effort.
	ReasoningEffortHigh ReasoningEffort = 31999
)

// Budget tokens commonly used by providers (best-effort; providers may ignore).
func (e ReasoningEffort) Budget() int64 {
	return int64(e)
}

var (
	openaiMinimalReasoningEffort = shared.ReasoningEffortMinimal
	openaiLowReasoningEffort     = shared.ReasoningEffortLow
	openaiMediumReasoningEffort  = shared.ReasoningEffortMedium
	openaiHighReasoningEffort    = shared.ReasoningEffortHigh
)

// OpenAIReasoningEffort returns the OpenAI-compatible reasoning effort level.
func (e ReasoningEffort) OpenAIReasoningEffort() *shared.ReasoningEffort {
	if !e.IsThinkingEnabled() {
		return nil
	} else if e <= 1000 {
		return &openaiMinimalReasoningEffort
	} else if e <= 1024 {
		return &openaiLowReasoningEffort
	} else if e <= 8192 {
		return &openaiMediumReasoningEffort
	} else {
		return &openaiHighReasoningEffort
	}
}

// IsThinkingEnabled returns true if reasoning is enabled.
func (e ReasoningEffort) IsThinkingEnabled() bool {
	return e > 0
}

var (
	openrouterLowReasoningEffort    = openrouter.ReasoningEffortLow
	openrouterMediumReasoningEffort = openrouter.ReasoningEffortMedium
	openrouterHighReasoningEffort   = openrouter.ReasoningEffortHigh
)

// OpenRouterReasoningEffort returns the OpenRouter-compatible reasoning effort level.
func (e ReasoningEffort) OpenRouterReasoningEffort() *openrouter.ReasoningEffort {
	if !e.IsThinkingEnabled() {
		return nil
	} else if e <= 1024 {
		return &openrouterLowReasoningEffort
	} else if e <= 8192 {
		return &openrouterMediumReasoningEffort
	} else {
		return &openrouterHighReasoningEffort
	}
}

// Apply returns a copy of ProviderOptions with reasoning effort and optional max tokens set.
func (e ReasoningEffort) Apply(opts genai.ProviderOptions) genai.ProviderOptions {
	clone := maps.Clone(opts)

	// OpenAI (native)
	if v, ok := clone[openai.Name]; ok {
		if po, ok := v.(*openai.ProviderOptions); ok {
			po.ReasoningEffort = e.OpenAIReasoningEffort()
		}
	} else {
		clone[openai.Name] = &openai.ProviderOptions{
			ReasoningEffort: e.OpenAIReasoningEffort(),
		}
	}

	// OpenAI-compatible (openaicompat)
	if v, ok := clone[openaicompat.Name]; ok {
		if po, ok := v.(*openaicompat.ProviderOptions); ok {
			po.ReasoningEffort = e.OpenAIReasoningEffort()
		}
	} else {
		clone[openaicompat.Name] = &openaicompat.ProviderOptions{
			ReasoningEffort: e.OpenAIReasoningEffort(),
		}
	}

	// OpenRouter (supports effort + max_tokens)
	if v, ok := clone[openrouter.Name]; ok {
		if po, ok := v.(*openrouter.ProviderOptions); ok {
			if po.Reasoning == nil {
				po.Reasoning = &openrouter.ReasoningOptions{}
			}
			po.Reasoning.Effort = e.OpenRouterReasoningEffort()
			if e != 0 {
				po.Reasoning.MaxTokens = (*int64)(&e)
			}
		}
	} else {
		var mtPtr *int64
		if e.IsThinkingEnabled() {
			mt := int64(e)
			mtPtr = &mt
		}

		clone[openrouter.Name] = &openrouter.ProviderOptions{
			Reasoning: &openrouter.ReasoningOptions{
				Effort:    e.OpenRouterReasoningEffort(),
				MaxTokens: mtPtr,
			},
		}
	}

	// Anthropic (uses thinking.budget_tokens)
	if v, ok := clone[anthropic.Name]; ok {
		if po, ok := v.(*anthropic.ProviderOptions); ok {
			if e.IsThinkingEnabled() {
				po.Thinking = &anthropic.ThinkingProviderOption{
					BudgetTokens: e.Budget(),
				}
			} else {
				po.Thinking = nil
			}
		}
	} else if e.IsThinkingEnabled() {
		clone[anthropic.Name] = &anthropic.ProviderOptions{
			Thinking: &anthropic.ThinkingProviderOption{
				BudgetTokens: e.Budget(),
			},
		}
	} else {
		clone[anthropic.Name] = &anthropic.ProviderOptions{}
	}

	return clone
}
