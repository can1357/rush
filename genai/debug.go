// Copyright 2025 Can Boluk
//
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package genai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DebugDumpRequest writes LLM request/response to a markdown file for debugging.
func DebugDumpRequest(call Call, resp *Response, modelID string) {
	debugDir := "debug-msgs"
	if err := os.MkdirAll(debugDir, 0o755); err != nil {
		return // silently fail
	}

	now := time.Now()
	msgID := fmt.Sprintf("%d", now.UnixNano())
	filename := filepath.Join(debugDir, fmt.Sprintf("%s.md", msgID))

	var content strings.Builder

	// Header with timestamp
	content.WriteString(fmt.Sprintf("# LLM Request Debug Dump\n\n"))
	content.WriteString(fmt.Sprintf("**Timestamp:** %s\n\n", now.Format("2006-01-02 15:04:05.000")))

	// Write messages grouped by role
	for _, msg := range call.Prompt {
		switch msg.Role {
		case MessageRoleSystem:
			content.WriteString("## System:\n\n")
		case MessageRoleUser:
			content.WriteString("## User:\n\n")
		case MessageRoleAssistant:
			content.WriteString("## Assistant:\n\n")
		case MessageRoleTool:
			content.WriteString("## Tool:\n\n")
		}

		for _, part := range msg.Content {
			switch p := part.(type) {
			case TextPart:
				content.WriteString(p.Text)
				content.WriteString("\n\n")
			case ToolCallPart:
				content.WriteString(fmt.Sprintf("**Tool Call: %s**\n```json\n%s\n```\n\n", p.ToolName, p.Input))
			case ToolResultPart:
				json, err := json.Marshal(p)
				if err != nil {
					continue
				}
				content.WriteString(fmt.Sprintf("**Tool Result (ID: %s)**\n```\n%s\n```\n\n", p.ToolCallID, string(json)))
			case ReasoningPart:
				content.WriteString(fmt.Sprintf("**Reasoning:**\n```\n%s\n```\n\n", p.Text))
			case FilePart:
				content.WriteString(fmt.Sprintf("**File: %s** (binary data, %d bytes)\n\n", p.Filename, len(p.Data)))
			}
		}

		content.WriteString("---\n\n")
	}

	// Write usage info if available
	if resp != nil {
		content.WriteString("## Response Metadata:\n\n")
		content.WriteString(fmt.Sprintf("- **Model:** %s\n", modelID))
		content.WriteString(fmt.Sprintf("- **Finish Reason:** %s\n", resp.FinishReason))
		content.WriteString(fmt.Sprintf("- **Request Input Tokens:** %d (this specific API call)\n", resp.Usage.InputTokens))
		content.WriteString(fmt.Sprintf("- **Request Output Tokens:** %d\n", resp.Usage.OutputTokens))
		content.WriteString(fmt.Sprintf("- **Request Total Tokens:** %d\n", resp.Usage.TotalTokens))
		if resp.Usage.ReasoningTokens > 0 {
			content.WriteString(fmt.Sprintf("- **Reasoning Tokens:** %d\n", resp.Usage.ReasoningTokens))
		}
		if resp.Usage.CacheCreationTokens > 0 {
			content.WriteString(fmt.Sprintf("- **Cache Creation Tokens:** %d\n", resp.Usage.CacheCreationTokens))
		}
		if resp.Usage.CacheReadTokens > 0 {
			content.WriteString(fmt.Sprintf("- **Cache Read Tokens:** %d\n", resp.Usage.CacheReadTokens))
		}

		// Write response content
		content.WriteString("\n## Response Content:\n\n")
		for _, c := range resp.Content {
			switch contentPart := c.(type) {
			case TextContent:
				content.WriteString("**Text:**\n")
				content.WriteString(contentPart.Text)
				content.WriteString("\n\n")
			case ReasoningContent:
				content.WriteString("**Reasoning:**\n```\n")
				content.WriteString(contentPart.Text)
				content.WriteString("\n```\n\n")
			case ToolCallContent:
				content.WriteString(fmt.Sprintf("**Tool Call: %s (ID: %s)**\n```json\n%s\n```\n\n",
					contentPart.ToolName, contentPart.ToolCallID, contentPart.Input))
			}
		}
	}

	os.WriteFile(filename, []byte(content.String()), 0o644)
}
