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

	"github.com/can1357/rush/genai/providers/openrouter"
	"github.com/can1357/rush/home"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/catwalk/pkg/embedded"
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
	// Check if this is OpenRouter and use OpenRouter-specific discovery
	if cfg.ID == string(catwalk.InferenceProviderOpenRouter) || cfg.Type == catwalk.TypeOpenRouter {
		return discoverOpenRouterModels(cfg, resolver)
	}

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
		model := toCatwalkModel(m, cfg.DefaultModelMetadata)
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

func getModelUniqueID(modelID string) (string, bool) {
	if idx := strings.LastIndex(modelID, "/"); idx != -1 {
		return modelID[idx+1:], true
	}
	if len(modelID) > 0 {
		return modelID, true
	}
	return "", false
}

const (
	// BaseContextWindow is the default context window for discovered models
	BaseContextWindow = 200000
	// BaseDefaultMaxTokens is the default max tokens for discovered models
	BaseDefaultMaxTokens = 8192
)

// toCatwalkModel converts an OpenAI model to catwalk.Model format
func toCatwalkModel(oai OpenAIModel, defaults *DefaultModelMetadata) catwalk.Model {
	id := oai.ID

	if modelUid, ok := getModelUniqueID(id); ok {
		providers := embedded.GetAll()
		for p := range providers {
			prov := &providers[p]
			for m := range prov.Models {
				m2 := &prov.Models[m]

				uid, ok := getModelUniqueID(m2.ID)
				if ok && uid == modelUid {
					result := *m2
					result.ID = id
					return result
				}
			}
		}

		for p := range providers {
			prov := &providers[p]
			for m := range prov.Models {
				m2 := &prov.Models[m]
				if m2.Name == id {
					result := *m2
					result.ID = id
					return result
				}
			}
		}
	}

	model := catwalk.Model{
		ID:               id,
		Name:             id,
		ContextWindow:    BaseContextWindow,
		DefaultMaxTokens: BaseDefaultMaxTokens,
	}
	if defaults != nil && defaults.ContextWindow > 0 {
		model.ContextWindow = defaults.ContextWindow
	}
	if defaults != nil && defaults.DefaultMaxTokens > 0 {
		model.DefaultMaxTokens = defaults.DefaultMaxTokens
	}
	if defaults != nil && defaults.CostPer1MIn > 0 {
		model.CostPer1MIn = defaults.CostPer1MIn
	}
	if defaults != nil && defaults.CostPer1MOut > 0 {
		model.CostPer1MOut = defaults.CostPer1MOut
	}
	if defaults != nil && defaults.CostPer1MInCached > 0 {
		model.CostPer1MInCached = defaults.CostPer1MInCached
	}
	if defaults != nil && defaults.CostPer1MOutCached > 0 {
		model.CostPer1MOutCached = defaults.CostPer1MOutCached
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

// discoverOpenRouterModels uses OpenRouter-specific API to discover models with full metadata
func discoverOpenRouterModels(cfg ProviderConfig, resolver VariableResolver) ([]catwalk.Model, error) {
	apiKey := ""
	if cfg.APIKey != "" {
		var err error
		apiKey, err = resolver.ResolveValue(cfg.APIKey)
		if err != nil {
			slog.Warn("Failed to resolve API key for OpenRouter model discovery", "provider", cfg.ID, "error", err)
		}
	}

	baseURL, err := resolver.ResolveValue(cfg.BaseURL)
	if err != nil {
		baseURL = openrouter.DefaultURL // Use default if resolution fails
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Call OpenRouter-specific discovery
	models, err := openrouter.DiscoverModels(apiKey, client)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter model discovery failed: %w", err)
	}

	if len(models) == 0 {
		slog.Warn("No models discovered from OpenRouter", "provider", cfg.ID)
		return nil, nil
	}

	slog.Info("Discovered models from OpenRouter",
		"provider", cfg.ID,
		"count", len(models),
		"base_url", baseURL)

	// Save to cache for offline use
	if err := SaveDiscoveredModelsToCache(cfg.ID, baseURL, models); err != nil {
		slog.Warn("Failed to save OpenRouter models to cache", "provider", cfg.ID, "error", err)
	}

	return models, nil
}
