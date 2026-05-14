# CCP

CCP is a local CLI and HTTP proxy for using Claude Code with configurable model providers through an Anthropic-compatible endpoint.

It is not a Claude login bypass. It uses Claude Code's supported `ANTHROPIC_BASE_URL` and `ANTHROPIC_API_KEY` path.

## Build

```powershell
go build -o .\bin\ccp.exe .\cmd\ccp
```

## Configure

Copy `configs/config.example.yaml` to `~/.ccp/config.yaml` or pass it explicitly with `--config`.

Provider keys can be direct values or environment references:

```yaml
api_key: ${DEEPSEEK_API_KEY}
api_key: sk-local-value
```

Each provider can also control its own outbound proxy:

```yaml
providers:
  openai:
    type: openai-compatible
    base_url: https://api.openai.com
    api_key: ${OPENAI_API_KEY}
    proxy:
      enabled: true
      url: http://127.0.0.1:7897
```

Set `proxy.enabled: false` or omit the block to connect directly.

## Concurrency Protection

CCP supports global and per-provider concurrency limits so a burst of Claude Code or subagent requests does not exhaust the local proxy process:

```yaml
server:
  max_concurrent_requests: 64

providers:
  openai:
    max_concurrent_requests: 16
```

When a limit is full, CCP waits briefly for capacity. If no slot becomes available, it returns `503 busy` instead of letting the proxy overload.

Recommended local setup:

```powershell
$env:DEEPSEEK_API_KEY="<set-this-in-your-current-shell>"
```

Default DeepSeek aliases:

```yaml
aliases:
  haiku: deepseek-anthropic:deepseek-v4-flash
  sonnet: deepseek-anthropic:deepseek-v4-pro
  opus: deepseek-anthropic:deepseek-v4-pro
```

## Validate

```powershell
.\bin\ccp.exe --config .\configs\config.example.yaml doctor
.\bin\ccp.exe --config .\configs\config.example.yaml test haiku "hello"
```

## Run With Claude Code

Start the proxy:

```powershell
.\bin\ccp.exe --config .\configs\config.example.yaml serve
```

In another shell:

```powershell
$env:ANTHROPIC_BASE_URL="http://127.0.0.1:8787"
$env:ANTHROPIC_API_KEY="local-dev-key"
claude
```

You can print the environment commands with:

```powershell
.\bin\ccp.exe --config .\configs\config.example.yaml env
```

## Logs

Default request log:

```text
~/.ccp/logs/ccp.log
```

Default usage log:

```text
~/.ccp/logs/usage.jsonl
```

Prompts and full API keys are not logged by default.
