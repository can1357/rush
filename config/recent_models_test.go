package config

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// readConfigJSON reads and unmarshals the JSON config file at path.
func readConfigJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	baseDir := filepath.Dir(path)
	fileName := filepath.Base(path)
	b, err := fs.ReadFile(os.DirFS(baseDir), fileName)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(b, &out))
	return out
}

// readRecentModels reads the recent_models section from the config file.
func readRecentModels(t *testing.T, path string) map[string]any {
	t.Helper()
	out := readConfigJSON(t, path)
	rm, ok := out["recent_models"].(map[string]any)
	require.True(t, ok)
	return rm
}

func TestRecordRecentModel_AddsAndPersists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	cfg.dataConfigDir = filepath.Join(dir, "config.json")

	err := cfg.recordRecentModel(BalancedAI, ModelSelection{Provider: "openai", Model: "gpt-4o"})
	require.NoError(t, err)

	// in-memory state
	require.Len(t, cfg.RecentModels[BalancedAI], 1)
	require.Equal(t, "openai", cfg.RecentModels[BalancedAI][0].Provider)
	require.Equal(t, "gpt-4o", cfg.RecentModels[BalancedAI][0].Model)

	// persisted state
	rm := readRecentModels(t, cfg.dataConfigDir)
	large, ok := rm[BalancedAI.String()].([]any)
	require.True(t, ok)
	require.Len(t, large, 1)
	item, ok := large[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "openai", item["provider"])
	require.Equal(t, "gpt-4o", item["model"])
}

func TestRecordRecentModel_DedupeAndMoveToFront(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	cfg.dataConfigDir = filepath.Join(dir, "config.json")

	// Add two entries
	require.NoError(t, cfg.recordRecentModel(BalancedAI, ModelSelection{Provider: "openai", Model: "gpt-4o"}))
	require.NoError(t, cfg.recordRecentModel(BalancedAI, ModelSelection{Provider: "anthropic", Model: "claude"}))
	// Re-add first; should move to front and not duplicate
	require.NoError(t, cfg.recordRecentModel(BalancedAI, ModelSelection{Provider: "openai", Model: "gpt-4o"}))

	got := cfg.RecentModels[BalancedAI]
	require.Len(t, got, 2)
	require.Equal(t, ModelSelection{Provider: "openai", Model: "gpt-4o"}, got[0])
	require.Equal(t, ModelSelection{Provider: "anthropic", Model: "claude"}, got[1])
}

func TestRecordRecentModel_TrimsToMax(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	cfg.dataConfigDir = filepath.Join(dir, "config.json")

	// Insert 6 unique models; max is 5
	entries := []ModelSelection{
		{Provider: "p1", Model: "m1"},
		{Provider: "p2", Model: "m2"},
		{Provider: "p3", Model: "m3"},
		{Provider: "p4", Model: "m4"},
		{Provider: "p5", Model: "m5"},
		{Provider: "p6", Model: "m6"},
	}
	for _, e := range entries {
		require.NoError(t, cfg.recordRecentModel(BalancedAI, e))
	}

	// in-memory state
	got := cfg.RecentModels[BalancedAI]
	require.Len(t, got, 5)
	// Newest first, capped at 5: p6..p2
	require.Equal(t, ModelSelection{Provider: "p6", Model: "m6"}, got[0])
	require.Equal(t, ModelSelection{Provider: "p5", Model: "m5"}, got[1])
	require.Equal(t, ModelSelection{Provider: "p4", Model: "m4"}, got[2])
	require.Equal(t, ModelSelection{Provider: "p3", Model: "m3"}, got[3])
	require.Equal(t, ModelSelection{Provider: "p2", Model: "m2"}, got[4])

	// persisted state: verify trimmed to 5 and newest-first order
	rm := readRecentModels(t, cfg.dataConfigDir)
	large, ok := rm[BalancedAI.String()].([]any)
	require.True(t, ok)
	require.Len(t, large, 5)
	// Build provider:model IDs and verify order
	var ids []string
	for _, v := range large {
		m := v.(map[string]any)
		ids = append(ids, m["provider"].(string)+":"+m["model"].(string))
	}
	require.Equal(t, []string{"p6:m6", "p5:m5", "p4:m4", "p3:m3", "p2:m2"}, ids)
}

func TestRecordRecentModel_SkipsEmptyValues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	cfg.dataConfigDir = filepath.Join(dir, "config.json")

	// Missing provider
	require.NoError(t, cfg.recordRecentModel(BalancedAI, ModelSelection{Provider: "", Model: "m"}))
	// Missing model
	require.NoError(t, cfg.recordRecentModel(BalancedAI, ModelSelection{Provider: "p", Model: ""}))

	_, ok := cfg.RecentModels[BalancedAI]
	// Map may be initialized, but should have no entries
	if ok {
		require.Len(t, cfg.RecentModels[BalancedAI], 0)
	}
	// No file should be written (stat via fs.FS)
	baseDir := filepath.Dir(cfg.dataConfigDir)
	fileName := filepath.Base(cfg.dataConfigDir)
	_, err := fs.Stat(os.DirFS(baseDir), fileName)
	require.True(t, os.IsNotExist(err))
}

func TestRecordRecentModel_NoPersistOnNoop(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	cfg.dataConfigDir = filepath.Join(dir, "config.json")

	entry := ModelSelection{Provider: "openai", Model: "gpt-4o"}
	require.NoError(t, cfg.recordRecentModel(BalancedAI, entry))

	baseDir := filepath.Dir(cfg.dataConfigDir)
	fileName := filepath.Base(cfg.dataConfigDir)
	before, err := fs.ReadFile(os.DirFS(baseDir), fileName)
	require.NoError(t, err)

	// Get file ModTime to verify no write occurs
	stBefore, err := fs.Stat(os.DirFS(baseDir), fileName)
	require.NoError(t, err)
	beforeMod := stBefore.ModTime()

	// Re-record same entry should be a no-op (no write)
	require.NoError(t, cfg.recordRecentModel(BalancedAI, entry))

	after, err := fs.ReadFile(os.DirFS(baseDir), fileName)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))

	// Verify ModTime unchanged to ensure truly no write occurred
	stAfter, err := fs.Stat(os.DirFS(baseDir), fileName)
	require.NoError(t, err)
	require.True(t, stAfter.ModTime().Equal(beforeMod), "file ModTime should not change on noop")
}

func TestUpdatePreferredModel_UpdatesRecents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	cfg.dataConfigDir = filepath.Join(dir, "config.json")

	sel := ModelSelection{Provider: "openai", Model: "gpt-4o"}
	require.NoError(t, cfg.UpdatePreferredModel(LiteAI, sel))

	// in-memory
	require.Equal(t, sel, cfg.Models[LiteAI])
	require.Len(t, cfg.RecentModels[LiteAI], 1)

	// persisted (read via fs.FS)
	rm := readRecentModels(t, cfg.dataConfigDir)
	small, ok := rm[LiteAI.String()].([]any)
	require.True(t, ok)
	require.Len(t, small, 1)
}

func TestRecordRecentModel_TypeIsolation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	cfg.dataConfigDir = filepath.Join(dir, "config.json")

	// Add models to both large and small types
	largeModel := ModelSelection{Provider: "openai", Model: "gpt-4o"}
	smallModel := ModelSelection{Provider: "anthropic", Model: "claude"}

	require.NoError(t, cfg.recordRecentModel(BalancedAI, largeModel))
	require.NoError(t, cfg.recordRecentModel(LiteAI, smallModel))

	// in-memory: verify types maintain separate histories
	require.Len(t, cfg.RecentModels[BalancedAI], 1)
	require.Len(t, cfg.RecentModels[LiteAI], 1)
	require.Equal(t, largeModel, cfg.RecentModels[BalancedAI][0])
	require.Equal(t, smallModel, cfg.RecentModels[LiteAI][0])

	// Add another to large, verify small unchanged
	anotherLarge := ModelSelection{Provider: "google", Model: "gemini"}
	require.NoError(t, cfg.recordRecentModel(BalancedAI, anotherLarge))

	require.Len(t, cfg.RecentModels[BalancedAI], 2)
	require.Len(t, cfg.RecentModels[LiteAI], 1)
	require.Equal(t, smallModel, cfg.RecentModels[LiteAI][0])

	// persisted state: verify both types exist with correct lengths and contents
	rm := readRecentModels(t, cfg.dataConfigDir)

	large, ok := rm[BalancedAI.String()].([]any)
	require.True(t, ok)
	require.Len(t, large, 2)
	// Verify newest first for large type
	require.Equal(t, "google", large[0].(map[string]any)["provider"])
	require.Equal(t, "gemini", large[0].(map[string]any)["model"])
	require.Equal(t, "openai", large[1].(map[string]any)["provider"])
	require.Equal(t, "gpt-4o", large[1].(map[string]any)["model"])

	small, ok := rm[LiteAI.String()].([]any)
	require.True(t, ok)
	require.Len(t, small, 1)
	require.Equal(t, "anthropic", small[0].(map[string]any)["provider"])
	require.Equal(t, "claude", small[0].(map[string]any)["model"])
}
