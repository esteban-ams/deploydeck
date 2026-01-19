# FastShip Architecture

This document describes the internal architecture of FastShip for developers who want to contribute or understand how it works.

## Overview

FastShip is built with simplicity and reliability in mind. It's a single Go binary that:

1. Listens for HTTP webhooks
2. Verifies the request authenticity
3. Executes Docker Compose commands
4. Monitors deployment health
5. Optionally rolls back on failure

## Component Diagram

```
                                    ┌─────────────────────────────────────┐
                                    │            FastShip                 │
                                    │                                     │
┌──────────────┐                    │  ┌─────────────┐  ┌─────────────┐  │
│   CI/CD      │  POST /api/deploy  │  │   Webhook   │  │   Deploy    │  │
│  (GitHub,    │───────────────────►│  │   Handler   │─►│   Engine    │  │
│   GitLab)    │                    │  └─────────────┘  └──────┬──────┘  │
└──────────────┘                    │         │                │         │
                                    │         ▼                ▼         │
                                    │  ┌─────────────┐  ┌─────────────┐  │
                                    │  │    Auth     │  │   Docker    │  │
                                    │  │  Verifier   │  │   Client    │  │
                                    │  └─────────────┘  └──────┬──────┘  │
                                    │                          │         │
┌──────────────┐                    │  ┌─────────────┐         │         │
│   Browser    │  GET /dashboard    │  │    Web      │         │         │
│              │◄──────────────────►│  │  Dashboard  │         │         │
└──────────────┘                    │  └─────────────┘         │         │
                                    │         │                │         │
                                    │         ▼                ▼         │
                                    │  ┌─────────────────────────────┐   │
                                    │  │         Store (SQLite)      │   │
                                    │  │   - Deployment history      │   │
                                    │  │   - Image versions          │   │
                                    │  └─────────────────────────────┘   │
                                    └─────────────────────────────────────┘
                                                       │
                                                       │ Docker Socket
                                                       ▼
                                    ┌─────────────────────────────────────┐
                                    │           Docker Engine             │
                                    │  ┌─────────┐ ┌─────────┐ ┌───────┐ │
                                    │  │Container│ │Container│ │  ...  │ │
                                    │  └─────────┘ └─────────┘ └───────┘ │
                                    └─────────────────────────────────────┘
```

## Core Components

### 1. Webhook Handler (`internal/webhook/`)

Responsible for:
- Parsing incoming HTTP requests
- Extracting service name from URL path
- Passing to auth verifier
- Returning appropriate responses

```go
// Handler interface
type Handler interface {
    HandleDeploy(w http.ResponseWriter, r *http.Request)
    HandleRollback(w http.ResponseWriter, r *http.Request)
    HandleHealth(w http.ResponseWriter, r *http.Request)
}
```

### 2. Auth Verifier (`internal/webhook/verify.go`)

Implements HMAC-SHA256 verification compatible with GitHub webhooks:

```go
// Verification flow
func VerifySignature(secret string, body []byte, signature string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

Supports multiple auth methods:
- `X-Hub-Signature-256`: GitHub-style HMAC
- `X-FastShip-Secret`: Simple shared secret
- `X-GitLab-Token`: GitLab webhook token

### 3. Deploy Engine (`internal/deploy/`)

Orchestrates the deployment process:

```
┌─────────────────────────────────────────────────────────────┐
│                    Deployment Flow                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. Receive deploy request                                  │
│           │                                                 │
│           ▼                                                 │
│  2. Save current image (for rollback)                       │
│           │                                                 │
│           ▼                                                 │
│  3. docker compose pull <service>                           │
│           │                                                 │
│           ▼                                                 │
│  4. docker compose up -d <service>                          │
│           │                                                 │
│           ▼                                                 │
│  5. Health check loop (with timeout)                        │
│           │                                                 │
│      ┌────┴────┐                                            │
│      ▼         ▼                                            │
│  SUCCESS    FAILURE                                         │
│      │         │                                            │
│      │         ▼                                            │
│      │    6. Rollback to previous image                     │
│      │         │                                            │
│      ▼         ▼                                            │
│  7. Record deployment in store                              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 4. Docker Client (`internal/docker/`)

Wraps Docker and Docker Compose operations:

```go
type Client interface {
    // Pull a specific image
    Pull(ctx context.Context, image string) error

    // Execute docker compose commands
    ComposeUp(ctx context.Context, opts ComposeOptions) error
    ComposePull(ctx context.Context, opts ComposeOptions) error

    // Get current image for a container
    GetCurrentImage(ctx context.Context, container string) (string, error)

    // Health check
    IsHealthy(ctx context.Context, container string) (bool, error)
}

type ComposeOptions struct {
    ComposeFile string
    Service     string
    WorkingDir  string
    Env         map[string]string
}
```

Implementation uses `os/exec` to call `docker` and `docker compose` commands directly, rather than the Docker SDK, for simplicity and compatibility.

### 5. Store (`internal/store/`)

SQLite-based storage for deployment history:

```sql
-- Schema
CREATE TABLE deployments (
    id TEXT PRIMARY KEY,
    service TEXT NOT NULL,
    status TEXT NOT NULL,  -- pending, running, success, failed, rolled_back
    image TEXT,
    previous_image TEXT,
    started_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    error_message TEXT,
    triggered_by TEXT      -- github, gitlab, manual, api
);

CREATE TABLE images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    service TEXT NOT NULL,
    image TEXT NOT NULL,
    deployed_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_deployments_service ON deployments(service);
CREATE INDEX idx_deployments_started_at ON deployments(started_at DESC);
```

### 6. Web Dashboard (`web/`)

Built with:
- **templ**: Type-safe HTML templates
- **HTMX**: Dynamic updates without JavaScript
- **Echo**: HTTP router

Features:
- Real-time deployment status
- Deployment history
- Manual deploy/rollback buttons
- Service overview

## Request Flow

### Deploy Request

```
1. HTTP POST /api/deploy/myapp
   Headers: X-FastShip-Secret: <secret>
   Body: {"image": "ghcr.io/user/app:latest"}

2. webhook.Handler.HandleDeploy()
   - Extract service name from path
   - Read and validate body

3. webhook.Verify()
   - Check signature/secret
   - Return 401 if invalid

4. deploy.Engine.Deploy()
   - Create deployment record (status: pending)
   - Get current image (for rollback)
   - Update status: running

5. docker.Client.ComposePull()
   - Execute: docker compose -f <file> pull <service>

6. docker.Client.ComposeUp()
   - Execute: docker compose -f <file> up -d <service>

7. deploy.HealthChecker.Wait()
   - Poll health endpoint until healthy or timeout

8. On success:
   - Update deployment status: success
   - Return 200 OK

9. On failure:
   - Execute rollback
   - Update deployment status: failed
   - Return 500 with error
```

## Configuration Loading

```go
type Config struct {
    Server    ServerConfig             `yaml:"server"`
    Auth      AuthConfig               `yaml:"auth"`
    Dashboard DashboardConfig          `yaml:"dashboard"`
    Logging   LoggingConfig            `yaml:"logging"`
    Services  map[string]ServiceConfig `yaml:"services"`
}

// Loading priority:
// 1. CLI flags (--port, --config)
// 2. Environment variables (FASTSHIP_*)
// 3. Config file (config.yaml)
// 4. Defaults
```

## Error Handling

FastShip uses structured errors with context:

```go
type DeployError struct {
    Service   string
    Phase     string // pull, up, health_check, rollback
    Cause     error
    Timestamp time.Time
}

func (e *DeployError) Error() string {
    return fmt.Sprintf("deploy %s failed at %s: %v",
        e.Service, e.Phase, e.Cause)
}
```

## Concurrency Model

- Each deployment runs in its own goroutine
- Deployments for the same service are serialized (mutex per service)
- Different services can deploy concurrently
- Store access is serialized via SQLite's built-in locking

```go
type Engine struct {
    mu       sync.Mutex
    services map[string]*sync.Mutex  // Per-service locks
    // ...
}

func (e *Engine) Deploy(ctx context.Context, service string, opts DeployOptions) {
    // Get or create service lock
    e.mu.Lock()
    svcMu, ok := e.services[service]
    if !ok {
        svcMu = &sync.Mutex{}
        e.services[service] = svcMu
    }
    e.mu.Unlock()

    // Lock this service
    svcMu.Lock()
    defer svcMu.Unlock()

    // ... deployment logic
}
```

## Testing Strategy

### Unit Tests
- Config parsing
- Auth verification
- Deployment state machine

### Integration Tests
- Docker client operations (requires Docker)
- Full deploy flow with mock containers

### E2E Tests
- Complete webhook -> deploy flow
- Dashboard interactions

## Future Considerations

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
