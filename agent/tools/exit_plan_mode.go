package tools

import (
	"context"
	_ "embed"

	"charm.land/fantasy"
	"github.com/can1357/rush/session"
)

//go:embed exit_plan_mode.md
var exitPlanModeDescription []byte

const ExitPlanModeToolName = "exit_plan_mode"

type ExitPlanModeParams struct {
	Plan string `json:"plan" description:"The implementation plan that you're ready to execute"`
}

type ExitPlanModeResponseMetadata struct {
	SessionID string `json:"session_id"`
	PlanMode  bool   `json:"plan_mode"`
}

func NewExitPlanModeTool(sessionService session.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ExitPlanModeToolName,
		string(exitPlanModeDescription),
		func(ctx context.Context, params ExitPlanModeParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session ID is required"), nil
			}

			sess, err := sessionService.Get(ctx, sessionID)
			if err != nil {
				return fantasy.NewTextErrorResponse("failed to get session"), nil
			}

			if !sess.PlanMode {
				return fantasy.NewTextErrorResponse("session is not in plan mode"), nil
			}

			// Exit plan mode
			sess.PlanMode = false
			_, err = sessionService.Save(ctx, sess)
			if err != nil {
				return fantasy.NewTextErrorResponse("failed to exit plan mode"), nil
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse("Successfully exited plan mode. You now have full access to all tools including write, edit, and bash commands. You can begin implementing your plan."),
				ExitPlanModeResponseMetadata{
					SessionID: sessionID,
					PlanMode:  false,
				},
			), nil
		})
}
