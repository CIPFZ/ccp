# CCP MVP Design

## Goal

Build a local CLI and HTTP proxy named `ccp` that lets Claude Code call multiple model providers through a single Anthropic-compatible endpoint.

The MVP validates the core path:

1. User starts `ccp serve`.
2. Claude Code points `ANTHROPIC_BASE_URL` at the local proxy.
3. Claude Code sends Anthropic Messages API requests to `ccp`.
4. `ccp` resolves model aliases and forwards the request to a configured provider.
5. `ccp` returns an Anthropic-compatible response to Claude Code.

The MVP is not a login bypass. It uses Claude Code's supported API key and base URL configuration path.

## Runtime And Distribution

Use Go for the implementation.

Reasons:

- Produces a single Windows-friendly binary.
- Avoids Node, npm, pnpm, and runtime environment setup.
- Fits a local long-running proxy well.
- Standard library covers most HTTP, streaming, logging, and file needs.

Initial stack:

- Go 1.22 or newer.
- `net/http` for the HTTP server.
- `log/slog` for structured logging.
- `gopkg.in/yaml.v3` for YAML config.
- `github.com/spf13/cobra` for CLI commands.

Avoid SDKs for provider calls in the MVP. Use direct HTTP so provider differences are visible and controllable.

## Claude Code Integration

Claude Code already supports API-key and base-URL based usage through these environment variables:

```powershell
$env:ANTHROPIC_BASE_URL="http://127.0.0.1:8787"
$env:ANTHROPIC_API_KEY="local-dev-key"
claude
```

`ANTHROPIC_API_KEY` is only used to satisfy Claude Code and authenticate to the local proxy if local auth is enabled later. Provider API keys are configured in `ccp`.

## CLI

The MVP exposes these commands:

- `ccp serve`: start the local proxy.
- `ccp doctor`: validate config, provider keys, directories, and port availability.
- `ccp env`: print shell commands for configuring Claude Code.
- `ccp test <alias> <prompt>`: send a direct test request through the configured provider path.

Future command:

- `ccp usage`: summarize request and token usage from usage logs.

## HTTP API

MVP route:

- `POST /v1/messages`

Optional diagnostic routes:

- `GET /healthz`
- `GET /v1/models`

`/v1/messages` supports:

- Non-streaming requests.
- Streaming requests with `stream: true`.
- Basic text messages.
- System prompt.
- Tool definitions and tool calls as required by Claude Code agent loops.

## Provider Types

The MVP supports two provider types.

### `anthropic-compatible`

This provider type accepts Anthropic Messages API requests directly.

For DeepSeek:

```yaml
base_url: https://api.deepseek.com/anthropic
```

This is the fastest validation path because the proxy mostly handles alias routing, auth headers, request ids, logging, and error normalization.

### `openai-compatible`

This provider type accepts OpenAI-compatible chat/completions requests.

For DeepSeek:

```yaml
base_url: https://api.deepseek.com
```

This validates the protocol conversion needed for OpenAI, DeepSeek OpenAI mode, MinMax, and other OpenAI-compatible providers.

## Model Aliases

Aliases map Claude-style names to configured provider models.

MVP alias format:

```text
<alias>: <provider-name>:<provider-model>
```

Default DeepSeek validation mapping:

```yaml
aliases:
  haiku: deepseek-anthropic:deepseek-v4-flash
  sonnet: deepseek-anthropic:deepseek-v4-pro
  opus: deepseek-anthropic:deepseek-v4-pro
```

The proxy also resolves full Claude model ids when possible. For example, a Claude Code request for an Opus model can route through the `opus` alias.

## Config

Default path:

```text
~/.ccp/config.yaml
```

Example:

```yaml
server:
  host: 127.0.0.1
  port: 8787

log:
  level: info
  file: ~/.ccp/logs/ccp.log
  prompt: false

usage:
  enabled: true
  file: ~/.ccp/logs/usage.jsonl

aliases:
  haiku: deepseek-anthropic:deepseek-v4-flash
  sonnet: deepseek-anthropic:deepseek-v4-pro
  opus: deepseek-anthropic:deepseek-v4-pro

providers:
  deepseek-anthropic:
    type: anthropic-compatible
    base_url: https://api.deepseek.com/anthropic
    api_key: ${DEEPSEEK_API_KEY}

  deepseek-openai:
    type: openai-compatible
    base_url: https://api.deepseek.com
    api_key: ${DEEPSEEK_API_KEY}
```

## API Key Resolution

`api_key` supports both direct values and environment references.

Supported forms:

```yaml
api_key: ${DEEPSEEK_API_KEY}
api_key: sk-xxxx
```

Rules:

- `${NAME}` reads environment variable `NAME`.
- Plain text is used directly.
- Missing or empty values are reported by `ccp doctor`.
- Logs and errors always mask keys.

The older shape below may be accepted later for compatibility, but it is not the primary MVP syntax:

```yaml
api_key_env: DEEPSEEK_API_KEY
```

## Logging

Default log path:

```text
~/.ccp/logs/ccp.log
```

Request logs include:

- Request id.
- Provider.
- Model.
- Alias.
- Streaming flag.
- Duration.
- HTTP status.
- Error summary.

By default logs do not include prompts, messages, tool payloads, or full API keys.

`log.prompt: true` can be added later for local debugging. It should remain off by default.

## Usage Statistics

Usage statistics are part of the overall plan, but not required to block the first proxy validation.

Initial storage:

```text
~/.ccp/logs/usage.jsonl
```

One JSON object per request:

```json
{"ts":"2026-05-13T12:00:00+08:00","provider":"deepseek-anthropic","model":"deepseek-v4-pro","alias":"sonnet","status":200,"duration_ms":1820,"input_tokens":1200,"output_tokens":360,"total_tokens":1560,"estimated":false}
```

Fields:

- `request_id`
- `ts`
- `provider`
- `model`
- `alias`
- `stream`
- `status`
- `duration_ms`
- `input_tokens`
- `output_tokens`
- `total_tokens`
- `estimated`
- `error`

Token source rules:

- If upstream returns usage, record it as real usage with `estimated: false`.
- If usage is unavailable, leave token fields empty or estimate later with `estimated: true`.
- Do not calculate money in the MVP.

Future command:

```powershell
ccp usage
ccp usage --today
ccp usage --provider deepseek-anthropic
ccp usage --since 2026-05-01
```

## Roadmap

### Phase 0: Project Skeleton

Create Go module, CLI command structure, config loader, logging setup, README, and sample config.

### Phase 1: DeepSeek Anthropic-Compatible Pass-Through

Implement `provider.type: anthropic-compatible` and `POST /v1/messages` pass-through.

This phase validates:

- Claude Code can call the local proxy.
- Alias resolution works.
- Config and API key resolution work.
- Non-streaming and streaming pass-through work against DeepSeek's Anthropic endpoint.

### Phase 2: OpenAI-Compatible Conversion

Implement `provider.type: openai-compatible`.

This phase validates:

- Anthropic request to OpenAI-compatible request conversion.
- OpenAI-compatible non-streaming response to Anthropic response conversion.
- OpenAI-compatible SSE to Anthropic SSE conversion.

### Phase 3: Tool Calling

Implement the minimum tool conversion needed for Claude Code agent loops.

Requirements:

- Convert Anthropic tool definitions to provider-compatible tool definitions.
- Convert provider tool calls to Anthropic `tool_use` blocks.
- Convert Anthropic `tool_result` messages into provider-compatible messages.

### Phase 4: Diagnostics And Config Hardening

Implement and harden:

- `ccp doctor`
- `ccp env`
- `ccp test`
- Better provider error messages.
- Config validation for aliases and providers.

### Phase 5: Usage Statistics

Add request usage recording and `ccp usage` summary command.

This phase should not block earlier validation.

### Phase 6: Provider Overrides

Add provider-specific options:

- Custom headers.
- Auth scheme.
- Model parameter mapping.
- Disable tools.
- Disable streaming.
- Max token field overrides.

### Phase 7: Packaging

Add Windows build script, release artifact layout, and quickstart documentation.

## Validation Strategy

Start with DeepSeek because it supports both Anthropic and OpenAI-compatible protocols.

Validation order:

1. `ccp test haiku "hello"` against `deepseek-anthropic:deepseek-v4-flash`.
2. `ccp test sonnet "hello"` against `deepseek-anthropic:deepseek-v4-pro`.
3. Claude Code with `ANTHROPIC_BASE_URL=http://127.0.0.1:8787`.
4. Repeat tests through `deepseek-openai` after OpenAI-compatible conversion exists.
5. Add GPT, MinMax, and other providers after the DeepSeek paths are stable.

## Security And Privacy

- Never log full API keys.
- Do not record prompts by default.
- Do not commit real API keys.
- Keep local proxy bound to `127.0.0.1` by default.
- Treat config files as local secrets if direct API keys are used.

