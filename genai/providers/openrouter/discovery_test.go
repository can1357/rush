// Copyright 2025 Charmbracelet, Inc.
// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.
//
// This is a fork of github.com/charmbracelet/fantasy modified for Rush.

package openrouter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverModels(t *testing.T) {
	tests := []struct {
		name            string
		response        OpenRouterModelsResponse
		statusCode      int
		wantErr         bool
		wantModelsLen   int
		checkFirstModel bool
	}{
		{
			name: "successful discovery with multiple models",
			response: OpenRouterModelsResponse{
				Data: []OpenRouterModel{
					{
						ID:                  "anthropic/claude-3.5-sonnet",
						Name:                "Anthropic: Claude 3.5 Sonnet",
						Created:             1690502400,
						Description:         "Claude 3.5 Sonnet",
						ContextLength:       200000,
						MaxCompletionTokens: 8192,
						Pricing: OpenRouterPricing{
							Prompt:     "0.000003",
							Completion: "0.000015",
							Image:      "0",
							Request:    "0",
						},
					},
					{
						ID:                  "openai/gpt-4o",
						Name:                "OpenAI: GPT-4o",
						Created:             1690502400,
						ContextLength:       128000,
						MaxCompletionTokens: 16384,
						Pricing: OpenRouterPricing{
							Prompt:     "0.0000025",
							Completion: "0.00001",
							Image:      "0",
							Request:    "0",
						},
					},
				},
			},
			statusCode:      http.StatusOK,
			wantErr:         false,
			wantModelsLen:   2,
			checkFirstModel: true,
		},
		{
			name: "empty models list",
			response: OpenRouterModelsResponse{
				Data: []OpenRouterModel{},
			},
			statusCode:    http.StatusOK,
			wantErr:       false,
			wantModelsLen: 0,
		},
		{
			name:       "API error",
			response:   OpenRouterModelsResponse{},
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, "/models", r.URL.Path)
				assert.Equal(t, "GET", r.Method)

				// Set status code
				w.WriteHeader(tt.statusCode)

				// Write response
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.response)
				} else {
					w.Write([]byte(`{"error": "unauthorized"}`))
				}
			}))
			defer server.Close()

			// Call discovery with test server URL
			models, err := DiscoverModelsWithURL(server.URL, "test-api-key", server.Client())

			// Check error
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Check models count
			assert.Len(t, models, tt.wantModelsLen)

			// Check first model if requested
			if tt.checkFirstModel && len(models) > 0 {
				model := models[0]
				assert.Equal(t, "anthropic/claude-3.5-sonnet", model.ID)
				assert.Equal(t, "Anthropic: Claude 3.5 Sonnet", model.Name)
				assert.Equal(t, int64(200000), model.ContextWindow)
				assert.Equal(t, int64(8192), model.DefaultMaxTokens)
				// Check pricing conversion: 0.000003 * 1M = 3.0
				assert.InDelta(t, 3.0, model.CostPer1MIn, 0.01)
				// Check pricing conversion: 0.000015 * 1M = 15.0
				assert.InDelta(t, 15.0, model.CostPer1MOut, 0.01)
			}
		})
	}
}

func TestToCatwalkModel(t *testing.T) {
	tests := []struct {
		name       string
		input      OpenRouterModel
		wantID     string
		wantCtx    int64
		wantMaxTok int64
		wantCostIn float64
	}{
		{
			name: "model with full metadata",
			input: OpenRouterModel{
				ID:                  "test/model",
				Name:                "Test Model",
				ContextLength:       100000,
				MaxCompletionTokens: 4096,
				Pricing: OpenRouterPricing{
					Prompt:     "0.000005",
					Completion: "0.000015",
				},
			},
			wantID:     "test/model",
			wantCtx:    100000,
			wantMaxTok: 4096,
			wantCostIn: 5.0, // 0.000005 * 1M
		},
		{
			name: "model with top provider metadata",
			input: OpenRouterModel{
				ID:            "test/model2",
				Name:          "Test Model 2",
				ContextLength: 50000,
				TopProvider: &OpenRouterTopProvider{
					MaxCompletionTokens: 8192,
				},
				Pricing: OpenRouterPricing{
					Prompt:     "0.000002",
					Completion: "0.000008",
				},
			},
			wantID:     "test/model2",
			wantCtx:    50000,
			wantMaxTok: 8192, // From TopProvider
			wantCostIn: 2.0,
		},
		{
			name: "model with defaults",
			input: OpenRouterModel{
				ID:            "test/model3",
				Name:          "Test Model 3",
				ContextLength: 32000,
				Pricing: OpenRouterPricing{
					Prompt:     "0",
					Completion: "0",
				},
			},
			wantID:     "test/model3",
			wantCtx:    32000,
			wantMaxTok: 4096, // Default
			wantCostIn: 0.0,  // Free model
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := toCatwalkModel(tt.input)
			require.NoError(t, err)

			assert.Equal(t, tt.wantID, model.ID)
			assert.Equal(t, tt.wantCtx, model.ContextWindow)
			assert.Equal(t, tt.wantMaxTok, model.DefaultMaxTokens)
			assert.InDelta(t, tt.wantCostIn, model.CostPer1MIn, 0.01)
		})
	}
}

func TestParsePricingString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"valid decimal", "0.000005", 0.000005, false},
		{"zero", "0", 0, false},
		{"empty string", "", 0, false},
		{"scientific notation", "5e-6", 0.000005, false},
		{"invalid", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePricingString(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tt.want, got, 1e-10)
		})
	}
}
