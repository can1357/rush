# Rush

<p align="center">
    <a href="https://stuff.charm.sh/rush/charm-rush.png"><img width="450" alt="Charm Rush Logo" src="https://github.com/user-attachments/assets/adc1a6f4-b284-4603-836c-59038caa2e8b" /></a><br />
    <a href="https://github.com/can1357/rush/releases"><img src="https://img.shields.io/github/release/can1357/rush" alt="Latest Release"></a>
    <a href="https://github.com/can1357/rush/actions"><img src="https://github.com/can1357/rush/actions/workflows/build.yml/badge.svg" alt="Build Status"></a>
</p>

<p align="center">Your new coding bestie, now available in your favourite terminal.<br />Your tools, your code, and your workflows, wired into your LLM of choice.</p>
<p align="center">你的新编程伙伴，现在就在你最爱的终端中。<br />你的工具、代码和工作流，都与您选择的 LLM 模型紧密相连。</p>

<p align="center"><img width="800" alt="Rush Demo" src="https://github.com/user-attachments/assets/58280caf-851b-470a-b6f7-d5c4ea8a1968" /></p>

---

> This is a fork of [Charm's Crush](https://github.com/charmbracelet/crush) - an AI coding assistant for the terminal.
>
> All credit for the original project goes to [Charm](https://charm.land) and the Crush contributors.

---

## Features

- **Multi-Model:** choose from a wide range of LLMs or add your own via OpenAI- or Anthropic-compatible APIs
- **Flexible:** switch LLMs mid-session while preserving context
- **Session-Based:** maintain multiple work sessions and contexts per project
- **LSP-Enhanced:** Rush uses LSPs for additional context, just like you do
- **Extensible:** add capabilities via MCPs (`http`, `stdio`, and `sse`)
- **Works Everywhere:** first-class support in every terminal on macOS, Linux, Windows (PowerShell and WSL), FreeBSD, OpenBSD, and NetBSD

## Installation

### From Source

Install directly from this fork using Go:

```
go install github.com/can1357/rush@latest
```

### Original Crush Installation

For the upstream Charm Crush project, see the [original installation instructions](https://github.com/charmbracelet/crush#installation) which include Homebrew, npm, Nix, and platform-specific package managers.

> [!WARNING]
> Productivity may increase when using Rush and you may find yourself nerd
> sniped when first using the application. If the symptoms persist, join the
> [Discord][discord] and nerd snipe the rest of us.

## Getting Started

The quickest way to get started is to grab an API key for your preferred
provider such as Anthropic, OpenAI, Groq, or OpenRouter and just start
Rush. You'll be prompted to enter your API key.

That said, you can also set environment variables for preferred providers.

| Environment Variable        | Provider                                           |
| --------------------------- | -------------------------------------------------- |
| `ANTHROPIC_API_KEY`         | Anthropic                                          |
| `OPENAI_API_KEY`            | OpenAI                                             |
| `OPENROUTER_API_KEY`        | OpenRouter                                         |
| `GEMINI_API_KEY`            | Google Gemini                                      |
| `CEREBRAS_API_KEY`          | Cerebras                                           |
| `HF_TOKEN`                  | Huggingface Inference                              |
| `VERTEXAI_PROJECT`          | Google Cloud VertexAI (Gemini)                     |
| `VERTEXAI_LOCATION`         | Google Cloud VertexAI (Gemini)                     |
| `GROQ_API_KEY`              | Groq                                               |
| `AWS_ACCESS_KEY_ID`         | AWS Bedrock (Claude)                               |
| `AWS_SECRET_ACCESS_KEY`     | AWS Bedrock (Claude)                               |
| `AWS_REGION`                | AWS Bedrock (Claude)                               |
| `AWS_PROFILE`               | AWS Bedrock (Custom Profile)                       |
| `AWS_BEARER_TOKEN_BEDROCK`  | AWS Bedrock                                        |
| `AZURE_OPENAI_API_ENDPOINT` | Azure OpenAI models                                |
| `AZURE_OPENAI_API_KEY`      | Azure OpenAI models (optional when using Entra ID) |
| `AZURE_OPENAI_API_VERSION`  | Azure OpenAI models                                |

### By the Way

Is there a provider you’d like to see in Rush? Is there an existing model that needs an update?

Rush’s default model listing is managed in [Catwalk](https://github.com/charmbracelet/catwalk), a community-supported, open source repository of Rush-compatible models, and you’re welcome to contribute.

<a href="https://github.com/charmbracelet/catwalk"><img width="174" height="174" alt="Catwalk Badge" src="https://github.com/user-attachments/assets/95b49515-fe82-4409-b10d-5beb0873787d" /></a>

## Configuration

Rush runs great with no configuration. That said, if you do need or want to
customize Rush, configuration can be added either local to the project itself,
or globally, with the following priority:

1. `.rush.json`
2. `rush.json`
3. `$HOME/.rush/config.json`

Configuration itself is stored as a JSON object:

```json
{
	"this-setting": { "this": "that" },
	"that-setting": ["ceci", "cela"]
}
```

As an additional note, Rush also stores ephemeral data, such as application state, at:

```bash
$HOME/.rush/config.json
```

### LSPs

Rush can use LSPs for additional context to help inform its decisions, just
like you would. LSPs can be added manually like so:

```json
{
	"$schema": "https://raw.githubusercontent.com/can1357/rush/main/schema.json",
	"lsp": {
		"go": {
			"command": "gopls",
			"env": {
				"GOTOOLCHAIN": "go1.24.5"
			}
		},
		"typescript": {
			"command": "typescript-language-server",
			"args": ["--stdio"]
		},
		"nix": {
			"command": "nil"
		}
	}
}
```

### MCPs

Rush also supports Model Context Protocol (MCP) servers through three
transport types: `stdio` for command-line servers, `http` for HTTP endpoints,
and `sse` for Server-Sent Events. Environment variable expansion is supported
using `$(echo $VAR)` syntax.

```json
{
	"$schema": "https://raw.githubusercontent.com/can1357/rush/main/schema.json",
	"mcp": {
		"filesystem": {
			"type": "stdio",
			"command": "node",
			"args": ["/path/to/mcp-server.js"],
			"timeout": 120,
			"disabled": false,
			"env": {
				"NODE_ENV": "production"
			}
		},
		"github": {
			"type": "http",
			"url": "https://api.githubcopilot.com/mcp/",
			"timeout": 120,
			"disabled": false,
			"headers": {
				"Authorization": "Bearer $GH_PAT"
			}
		},
		"streaming-service": {
			"type": "sse",
			"url": "https://example.com/mcp/sse",
			"timeout": 120,
			"disabled": false,
			"headers": {
				"API-Key": "$(echo $API_KEY)"
			}
		}
	}
}
```

### Ignoring Files

Rush respects `.gitignore` files by default, but you can also create a
`.rushignore` file to specify additional files and directories that Rush
should ignore. This is useful for excluding files that you want in version
control but don't want Rush to consider when providing context.

The `.rushignore` file uses the same syntax as `.gitignore` and can be placed
in the root of your project or in subdirectories.

### Allowing Tools

By default, Rush will ask you for permission before running tool calls. If
you'd like, you can allow tools to be executed without prompting you for
permissions. Use this with care.

```json
{
	"$schema": "https://raw.githubusercontent.com/can1357/rush/main/schema.json",
	"permissions": {
		"allowed_tools": ["view", "ls", "grep", "edit", "mcp_context7_get-library-doc"]
	}
}
```

You can also skip all permission prompts entirely by running Rush with the
`--yolo` flag. Be very, very careful with this feature.

### Initialization

When you initialize a project, Rush analyzes your codebase and creates
a context file that helps it work more effectively in future sessions.
By default, this file is named `AGENTS.md`, but you can customize the
name and location with the `initialize_as` option:

```json
{
	"$schema": "https://raw.githubusercontent.com/can1357/rush/main/schema.json",
	"options": {
		"initialize_as": "AGENTS.md"
	}
}
```

This is useful if you prefer a different naming convention or want to
place the file in a specific directory (e.g., `RUSH.md` or
`docs/LLMs.md`). Rush will fill the file with project-specific context
like build commands, code patterns, and conventions it discovered during
initialization.

### Custom Providers

Rush supports custom provider configurations for both OpenAI-compatible and
Anthropic-compatible APIs.

> [!NOTE]
> Note that we support two "types" for OpenAI. Make sure to choose the right one
> to ensure the best experience!
>
> - `openai` should be used when proxying or routing requests through OpenAI.
> - `openai-compat` should be used when using non-OpenAI providers that have OpenAI-compatible APIs.

#### OpenAI-Compatible APIs

Here’s an example configuration for Deepseek, which uses an OpenAI-compatible
API. Don't forget to set `DEEPSEEK_API_KEY` in your environment.

```json
{
	"$schema": "https://raw.githubusercontent.com/can1357/rush/main/schema.json",
	"providers": {
		"deepseek": {
			"type": "openai-compat",
			"base_url": "https://api.deepseek.com/v1",
			"api_key": "$DEEPSEEK_API_KEY",
			"models": [
				{
					"id": "deepseek-chat",
					"name": "Deepseek V3",
					"cost_per_1m_in": 0.27,
					"cost_per_1m_out": 1.1,
					"cost_per_1m_in_cached": 0.07,
					"cost_per_1m_out_cached": 1.1,
					"context_window": 64000,
					"default_max_tokens": 5000
				}
			]
		}
	}
}
```

#### Anthropic-Compatible APIs

Custom Anthropic-compatible providers follow this format:

```json
{
	"$schema": "https://raw.githubusercontent.com/can1357/rush/main/schema.json",
	"providers": {
		"custom-anthropic": {
			"type": "anthropic",
			"base_url": "https://api.anthropic.com/v1",
			"api_key": "$ANTHROPIC_API_KEY",
			"extra_headers": {
				"anthropic-version": "2023-06-01"
			},
			"models": [
				{
					"id": "claude-sonnet-4-20250514",
					"name": "Claude Sonnet 4",
					"cost_per_1m_in": 3,
					"cost_per_1m_out": 15,
					"cost_per_1m_in_cached": 3.75,
					"cost_per_1m_out_cached": 0.3,
					"context_window": 200000,
					"default_max_tokens": 50000,
					"can_reason": true,
					"supports_attachments": true
				}
			]
		}
	}
}
```

#### LiteLLM with Model Discovery

LiteLLM is a unified proxy for 100+ LLM providers. Rush supports automatic model discovery from LiteLLM, eliminating the need to manually configure models.

**Setup:**

1. Install and run LiteLLM proxy:

   ```bash
   pip install litellm
   litellm --config litellm_config.yaml
   ```

2. **Option A:** Auto-detection via environment variables (recommended)

   Set `RUSH_LITELLM_BASE_URL` and optionally `RUSH_LITELLM_API_KEY`:

   ```bash
   export RUSH_LITELLM_BASE_URL="http://localhost:4000/v1"
   export RUSH_LITELLM_API_KEY="your-master-key"  # Optional for local instances
   rush
   ```

   Rush will automatically detect LiteLLM and discover available models on startup.

3. **Option B:** Manual configuration with automatic model discovery:

```json
{
	"$schema": "https://raw.githubusercontent.com/can1357/rush/main/schema.json",
	"providers": {
		"litellm": {
			"type": "openai-compat",
			"base_url": "http://localhost:4000/v1",
			"api_key": "$LITELLM_MASTER_KEY",
			"discover_models": true,
			"default_model_metadata": {
				"context_window": 8192,
				"default_max_tokens": 4096
			}
		}
	}
}
```

**How it works:**

- `discover_models: true` enables automatic model discovery from the `/v1/models` endpoint (defaults to `true`)
- Rush fetches available models on startup and caches them in `~/.rush/discovered-models.json`
- `default_model_metadata` provides fallback values for models that don't report their capabilities
- Models are merged with any manually configured models (manual configs take precedence)

**Manual model discovery:**

```bash
# Discover models from LiteLLM provider
rush discover-models litellm

# Discover from all providers with discovery enabled
rush discover-models --all

# Save discovered models to config
rush discover-models litellm --save
```

**Benefits:**

- Zero configuration: Just point to your LiteLLM proxy
- Automatic updates: New models appear without config changes
- Multi-provider support: Access all LiteLLM-supported providers through one endpoint
- Offline fallback: Cached models available when proxy is unreachable

### Amazon Bedrock

Rush currently supports running Anthropic models through Bedrock, with caching disabled.

- A Bedrock provider will appear once you have AWS configured, i.e. `aws configure`
- Rush also expects the `AWS_REGION` or `AWS_DEFAULT_REGION` to be set
- To use a specific AWS profile set `AWS_PROFILE` in your environment, i.e. `AWS_PROFILE=myprofile rush`
- Alternatively to `aws configure`, you can also just set `AWS_BEARER_TOKEN_BEDROCK`

### Vertex AI Platform

Vertex AI will appear in the list of available providers when `VERTEXAI_PROJECT` and `VERTEXAI_LOCATION` are set. You will also need to be authenticated:

```bash
gcloud auth application-default login
```

To add specific models to the configuration, configure as such:

```json
{
	"$schema": "https://raw.githubusercontent.com/can1357/rush/main/schema.json",
	"providers": {
		"vertexai": {
			"models": [
				{
					"id": "claude-sonnet-4@20250514",
					"name": "VertexAI Sonnet 4",
					"cost_per_1m_in": 3,
					"cost_per_1m_out": 15,
					"cost_per_1m_in_cached": 3.75,
					"cost_per_1m_out_cached": 0.3,
					"context_window": 200000,
					"default_max_tokens": 50000,
					"can_reason": true,
					"supports_attachments": true
				}
			]
		}
	}
}
```

### Local Models

Local models can also be configured via OpenAI-compatible API. Here are two common examples:

#### Ollama

```json
{
	"providers": {
		"ollama": {
			"name": "Ollama",
			"base_url": "http://localhost:11434/v1/",
			"type": "openai-compat",
			"models": [
				{
					"name": "Qwen 3 30B",
					"id": "qwen3:30b",
					"context_window": 256000,
					"default_max_tokens": 20000
				}
			]
		}
	}
}
```

#### LM Studio

```json
{
	"providers": {
		"lmstudio": {
			"name": "LM Studio",
			"base_url": "http://localhost:1234/v1/",
			"type": "openai-compat",
			"models": [
				{
					"name": "Qwen 3 30B",
					"id": "qwen/qwen3-30b-a3b-2507",
					"context_window": 256000,
					"default_max_tokens": 20000
				}
			]
		}
	}
}
```

## Logging

Sometimes you need to look at logs. Luckily, Rush logs all sorts of
stuff. Logs are stored in `./.rush/logs/rush.log` relative to the project.

The CLI also contains some helper commands to make perusing recent logs easier:

```bash
# Print the last 1000 lines
rush logs

# Print the last 500 lines
rush logs --tail 500

# Follow logs in real time
rush logs --follow
```

Want more logging? Run `rush` with the `--debug` flag, or enable it in the
config:

```json
{
	"$schema": "https://raw.githubusercontent.com/can1357/rush/main/schema.json",
	"options": {
		"debug": true,
		"debug_lsp": true
	}
}
```

## Provider Auto-Updates

By default, Rush automatically checks for the latest and greatest list of
providers and models from [Catwalk](https://github.com/charmbracelet/catwalk),
the open source Rush provider database. This means that when new providers and
models are available, or when model metadata changes, Rush automatically
updates your local configuration.

### Disabling automatic provider updates

For those with restricted internet access, or those who prefer to work in
air-gapped environments, this might not be want you want, and this feature can
be disabled.

To disable automatic provider updates, set `disable_provider_auto_update` into
your `rush.json` config:

```json
{
	"$schema": "https://raw.githubusercontent.com/can1357/rush/main/schema.json",
	"options": {
		"disable_provider_auto_update": true
	}
}
```

Or set the `RUSH_DISABLE_PROVIDER_AUTO_UPDATE` environment variable:

```bash
export RUSH_DISABLE_PROVIDER_AUTO_UPDATE=1
```

### Manually updating providers

Manually updating providers is possible with the `rush update-providers`
command:

```bash
# Update providers remotely from Catwalk.
rush update-providers

# Update providers from a custom Catwalk base URL.
rush update-providers https://example.com/

# Update providers from a local file.
rush update-providers /path/to/local-providers.json

# Reset providers to the embedded version, embedded at rush at build time.
rush update-providers embedded

# For more info:
rush update-providers --help
```

## Metrics

Rush records pseudonymous usage metrics (tied to a device-specific hash),
which maintainers rely on to inform development and support priorities. The
metrics include solely usage metadata; prompts and responses are NEVER
collected.

Details on exactly what’s collected are in the source code ([here](https://github.com/can1357/rush/tree/main/internal/event)
and [here](https://github.com/can1357/rush/blob/main/internal/llm/agent/event.go)).

You can opt out of metrics collection at any time by setting the environment
variable by setting the following in your environment:

```bash
export RUSH_DISABLE_METRICS=1
```

Or by setting the following in your config:

```json
{
	"options": {
		"disable_metrics": true
	}
}
```

Rush also respects the [`DO_NOT_TRACK`](https://consoledonottrack.com)
convention which can be enabled via `export DO_NOT_TRACK=1`.

## A Note on Claude Max and GitHub Copilot

Rush only supports model providers through official, compliant APIs. We do not
support or endorse any methods that rely on personal Claude Max and GitHub
Copilot accounts or OAuth workarounds, which violate Anthropic and
Microsoft’s Terms of Service.

We’re committed to building sustainable, trusted integrations with model
providers. If you’re a provider interested in working with us,
[reach out](mailto:vt100@charm.sh).

## Contributing

See the [contributing guide](https://github.com/can1357/rush?tab=contributing-ov-file#contributing).

## Whatcha think?

We’d love to hear your thoughts on this project. Need help? We gotchu. You can find us on:

- [Twitter](https://twitter.com/charmcli)
- [Slack](https://charm.land/slack)
- [Discord][discord]
- [The Fediverse](https://mastodon.social/@charmcli)
- [Bluesky](https://bsky.app/profile/charm.land)

[discord]: https://charm.land/discord

## License

This fork maintains the original [FSL-1.1-MIT](https://github.com/can1357/rush/raw/main/LICENSE.md) license from Charm's Crush project.

---

Original project by [Charm](https://charm.land).

<a href="https://charm.land/"><img alt="The Charm logo" width="400" src="https://stuff.charm.sh/charm-banner-next.jpg" /></a>

<!--prettier-ignore-->
Charm热爱开源 • Charm loves open source
