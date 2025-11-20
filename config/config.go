package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/can1357/rush/csync"
	"github.com/can1357/rush/env"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/tidwall/sjson"
)

const (
	appName              = "rush"
	defaultDataDirectory = ".rush"
	defaultInitializeAs  = "AGENTS.md"
)

var defaultContextPaths = []string{
	".github/copilot-instructions.md",
	".cursorrules",
	".cursor/rules/",
	"CLAUDE.md",
	"CLAUDE.local.md",
	"GEMINI.md",
	"gemini.md",
	"rush.md",
	"rush.local.md",
	"Rush.md",
	"Rush.local.md",
	"RUSH.md",
	"RUSH.local.md",
	"AGENTS.md",
	"agents.md",
	"Agents.md",
}

type ModelType uint8

func (m ModelType) Name() string {
	switch m {
	case LiteAI:
		return "lite"
	case BalancedAI:
		return "balanced"
	case ProAI:
		return "pro"
	default:
		return "unknown"
	}
}

func (m ModelType) String() string {
	switch m {
	case LiteAI:
		return "lite"
	case BalancedAI:
		return "balanced"
	case ProAI:
		return "pro"
	default:
		return "unknown"
	}
}

func (m ModelType) PlaceholderText() string {
	switch m {
	case LiteAI:
		return "Choose a fast model for simple tasks"
	case BalancedAI:
		return "Choose a balanced model for day to day tasks"
	case ProAI:
		return "Choose a powerful model for complex tasks"
	default:
		panic(fmt.Sprintf("unknown model type: %s", m))
	}
}

func (m ModelType) LabelText() string {
	switch m {
	case LiteAI:
		return "Lite"
	case BalancedAI:
		return "Balanced"
	case ProAI:
		return "Pro"
	default:
		panic(fmt.Sprintf("unknown model type: %s", m))
	}
}

func (m ModelType) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

func (m *ModelType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	switch s {
	case "lite":
		*m = LiteAI
	case "balanced":
		*m = BalancedAI
	case "pro":
		*m = ProAI
	default:
		return fmt.Errorf("unknown model type: %s", s)
	}
	return nil
}

// MarshalText implements encoding.TextMarshaler for map keys.
func (m ModelType) MarshalText() ([]byte, error) {
	return []byte(m.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for map keys.
func (m *ModelType) UnmarshalText(data []byte) error {
	s := string(data)
	switch s {
	case "lite":
		*m = LiteAI
	case "balanced":
		*m = BalancedAI
	case "pro":
		*m = ProAI
	case "inherit":
		*m = BalancedAI
	default:
		return fmt.Errorf("unknown model type: %s", s)
	}
	return nil
}

const (
	LiteAI ModelType = iota
	BalancedAI
	ProAI
)

const (
	AgentMaestro string = "maestro"
	AgentTask    string = "task"
	AgentExplore string = "explore"
)

type ModelSelection struct {
	// The model id as used by the provider API.
	// Required.
	Model string `json:"model" jsonschema:"required,description=The model ID as used by the provider API,example=gpt-4o"`
	// The model provider, same as the key/id used in the providers config.
	// Required.
	Provider string `json:"provider" jsonschema:"required,description=The model provider ID that matches a key in the providers config,example=openai"`

	// Overrides the default model configuration.
	MaxTokens        int64    `json:"max_tokens,omitempty" jsonschema:"description=Maximum number of tokens for model responses,minimum=1,maximum=200000,example=4096"`
	Temperature      *float64 `json:"temperature,omitempty" jsonschema:"description=Sampling temperature,minimum=0,maximum=1,example=0.7"`
	TopP             *float64 `json:"top_p,omitempty" jsonschema:"description=Top-p (nucleus) sampling parameter,minimum=0,maximum=1,example=0.9"`
	TopK             *int64   `json:"top_k,omitempty" jsonschema:"description=Top-k sampling parameter"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty" jsonschema:"description=Frequency penalty to reduce repetition"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty" jsonschema:"description=Presence penalty to increase topic diversity"`

	// Override provider specific options.
	ProviderOptions map[string]any `json:"provider_options,omitempty" jsonschema:"description=Additional provider-specific options for the model"`
}

type ProviderConfig struct {
	// The provider's id.
	ID string `json:"id,omitempty" jsonschema:"description=Unique identifier for the provider,example=openai"`
	// The provider's name, used for display purposes.
	Name string `json:"name,omitempty" jsonschema:"description=Human-readable name for the provider,example=OpenAI"`
	// The provider's API endpoint.
	BaseURL string `json:"base_url,omitempty" jsonschema:"description=Base URL for the provider's API,format=uri,example=https://api.openai.com/v1"`
	// The provider type, e.g. "openai", "anthropic", etc. if empty it defaults to openai.
	Type catwalk.Type `json:"type,omitempty" jsonschema:"description=Provider type that determines the API format,enum=openai,enum=openai-compat,enum=anthropic,enum=gemini,enum=azure,enum=vertexai,default=openai"`
	// The provider's API key.
	APIKey string `json:"api_key,omitempty" jsonschema:"description=API key for authentication with the provider,example=$OPENAI_API_KEY"`
	// Marks the provider as disabled.
	Disable bool `json:"disable,omitempty" jsonschema:"description=Whether this provider is disabled,default=false"`

	// Custom system prompt prefix.
	SystemPromptPrefix string `json:"system_prompt_prefix,omitempty" jsonschema:"description=Custom prefix to add to system prompts for this provider"`

	// Extra headers to send with each request to the provider.
	ExtraHeaders map[string]string `json:"extra_headers,omitempty" jsonschema:"description=Additional HTTP headers to send with requests"`
	// Extra body
	ExtraBody map[string]any `json:"extra_body,omitempty" jsonschema:"description=Additional fields to include in request bodies, only works with openai-compatible providers"`

	ProviderOptions map[string]any `json:"provider_options,omitempty" jsonschema:"description=Additional provider-specific options for this provider"`

	// Used to pass extra parameters to the provider.
	ExtraParams map[string]string `json:"-"`

	// Enable automatic model discovery from provider's /v1/models endpoint (openai-compat only)
	DiscoverModels bool `json:"discover_models,omitempty" jsonschema:"description=Enable automatic model discovery from provider API,default=true"`

	// Default metadata to apply to discovered models (context window, max tokens, etc.)
	DefaultModelMetadata *DefaultModelMetadata `json:"default_model_metadata,omitempty" jsonschema:"description=Default metadata for discovered models"`

	// The provider models
	Models []catwalk.Model `json:"models,omitempty" jsonschema:"description=List of models available from this provider"`
}

// DefaultModelMetadata provides default values for discovered models
type DefaultModelMetadata struct {
	ContextWindow      int64   `json:"context_window,omitempty" jsonschema:"description=Default context window for discovered models,example=200000"`
	DefaultMaxTokens   int64   `json:"default_max_tokens,omitempty" jsonschema:"description=Default max tokens for discovered models,example=8192"`
	CostPer1MIn        float64 `json:"cost_per_1m_in,omitempty" jsonschema:"description=Default input cost per 1M tokens,example=0.5"`
	CostPer1MOut       float64 `json:"cost_per_1m_out,omitempty" jsonschema:"description=Default output cost per 1M tokens,example=1.5"`
	CostPer1MInCached  float64 `json:"cost_per_1m_in_cached,omitempty" jsonschema:"description=Default input cost per 1M tokens for cached requests,example=0.25"`
	CostPer1MOutCached float64 `json:"cost_per_1m_out_cached,omitempty" jsonschema:"description=Default output cost per 1M tokens for cached requests,example=0.75"`
}

type MCPType string

const (
	MCPStdio MCPType = "stdio"
	MCPSSE   MCPType = "sse"
	MCPHttp  MCPType = "http"
)

type MCPConfig struct {
	Command  string            `json:"command,omitempty" jsonschema:"description=Command to execute for stdio MCP servers,example=npx"`
	Env      map[string]string `json:"env,omitempty" jsonschema:"description=Environment variables to set for the MCP server"`
	Args     []string          `json:"args,omitempty" jsonschema:"description=Arguments to pass to the MCP server command"`
	Type     MCPType           `json:"type" jsonschema:"required,description=Type of MCP connection,enum=stdio,enum=sse,enum=http,default=stdio"`
	URL      string            `json:"url,omitempty" jsonschema:"description=URL for HTTP or SSE MCP servers,format=uri,example=http://localhost:3000/mcp"`
	Disabled bool              `json:"disabled,omitempty" jsonschema:"description=Whether this MCP server is disabled,default=false"`
	Timeout  int               `json:"timeout,omitempty" jsonschema:"description=Timeout in seconds for MCP server connections,default=15,example=30,example=60,example=120"`

	// TODO: maybe make it possible to get the value from the env
	Headers map[string]string `json:"headers,omitempty" jsonschema:"description=HTTP headers for HTTP/SSE MCP servers"`
}

type LSPConfig struct {
	Disabled    bool              `json:"disabled,omitempty" jsonschema:"description=Whether this LSP server is disabled,default=false"`
	Command     string            `json:"command,omitempty" jsonschema:"required,description=Command to execute for the LSP server,example=gopls"`
	Args        []string          `json:"args,omitempty" jsonschema:"description=Arguments to pass to the LSP server command"`
	Env         map[string]string `json:"env,omitempty" jsonschema:"description=Environment variables to set to the LSP server command"`
	FileTypes   []string          `json:"filetypes,omitempty" jsonschema:"description=File types this LSP server handles,example=go,example=mod,example=rs,example=c,example=js,example=ts"`
	RootMarkers []string          `json:"root_markers,omitempty" jsonschema:"description=Files or directories that indicate the project root,example=go.mod,example=package.json,example=Cargo.toml"`
	InitOptions map[string]any    `json:"init_options,omitempty" jsonschema:"description=Initialization options passed to the LSP server during initialize request"`
	Options     map[string]any    `json:"options,omitempty" jsonschema:"description=LSP server-specific settings passed during initialization"`
}

type TUIOptions struct {
	CompactMode bool   `json:"compact_mode,omitempty" jsonschema:"description=Enable compact mode for the TUI interface,default=false"`
	DiffMode    string `json:"diff_mode,omitempty" jsonschema:"description=Diff mode for the TUI interface,enum=unified,enum=split"`
	// Here we can add themes later or any TUI related options
	//

	Completions Completions `json:"completions,omitzero" jsonschema:"description=Completions UI options"`
}

// Completions defines options for the completions UI.
type Completions struct {
	MaxDepth *int `json:"max_depth,omitempty" jsonschema:"description=Maximum depth for the ls tool,default=0,example=10"`
	MaxItems *int `json:"max_items,omitempty" jsonschema:"description=Maximum number of items to return for the ls tool,default=1000,example=100"`
}

func (c Completions) Limits() (depth, items int) {
	return ptrValOr(c.MaxDepth, 0), ptrValOr(c.MaxItems, 0)
}

type Permissions struct {
	AllowedTools []string `json:"allowed_tools,omitempty" jsonschema:"description=List of tools that don't require permission prompts,example=bash,example=view"` // Tools that don't require permission prompts
	SkipRequests bool     `json:"-"`                                                                                                                              // Automatically accept all permissions (YOLO mode)
}

type Options struct {
	ContextPaths              []string    `json:"context_paths,omitempty" jsonschema:"description=Paths to files containing context information for the AI,example=.cursorrules,example=RUSH.md"`
	TUI                       *TUIOptions `json:"tui,omitempty" jsonschema:"description=Terminal user interface options"`
	Debug                     bool        `json:"debug,omitempty" jsonschema:"description=Enable debug logging,default=false"`
	DebugLSP                  bool        `json:"debug_lsp,omitempty" jsonschema:"description=Enable debug logging for LSP servers,default=false"`
	DisableAutoSummarize      bool        `json:"disable_auto_summarize,omitempty" jsonschema:"description=Disable automatic conversation summarization,default=false"`
	DataDirectory             string      `json:"data_directory,omitempty" jsonschema:"description=Directory for storing application data (relative to working directory),default=.rush,example=.rush"` // Relative to the cwd
	DisabledTools             []string    `json:"disabled_tools" jsonschema:"description=Tools to disable"`
	DisableProviderAutoUpdate bool        `json:"disable_provider_auto_update,omitempty" jsonschema:"description=Disable providers auto-update,default=false"`
	DisableMetrics            bool        `json:"disable_metrics,omitempty" jsonschema:"description=Disable sending metrics,default=false"`
	InitializeAs              string      `json:"initialize_as,omitempty" jsonschema:"description=Name of the context file to create/update during project initialization,default=AGENTS.md,example=AGENTS.md,example=RUSH.md,example=CLAUDE.md,example=docs/LLMs.md"`
}

type MCPs map[string]MCPConfig

type MCP struct {
	Name string    `json:"name"`
	MCP  MCPConfig `json:"mcp"`
}

func (m MCPs) Sorted() []MCP {
	sorted := make([]MCP, 0, len(m))
	for k, v := range m {
		sorted = append(sorted, MCP{
			Name: k,
			MCP:  v,
		})
	}
	slices.SortFunc(sorted, func(a, b MCP) int {
		return strings.Compare(a.Name, b.Name)
	})
	return sorted
}

type LSPs map[string]LSPConfig

type LSP struct {
	Name string    `json:"name"`
	LSP  LSPConfig `json:"lsp"`
}

func (l LSPs) Sorted() []LSP {
	sorted := make([]LSP, 0, len(l))
	for k, v := range l {
		sorted = append(sorted, LSP{
			Name: k,
			LSP:  v,
		})
	}
	slices.SortFunc(sorted, func(a, b LSP) int {
		return strings.Compare(a.Name, b.Name)
	})
	return sorted
}

func (l LSPConfig) ResolvedEnv() []string {
	return resolveEnvs(l.Env)
}

func (m MCPConfig) ResolvedEnv() []string {
	return resolveEnvs(m.Env)
}

func (m MCPConfig) ResolvedHeaders() map[string]string {
	resolver := NewShellVariableResolver(env.New())
	for e, v := range m.Headers {
		var err error
		m.Headers[e], err = resolver.ResolveValue(v)
		if err != nil {
			slog.Error("error resolving header variable", "error", err, "variable", e, "value", v)
			continue
		}
	}
	return m.Headers
}

type Agent struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// This is the id of the system prompt used by the agent
	Disabled bool `json:"disabled,omitempty"`

	Model ModelType `json:"model" jsonschema:"required,description=The model type to use for this agent,enum=,enum=small,enum=extra,default=large"`

	// The available tools for the agent
	//  if this is nil, all tools are available
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// this tells us which MCPs are available for this agent
	//  if this is empty all mcps are available
	//  the string array is the list of tools from the AllowedMCP the agent has available
	//  if the string array is nil, all tools from the AllowedMCP are available
	AllowedMCP map[string][]string `json:"allowed_mcp,omitempty"`

	// Overrides the context paths for this agent
	ContextPaths []string `json:"context_paths,omitempty"`

	// Maximum number of turns (request/response cycles) for this agent
	// Zero means use default (50 for maestro, 20 for task/explore)
	MaxTurns int `json:"max_turns,omitempty"`
}

type Tools struct {
	Ls ToolLs `json:"ls,omitzero"`
}

type ToolLs struct {
	MaxDepth *int `json:"max_depth,omitempty" jsonschema:"description=Maximum depth for the ls tool,default=0,example=10"`
	MaxItems *int `json:"max_items,omitempty" jsonschema:"description=Maximum number of items to return for the ls tool,default=1000,example=100"`
}

func (t ToolLs) Limits() (depth, items int) {
	return ptrValOr(t.MaxDepth, 0), ptrValOr(t.MaxItems, 0)
}

// Config holds the configuration for rush.
type Config struct {
	Schema string `json:"$schema,omitempty"`

	// We currently only support large/small as values here.
	Models map[ModelType]ModelSelection `json:"models,omitempty" jsonschema:"description=Model configurations for different model types,example={\"large\":{\"model\":\"gpt-4o\",\"provider\":\"openai\"}}"`
	// Recently used models stored in the data directory config.
	RecentModels map[ModelType][]ModelSelection `json:"recent_models,omitempty" jsonschema:"description=Recently used models sorted by most recent first"`

	// The providers that are configured
	Providers *csync.Map[string, ProviderConfig] `json:"providers,omitempty" jsonschema:"description=AI provider configurations"`

	MCP MCPs `json:"mcp,omitempty" jsonschema:"description=Model Context Protocol server configurations"`

	LSP LSPs `json:"lsp,omitempty" jsonschema:"description=Language Server Protocol configurations"`

	Options *Options `json:"options,omitempty" jsonschema:"description=General application options"`

	Permissions *Permissions `json:"permissions,omitempty" jsonschema:"description=Permission settings for tool usage"`

	Tools Tools `json:"tools,omitzero" jsonschema:"description=Tool configurations"`

	Agents map[string]Agent `json:"-"`

	// Internal
	workingDir string `json:"-"`
	// TODO: find a better way to do this this should probably not be part of the config
	resolver       VariableResolver
	dataConfigDir  string             `json:"-"`
	knownProviders []catwalk.Provider `json:"-"`
}

func (c *Config) WorkingDir() string {
	return c.workingDir
}

func (c *Config) EnabledProviders() []ProviderConfig {
	var enabled []ProviderConfig
	for p := range c.Providers.Seq() {
		if !p.Disable {
			enabled = append(enabled, p)
		}
	}
	return enabled
}

// IsConfigured  return true if at least one provider is configured
func (c *Config) IsConfigured() bool {
	return len(c.EnabledProviders()) > 0
}

func (c *Config) GetModel(provider, model string) *catwalk.Model {
	if providerConfig, ok := c.Providers.Get(provider); ok {
		for _, m := range providerConfig.Models {
			if m.ID == model {
				return &m
			}
		}
	}
	return nil
}

func (c *Config) GetProviderForModel(modelType ModelType) *ProviderConfig {
	model, ok := c.Models[modelType]
	if !ok {
		return nil
	}
	if providerConfig, ok := c.Providers.Get(model.Provider); ok {
		return &providerConfig
	}
	return nil
}

func (c *Config) GetModelByType(modelType ModelType) *catwalk.Model {
	model, ok := c.Models[modelType]
	if !ok {
		return nil
	}
	return c.GetModel(model.Provider, model.Model)
}

func (c *Config) ModelByClass(modelClass ModelType) *catwalk.Model {
	selectedModel, ok := c.Models[modelClass]
	if !ok {
		return nil
	}
	return c.GetModel(selectedModel.Provider, selectedModel.Model)
}

func (c *Config) SetCompactMode(enabled bool) error {
	if c.Options == nil {
		c.Options = &Options{}
	}
	c.Options.TUI.CompactMode = enabled
	return c.SetConfigField("options.tui.compact_mode", enabled)
}

func (c *Config) Resolve(key string) (string, error) {
	if c.resolver == nil {
		return "", fmt.Errorf("no variable resolver configured")
	}
	return c.resolver.ResolveValue(key)
}

func (c *Config) UpdatePreferredModel(modelType ModelType, model ModelSelection) error {
	c.Models[modelType] = model
	if err := c.SetConfigField(fmt.Sprintf("models.%s", modelType), model); err != nil {
		return fmt.Errorf("failed to update preferred model: %w", err)
	}
	if err := c.recordRecentModel(modelType, model); err != nil {
		return err
	}
	return nil
}

func (c *Config) SetConfigField(key string, value any) error {
	// read the data
	data, err := os.ReadFile(c.dataConfigDir)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("{}")
		} else {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	newValue, err := sjson.Set(string(data), key, value)
	if err != nil {
		return fmt.Errorf("failed to set config field %s: %w", key, err)
	}
	if err := os.WriteFile(c.dataConfigDir, []byte(newValue), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

func (c *Config) SetProviderAPIKey(providerID, apiKey string) error {
	// First save to the config file
	err := c.SetConfigField("providers."+providerID+".api_key", apiKey)
	if err != nil {
		return fmt.Errorf("failed to save API key to config file: %w", err)
	}

	providerConfig, exists := c.Providers.Get(providerID)
	if exists {
		providerConfig.APIKey = apiKey
		c.Providers.Set(providerID, providerConfig)
		return nil
	}

	var foundProvider *catwalk.Provider
	for _, p := range c.knownProviders {
		if string(p.ID) == providerID {
			foundProvider = &p
			break
		}
	}

	if foundProvider != nil {
		// Create new provider config based on known provider
		providerConfig = ProviderConfig{
			ID:           providerID,
			Name:         foundProvider.Name,
			BaseURL:      foundProvider.APIEndpoint,
			Type:         foundProvider.Type,
			APIKey:       apiKey,
			Disable:      false,
			ExtraHeaders: make(map[string]string),
			ExtraParams:  make(map[string]string),
			Models:       foundProvider.Models,
		}
	} else {
		return fmt.Errorf("provider with ID %s not found in known providers", providerID)
	}
	// Store the updated provider config
	c.Providers.Set(providerID, providerConfig)
	return nil
}

const maxRecentModelsPerType = 5

func (c *Config) recordRecentModel(modelType ModelType, model ModelSelection) error {
	if model.Provider == "" || model.Model == "" {
		return nil
	}

	if c.RecentModels == nil {
		c.RecentModels = make(map[ModelType][]ModelSelection)
	}

	eq := func(a, b ModelSelection) bool {
		return a.Provider == b.Provider && a.Model == b.Model
	}

	entry := ModelSelection{
		Provider: model.Provider,
		Model:    model.Model,
	}

	current := c.RecentModels[modelType]
	withoutCurrent := slices.DeleteFunc(slices.Clone(current), func(existing ModelSelection) bool {
		return eq(existing, entry)
	})

	updated := append([]ModelSelection{entry}, withoutCurrent...)
	if len(updated) > maxRecentModelsPerType {
		updated = updated[:maxRecentModelsPerType]
	}

	if slices.EqualFunc(current, updated, eq) {
		return nil
	}

	c.RecentModels[modelType] = updated

	if err := c.SetConfigField(fmt.Sprintf("recent_models.%s", modelType), updated); err != nil {
		return fmt.Errorf("failed to persist recent models: %w", err)
	}

	return nil
}

func allToolNames() []string {
	return []string{
		"agent",
		"ask_user_question",
		"bash",
		"job_output",
		"job_kill",
		"download",
		"edit",
		"exit_plan_mode",
		"multiedit",
		"lsp_diagnostics",
		"lsp_references",
		"fetch",
		"agentic_fetch",
		"glob",
		"grep",
		"ls",
		"sourcegraph",
		"todo_write",
		"view",
		"write",
	}
}

func resolveAllowedTools(allTools []string, disabledTools []string) []string {
	if disabledTools == nil {
		return allTools
	}
	// filter out disabled tools (exclude mode)
	return filterSlice(allTools, disabledTools, false)
}

func resolveReadOnlyTools(tools []string) []string {
	readOnlyTools := []string{"glob", "grep", "ls", "sourcegraph", "view"}
	// filter to only include tools that are in allowedtools (include mode)
	return filterSlice(tools, readOnlyTools, true)
}

func filterSlice(data []string, mask []string, include bool) []string {
	filtered := []string{}
	for _, s := range data {
		// if include is true, we include items that ARE in the mask
		// if include is false, we include items that are NOT in the mask
		if include == slices.Contains(mask, s) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func (c *Config) SetupAgents() {
	allowedTools := resolveAllowedTools(allToolNames(), c.Options.DisabledTools)

	agents := map[string]Agent{
		AgentMaestro: {
			ID:           AgentMaestro,
			Name:         "Maestro",
			Description:  "Main agent for coding tasks, software engineering, and general assistance.",
			Model:        BalancedAI,
			ContextPaths: c.Options.ContextPaths,
			AllowedTools: allowedTools,
			MaxTurns:     50, // Main agent gets more turns for complex tasks
		},

		AgentTask: {
			ID:           AgentTask,
			Name:         "Task",
			Description:  "General-purpose agent for researching complex questions, searching for code, and executing multi-step tasks. When you are searching for a keyword or file and are not confident that you will find the right match in the first few tries use this agent to perform the search for you.",
			Model:        BalancedAI,
			ContextPaths: c.Options.ContextPaths,
			AllowedTools: resolveReadOnlyTools(allowedTools),
			// NO MCPs or LSPs by default
			AllowedMCP: map[string][]string{},
			MaxTurns:   20, // Task agent gets fewer turns for focused work
		},

		AgentExplore: {
			ID:           AgentExplore,
			Name:         "Explore",
			Description:  "Fast agent specialized for exploring codebases. Use this when you need to quickly find files by patterns (eg. \"src/components/**/*.tsx\"), search code for keywords (eg. \"API endpoints\"), or answer questions about the codebase (eg. \"how do API endpoints work?\").",
			Model:        LiteAI,
			ContextPaths: c.Options.ContextPaths,
			AllowedTools: resolveReadOnlyTools(allowedTools),
			// NO MCPs or LSPs by default
			AllowedMCP: map[string][]string{},
			MaxTurns:   20, // Explore agent gets fewer turns for quick searches
		},
	}
	c.Agents = agents
}

func (c *Config) Resolver() VariableResolver {
	return c.resolver
}

func (c *ProviderConfig) TestConnection(resolver VariableResolver) error {
	testURL := ""
	headers := make(map[string]string)
	apiKey, _ := resolver.ResolveValue(c.APIKey)
	switch c.Type {
	case catwalk.TypeOpenAI, catwalk.TypeOpenAICompat, catwalk.TypeOpenRouter:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		if c.ID == string(catwalk.InferenceProviderOpenRouter) {
			testURL = baseURL + "/credits"
		} else {
			testURL = baseURL + "/models"
		}
		headers["Authorization"] = "Bearer " + apiKey
	case catwalk.TypeAnthropic:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		testURL = baseURL + "/models"
		headers["x-api-key"] = apiKey
		headers["anthropic-version"] = "2023-06-01"
	case catwalk.TypeGoogle:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com"
		}
		testURL = baseURL + "/v1beta/models?key=" + url.QueryEscape(apiKey)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for provider %s: %w", c.ID, err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for k, v := range c.ExtraHeaders {
		req.Header.Set(k, v)
	}
	b, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create request for provider %s: %w", c.ID, err)
	}
	if c.ID == string(catwalk.InferenceProviderZAI) {
		if b.StatusCode == http.StatusUnauthorized {
			// for z.ai just check if the http response is not 401
			return fmt.Errorf("failed to connect to provider %s: %s", c.ID, b.Status)
		}
	} else {
		if b.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to connect to provider %s: %s", c.ID, b.Status)
		}
	}
	_ = b.Body.Close()
	return nil
}

func resolveEnvs(envs map[string]string) []string {
	resolver := NewShellVariableResolver(env.New())
	for e, v := range envs {
		var err error
		envs[e], err = resolver.ResolveValue(v)
		if err != nil {
			slog.Error("error resolving environment variable", "error", err, "variable", e, "value", v)
			continue
		}
	}

	res := make([]string, 0, len(envs))
	for k, v := range envs {
		res = append(res, fmt.Sprintf("%s=%s", k, v))
	}
	return res
}

func ptrValOr[T any](t *T, el T) T {
	if t == nil {
		return el
	}
	return *t
}
