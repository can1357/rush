// Copyright 2025 Charmbracelet, Inc.
// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.
//
// This is a fork of github.com/charmbracelet/fantasy modified for Rush.

package google

import (
	"cmp"
	"errors"

	"github.com/can1357/rush/genai"
	gai "google.golang.org/genai"
)

func toProviderErr(err error) error {
	var apiErr gai.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	return &genai.ProviderError{
		Message:      apiErr.Message,
		Title:        cmp.Or(genai.ErrorTitleForStatusCode(apiErr.Code), "provider request failed"),
		Cause:        err,
		StatusCode:   apiErr.Code,
		ResponseBody: []byte(apiErr.Message),
	}
}
