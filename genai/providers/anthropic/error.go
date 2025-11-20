// Copyright 2025 Charmbracelet, Inc.
// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.
//
// This is a fork of github.com/charmbracelet/fantasy modified for Rush.

package anthropic

import (
	"cmp"
	"errors"
	"net/http"
	"strings"

	"github.com/can1357/rush/genai"
	"github.com/charmbracelet/anthropic-sdk-go"
)

func toProviderErr(err error) error {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		return &genai.ProviderError{
			Title:           cmp.Or(genai.ErrorTitleForStatusCode(apiErr.StatusCode), "provider request failed"),
			Message:         apiErr.Error(),
			Cause:           apiErr,
			URL:             apiErr.Request.URL.String(),
			StatusCode:      apiErr.StatusCode,
			RequestBody:     apiErr.DumpRequest(true),
			ResponseHeaders: toHeaderMap(apiErr.Response.Header),
			ResponseBody:    apiErr.DumpResponse(true),
		}
	}
	return err
}

func toHeaderMap(in http.Header) (out map[string]string) {
	out = make(map[string]string, len(in))
	for k, v := range in {
		if l := len(v); l > 0 {
			out[k] = v[l-1]
			in[strings.ToLower(k)] = v
		}
	}
	return out
}
