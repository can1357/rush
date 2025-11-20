// Copyright 2025 Charmbracelet, Inc.
// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.
//
// This is a fork of github.com/charmbracelet/fantasy modified for Rush.

package genai

import "github.com/go-viper/mapstructure/v2"

// Opt creates a pointer to the given value.
func Opt[T any](v T) *T {
	return &v
}

// ParseOptions parses the given options map into the provided struct.
func ParseOptions[T any](options map[string]any, m *T) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  m,
	})
	if err != nil {
		return err
	}
	return decoder.Decode(options)
}
