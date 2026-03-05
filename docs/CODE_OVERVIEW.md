# DeployDeck Code Overview

A guided tour through the DeployDeck codebase for developers.

## Project Structure

```
deploydeck/
├── cmd/deploydeck/
│   └── main.go                 # Application entry point (87 lines)
├── internal/
│   ├── config/
│   │   └── config.go           # Configuration parsing
│   ├── webhook/
│   │   ├── verify.go           # Auth verification
│   │   ├── handler.go          # HTTP handlers
│   │   └── payload.go          # Webhook payload parsing
│   ├── deploy/
│   │   ├── deploy.go           # Deployment engine
│   │   └── health.go           # Health checking
│   ├── docker/
│   │   └── docker.go           # Docker operations
│   ├── git/
│   │   └── git.go              # Git clone for build mode
│   └── ratelimit/
│       └── ratelimit.go        # Per-IP rate limiting middleware
├── .github/workflows/
│   └── build.yml               # CI/CD pipeline
├── docs/
│   ├── ARCHITECTURE.md         # Architecture documentation
│   ├── CASE_STUDY.md           # Production case study
│   └── CODE_OVERVIEW.md        # This file
├── config.example.yaml         # Example configuration
├── docker-compose.yml          # DeployDeck deployment
├── docker-compose.example.yml  # Example service
├── deploydeck.service            # Systemd service
├── Dockerfile                  # Container image
├── Makefile                    # Build commands
├── go.mod                      # Go dependencies
├── README.md                   # User documentation
├── QUICKSTART.md               # Quick start guide
├── ROADMAP.md                  # Development roadmap
├── TODO.md                     # Task tracking
└── LICENSE                     # MIT License
```

**Total: 9 Go files, ~1,461 lines of code, 5 packages**

## Core Flow: Deployment Request

### Pull Mode (pre-built image)

```
1. HTTP POST /api/deploy/myapp
   │
   ▼
2. webhook.Handler.HandleDeploy()
   ├─ Extract service name from URL path
   ├─ Read request body
   ├─ Verify authentication (3 methods)
   └─ Parse image from request body
   │
   ▼
3. deploy.Engine.Deploy()
   ├─ Create deployment record (status: pending)
   ├─ Get per-service lock
   └─ Start goroutine with timeout context
   │
   ▼
4. deploy.Engine.executeDeploy() [async, 7 steps]
   ├─ Step 1: Tag current image as rollback snapshot
   ├─ Step 2: docker compose pull (new image)
   ├─ Step 3: docker compose up -d
   ├─ Step 4: Health check loop
   ├─ Step 5: Update status (success/failed/rolled_back)
   ├─ Step 6: Clean up old rollback tags (keep_images)
   └─ On failure: restore rollback image + compose up
   │
   ▼
5. HTTP Response (immediate, before deployment completes)
   └─ {"status": "deploying", "deployment_id": "dep_123", "service": "myapp"}
```

### Build Mode (clone + build from source)

```
1. HTTP POST /api/deploy/myapp  (or GitHub/GitLab webhook)
   │
   ▼
2. webhook.Handler.HandleDeploy()
   ├─ Verify authentication
   ├─ Parse webhook payload (GitHub/GitLab push event)
   └─ Check branch filter (skip if wrong branch)
   │
   ▼
3. deploy.Engine.Deploy()
   ├─ Create deployment record
   └─ Start goroutine with timeout context
   │
   ▼
4. deploy.Engine.executeDeploy() [async, 8 steps]
   ├─ Step 1: Tag current image as rollback snapshot
   ├─ Step 2: git clone --depth 1 (with token injection)
   ├─ Step 3: docker compose build
   ├─ Step 4: docker compose up -d
   ├─ Step 5: Health check loop
   ├─ Step 6: Update status (success/failed/rolled_back)
   ├─ Step 7: Clean up old rollback tags
   ├─ Step 8: Auto-prune build cache (if enabled)
   └─ On failure: restore rollback image + compose up
   │
   ▼
5. HTTP Response
   └─ {"status": "deploying", ...}
```

## Key Components Explained

### 1. cmd/deploydeck/main.go

**Purpose**: Application entry point

**Responsibilities**:
- Parse CLI flags (--config, --port, --version)
- Load configuration
- Initialize deployment engine
- Setup HTTP routes with Echo
- Start server

**Key Code**:
```go
cfg, err := config.Load(*configPath)
engine := deploy.NewEngine(cfg)
handler := webhook.NewHandler(cfg, engine)

e := echo.New()
api := e.Group("/api")
api.POST("/deploy/:service", handler.HandleDeploy)
api.POST("/rollback/:service", handler.HandleRollback)
api.GET("/deployments", handler.HandleListDeployments)
api.GET("/health", handler.HandleHealth)
```

### 2. internal/config/config.go

**Purpose**: Configuration management

**Responsibilities**:
- Parse YAML configuration
- Apply environment variable overrides
- Set default values (mode, timeouts, health check params)
- Validate configuration
- Resolve clone tokens (file, env, inline)

**Configuration Structure**:
```yaml
server:     # HTTP server settings (port, host)
auth:       # Webhook authentication (webhook_secret)
services:   # Deployable services
  myapp:
    mode: "pull"              # or "build"
    branch: "main"            # branch filter (build mode)
    repo: "https://..."       # repo URL (build mode)
    clone_token_file: "..."   # token from file (build mode)
    compose_file: "..."
    service_name: "..."
    working_dir: "..."
    timeout: 5m               # deployment timeout
    prune_after_build: false   # auto-prune cache
    health_check: {...}
    rollback: {...}
```

**Key Functions**:
- `Load(path string)` - Main entry point
- `applyEnvOverrides()` - DEPLOYDECK_* env vars
- `applyDefaults()` - Smart defaults (5m pull, 10m build, mode "pull", branch "main")
- `validate()` - Configuration validation
- `resolveTokenFiles()` - Read tokens from files (Docker Secrets pattern)

### 3. internal/webhook/verify.go

**Purpose**: Webhook authentication

**Responsibilities**:
- Verify HMAC-SHA256 signatures
- Support multiple auth methods with priority
- Prevent timing attacks (constant-time comparison)
- Return which auth method was used

**Supported Auth Methods** (checked in order):
1. **GitHub Style**: `X-Hub-Signature-256: sha256=...` (HMAC-SHA256)
2. **GitLab Style**: `X-GitLab-Token: secret` (constant-time comparison)
3. **DeployDeck Style**: `X-DeployDeck-Secret: secret` or `sha256=...` (both supported)

**Key Code**:
```go
func (v *Verifier) Verify(headers http.Header, body []byte) (bool, AuthMethod) {
    // Check GitHub HMAC first
    if sig := headers.Get("X-Hub-Signature-256"); sig != "" {
        return v.verifyHMAC(body, sig), AuthGitHub
    }
    // Then GitLab token
    if token := headers.Get("X-Gitlab-Token"); token != "" {
        return v.verifySecret(token), AuthGitLab
    }
    // Then DeployDeck secret (HMAC or plain)
    if secret := headers.Get("X-DeployDeck-Secret"); secret != "" { ... }
}
```

### 4. internal/webhook/payload.go

**Purpose**: Parse CI/CD webhook payloads

**Responsibilities**:
- Parse GitHub push event payloads
- Parse GitLab push event payloads
- Extract branch name from refs (e.g., `refs/heads/main` -> `main`)

**Key Functions**:
- `ParseGitHubPush(body)` - Extracts branch and clone URL from GitHub payload
- `ParseGitLabPush(body)` - Extracts branch and clone URL from GitLab payload
- `extractBranch(ref)` - Strips `refs/heads/` prefix from ref strings

**Key Structure**:
```go
type PushEvent struct {
    Branch   string
    CloneURL string
}
```

### 5. internal/webhook/handler.go

**Purpose**: HTTP request handlers

**Endpoints**:
- `POST /api/deploy/:service` - Trigger deployment (requires auth)
- `POST /api/rollback/:service` - Manual rollback (requires auth)
- `GET /api/deployments` - List all deployments
- `GET /api/health` - Server health check

**Deploy Handler Logic**:
1. Extract service name from path
2. Read and verify authentication
3. Look up service configuration
4. **Build mode**: parse webhook payload, check branch filter, skip if wrong branch
5. **Pull mode**: extract image from request body
6. Call deployment engine
7. Return JSON response

**Branch Filtering** (build mode only):
```go
// Returns 200 with "skipped" if push is not to configured branch
if pushEvent.Branch != svcCfg.Branch {
    return c.JSON(200, {"status": "skipped", "reason": "branch mismatch"})
}
```

### 6. internal/deploy/deploy.go

**Purpose**: Deployment orchestration (7-step pipeline)

**Responsibilities**:
- Manage deployment lifecycle with timeout
- Serialize per-service deployments (mutex per service)
- Track deployment state in memory
- Handle rollback on failure via image tagging
- Clean up old rollback images

**Deployment States**:
- `pending` - Created, not started
- `running` - In progress
- `success` - Completed successfully
- `failed` - Failed (no rollback available)
- `rolled_back` - Failed and rolled back to previous image

**Concurrency Model**:
```go
type Engine struct {
    config       *config.Config
    docker       DockerClient
    gitClient    *git.Client
    deployments  map[string]*Deployment
    serviceLocks map[string]*sync.Mutex
    mu           sync.RWMutex
}

// Per-service mutex prevents concurrent deploys to same service
// Different services deploy concurrently
go func() {
    svcLock.Lock()
    defer svcLock.Unlock()
    e.executeDeploy(ctx, cancel, deployment, svcCfg)
}()
```

**7-Step Pipeline** (pull mode):
1. Tag current image as `service:rollback-{timestamp}` (snapshot)
2. `docker compose pull` - Pull new image
3. `docker compose up -d` - Deploy service
4. Health check loop - Wait for healthy
5. Update status (success or trigger rollback)
6. Clean up old rollback tags (keep latest N per `keep_images`)
7. On failure: restore rollback image, `docker compose up -d`

**Build Mode** adds clone + build steps and auto-prune.

### 7. internal/deploy/health.go

**Purpose**: Health checking with configurable retries

**Responsibilities**:
- Poll HTTP endpoints at configured intervals
- Respect timeout, retry limits, and context cancellation
- Return nil on success, error on timeout/max retries

**Configuration**:
```yaml
health_check:
  enabled: true
  url: "http://localhost:8080/health"
  timeout: 30s      # Total timeout
  interval: 2s      # Time between checks
  retries: 10       # Max attempts
```

### 8. internal/docker/docker.go

**Purpose**: Docker CLI wrapper via os/exec

**Why Not Docker SDK?**
- Simpler code, no version conflicts
- Works with any Docker version
- Easy to debug (see exact commands)
- Smaller binary, fewer dependencies

**9 Operations**:
```go
ComposePull(file, workDir, service)          // docker compose pull
ComposeBuild(file, workDir, service)         // docker compose build
ComposeUp(file, workDir, service)            // docker compose up -d
GetCurrentImage(file, workDir, service)      // docker compose images
GetContainerName(file, workDir, service)     // docker compose ps
TagImage(source, target)                     // docker tag
RemoveImage(image)                           // docker rmi
ListImagesByFilter(filter)                   // docker images --filter
BuilderPrune()                               // docker builder prune
```

### 9. internal/git/git.go

**Purpose**: Git clone for build mode deployments

**Responsibilities**:
- Shallow clone repositories (`--depth 1`, single branch)
- Inject authentication tokens into clone URLs
- Support GitHub, GitLab, and generic Git hosts
- Clean up clone directory before re-cloning

**Token Injection by Provider**:
| Provider | URL Format |
|----------|-----------|
| GitHub   | `https://x-access-token:{token}@github.com/...` |
| GitLab   | `https://oauth2:{token}@gitlab.com/...` |
| Other    | `https://token:{token}@host/...` |

**Key Code**:
```go
func (c *Client) Clone(opts CloneOptions) error {
    repoURL := opts.RepoURL
    if opts.Token != "" {
        repoURL = injectToken(repoURL, opts.Token)
    }
    // git clone --depth 1 --branch <branch> --single-branch <url> <dir>
}
```

## Design Decisions

### 1. Why os/exec Instead of Docker SDK?

**Pros**: Simpler code, no version compatibility issues, works with any Docker version, easy to debug, smaller binary.

**Cons**: Must parse command output, less type safety, can't use advanced API features.

**Decision**: Simplicity wins for this use case.

### 2. Why In-Memory State Instead of Database?

**Current**: Faster development, no migration complexity, fewer dependencies.

**Planned**: SQLite for persistence (Phase B in roadmap).

### 3. Why Goroutines Instead of Worker Pool?

One goroutine per deployment with per-service mutex. Simple, adequate for typical load. Worker pool deferred until needed (YAGNI).

### 4. Why Echo Instead of net/http?

Path parameters (`:service`), middleware, JSON helpers, better DX. One dependency, worth it.

### 5. Why Two Deploy Modes?

**Pull mode**: Fast (just pull), ideal when CI/CD builds images externally (GitHub Actions + GHCR).

**Build mode**: No registry needed, builds on server. Ideal for private repos, small teams, or when you don't want to set up a container registry.

## Testing Strategy

### Unit Tests (Planned for Phase D)
```bash
go test ./internal/config    # Config parsing, validation, defaults
go test ./internal/webhook   # Auth verification, payload parsing
go test ./internal/deploy    # Health check logic
```

### Manual Testing
```bash
# Health endpoint
curl http://localhost:9000/api/health

# Trigger pull mode deploy
curl -X POST http://localhost:9000/api/deploy/myapp \
  -H "X-DeployDeck-Secret: secret" \
  -H "Content-Type: application/json" \
  -d '{"image": "myapp:latest"}'
```

## Common Tasks

### Adding a New Endpoint

1. Add handler in `internal/webhook/handler.go`:
```go
func (h *Handler) HandleNewEndpoint(c echo.Context) error {
    return c.JSON(http.StatusOK, response)
}
```

2. Register route in `cmd/deploydeck/main.go`:
```go
api.GET("/new-endpoint", handler.HandleNewEndpoint)
```

### Adding Configuration Option

1. Add field in `internal/config/config.go`
2. Add default in `applyDefaults()`
3. Add validation in `validate()` if needed
4. Use in code via `cfg.Services[name].NewOption`

### Adding a Deployment Step

Edit `internal/deploy/deploy.go` `executeDeploy()` function. Follow the existing step pattern with error handling and rollback support.

## Security Checklist

- [x] HMAC-SHA256 signature verification
- [x] Constant-time comparison (prevents timing attacks)
- [x] No secrets in logs
- [x] Context cancellation and timeouts
- [x] Token files (Docker Secrets pattern)
- [x] Branch filtering (deploy only on correct branch)
- [x] Rate limiting (per-IP token bucket)
- [x] Auth on GET /api/deployments
- [ ] IP whitelisting (planned)

## Getting Help

- [README.md](../README.md) - User documentation
- [ARCHITECTURE.md](./ARCHITECTURE.md) - System design
- [CASE_STUDY.md](./CASE_STUDY.md) - Production case study
- [QUICKSTART.md](../QUICKSTART.md) - Quick start guide
- [ROADMAP.md](../ROADMAP.md) - Development roadmap
