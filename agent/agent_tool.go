package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/can1357/rush/agent/prompt"
	"github.com/can1357/rush/agent/tools"
	"github.com/can1357/rush/config"
	"github.com/can1357/rush/genai"
)

// generateAgentToolDescription creates a dynamic description for the Task tool
// based on available agent types and their configurations.
func generateAgentToolDescription(agents map[string]config.Agent) string {
	var sb strings.Builder

	sb.WriteString("Launch a new agent to handle complex, multi-step tasks autonomously. \n\n")
	sb.WriteString("The Task tool launches specialized agents (subprocesses) that autonomously handle complex tasks. Each agent type has specific capabilities and tools available to it.\n\n")
	sb.WriteString("Available agent types and the tools they have access to:\n")

	// Only include task and explore agents (exclude root)
	agentTypes := []string{config.AgentTask, config.AgentExplore}

	for _, agentID := range agentTypes {
		agent, ok := agents[agentID]
		if !ok || agent.Disabled {
			continue
		}

		// Format: - agent_id: Description (Model: large/small; Tools: tool1, tool2, ...)
		modelType := "large"
		if agent.Model == config.SelectedModelTypeSmall {
			modelType = "small"
		}

		toolsList := strings.Join(agent.AllowedTools, ", ")
		if toolsList == "" {
			toolsList = "All tools"
		}

		sb.WriteString(fmt.Sprintf("- %s: %s (Model: %s; Tools: %s)\n",
			agentID,
			agent.Description,
			modelType,
			toolsList,
		))
	}

	sb.WriteString("\nWhen using the Task tool, you must specify a subagent_type parameter to select which agent type to use.\n\n")
	sb.WriteString("When NOT to use the Task tool:\n")
	sb.WriteString("- If you want to read a specific file path, use the Read or Glob tool instead of the Task tool, to find the match more quickly\n")
	sb.WriteString("- If you are searching for a specific class definition like \"class Foo\", use the Glob tool instead, to find the match more quickly\n")
	sb.WriteString("- If you are searching for code within a specific file or set of 2-3 files, use the Read tool instead of the Task tool, to find the match more quickly\n")
	sb.WriteString("- Other tasks that are not related to the agent descriptions above\n\n")
	sb.WriteString("\nUsage notes:\n")
	sb.WriteString("- Launch multiple agents concurrently whenever possible, to maximize performance; to do that, use a single message with multiple tool uses\n")
	sb.WriteString("- When the agent is done, it will return a single message back to you. The result returned by the agent is not visible to the user. To show the user the result, you should send a text message back to the user with a concise summary of the result.\n")
	sb.WriteString("- Each agent invocation is stateless. You will not be able to send additional messages to the agent, nor will the agent be able to communicate with you outside of its final report. Therefore, your prompt should contain a highly detailed task description for the agent to perform autonomously and you should specify exactly what information the agent should return back to you in its final and only message to you.\n")
	sb.WriteString("- Agents with \"access to current context\" can see the full conversation history before the tool call. When using these agents, you can write concise prompts that reference earlier context (e.g., \"investigate the error discussed above\") instead of repeating information. The agent will receive all prior messages and understand the context.\n")
	sb.WriteString("- The agent's outputs should generally be trusted\n")
	sb.WriteString("- Clearly tell the agent whether you expect it to write code or just to do research (search, file reads, web fetches, etc.), since it is not aware of the user's intent\n")
	sb.WriteString("- If the agent description mentions that it should be used proactively, then you should try your best to use it without the user having to ask for it first. Use your judgement.\n")
	sb.WriteString("- If the user specifies that they want you to run agents \"in parallel\", you MUST send a single message with multiple Task tool use content blocks. For example, if you need to launch both a code-reviewer agent and a test-runner agent in parallel, send a single message with both tool calls.\n\n")

	sb.WriteString("Example usage:\n\n")
	sb.WriteString("<example_agent_descriptions>\n")
	sb.WriteString("\"code-reviewer\": use this agent after you are done writing a signficant piece of code\n")
	sb.WriteString("\"greeting-responder\": use this agent when to respond to user greetings with a friendly joke\n")
	sb.WriteString("</example_agent_description>\n\n")
	sb.WriteString("<example>\n")
	sb.WriteString("user: \"Please write a function that checks if a number is prime\"\n")
	sb.WriteString("assistant: Sure let me write a function that checks if a number is prime\n")
	sb.WriteString("assistant: First let me use the Write tool to write a function that checks if a number is prime\n")
	sb.WriteString("assistant: I'm going to use the Write tool to write the following code:\n")
	sb.WriteString("<code>\n")
	sb.WriteString("function isPrime(n) {\n")
	sb.WriteString("  if (n <= 1) return false\n")
	sb.WriteString("  for (let i = 2; i * i <= n; i++) {\n")
	sb.WriteString("    if (n % i === 0) return false\n")
	sb.WriteString("  }\n")
	sb.WriteString("  return true\n")
	sb.WriteString("}\n")
	sb.WriteString("</code>\n")
	sb.WriteString("<commentary>\n")
	sb.WriteString("Since a signficant piece of code was written and the task was completed, now use the code-reviewer agent to review the code\n")
	sb.WriteString("</commentary>\n")
	sb.WriteString("assistant: Now let me use the code-reviewer agent to review the code\n")
	sb.WriteString("assistant: Uses the Task tool to launch the code-reviewer agent \n")
	sb.WriteString("</example>\n\n")
	sb.WriteString("<example>\n")
	sb.WriteString("user: \"Hello\"\n")
	sb.WriteString("<commentary>\n")
	sb.WriteString("Since the user is greeting, use the greeting-responder agent to respond with a friendly joke\n")
	sb.WriteString("</commentary>\n")
	sb.WriteString("assistant: \"I'm going to use the Task tool to launch the greeting-responder agent\"\n")
	sb.WriteString("</example>\n")

	return sb.String()
}

type AgentParams struct {
	Description  string `json:"description" description:"A short (3-5 word) description of the task"`
	Prompt       string `json:"prompt" description:"The task for the agent to perform"`
	SubagentType string `json:"subagent_type" description:"The type of specialized agent to use for this task"`
}

const (
	AgentToolName = "agent"
)

func (c *coordinator) agentTool(ctx context.Context, description string) (genai.AgentTool, error) {
	return genai.NewAgentTool(
		AgentToolName,
		description,
		func(ctx context.Context, params AgentParams, call genai.ToolCall) (genai.ToolResponse, error) {
			// Validate required parameters
			if params.Prompt == "" {
				return genai.NewTextErrorResponse("prompt is required"), nil
			}
			if params.Description == "" {
				return genai.NewTextErrorResponse("description is required"), nil
			}
			if params.SubagentType == "" {
				return genai.NewTextErrorResponse("subagent_type is required"), nil
			}

			// Route to the correct agent based on subagent_type
			var agentCfg config.Agent
			var agentPrompt *prompt.Prompt
			var err error

			switch params.SubagentType {
			case config.AgentTask:
				var ok bool
				agentCfg, ok = c.cfg.Agents[config.AgentTask]
				if !ok {
					return genai.NewTextErrorResponse("task agent not configured"), nil
				}
				agentPrompt, err = taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
				if err != nil {
					return genai.ToolResponse{}, fmt.Errorf("error creating task prompt: %w", err)
				}

			case config.AgentExplore:
				var ok bool
				agentCfg, ok = c.cfg.Agents[config.AgentExplore]
				if !ok {
					return genai.NewTextErrorResponse("explore agent not configured"), nil
				}
				agentPrompt, err = explorePrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
				if err != nil {
					return genai.ToolResponse{}, fmt.Errorf("error creating explore prompt: %w", err)
				}

			default:
				return genai.NewTextErrorResponse(fmt.Sprintf("unknown subagent_type: %s. Valid types are: task, explore", params.SubagentType)), nil
			}

			// Build the agent for this specific call
			agent, err := c.buildAgent(ctx, agentPrompt, agentCfg)
			if err != nil {
				return genai.ToolResponse{}, fmt.Errorf("error building agent: %w", err)
			}

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return genai.ToolResponse{}, errors.New("session id missing from context")
			}

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return genai.ToolResponse{}, errors.New("agent message id missing from context")
			}

			agentToolSessionID := c.sessions.CreateAgentToolSessionID(agentMessageID, call.ID)
			// Include agent type in session title for visibility
			sessionTitle := fmt.Sprintf("[%s] %s", params.SubagentType, params.Description)
			session, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, sessionID, sessionTitle)
			if err != nil {
				return genai.ToolResponse{}, fmt.Errorf("error creating session: %s", err)
			}

			model := agent.Model()
			maxTokens := model.CatwalkCfg.DefaultMaxTokens
			if model.ModelCfg.MaxTokens != 0 {
				maxTokens = model.ModelCfg.MaxTokens
			}

			providerCfg, ok := c.cfg.Providers.Get(model.ModelCfg.Provider)
			if !ok {
				return genai.ToolResponse{}, errors.New("model provider not configured")
			}

			result, err := agent.Run(ctx, SessionAgentCall{
				SessionID:        session.ID,
				Prompt:           params.Prompt,
				MaxOutputTokens:  maxTokens,
				ProviderOptions:  getProviderOptions(model, providerCfg),
				Temperature:      model.ModelCfg.Temperature,
				TopP:             model.ModelCfg.TopP,
				TopK:             model.ModelCfg.TopK,
				FrequencyPenalty: model.ModelCfg.FrequencyPenalty,
				PresencePenalty:  model.ModelCfg.PresencePenalty,
			})
			if err != nil {
				return genai.NewTextErrorResponse("error generating response"), nil
			}

			updatedSession, err := c.sessions.Get(ctx, session.ID)
			if err != nil {
				return genai.ToolResponse{}, fmt.Errorf("error getting session: %s", err)
			}

			parentSession, err := c.sessions.Get(ctx, sessionID)
			if err != nil {
				return genai.ToolResponse{}, fmt.Errorf("error getting parent session: %s", err)
			}

			parentSession.Cost += updatedSession.Cost

			_, err = c.sessions.Save(ctx, parentSession)
			if err != nil {
				return genai.ToolResponse{}, fmt.Errorf("error saving parent session: %s", err)
			}

			return genai.NewTextResponse(result.Response.Content.Text()), nil
		}), nil
}
