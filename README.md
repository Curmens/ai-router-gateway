# auto-router

An OpenAI-compatible LLM gateway that automatically routes requests across multiple providers based on prompt complexity, budget limits, and provider health.

## How it works

```
model: "auto"  →  Classifier  →  low    → Ollama (free, local)
                               →  medium → agy (Antigravity)
                               →  high   → Claude (subscription)

model: "orchestrated"  →  Decompose → parallel subtasks → Synthesize
model: "<explicit>"    →  Registry lookup → direct provider
```

Every request goes through: budget enforcement → health check → circuit breaker → failover chain.

---

## Providers

| Name | Model | Tier | Cost |
|---|---|---|---|
| `ollama` | qwen2.5:latest | low complexity | free |
| `agy` | agy | medium complexity | free |
| `gemini` | gemini-2.5-flash | medium fallback | ~$0.0001/1k |
| `subscription` | claude-sonnet-4-5 | high complexity | subscription |
| `openai` | gpt-4o | disabled | needs key |

---

## Running

```bash
# Start all services
docker compose -f deployments/docker-compose.yml up -d

# Rebuild after code changes
docker compose -f deployments/docker-compose.yml build router
docker compose -f deployments/docker-compose.yml up -d router
```

API available at `http://localhost:8080`.

---

## API Keys (configured in config.yaml)

| Key | Role | Rate limit |
|---|---|---|
| `sk-router-admin-12345` | admin | 1000 req/min |
| `sk-router-dev-67890` | developer | 100 req/min |
| `sk-router-user-54321` | user | 10 req/min |

---

## Endpoints

```
POST /v1/chat/completions    OpenAI-compatible inference
GET  /v1/models              All registered models
GET  /v1/providers           Provider health + latency
GET  /v1/logs                Full request traces (routing decisions, complexity, reason)
GET  /v1/usage/logs          Token usage + cost summary
GET  /v1/orchestration/:id   Live blackboard status for orchestrated requests
GET  /health
GET  /metrics                Prometheus metrics
```

### Filtering /v1/logs

```
?provider=subscription    only Claude requests
?routing_type=auto        auto-routed only
?complexity=high          high-complexity classifications
?status=500               errors only
?from=2026-01-01          date range
?limit=20&offset=40       pagination
```

---

## Usage examples

### Auto-routing
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-router-admin-12345" \
  -H "Content-Type: application/json" \
  -d '{"model": "auto", "messages": [{"role": "user", "content": "your prompt"}]}'
```

### Explicit model
```bash
# Use Claude directly
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-router-admin-12345" \
  -H "Content-Type: application/json" \
  -d '{"model": "claude-sonnet-4-5", "messages": [{"role": "user", "content": "..."}]}'
```

### Multi-agent orchestration
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-router-admin-12345" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "orchestrated",
    "messages": [{"role": "user", "content": "Build a Go REST API with models, handlers, schema, and tests"}]
  }'
```

The orchestrator decomposes the prompt into subtasks, runs them in parallel across providers, then synthesizes a single response. Good for large multi-part requests.

### Check orchestration status
```bash
curl http://localhost:8080/v1/orchestration/<request_id> \
  -H "Authorization: Bearer sk-router-admin-12345"
```

### Streaming
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-router-admin-12345" \
  -H "Content-Type: application/json" \
  -d '{"model": "auto", "stream": true, "messages": [{"role": "user", "content": "..."}]}'
```

---

## Connecting OpenCode

Add to `~/.config/opencode/opencode.jsonc`:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "openai": {
      "api": "http://localhost:8080/v1",
      "env": ["AUTO_ROUTER_API_KEY"]
    }
  },
  "model": "openai/auto",
  "small_model": "openai/auto"
}
```

Set the environment variable:
```bash
# Windows (PowerShell - persists)
[Environment]::SetEnvironmentVariable("AUTO_ROUTER_API_KEY", "sk-router-admin-12345", "User")

# Linux/macOS
export AUTO_ROUTER_API_KEY=sk-router-admin-12345
```

Then restart OpenCode. All requests will route through the auto-router.

---

## Connecting Open WebUI

1. Go to `http://localhost:3000` → Settings → Connections
2. Add OpenAI connection:
   - **URL:** `http://host.docker.internal:8080/v1`
   - **Key:** `sk-router-admin-12345`
3. Save and refresh models

---

## Monitoring

```bash
# Live request logs
docker logs -f ai-router

# Prometheus metrics
open http://localhost:9090

# Provider health
curl http://localhost:8080/v1/providers -H "Authorization: Bearer sk-router-admin-12345"
```

---

## Configuration

Edit `configs/config.yaml` — the file is volume-mounted so most changes take effect after a container restart (`docker compose restart router`). Code changes require a rebuild.

Key sections:
- `providers.*` — enable/disable providers, set API keys, list models
- `routing.auto_routing` — enable/disable classifier
- `routing.failover.chains` — control provider fallback order
- `server.api_keys` — manage access keys and rate limits
