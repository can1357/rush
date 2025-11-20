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

// String returns the string representation of the reasoning effort level.
func (e ReasoningEffort) String() string {
	switch e {
	case ReasoningEffortOff:
		return ""
	case ReasoningEffortLow:
		return "low"
	case ReasoningEffortMedium:
		return "medium"
	case ReasoningEffortHigh:
		return "high"
	default:
		return ""
	}
}

// ParseReasoningEffort converts a string reasoning effort level to ReasoningEffort.
// Returns ReasoningEffortOff for empty string or unrecognized values.
func ParseReasoningEffort(s string) ReasoningEffort {
	switch s {
	case "minimal", "low":
		return ReasoningEffortLow
	case "medium":
		return ReasoningEffortMedium
	case "high":
		return ReasoningEffortHigh
	default:
		return ReasoningEffortOff
	}
}

// DefaultEffortFromModel extracts the default reasoning effort from a model's DefaultReasoningEffort field.
func DefaultEffortFromModel(defaultReasoningEffort string) ReasoningEffort {
	return ParseReasoningEffort(defaultReasoningEffort)
}

// ExtractFromProviderOptionsMap attempts to extract reasoning effort from a ProviderOptions map.
// This is a convenience wrapper for ModelSelection.ProviderOptions which is stored as map[string]any.
// Returns ReasoningEffortOff if no reasoning configuration is found.
func ExtractFromProviderOptionsMap(optsMap map[string]any) ReasoningEffort {
	if optsMap == nil {
		return ReasoningEffortOff
	}

	// Convert map to genai.ProviderOptions
	opts := make(genai.ProviderOptions)
	for k, v := range optsMap {
		if data, ok := v.(genai.ProviderOptionsData); ok {
			opts[k] = data
		}
	}

	return ExtractFromProviderOptions(opts)
}

// ExtractFromProviderOptions attempts to extract reasoning effort from ProviderOptions.
// Returns ReasoningEffortOff if no reasoning configuration is found.
func ExtractFromProviderOptions(opts genai.ProviderOptions) ReasoningEffort {
	// Check Anthropic provider
	if v, ok := opts[anthropic.Name]; ok {
		if po, ok := v.(*anthropic.ProviderOptions); ok && po.Thinking != nil {
			return ReasoningEffort(po.Thinking.BudgetTokens)
		}
	}

	// Check OpenAI provider
	if v, ok := opts[openai.Name]; ok {
		if po, ok := v.(*openai.ProviderOptions); ok && po.ReasoningEffort != nil {
			// Map shared.ReasoningEffort back to our type
			switch *po.ReasoningEffort {
			case shared.ReasoningEffortMinimal, shared.ReasoningEffortLow:
				return ReasoningEffortLow
			case shared.ReasoningEffortMedium:
				return ReasoningEffortMedium
			case shared.ReasoningEffortHigh:
				return ReasoningEffortHigh
			}
		}
	}

	// Check OpenAI-compatible provider
	if v, ok := opts[openaicompat.Name]; ok {
		if po, ok := v.(*openaicompat.ProviderOptions); ok && po.ReasoningEffort != nil {
			switch *po.ReasoningEffort {
			case shared.ReasoningEffortMinimal, shared.ReasoningEffortLow:
				return ReasoningEffortLow
			case shared.ReasoningEffortMedium:
				return ReasoningEffortMedium
			case shared.ReasoningEffortHigh:
				return ReasoningEffortHigh
			}
		}
	}

	// Check OpenRouter provider
	if v, ok := opts[openrouter.Name]; ok {
		if po, ok := v.(*openrouter.ProviderOptions); ok && po.Reasoning != nil {
			if po.Reasoning.MaxTokens != nil {
				return ReasoningEffort(*po.Reasoning.MaxTokens)
			}
		}
	}

	return ReasoningEffortOff
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

// ApplyToMap returns a copy of ProviderOptions map with reasoning effort set.
// This is a convenience wrapper for ModelSelection.ProviderOptions which is stored as map[string]any.
func (e ReasoningEffort) ApplyToMap(optsMap map[string]any) map[string]any {
	// Convert map to genai.ProviderOptions
	opts := make(genai.ProviderOptions)
	if optsMap != nil {
		for k, v := range optsMap {
			if data, ok := v.(genai.ProviderOptionsData); ok {
				opts[k] = data
			}
		}
	}

	// Apply reasoning effort
	opts = e.Apply(opts)

	// Convert back to map[string]any
	result := make(map[string]any)
	for k, v := range opts {
		result[k] = v
	}
	return result
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
