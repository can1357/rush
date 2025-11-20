package agent

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/can1357/rush/agent/prompt"
	"github.com/can1357/rush/agent/reminder"
	"github.com/can1357/rush/agent/tools"
	"github.com/can1357/rush/config"
	"github.com/can1357/rush/csync"
	"github.com/can1357/rush/genai"
	"github.com/can1357/rush/history"
	"github.com/can1357/rush/log"
	"github.com/can1357/rush/lsp"
	"github.com/can1357/rush/message"
	"github.com/can1357/rush/permission"
	"github.com/can1357/rush/question"
	"github.com/can1357/rush/session"
	"github.com/can1357/rush/todo"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"golang.org/x/sync/errgroup"

	"github.com/can1357/rush/genai/providers/anthropic"
	"github.com/can1357/rush/genai/providers/azure"
	"github.com/can1357/rush/genai/providers/bedrock"
	"github.com/can1357/rush/genai/providers/google"
	"github.com/can1357/rush/genai/providers/openai"
	"github.com/can1357/rush/genai/providers/openaicompat"
	"github.com/can1357/rush/genai/providers/openrouter"
	openaisdk "github.com/openai/openai-go/v2/option"
	"github.com/qjebbs/go-jsons"
)

type Coordinator interface {
	// INFO: (kujtim) this is not used yet we will use this when we have multiple agents
	// SetMainAgent(string)
	Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*genai.AgentResult, error)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	QueuedPrompts(sessionID string) int
	ClearQueue(sessionID string)
	Summarize(context.Context, string) error
	Model() Model
	UpdateMaestroModel(ctx context.Context, model config.ModelSelection) error
}

type coordinator struct {
	cfg         *config.Config
	sessions    session.Service
	messages    message.Service
	permissions permission.Service
	history     history.Service
	todos       todo.Service
	questions   question.Service
	lspClients  *csync.Map[string, *lsp.Client]
	modelCache  *csync.Map[string, genai.LanguageModel]

	currentAgent SessionAgent
	agents       map[string]SessionAgent

	readyWg errgroup.Group
}

func NewCoordinator(
	ctx context.Context,
	cfg *config.Config,
	sessions session.Service,
	messages message.Service,
	permissions permission.Service,
	history history.Service,
	todos todo.Service,
	questions question.Service,
	lspClients *csync.Map[string, *lsp.Client],
) (Coordinator, error) {
	c := &coordinator{
		cfg:         cfg,
		sessions:    sessions,
		messages:    messages,
		permissions: permissions,
		history:     history,
		todos:       todos,
		questions:   questions,
		lspClients:  lspClients,
		agents:      make(map[string]SessionAgent),
	}

	agentCfg, ok := cfg.Agents[config.AgentMaestro]
	if !ok {
		return nil, errors.New("maestro agent not configured")
	}

	// TODO: make this dynamic when we support multiple agents
	prompt, err := maestroPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}

	agent, err := c.buildAgent(ctx, prompt, agentCfg)
	if err != nil {
		return nil, err
	}
	c.currentAgent = agent
	c.agents[config.AgentMaestro] = agent
	return c, nil
}

// Run implements Coordinator.
func (c *coordinator) Run(ctx context.Context, sessionID string, prompt string, attachments ...message.Attachment) (*genai.AgentResult, error) {
	if err := c.readyWg.Wait(); err != nil {
		return nil, err
	}

	model := c.currentAgent.Model()
	maxTokens := model.Props.DefaultMaxTokens
	if model.Selection.MaxTokens != 0 {
		maxTokens = model.Selection.MaxTokens
	}

	if !model.Props.SupportsImages && attachments != nil {
		attachments = nil
	}

	providerCfg, ok := c.cfg.Providers.Get(model.Selection.Provider)
	if !ok {
		return nil, errors.New("model provider not configured")
	}

	mergedOptions, temp, topP, topK, freqPenalty, presPenalty := mergeCallOptions(model, providerCfg)

	return c.currentAgent.Run(ctx, SessionAgentCall{
		SessionID:        sessionID,
		Prompt:           prompt,
		Attachments:      attachments,
		MaxOutputTokens:  maxTokens,
		ProviderOptions:  mergedOptions,
		Temperature:      temp,
		TopP:             topP,
		TopK:             topK,
		FrequencyPenalty: freqPenalty,
		PresencePenalty:  presPenalty,
	})
}

func getProviderOptions(model Model, providerCfg config.ProviderConfig) genai.ProviderOptions {
	options := genai.ProviderOptions{}

	cfgOpts := []byte("{}")
	providerCfgOpts := []byte("{}")
	catwalkOpts := []byte("{}")

	if model.Selection.ProviderOptions != nil {
		data, err := json.Marshal(model.Selection.ProviderOptions)
		if err == nil {
			cfgOpts = data
		}
	}

	if providerCfg.ProviderOptions != nil {
		data, err := json.Marshal(providerCfg.ProviderOptions)
		if err == nil {
			providerCfgOpts = data
		}
	}

	if model.Props.Options.ProviderOptions != nil {
		data, err := json.Marshal(model.Props.Options.ProviderOptions)
		if err == nil {
			catwalkOpts = data
		}
	}

	readers := []io.Reader{
		bytes.NewReader(catwalkOpts),
		bytes.NewReader(providerCfgOpts),
		bytes.NewReader(cfgOpts),
	}

	got, err := jsons.Merge(readers)
	if err != nil {
		slog.Error("Could not merge call config", "err", err)
		return options
	}

	mergedOptions := make(map[string]any)

	err = json.Unmarshal([]byte(got), &mergedOptions)
	if err != nil {
		slog.Error("Could not create config for call", "err", err)
		return options
	}

	switch providerCfg.Type {
	case openai.Name, azure.Name:
		_, hasReasoningEffort := mergedOptions["reasoning_effort"]
		if !hasReasoningEffort && model.Selection.ReasoningEffort != "" {
			mergedOptions["reasoning_effort"] = model.Selection.ReasoningEffort
		}
		if openai.IsResponsesModel(model.Props.ID) {
			if openai.IsResponsesReasoningModel(model.Props.ID) {
				mergedOptions["reasoning_summary"] = "auto"
				mergedOptions["include"] = []openai.IncludeType{openai.IncludeReasoningEncryptedContent}
			}
			parsed, err := openai.ParseResponsesOptions(mergedOptions)
			if err == nil {
				options[openai.Name] = parsed
			}
		} else {
			parsed, err := openai.ParseOptions(mergedOptions)
			if err == nil {
				options[openai.Name] = parsed
			}
		}
	case anthropic.Name:
		_, hasThink := mergedOptions["thinking"]
		if !hasThink && model.Selection.Think {
			mergedOptions["thinking"] = map[string]any{
				// TODO: kujtim see if we need to make this dynamic
				"budget_tokens": 2000,
			}
		}
		parsed, err := anthropic.ParseOptions(mergedOptions)
		if err == nil {
			options[anthropic.Name] = parsed
		}

	case openrouter.Name:
		_, hasReasoning := mergedOptions["reasoning"]
		if !hasReasoning && model.Selection.ReasoningEffort != "" {
			mergedOptions["reasoning"] = map[string]any{
				"enabled": true,
				"effort":  model.Selection.ReasoningEffort,
			}
		}
		parsed, err := openrouter.ParseOptions(mergedOptions)
		if err == nil {
			options[openrouter.Name] = parsed
		}
	case google.Name:
		_, hasReasoning := mergedOptions["thinking_config"]
		if !hasReasoning {
			mergedOptions["thinking_config"] = map[string]any{
				"thinking_budget":  2000,
				"include_thoughts": true,
			}
		}
		parsed, err := google.ParseOptions(mergedOptions)
		if err == nil {
			options[google.Name] = parsed
		}
	case openaicompat.Name:
		_, hasReasoningEffort := mergedOptions["reasoning_effort"]
		if !hasReasoningEffort && model.Selection.ReasoningEffort != "" {
			mergedOptions["reasoning_effort"] = model.Selection.ReasoningEffort
		}
		parsed, err := openaicompat.ParseOptions(mergedOptions)
		if err == nil {
			options[openaicompat.Name] = parsed
		}
	}

	return options
}

func mergeCallOptions(model Model, cfg config.ProviderConfig) (genai.ProviderOptions, *float64, *float64, *int64, *float64, *float64) {
	modelOptions := getProviderOptions(model, cfg)
	temp := cmp.Or(model.Selection.Temperature, model.Props.Options.Temperature)
	topP := cmp.Or(model.Selection.TopP, model.Props.Options.TopP)
	topK := cmp.Or(model.Selection.TopK, model.Props.Options.TopK)
	freqPenalty := cmp.Or(model.Selection.FrequencyPenalty, model.Props.Options.FrequencyPenalty)
	presPenalty := cmp.Or(model.Selection.PresencePenalty, model.Props.Options.PresencePenalty)
	return modelOptions, temp, topP, topK, freqPenalty, presPenalty
}

func (c *coordinator) buildAgent(ctx context.Context, prompt *prompt.Prompt, agent config.Agent) (SessionAgent, error) {
	model := c.Model()
	systemPrompt, err := prompt.Build(ctx, model.Model.Provider(), model.Model.Model(), *c.cfg)
	if err != nil {
		return nil, err
	}

	smallModel, err := c.buildAgentModel(ctx, c.cfg.Models[config.SmallAI])
	if err != nil {
		return nil, fmt.Errorf("error building small model: %w", err)
	}

	modelProviderCfg, _ := c.cfg.Providers.Get(model.Selection.Provider)
	reminderService := reminder.NewService(c.todos, c.sessions)
	result := NewSessionAgent(SessionAgentOptions{
		Model:                model,
		SmallModel:           smallModel,
		SystemPromptPrefix:   modelProviderCfg.SystemPromptPrefix,
		SystemPrompt:         systemPrompt,
		DisableAutoSummarize: c.cfg.Options.DisableAutoSummarize,
		IsYolo:               c.permissions.SkipRequests(),
		Sessions:             c.sessions,
		Messages:             c.messages,
		Reminders:            reminderService,
		Tools:                nil,
	})
	c.readyWg.Go(func() error {
		tools, err := c.buildTools(ctx, agent)
		if err != nil {
			return err
		}
		result.SetTools(tools)
		return nil
	})

	return result, nil
}

func (c *coordinator) buildTools(ctx context.Context, agent config.Agent) ([]genai.AgentTool, error) {
	var allTools []genai.AgentTool
	if slices.Contains(agent.AllowedTools, AgentToolName) {
		// Generate dynamic description based on available agents
		description := generateAgentToolDescription(c.cfg.Agents)
		agentTool, err := c.agentTool(ctx, description)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, agentTool)
	}

	if slices.Contains(agent.AllowedTools, tools.AgenticFetchToolName) {
		agenticFetchTool, err := c.agenticFetchTool(ctx, nil)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, agenticFetchTool)
	}

	allTools = append(allTools,
		tools.NewAskUserQuestionTool(c.questions),
		tools.NewBashTool(c.permissions, c.cfg.WorkingDir()),
		tools.NewJobOutputTool(),
		tools.NewJobKillTool(),
		tools.NewDownloadTool(c.permissions, c.cfg.WorkingDir(), nil),
		tools.NewEditTool(c.lspClients, c.permissions, c.history, c.cfg.WorkingDir()),
		tools.NewMultiEditTool(c.lspClients, c.permissions, c.history, c.cfg.WorkingDir()),
		tools.NewExitPlanModeTool(c.sessions),
		tools.NewFetchTool(c.permissions, c.cfg.WorkingDir(), nil),
		tools.NewGlobTool(c.cfg.WorkingDir()),
		tools.NewGrepTool(c.cfg.WorkingDir()),
		tools.NewLsTool(c.permissions, c.cfg.WorkingDir(), c.cfg.Tools.Ls),
		tools.NewSourcegraphTool(nil),
		tools.NewTodoTool(c.todos, c.sessions),
		tools.NewViewTool(c.lspClients, c.permissions, c.cfg.WorkingDir()),
		tools.NewWriteTool(c.lspClients, c.permissions, c.history, c.cfg.WorkingDir()),
	)

	if len(c.cfg.LSP) > 0 {
		allTools = append(allTools, tools.NewDiagnosticsTool(c.lspClients), tools.NewReferencesTool(c.lspClients))
	}

	var filteredTools []genai.AgentTool
	for _, tool := range allTools {
		if slices.Contains(agent.AllowedTools, tool.Info().Name) {
			filteredTools = append(filteredTools, tool)
		}
	}

	for _, tool := range tools.GetMCPTools(c.permissions, c.cfg.WorkingDir()) {
		if agent.AllowedMCP == nil {
			// No MCP restrictions
			filteredTools = append(filteredTools, tool)
			continue
		}
		if len(agent.AllowedMCP) == 0 {
			// No MCPs allowed
			slog.Debug("no MCPs allowed", "tool", tool.Name(), "agent", agent.Name)
			break
		}

		for mcp, tools := range agent.AllowedMCP {
			if mcp != tool.MCP() {
				continue
			}
			if len(tools) == 0 || slices.Contains(tools, tool.MCPToolName()) {
				filteredTools = append(filteredTools, tool)
			}
		}
		slog.Debug("MCP not allowed", "tool", tool.Name(), "agent", agent.Name)
	}
	slices.SortFunc(filteredTools, func(a, b genai.AgentTool) int {
		return strings.Compare(a.Info().Name, b.Info().Name)
	})
	return filteredTools, nil
}

func (c *coordinator) buildAgentModel(ctx context.Context, sel config.ModelSelection) (Model, error) {
	providerCfg, ok := c.cfg.Providers.Get(sel.Provider)
	if !ok {
		return Model{}, errors.New("model provider not configured")
	}

	provider, err := c.buildProvider(providerCfg, sel)
	if err != nil {
		return Model{}, err
	}

	var props *catwalk.Model

	for i := range providerCfg.Models {
		m := &providerCfg.Models[i]
		if m.ID == sel.Model {
			props = m
			break
		}
	}

	if props == nil {
		return Model{}, fmt.Errorf("model not found in provider config: %s", sel.Model)
	}

	model, err := provider.LanguageModel(ctx, sel.Model)
	if err != nil {
		return Model{}, err
	}

	return Model{Model: model, Props: props, Selection: sel}, nil
}

func (c *coordinator) buildAnthropicProvider(baseURL, apiKey string, headers map[string]string) (genai.Provider, error) {
	hasBearerAuth := false
	for key := range headers {
		if strings.ToLower(key) == "authorization" {
			hasBearerAuth = true
			break
		}
	}

	isBearerToken := strings.HasPrefix(apiKey, "Bearer ")

	var opts []anthropic.Option
	if apiKey != "" && !hasBearerAuth {
		if isBearerToken {
			slog.Debug("API key starts with 'Bearer ', using as Authorization header")
			headers["Authorization"] = apiKey
			apiKey = "" // clear apiKey to avoid using X-Api-Key header
		}
	}

	if apiKey != "" {
		// Use standard X-Api-Key header
		opts = append(opts, anthropic.WithAPIKey(apiKey))
	}

	if len(headers) > 0 {
		opts = append(opts, anthropic.WithHeaders(headers))
	}

	if baseURL != "" {
		opts = append(opts, anthropic.WithBaseURL(baseURL))
	}

	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, anthropic.WithHTTPClient(httpClient))
	}

	return anthropic.New(opts...)
}

func (c *coordinator) buildOpenaiProvider(baseURL, apiKey string, headers map[string]string) (genai.Provider, error) {
	opts := []openai.Option{
		openai.WithAPIKey(apiKey),
		openai.WithUseResponsesAPI(),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, openai.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, openai.WithHeaders(headers))
	}
	if baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}
	return openai.New(opts...)
}

func (c *coordinator) buildOpenrouterProvider(_, apiKey string, headers map[string]string) (genai.Provider, error) {
	opts := []openrouter.Option{
		openrouter.WithAPIKey(apiKey),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, openrouter.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, openrouter.WithHeaders(headers))
	}
	return openrouter.New(opts...)
}

func (c *coordinator) buildOpenaiCompatProvider(baseURL, apiKey string, headers map[string]string, extraBody map[string]any) (genai.Provider, error) {
	opts := []openaicompat.Option{
		openaicompat.WithBaseURL(baseURL),
		openaicompat.WithAPIKey(apiKey),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, openaicompat.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, openaicompat.WithHeaders(headers))
	}

	for extraKey, extraValue := range extraBody {
		opts = append(opts, openaicompat.WithSDKOptions(openaisdk.WithJSONSet(extraKey, extraValue)))
	}

	return openaicompat.New(opts...)
}

func (c *coordinator) buildAzureProvider(baseURL, apiKey string, headers map[string]string, options map[string]string) (genai.Provider, error) {
	opts := []azure.Option{
		azure.WithBaseURL(baseURL),
		azure.WithAPIKey(apiKey),
		azure.WithUseResponsesAPI(),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, azure.WithHTTPClient(httpClient))
	}
	if options == nil {
		options = make(map[string]string)
	}
	if apiVersion, ok := options["apiVersion"]; ok {
		opts = append(opts, azure.WithAPIVersion(apiVersion))
	}
	if len(headers) > 0 {
		opts = append(opts, azure.WithHeaders(headers))
	}

	return azure.New(opts...)
}

func (c *coordinator) buildBedrockProvider(headers map[string]string) (genai.Provider, error) {
	var opts []bedrock.Option
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, bedrock.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, bedrock.WithHeaders(headers))
	}
	bearerToken := os.Getenv("AWS_BEARER_TOKEN_BEDROCK")
	if bearerToken != "" {
		opts = append(opts, bedrock.WithAPIKey(bearerToken))
	}
	return bedrock.New(opts...)
}

func (c *coordinator) buildGoogleProvider(baseURL, apiKey string, headers map[string]string) (genai.Provider, error) {
	opts := []google.Option{
		google.WithBaseURL(baseURL),
		google.WithGeminiAPIKey(apiKey),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, google.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, google.WithHeaders(headers))
	}
	return google.New(opts...)
}

func (c *coordinator) buildGoogleVertexProvider(headers map[string]string, options map[string]string) (genai.Provider, error) {
	opts := []google.Option{}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, google.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, google.WithHeaders(headers))
	}

	project := options["project"]
	location := options["location"]

	opts = append(opts, google.WithVertex(project, location))

	return google.New(opts...)
}

func (c *coordinator) isAnthropicThinking(model config.ModelSelection) bool {
	if model.Think {
		return true
	}

	if model.ProviderOptions == nil {
		return false
	}

	opts, err := anthropic.ParseOptions(model.ProviderOptions)
	if err != nil {
		return false
	}
	if opts.Thinking != nil {
		return true
	}
	return false
}

func (c *coordinator) buildProvider(providerCfg config.ProviderConfig, model config.ModelSelection) (genai.Provider, error) {
	headers := maps.Clone(providerCfg.ExtraHeaders)
	if headers == nil {
		headers = make(map[string]string)
	}

	// handle special headers for anthropic
	if providerCfg.Type == anthropic.Name && c.isAnthropicThinking(model) {
		if v, ok := headers["anthropic-beta"]; ok {
			headers["anthropic-beta"] = v + ",interleaved-thinking-2025-05-14"
		} else {
			headers["anthropic-beta"] = "interleaved-thinking-2025-05-14"
		}
	}

	apiKey, _ := c.cfg.Resolve(providerCfg.APIKey)
	baseURL, _ := c.cfg.Resolve(providerCfg.BaseURL)

	switch providerCfg.Type {
	case openai.Name:
		return c.buildOpenaiProvider(baseURL, apiKey, headers)
	case anthropic.Name:
		return c.buildAnthropicProvider(baseURL, apiKey, headers)
	case openrouter.Name:
		return c.buildOpenrouterProvider(baseURL, apiKey, headers)
	case azure.Name:
		return c.buildAzureProvider(baseURL, apiKey, headers, providerCfg.ExtraParams)
	case bedrock.Name:
		return c.buildBedrockProvider(headers)
	case google.Name:
		return c.buildGoogleProvider(baseURL, apiKey, headers)
	case "google-vertex":
		return c.buildGoogleVertexProvider(headers, providerCfg.ExtraParams)
	case openaicompat.Name:
		return c.buildOpenaiCompatProvider(baseURL, apiKey, headers, providerCfg.ExtraBody)
	default:
		return nil, fmt.Errorf("provider type not supported: %q", providerCfg.Type)
	}
}

func isExactoSupported(modelID string) bool {
	supportedModels := []string{
		"moonshotai/kimi-k2-0905",
		"deepseek/deepseek-v3.1-terminus",
		"z-ai/glm-4.6",
		"openai/gpt-oss-120b",
		"qwen/qwen3-coder",
	}
	return slices.Contains(supportedModels, modelID)
}

func (c *coordinator) Cancel(sessionID string) {
	c.currentAgent.Cancel(sessionID)
}

func (c *coordinator) CancelAll() {
	c.currentAgent.CancelAll()
}

func (c *coordinator) ClearQueue(sessionID string) {
	c.currentAgent.ClearQueue(sessionID)
}

func (c *coordinator) IsBusy() bool {
	return c.currentAgent.IsBusy()
}

func (c *coordinator) IsSessionBusy(sessionID string) bool {
	return c.currentAgent.IsSessionBusy(sessionID)
}

func (c *coordinator) Model() Model {
	return c.currentAgent.Model()
}

func (c *coordinator) UpdateMaestroModel(ctx context.Context, model config.ModelSelection) error {
	currentModel := c.currentAgent.Model()
	c.currentAgent.SetModel(Model{
		Model:     currentModel.Model,
		Props:     currentModel.Props,
		Selection: model,
	})

	agentCfg, ok := c.cfg.Agents[config.AgentMaestro]
	if !ok {
		return errors.New("maestro agent not configured")
	}

	tools, err := c.buildTools(ctx, agentCfg)
	if err != nil {
		return err
	}
	c.currentAgent.SetTools(tools)
	return nil
}

func (c *coordinator) QueuedPrompts(sessionID string) int {
	return c.currentAgent.QueuedPrompts(sessionID)
}

func (c *coordinator) Summarize(ctx context.Context, sessionID string) error {
	providerCfg, ok := c.cfg.Providers.Get(c.currentAgent.Model().Selection.Provider)
	if !ok {
		return errors.New("model provider not configured")
	}
	return c.currentAgent.Summarize(ctx, sessionID, getProviderOptions(c.currentAgent.Model(), providerCfg))
}
