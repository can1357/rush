// Copyright 2025 Charmbracelet, Inc.
// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.
//
// This is a fork of github.com/charmbracelet/fantasy modified for Rush.

package genai

import (
	"context"
	"fmt"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/catwalk/pkg/embedded"
)

// Provider represents a provider of language models.
type Provider interface {
	Name() string
	Models() []catwalk.Model
	ModelDescription(modelID string) *catwalk.Model
	LanguageModel(ctx context.Context, modelID string) (LanguageModel, error)
}

func GetKnownProviderInfo(id catwalk.InferenceProvider) *catwalk.Provider {
	providers := embedded.GetAll()
	for i := range providers {
		if providers[i].ID == id {
			return &providers[i]
		}
	}
	panic(fmt.Errorf("provider %s not found", id))
}
