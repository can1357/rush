// Copyright 2025 Charmbracelet, Inc.
// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.
//
// This is a fork of github.com/charmbracelet/fantasy modified for Rush.

package anthropic

import (
	"golang.org/x/oauth2"
)

type googleDummyTokenSource struct{}

func (googleDummyTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "dummy-token"}, nil
}
