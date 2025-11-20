package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/can1357/rush/home"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
)

// OpenAIModelsResponse represents the response from /v1/models endpoint
type OpenAIModelsResponse struct {
	Data []OpenAIModel `json:"data"`
}

// OpenAIModel represents a single model in the OpenAI API response
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// DiscoverModelsFromProvider fetches available models from an OpenAI-compatible provider
func DiscoverModelsFromProvider(cfg ProviderConfig, resolver VariableResolver) ([]catwalk.Model, error) {
	baseURL, err := resolver.ResolveValue(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base URL: %w", err)
	}

	apiKey := ""
	if cfg.APIKey != "" {
		apiKey, err = resolver.ResolveValue(cfg.APIKey)
		if err != nil {
			slog.Warn("Failed to resolve API key for model discovery", "provider", cfg.ID, "error", err)
		}
	}

	// Ensure base URL doesn't end with /
	baseURL = strings.TrimSuffix(baseURL, "/")
	modelsURL := baseURL + "/models"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header if API key is present
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Add any extra headers from provider config
	for k, v := range cfg.ExtraHeaders {
		resolvedValue, err := resolver.ResolveValue(v)
		if err != nil {
			slog.Warn("Failed to resolve extra header", "header", k, "error", err)
			resolvedValue = v
		}
		req.Header.Set(k, resolvedValue)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
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

	var modelsResp OpenAIModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	if len(modelsResp.Data) == 0 {
		slog.Warn("No models discovered from provider", "provider", cfg.ID, "url", modelsURL)
		return nil, nil
	}

	// Convert OpenAI models to catwalk.Model format
	models := make([]catwalk.Model, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		// Skip wildcard entries (e.g., "anthropic/*")
		if strings.Contains(m.ID, "*") {
			continue
		}
		model := convertToCarwalkModel(m, cfg.DefaultModelMetadata)
		models = append(models, model)
	}

	slog.Info("Discovered models from provider",
		"provider", cfg.ID,
		"count", len(models),
		"url", modelsURL)

	// Save to cache for offline use
	if err := SaveDiscoveredModelsToCache(cfg.ID, baseURL, models); err != nil {
		slog.Warn("Failed to save discovered models to cache", "provider", cfg.ID, "error", err)
	}

	return models, nil
}

// convertToCarwalkModel converts an OpenAI model to catwalk.Model format
func convertToCarwalkModel(m OpenAIModel, defaults *DefaultModelMetadata) catwalk.Model {
	model := catwalk.Model{
		ID:   m.ID,
		Name: m.ID,
	}

	// Apply defaults if provided
	if defaults != nil {
		if defaults.ContextWindow > 0 {
			model.ContextWindow = defaults.ContextWindow
		}
		if defaults.DefaultMaxTokens > 0 {
			model.DefaultMaxTokens = defaults.DefaultMaxTokens
		}
		if defaults.CostPer1MIn > 0 {
			model.CostPer1MIn = defaults.CostPer1MIn
		}
		if defaults.CostPer1MOut > 0 {
			model.CostPer1MOut = defaults.CostPer1MOut
		}
	}

	// Apply sensible defaults if no metadata provided
	if model.ContextWindow == 0 {
		model.ContextWindow = 8192 // Conservative default
	}
	if model.DefaultMaxTokens == 0 {
		model.DefaultMaxTokens = 4096 // Conservative default
	}

	return model
}

// DiscoveredModelsCache stores cached discovered models for providers
type DiscoveredModelsCache struct {
	Providers map[string]CachedProviderModels `json:"providers"`
	Version   string                          `json:"version"`
}

// CachedProviderModels stores models and discovery metadata for a provider
type CachedProviderModels struct {
	Models       []catwalk.Model `json:"models"`
	DiscoveredAt time.Time       `json:"discovered_at"`
	BaseURL      string          `json:"base_url"`
}

// discoveredModelsCachePath returns the path to the discovered models cache file
func discoveredModelsCachePath() string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		return filepath.Join(xdgDataHome, appName, "discovered-models.json")
	}
	return filepath.Join(home.Dir(), ".rush", "discovered-models.json")
}

// SaveDiscoveredModelsToCache saves discovered models to cache
func SaveDiscoveredModelsToCache(providerID string, baseURL string, models []catwalk.Model) error {
	cachePath := discoveredModelsCachePath()

	// Load existing cache or create new one
	cache, err := LoadDiscoveredModelsFromCache()
	if err != nil {
		cache = &DiscoveredModelsCache{
			Providers: make(map[string]CachedProviderModels),
			Version:   "1",
		}
	}

	// Update cache for this provider
	cache.Providers[providerID] = CachedProviderModels{
		Models:       models,
		DiscoveredAt: time.Now(),
		BaseURL:      baseURL,
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Write to file
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(cachePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	slog.Debug("Saved discovered models to cache",
		"provider", providerID,
		"count", len(models),
		"path", cachePath)

	return nil
}

// LoadDiscoveredModelsFromCache loads the discovered models cache
func LoadDiscoveredModelsFromCache() (*DiscoveredModelsCache, error) {
	cachePath := discoveredModelsCachePath()

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("cache file does not exist")
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache DiscoveredModelsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache: %w", err)
	}

	return &cache, nil
}

// GetCachedModels retrieves cached models for a provider if they exist
func GetCachedModels(providerID string) ([]catwalk.Model, bool) {
	cache, err := LoadDiscoveredModelsFromCache()
	if err != nil {
		return nil, false
	}

	cached, ok := cache.Providers[providerID]
	if !ok {
		return nil, false
	}

	return cached.Models, true
}
