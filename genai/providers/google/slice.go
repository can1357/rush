// Copyright 2025 Charmbracelet, Inc.
// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.
//
// This is a fork of github.com/charmbracelet/fantasy modified for Rush.

package google

func depointerSlice[T any](s []*T) []T {
	result := make([]T, 0, len(s))
	for _, v := range s {
		if v != nil {
			result = append(result, *v)
		}
	}
	return result
}
