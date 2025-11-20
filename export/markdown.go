package export

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/can1357/rush/message"
	"github.com/can1357/rush/session"
)

type MarkdownRenderer struct {
	opts ExportOptions
	buf  *strings.Builder
}

func NewMarkdownRenderer(opts ExportOptions) *MarkdownRenderer {
	return &MarkdownRenderer{
		opts: opts,
		buf:  &strings.Builder{},
	}
}

func (r *MarkdownRenderer) Render(sess session.Session, messages []message.Message) (string, error) {
	r.buf.Reset()

	if r.opts.IncludeMetadata {
		r.writeMetadata(sess)
	}

	for _, msg := range messages {
		if msg.Role == "system" && !r.opts.IncludeSystemMessages {
			continue
		}

		if err := r.writeMessage(msg); err != nil {
			return "", err
		}
	}

	r.writeFooter(sess)

	return r.buf.String(), nil
}

func (r *MarkdownRenderer) writeMetadata(sess session.Session) {
	fmt.Fprintf(r.buf, "# Session: %s\n\n", sess.Title)
	fmt.Fprintf(r.buf, "**Created**: %s  \n", time.Unix(sess.CreatedAt, 0).Format("2006-01-02 15:04:05"))

	if sess.PromptTokens > 0 || sess.CompletionTokens > 0 {
		fmt.Fprintf(r.buf, "**Usage**: %d input / %d output tokens  \n", sess.PromptTokens, sess.CompletionTokens)
	}

	if sess.Cost > 0 {
		fmt.Fprintf(r.buf, "**Cost**: $%.4f  \n", sess.Cost)
	}

	fmt.Fprintf(r.buf, "**Messages**: %d\n\n", sess.MessageCount)
	r.buf.WriteString("---\n\n")
}

func (r *MarkdownRenderer) writeMessage(msg message.Message) error {
	// Role header
	role := strings.Title(string(msg.Role))
	fmt.Fprintf(r.buf, "## %s\n\n", role)

	// Parts are already parsed in message.Message
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case message.TextContent:
			r.buf.WriteString(p.Text)
			r.buf.WriteString("\n\n")

		case message.ReasoningContent:
			if r.opts.IncludeReasoning && p.Thinking != "" {
				r.buf.WriteString("<details>\n<summary>Reasoning</summary>\n\n")
				r.buf.WriteString(p.Thinking)
				r.buf.WriteString("\n\n</details>\n\n")
			}

		case message.ToolCall:
			if r.opts.IncludeToolCalls {
				fmt.Fprintf(r.buf, "### Tool Use: %s\n\n", p.Name)
				if formatted, err := json.MarshalIndent(p.Input, "", "  "); err == nil {
					fmt.Fprintf(r.buf, "```json\n%s\n```\n\n", string(formatted))
				}
			}

		case message.ToolResult:
			if r.opts.IncludeToolCalls {
				r.buf.WriteString("## Tool Result\n\n")
				status := "Success"
				if p.IsError {
					status = "Error"
				}
				fmt.Fprintf(r.buf, "**Status**: %s\n\n", status)

				content := p.Content
				if r.opts.TruncateResults && len(content) > r.opts.MaxResultLines*80 {
					content = content[:r.opts.MaxResultLines*80] + "\n\n... [truncated]"
				}

				fmt.Fprintf(r.buf, "```\n%s\n```\n\n", content)
			}

		case message.ImageURLContent:
			fmt.Fprintf(r.buf, "![Image](%s)\n\n", p.URL)

		case message.BinaryContent:
			size := formatBytes(int64(len(p.Data)))
			filename := p.Path
			if filename == "" {
				filename = "attachment"
			}
			fmt.Fprintf(r.buf, "**Attachment**: %s (%s, %s)\n\n", filename, p.MIMEType, size)

		case message.Finish:
			// Skip finish markers in export (they're meta)
		}
	}

	r.buf.WriteString("---\n\n")
	return nil
}

func (r *MarkdownRenderer) writeFooter(sess session.Session) {
	if sess.UpdatedAt != sess.CreatedAt {
		fmt.Fprintf(r.buf, "_Conversation ended: %s_\n",
			time.Unix(sess.UpdatedAt, 0).Format("2006-01-02 15:04:05"))
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
