package agent

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/x/vcr"
	"github.com/can1357/rush/agent/prompt"
	"github.com/can1357/rush/agent/tools"
	"github.com/can1357/rush/config"
	"github.com/can1357/rush/csync"
	"github.com/can1357/rush/db"
	"github.com/can1357/rush/genai"
	"github.com/can1357/rush/genai/providers/anthropic"
	"github.com/can1357/rush/genai/providers/openai"
	"github.com/can1357/rush/genai/providers/openaicompat"
	"github.com/can1357/rush/genai/providers/openrouter"
	"github.com/can1357/rush/history"
	"github.com/can1357/rush/lsp"
	"github.com/can1357/rush/message"
	"github.com/can1357/rush/permission"
	"github.com/can1357/rush/session"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"

	_ "github.com/joho/godotenv/autoload"
)

// fakeEnv is an environment for testing.
type fakeEnv struct {
	workingDir  string
	sessions    session.Service
	messages    message.Service
	permissions permission.Service
	history     history.Service
	lspClients  *csync.Map[string, *lsp.Client]
}

type builderFunc func(t *testing.T, r *vcr.Recorder) (genai.LanguageModel, error)

type modelPair struct {
	name       string
	largeModel builderFunc
	smallModel builderFunc
}

func anthropicBuilder(model string) builderFunc {
	return func(t *testing.T, r *vcr.Recorder) (genai.LanguageModel, error) {
		provider, err := anthropic.New(
			anthropic.WithAPIKey(os.Getenv("RUSH_ANTHROPIC_API_KEY")),
			anthropic.WithHTTPClient(&http.Client{Transport: r}),
		)
		if err != nil {
			return nil, err
		}
		return provider.LanguageModel(t.Context(), model)
	}
}

func openaiBuilder(model string) builderFunc {
	return func(t *testing.T, r *vcr.Recorder) (genai.LanguageModel, error) {
		provider, err := openai.New(
			openai.WithAPIKey(os.Getenv("RUSH_OPENAI_API_KEY")),
			openai.WithHTTPClient(&http.Client{Transport: r}),
		)
		if err != nil {
			return nil, err
		}
		return provider.LanguageModel(t.Context(), model)
	}
}

func openRouterBuilder(model string) builderFunc {
	return func(t *testing.T, r *vcr.Recorder) (genai.LanguageModel, error) {
		provider, err := openrouter.New(
			openrouter.WithAPIKey(os.Getenv("RUSH_OPENROUTER_API_KEY")),
			openrouter.WithHTTPClient(&http.Client{Transport: r}),
		)
		if err != nil {
			return nil, err
		}
		return provider.LanguageModel(t.Context(), model)
	}
}

func zAIBuilder(model string) builderFunc {
	return func(t *testing.T, r *vcr.Recorder) (genai.LanguageModel, error) {
		provider, err := openaicompat.New(
			openaicompat.WithBaseURL("https://api.z.ai/api/coding/paas/v4"),
			openaicompat.WithAPIKey(os.Getenv("RUSH_ZAI_API_KEY")),
			openaicompat.WithHTTPClient(&http.Client{Transport: r}),
			openaicompat.WithModels([]catwalk.Model{{ID: model}}),
		)
		if err != nil {
			return nil, err
		}
		return provider.LanguageModel(t.Context(), model)
	}
}

func testEnv(t *testing.T) fakeEnv {
	workingDir := filepath.Join("/tmp/rush-test/", t.Name())
	os.RemoveAll(workingDir)

	err := os.MkdirAll(workingDir, 0o755)
	require.NoError(t, err)

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)

	q := db.New(conn)
	sessions := session.NewService(q)
	messages := message.NewService(q)

	permissions := permission.NewPermissionService(workingDir, true, []string{})
	history := history.NewService(q, conn)
	lspClients := csync.NewMap[string, *lsp.Client]()

	t.Cleanup(func() {
		conn.Close()
		os.RemoveAll(workingDir)
	})

	return fakeEnv{
		workingDir,
		sessions,
		messages,
		permissions,
		history,
		lspClients,
	}
}

func testSessionAgent(env fakeEnv, large, small genai.LanguageModel, systemPrompt string, tools ...genai.AgentTool) SessionAgent {
	largeModel := Model{
		Model: large,
		Props: &catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 10000,
		},
	}
	smallModel := Model{
		Model: small,
		Props: &catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 4000,
		},
	}
	agent := NewSessionAgent(SessionAgentOptions{
		Model:                largeModel,
		SmallModel:           smallModel,
		SystemPromptPrefix:   "",
		SystemPrompt:         systemPrompt,
		DisableAutoSummarize: false,
		IsYolo:               true,
		Sessions:             env.sessions,
		Messages:             env.messages,
		Reminders:            nil,
		Tools:                tools,
	})
	return agent
}

func coderAgent(r *vcr.Recorder, env fakeEnv, large, small genai.LanguageModel) (SessionAgent, error) {
	fixedTime := func() time.Time {
		t, _ := time.Parse("1/2/2006", "1/1/2025")
		return t
	}
	prompt, err := maestroPrompt(
		prompt.WithTimeFunc(fixedTime),
		prompt.WithPlatform("linux"),
		prompt.WithWorkingDir(filepath.ToSlash(env.workingDir)),
	)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Init(env.workingDir, "", false)
	if err != nil {
		return nil, err
	}

	systemPrompt, err := prompt.Build(context.TODO(), large.Provider(), large.Model(), *cfg)
	if err != nil {
		return nil, err
	}

	allTools := []genai.AgentTool{
		tools.NewBashTool(env.permissions, env.workingDir),
		tools.NewDownloadTool(env.permissions, env.workingDir, r.GetDefaultClient()),
		tools.NewEditTool(env.lspClients, env.permissions, env.history, env.workingDir),
		tools.NewMultiEditTool(env.lspClients, env.permissions, env.history, env.workingDir),
		tools.NewFetchTool(env.permissions, env.workingDir, r.GetDefaultClient()),
		tools.NewGlobTool(env.workingDir),
		tools.NewGrepTool(env.workingDir),
		tools.NewReadDirTool(env.permissions, env.workingDir, cfg.Tools.Ls),
		tools.NewSourcegraphTool(r.GetDefaultClient()),
		tools.NewViewTool(env.lspClients, env.permissions, env.workingDir),
		tools.NewWriteTool(env.lspClients, env.permissions, env.history, env.workingDir),
	}

	return testSessionAgent(env, large, small, systemPrompt, allTools...), nil
}

// createSimpleGoProject creates a simple Go project structure in the given directory.
// It creates a go.mod file and a main.go file with a basic hello world program.
func createSimpleGoProject(t *testing.T, dir string) {
	goMod := `module example.com/testproject

go 1.23
`
	err := os.WriteFile(dir+"/go.mod", []byte(goMod), 0o644)
	require.NoError(t, err)

	mainGo := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	err = os.WriteFile(dir+"/main.go", []byte(mainGo), 0o644)
	require.NoError(t, err)
}
