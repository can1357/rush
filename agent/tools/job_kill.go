package tools

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/can1357/rush/genai"
	"github.com/can1357/rush/shell"
)

const (
	JobKillToolName = "JobKill"
)

//go:embed job_kill.md
var jobKillDescription []byte

type JobKillParams struct {
	ShellID string `json:"shell_id" description:"The ID of the background shell to terminate"`
}

type JobKillResponseMetadata struct {
	ShellID     string `json:"shell_id"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

func NewJobKillTool() genai.AgentTool {
	return genai.NewAgentTool(
		JobKillToolName,
		string(jobKillDescription),
		func(ctx context.Context, params JobKillParams, call genai.ToolCall) (genai.ToolResponse, error) {
			if params.ShellID == "" {
				return genai.NewTextErrorResponse("missing shell_id"), nil
			}

			bgManager := shell.GetBackgroundShellManager()

			bgShell, ok := bgManager.Get(params.ShellID)
			if !ok {
				return genai.NewTextErrorResponse(fmt.Sprintf("background shell not found: %s", params.ShellID)), nil
			}

			metadata := JobKillResponseMetadata{
				ShellID:     params.ShellID,
				Command:     bgShell.Command,
				Description: bgShell.Description,
			}

			err := bgManager.Kill(params.ShellID)
			if err != nil {
				return genai.NewTextErrorResponse(err.Error()), nil
			}

			result := fmt.Sprintf("Background shell %s terminated successfully", params.ShellID)
			return genai.WithResponseMetadata(genai.NewTextResponse(result), metadata), nil
		})
}
