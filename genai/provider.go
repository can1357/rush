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
)

// Provider represents a provider of language models.
type Provider interface {
	Name() string
	LanguageModel(ctx context.Context, modelID string) (LanguageModel, error)
}
