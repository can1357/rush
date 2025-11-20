// Copyright 2025 Charmbracelet, Inc.
// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.
//
// This is a fork of github.com/charmbracelet/fantasy modified for Rush.

// Package azure provides an implementation of the fantasy AI SDK for Azure's language models.
package azure

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/can1357/rush/genai"
	"github.com/can1357/rush/genai/providers/openai"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/openai/openai-go/v2/azure"
	"github.com/openai/openai-go/v2/option"
)

type provider struct {
	wrapped genai.Provider
	models  []catwalk.Model
}

type options struct {
	baseURL    string
	apiKey     string
	apiVersion string

	openaiOptions []openai.Option
	models        []catwalk.Model
}

const (
	// Name is the name of the Azure provider.
	Name = "azure"
	// defaultAPIVersion is the default API version for Azure.
	defaultAPIVersion = "2025-01-01-preview"
)

// azureURLPattern matches Azure OpenAI endpoint URLs in various formats:
// * https://resource-id.openai.azure.com;
// * https://resource-id.openai.azure.com/;
// * https://resource-id.cognitiveservices.azure.com;
// * https://resource-id.services.ai.azure.com/api/projects/project-name;
// * resource-id.openai.azure.com.
var azureURLPattern = regexp.MustCompile(`^(?:https?://)?([a-zA-Z0-9-]+)\.(?:openai|cognitiveservices|services\.ai)\.azure\.com(?:/.*)?$`)

// Option defines a function that configures Azure provider options.
type Option = func(*options)

func WithModels(models []catwalk.Model) Option {
	return func(o *options) {
		o.models = models
	}
}

// New creates a new Azure provider with the given options.
func New(opts ...Option) (genai.Provider, error) {
	o := options{
		apiVersion: defaultAPIVersion,
	}
	for _, opt := range opts {
		opt(&o)
	}

	if o.models == nil {
		o.models = genai.GetKnownProviderInfo(catwalk.InferenceProviderAzure).Models
	}

	wrapped, err := openai.New(
		append(
			o.openaiOptions,
			openai.WithName(Name),
			openai.WithBaseURL(o.baseURL),
			openai.WithModels(o.models),
			openai.WithSDKOptions(
				azure.WithAPIKey(o.apiKey),
			),
		)...,
	)
	if err != nil {
		return nil, err
	}

	return &provider{
		wrapped: wrapped,
		models:  o.models,
	}, nil
}

// WithBaseURL sets the base URL for the Azure provider.
func WithBaseURL(baseURL string) Option {
	return func(o *options) {
		o.baseURL = parseAzureURL(baseURL)
	}
}

// parseAzureURL extracts the resource ID from various Azure URL formats
// and returns the standardized OpenAI-compatible endpoint URL.
// If the URL doesn't match known Azure patterns, it returns the original URL.
func parseAzureURL(baseURL string) string {
	matches := azureURLPattern.FindStringSubmatch(baseURL)
	if len(matches) >= 2 {
		resourceID := matches[1]
		return fmt.Sprintf("https://%s.openai.azure.com/openai/v1", resourceID)
	}
	// fallback to use the provided url
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return "https://" + baseURL
	}
	return baseURL
}

// WithAPIKey sets the API key for the Azure provider.
func WithAPIKey(apiKey string) Option {
	return func(o *options) {
		o.apiKey = apiKey
	}
}

// WithHeaders sets the headers for the Azure provider.
func WithHeaders(headers map[string]string) Option {
	return func(o *options) {
		o.openaiOptions = append(o.openaiOptions, openai.WithHeaders(headers))
	}
}

// WithAPIVersion sets the API version for the Azure provider.
func WithAPIVersion(version string) Option {
	return func(o *options) {
		o.apiVersion = version
	}
}

// WithHTTPClient sets the HTTP client for the Azure provider.
func WithHTTPClient(client option.HTTPClient) Option {
	return func(o *options) {
		o.openaiOptions = append(o.openaiOptions, openai.WithHTTPClient(client))
	}
}

// WithUseResponsesAPI configures the provider to use the responses API for models that support it.
func WithUseResponsesAPI() Option {
	return func(o *options) {
		o.openaiOptions = append(o.openaiOptions, openai.WithUseResponsesAPI())
	}
}

// LanguageModel implements genai.Provider.
func (p *provider) LanguageModel(ctx context.Context, modelID string) (genai.LanguageModel, error) {
	model := p.ModelDescription(modelID)
	if model == nil {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}
	return p.wrapped.LanguageModel(ctx, modelID)
}

func (p *provider) Name() string {
	return Name
}

func (p *provider) Models() []catwalk.Model {
	return p.models
}

func (p *provider) ModelDescription(modelID string) *catwalk.Model {
	for i := range p.models {
		if p.models[i].ID == modelID {
			return &p.models[i]
		}
	}
	return nil
}
