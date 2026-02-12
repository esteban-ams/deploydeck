# FastShip Architecture

This document describes the internal architecture of FastShip for developers who want to contribute or understand how it works.

## Overview

FastShip is built with simplicity and reliability in mind. It's a single Go binary that:

1. Listens for HTTP webhooks
2. Verifies the request authenticity
3. Parses webhook payload (build mode) or deploy request (pull mode)
4. Executes Git clone (build mode) and Docker Compose commands
5. Monitors deployment health
6. Rolls back on failure via image tagging

## Component Diagram

```
                                    +-------------------------------------+
                                    |            FastShip                  |
                                    |                                     |
+------------------+                |  +-----------+    +-----------+     |
|   CI/CD          |  POST /deploy  |  |  Webhook  |    |  Deploy   |     |
|  (GitHub,        |--------------->|  |  Handler  |--->|  Engine   |     |
|   GitLab)        |                |  +-----------+    +-----+-----+     |
+------------------+                |       |                 |           |
                                    |       v                 v           |
                                    |  +-----------+    +-----------+     |
                                    |  |   Auth    |    |  Docker   |     |
                                    |  | Verifier  |    |  Client   |     |
                                    |  +-----------+    +-----------+     |
                                    |       |                 |           |
                                    |       v                 v           |
                                    |  +-----------+    +-----------+     |
                                    |  | Payload   |    |   Git     |     |
                                    |  |  Parser   |    |  Client   |     |
                                    |  +-----------+    +-----------+     |
                                    +-------------------------------------+
                                                          |
                                                          | Docker Socket
                                                          v
                                    +-------------------------------------+
                                    |           Docker Engine              |
                                    |  +---------+ +---------+ +-------+  |
                                    |  |Container| |Container| |  ...  |  |
                                    |  +---------+ +---------+ +-------+  |
                                    +-------------------------------------+
```

## Core Components

### 1. Webhook Handler (`internal/webhook/handler.go`)

Responsible for:
- Parsing incoming HTTP requests
- Extracting service name from URL path
- Routing to auth verifier
- Branching by deploy mode (pull vs build)
- Returning JSON responses

Endpoints:
- `POST /api/deploy/:service` — Trigger deployment
- `POST /api/rollback/:service` — Manual rollback (stub)
- `GET /api/deployments` — List deployments
- `GET /api/health` — Health check with uptime

### 2. Auth Verifier (`internal/webhook/verify.go`)

Implements webhook authentication with three methods:

| Method | Header | Verification |
|--------|--------|-------------|
| GitHub | `X-Hub-Signature-256` | HMAC-SHA256 |
| GitLab | `X-GitLab-Token` | Token comparison |
| FastShip | `X-FastShip-Secret` | HMAC or token |

All comparisons use `hmac.Equal()` for constant-time comparison (prevents timing attacks).

Returns `AuthMethod` to identify the provider for payload parsing.

### 3. Payload Parser (`internal/webhook/payload.go`)

Parses push event payloads from GitHub and GitLab webhooks:

- **GitHub**: Extracts `repository.clone_url`, `ref`, `after` (commit SHA)
- **GitLab**: Extracts `project.http_url`, `ref`, `after`
- **Branch extraction**: Converts `refs/heads/main` to `main`

Used in build mode to determine what to clone and which branch to build.

### 4. Deploy Engine (`internal/deploy/deploy.go`)

Orchestrates the 7-step deployment pipeline:

```
Step 1: Save current image + tag rollback snapshot
Step 2: Clone repo (build) or pull image (pull)
Step 3: Build image (build mode only)
Step 4: docker compose up -d
Step 5: Health check polling
Step 6: Mark success / handle failure + rollback
Step 7: Cleanup old rollback tags
Step 8: Auto-prune build cache (build mode, optional)
```

**Deployment States**: `pending` -> `running` -> `success` | `failed` | `rolled_back`

**Rollback mechanism**: Before each deploy, the current image is tagged as `service:rollback-<timestamp>`. On failure, the tag is restored and `docker compose up -d` brings back the previous version.

**Timeouts**: Each deployment gets a `context.WithTimeout` (default 5m pull, 10m build). Rollback gets its own 2-minute timeout.

### 5. Health Checker (`internal/deploy/health.go`)

Polls an HTTP endpoint until it returns 2xx or times out:
- Configurable timeout, interval, and retry count
- Context-aware cancellation
- Non-blocking checks

### 6. Docker Client (`internal/docker/docker.go`)

Wraps Docker CLI via `os/exec` (not Docker SDK):

| Method | Command |
|--------|---------|
| `ComposePull` | `docker compose pull <service>` |
| `ComposeBuild` | `docker compose build <service>` |
| `ComposeUp` | `docker compose up -d <service>` |
| `GetCurrentImage` | `docker inspect -f {{.Config.Image}}` |
| `GetContainerName` | `docker compose ps -q` + `docker inspect` |
| `TagImage` | `docker tag <source> <target>` |
| `RemoveImage` | `docker rmi <image>` |
| `ListImagesByFilter` | `docker images --filter reference=<pattern>` |
| `BuilderPrune` | `docker builder prune -f` |

Why `os/exec` instead of Docker SDK: simpler code, no version compatibility issues, works with any Docker version, easy to debug.

### 7. Git Client (`internal/git/git.go`)

Handles repository cloning for build mode:

- Shallow clone (`--depth 1`) for speed
- Automatic token injection per provider:
  - GitHub: `x-access-token:<token>`
  - GitLab: `oauth2:<token>`
  - Other: `token:<token>`
- Clean clone (removes existing directory first)

## Concurrency Model

- Each deployment runs in its own goroutine
- Deployments for the same service are serialized (per-service mutex)
- Different services deploy concurrently
- In-memory state protected by global mutex

```go
type Engine struct {
    mu            sync.Mutex
    serviceLocks  map[string]*sync.Mutex  // Per-service locks
    deployments   map[string]*Deployment  // In-memory state
    dockerClient  *docker.Client
    gitClient     *git.Client
    healthChecker *HealthChecker
    config        *config.Config
}
```

## Configuration Loading

```
1. CLI flags (--port, --config)
   |
   v (overrides)
2. Environment variables (FASTSHIP_*)
   |
   v (overrides)
3. Config file (config.yaml)
   |
   v (overrides)
4. Default values
```

Token resolution priority: `clone_token` (YAML) > `clone_token_file` > `FASTSHIP_CLONE_TOKEN` (env)

## Testing Strategy

### Unit Tests
- Config parsing, validation, defaults
- Auth verification (all 3 methods)
- Payload parsing (GitHub, GitLab)
- Health check polling

### Integration Tests
- Docker client operations (requires Docker)
- Full deploy flow with test containers

### E2E Tests
- Complete webhook -> deploy flow

## Future Considerations

### Planned
- SQLite persistence for deployment history
- Web dashboard (Templ + HTMX)
- Structured logging

### Potential Enhancements
- Kubernetes support
- Slack/Discord notifications
- Deployment approvals
- Multi-node support
- Prometheus metrics

### Non-Goals
- Full container orchestration (use Kubernetes)
- CI/CD pipeline features (use GitHub Actions)
- Log aggregation (use Loki/ELK)
