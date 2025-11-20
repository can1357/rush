// Copyright 2025 Charmbracelet, Inc.
// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.
//
// This is a fork of github.com/charmbracelet/fantasy modified for Rush.

package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
)

// OpenRouterModelsResponse represents the response from OpenRouter's /models endpoint
type OpenRouterModelsResponse struct {
	Data []OpenRouterModel `json:"data"`
}

// OpenRouterModel represents a single model in the OpenRouter API response
type OpenRouterModel struct {
	ID                   string                  `json:"id"`
	Name                 string                  `json:"name"`
	Created              int64                   `json:"created"`
	Description          string                  `json:"description,omitempty"`
	ContextLength        int                     `json:"context_length"`
	MaxCompletionTokens  int                     `json:"max_completion_tokens,omitempty"`
	Pricing              OpenRouterPricing       `json:"pricing"`
	Architecture         *OpenRouterArchitecture `json:"architecture,omitempty"`
	TopProvider          *OpenRouterTopProvider  `json:"top_provider,omitempty"`
	PerRequestLimits     *OpenRouterLimits       `json:"per_request_limits,omitempty"`
	SupportedParameters  []string                `json:"supported_parameters,omitempty"`
}

// OpenRouterPricing represents the pricing information for a model
type OpenRouterPricing struct {
	Prompt     string `json:"prompt"`     // Cost per token for input (as string to avoid float issues)
	Completion string `json:"completion"` // Cost per token for output
	Image      string `json:"image"`      // Cost per image
	Request    string `json:"request"`    // Cost per request
}

// OpenRouterArchitecture represents the model architecture details
type OpenRouterArchitecture struct {
	Modality     string `json:"modality"`
	Tokenizer    string `json:"tokenizer"`
	InstructType string `json:"instruct_type,omitempty"`
}

// OpenRouterTopProvider represents the top provider for a model
type OpenRouterTopProvider struct {
	ContextLength       int  `json:"context_length"`
	MaxCompletionTokens int  `json:"max_completion_tokens"`
	IsModerated         bool `json:"is_moderated"`
}

// OpenRouterLimits represents the per-request limits for a model
type OpenRouterLimits struct {
	Prompt     *int `json:"prompt_tokens,omitempty"`
	Completion *int `json:"completion_tokens,omitempty"`
}

// DiscoverModels fetches available models from OpenRouter's API
func DiscoverModels(apiKey string, httpClient *http.Client) ([]catwalk.Model, error) {
	return DiscoverModelsWithURL(DefaultURL, apiKey, httpClient)
}

// DiscoverModelsWithURL fetches available models from a specific OpenRouter API URL
func DiscoverModelsWithURL(baseURL string, apiKey string, httpClient *http.Client) ([]catwalk.Model, error) {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	modelsURL := baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization if API key is provided
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models from %s: %w", modelsURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("model discovery failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var modelsResp OpenRouterModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	if len(modelsResp.Data) == 0 {
		return nil, nil
	}

	// Convert OpenRouter models to catwalk.Model format
	models := make([]catwalk.Model, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		model, err := toCatwalkModel(m)
		if err != nil {
			// Log and skip models that fail conversion
			continue
		}
		models = append(models, model)
	}

	return models, nil
}

// toCatwalkModel converts an OpenRouter model to catwalk.Model format
func toCatwalkModel(m OpenRouterModel) (catwalk.Model, error) {
	model := catwalk.Model{
		ID:   m.ID,
		Name: m.Name,
	}

	// Use the actual context length from OpenRouter
	if m.ContextLength > 0 {
		model.ContextWindow = int64(m.ContextLength)
	}

	// Use max completion tokens if provided
	if m.MaxCompletionTokens > 0 {
		model.DefaultMaxTokens = int64(m.MaxCompletionTokens)
	} else if m.TopProvider != nil && m.TopProvider.MaxCompletionTokens > 0 {
		model.DefaultMaxTokens = int64(m.TopProvider.MaxCompletionTokens)
	} else {
		// Conservative default
		model.DefaultMaxTokens = 4096
	}

	// Parse pricing information
	// OpenRouter returns pricing as strings in USD per token
	// We need to convert to cost per 1M tokens
	if promptCost, err := parsePricingString(m.Pricing.Prompt); err == nil {
		// Convert from per-token to per-1M-tokens
		model.CostPer1MIn = promptCost * 1_000_000
	}

	if completionCost, err := parsePricingString(m.Pricing.Completion); err == nil {
		// Convert from per-token to per-1M-tokens
		model.CostPer1MOut = completionCost * 1_000_000
	}

	return model, nil
}

// parsePricingString parses OpenRouter's pricing strings (e.g., "0.000008") to float64
func parsePricingString(s string) (float64, error) {
	if s == "" || s == "0" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}
