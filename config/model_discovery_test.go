package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverModelsFromProvider(t *testing.T) {
	tests := []struct {
		name          string
		response      OpenAIModelsResponse
		statusCode    int
		wantErr       bool
		wantModels    int
		checkMetadata bool
	}{
		{
			name: "successful discovery with multiple models",
			response: OpenAIModelsResponse{
				Data: []OpenAIModel{
					{ID: "gpt-4", Object: "model", Created: 1677610602, OwnedBy: "openai"},
					{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai"},
					{ID: "claude-3-opus", Object: "model", Created: 1677610602, OwnedBy: "anthropic"},
				},
			},
			statusCode:    http.StatusOK,
			wantErr:       false,
			wantModels:    3,
			checkMetadata: true,
		},
		{
			name: "empty model list",
			response: OpenAIModelsResponse{
				Data: []OpenAIModel{},
			},
			statusCode: http.StatusOK,
			wantErr:    false,
			wantModels: 0,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/models", r.URL.Path)
				assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			// Create test config
			cfg := ProviderConfig{
				ID:      "test-provider",
				BaseURL: server.URL,
				APIKey:  "test-key",
			}

			resolver := &mockResolver{}
			models, err := DiscoverModelsFromProvider(cfg, resolver)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, models, tt.wantModels)

			if tt.checkMetadata && len(models) > 0 {
				// Check that default metadata is applied
				for _, m := range models {
					assert.NotEmpty(t, m.ID)
					assert.NotEmpty(t, m.Name)
					assert.Greater(t, m.ContextWindow, int64(0))
					assert.Greater(t, m.DefaultMaxTokens, int64(0))
				}
			}
		})
	}
}

func TestDiscoverModelsWithDefaultMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(OpenAIModelsResponse{
			Data: []OpenAIModel{
				{ID: "test-model", Object: "model", Created: 1677610602, OwnedBy: "test"},
			},
		})
	}))
	defer server.Close()

	cfg := ProviderConfig{
		ID:      "test-provider",
		BaseURL: server.URL,
		APIKey:  "test-key",
		DefaultModelMetadata: &DefaultModelMetadata{
			ContextWindow:    128000,
			DefaultMaxTokens: 8192,
			CostPer1MIn:      1.5,
			CostPer1MOut:     7.5,
		},
	}

	resolver := &mockResolver{}
	models, err := DiscoverModelsFromProvider(cfg, resolver)

	require.NoError(t, err)
	require.Len(t, models, 1)

	model := models[0]
	assert.Equal(t, "test-model", model.ID)
	assert.Equal(t, int64(128000), model.ContextWindow)
	assert.Equal(t, int64(8192), model.DefaultMaxTokens)
	assert.Equal(t, 1.5, model.CostPer1MIn)
	assert.Equal(t, 7.5, model.CostPer1MOut)
}

func TestConvertToCarwalkModel(t *testing.T) {
	tests := []struct {
		name     string
		input    OpenAIModel
		defaults *DefaultModelMetadata
		want     catwalk.Model
	}{
		{
			name: "model with provider prefix",
			input: OpenAIModel{
				ID:      "openai/gpt-4",
				OwnedBy: "openai",
			},
			want: catwalk.Model{
				ID:               "openai/gpt-4",
				Name:             "openai/gpt-4",
				ContextWindow:    8192,
				DefaultMaxTokens: 4096,
			},
		},
		{
			name: "model without prefix",
			input: OpenAIModel{
				ID:      "claude-3-opus-20240229",
				OwnedBy: "anthropic",
			},
			want: catwalk.Model{
				ID:               "claude-3-opus-20240229",
				Name:             "claude-3-opus-20240229",
				ContextWindow:    8192,
				DefaultMaxTokens: 4096,
			},
		},
		{
			name: "with custom defaults",
			input: OpenAIModel{
				ID:      "custom-llm",
				OwnedBy: "acme",
			},
			defaults: &DefaultModelMetadata{
				ContextWindow:    200000,
				DefaultMaxTokens: 16000,
				CostPer1MIn:      3.0,
				CostPer1MOut:     15.0,
			},
			want: catwalk.Model{
				ID:               "custom-llm",
				Name:             "custom-llm",
				ContextWindow:    200000,
				DefaultMaxTokens: 16000,
				CostPer1MIn:      3.0,
				CostPer1MOut:     15.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toCatwalkModel(tt.input, tt.defaults)
			assert.Equal(t, tt.want.ID, got.ID)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.ContextWindow, got.ContextWindow)
			assert.Equal(t, tt.want.DefaultMaxTokens, got.DefaultMaxTokens)
			if tt.defaults != nil {
				assert.Equal(t, tt.want.CostPer1MIn, got.CostPer1MIn)
				assert.Equal(t, tt.want.CostPer1MOut, got.CostPer1MOut)
			}
		})
	}
}

func TestModelCaching(t *testing.T) {
	// Create temporary cache for testing
	testProviderID := "test-litellm"
	testBaseURL := "http://localhost:4000/v1"
	testModels := []catwalk.Model{
		{ID: "model-1", Name: "Model 1", ContextWindow: 8192},
		{ID: "model-2", Name: "Model 2", ContextWindow: 16384},
	}

	// Save to cache
	err := SaveDiscoveredModelsToCache(testProviderID, testBaseURL, testModels)
	require.NoError(t, err)

	// Load from cache
	cache, err := LoadDiscoveredModelsFromCache()
	require.NoError(t, err)
	assert.NotNil(t, cache)

	// Verify cached data
	cached, ok := cache.Providers[testProviderID]
	require.True(t, ok)
	assert.Equal(t, testBaseURL, cached.BaseURL)
	assert.Len(t, cached.Models, 2)
	assert.Equal(t, "model-1", cached.Models[0].ID)
	assert.Equal(t, "model-2", cached.Models[1].ID)

	// Test GetCachedModels helper
	models, found := GetCachedModels(testProviderID)
	require.True(t, found)
	assert.Len(t, models, 2)
}

func TestDiscoverOpenRouterModels(t *testing.T) {
	// Create test server that returns OpenRouter-style response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/models", r.URL.Path)
		assert.Equal(t, "Bearer test-openrouter-key", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		response := map[string]any{
			"data": []map[string]any{
				{
					"id":                    "anthropic/claude-3.5-sonnet",
					"name":                  "Anthropic: Claude 3.5 Sonnet",
					"created":               1690502400,
					"description":           "Claude 3.5 Sonnet",
					"context_length":        200000,
					"max_completion_tokens": 8192,
					"pricing": map[string]string{
						"prompt":     "0.000003",
						"completion": "0.000015",
						"image":      "0",
						"request":    "0",
					},
				},
				{
					"id":                    "openai/gpt-4o",
					"name":                  "OpenAI: GPT-4o",
					"created":               1690502400,
					"context_length":        128000,
					"max_completion_tokens": 16384,
					"pricing": map[string]string{
						"prompt":     "0.0000025",
						"completion": "0.00001",
						"image":      "0",
						"request":    "0",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create OpenRouter provider config
	cfg := ProviderConfig{
		ID:      string(catwalk.InferenceProviderOpenRouter),
		Type:    catwalk.TypeOpenRouter,
		BaseURL: server.URL,
		APIKey:  "test-openrouter-key",
	}

	resolver := &mockResolver{}
	models, err := DiscoverModelsFromProvider(cfg, resolver)

	require.NoError(t, err)
	require.Len(t, models, 2)

	// Check first model (Claude)
	claude := models[0]
	assert.Equal(t, "anthropic/claude-3.5-sonnet", claude.ID)
	assert.Equal(t, "Anthropic: Claude 3.5 Sonnet", claude.Name)
	assert.Equal(t, int64(200000), claude.ContextWindow)
	assert.Equal(t, int64(8192), claude.DefaultMaxTokens)
	// Pricing: 0.000003 * 1M = 3.0
	assert.InDelta(t, 3.0, claude.CostPer1MIn, 0.01)
	// Pricing: 0.000015 * 1M = 15.0
	assert.InDelta(t, 15.0, claude.CostPer1MOut, 0.01)

	// Check second model (GPT-4o)
	gpt := models[1]
	assert.Equal(t, "openai/gpt-4o", gpt.ID)
	assert.Equal(t, "OpenAI: GPT-4o", gpt.Name)
	assert.Equal(t, int64(128000), gpt.ContextWindow)
	assert.Equal(t, int64(16384), gpt.DefaultMaxTokens)
	// Pricing: 0.0000025 * 1M = 2.5
	assert.InDelta(t, 2.5, gpt.CostPer1MIn, 0.01)
	// Pricing: 0.00001 * 1M = 10.0
	assert.InDelta(t, 10.0, gpt.CostPer1MOut, 0.01)
}

// mockResolver implements VariableResolver for testing
type mockResolver struct{}

func (m *mockResolver) ResolveValue(value string) (string, error) {
	return value, nil
}
