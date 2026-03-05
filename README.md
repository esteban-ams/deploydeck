# DeployDeck

Your container deployment command center. Deploy on push. No polling. No complexity.

[![Build](https://github.com/esteban-ams/deploydeck/actions/workflows/build.yml/badge.svg)](https://github.com/esteban-ams/deploydeck/actions/workflows/build.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/esteban-ams/deploydeck)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## The Problem

When you push code, your CI/CD builds a Docker image and pushes it to a registry. But how do you tell your server to pull the new image?

```
Developer  ──►  GitHub Actions  ──►  Registry
                                        │
                                   How to deploy?
                                        ▼
                                     ¿¿¿???
```

| Option | Problem |
|--------|---------|
| SSH manually | Log in every time |
| SSH from CI/CD | Exposes server IP + SSH keys |
| Watchtower | Polls every X minutes, not instant |
| Coolify/Portainer | Overkill for simple deployments |

## The Solution

DeployDeck runs on your server and listens for webhooks. It supports two deployment modes:

**Pull Mode** — Your CI/CD builds the image, pushes to a registry, then calls DeployDeck:

```
GitHub Actions ──► build + push image ──► POST /api/deploy/myapp ──► DeployDeck
                                                                        │
                                                              docker compose pull
                                                              docker compose up -d
                                                              health check ✓
```

**Build Mode** — DeployDeck receives a push webhook, clones the repo, and builds on the server:

```
git push ──► GitHub webhook ──► DeployDeck ──► git clone
                                               │
                                      docker compose build
                                      docker compose up -d
                                      health check ✓
```

## Features

- **Two deploy modes**: Pull pre-built images or build from source
- **Webhook auth**: HMAC-SHA256 (GitHub), token (GitLab), shared secret (DeployDeck)
- **Health checks**: Configurable timeout, interval, and retries
- **Automatic rollback**: Image tagging snapshots before each deploy; restores on failure
- **Branch filtering**: Only deploy pushes to the configured branch (build mode)
- **Per-service timeouts**: Default 5m (pull) or 10m (build), configurable
- **Token security**: Read tokens from files (Docker Secrets), env vars, or config
- **Auto-prune**: Clean Docker build cache after successful builds
- **Async deployments**: Per-service serialization, different services deploy concurrently

## Quick Start (Pull Mode)

### 1. Download

```bash
curl -L https://github.com/esteban-ams/deploydeck/releases/latest/download/deploydeck-linux-amd64 -o deploydeck
chmod +x deploydeck
```

### 2. Configure

```yaml
# config.yaml
server:
  port: 9000

auth:
  webhook_secret: "your-secret-here"  # openssl rand -hex 32

services:
  myapp:
    compose_file: "/opt/apps/docker-compose.yml"
    service_name: "myapp"
    health_check:
      enabled: true
      url: "http://localhost:8080/health"
    rollback:
      enabled: true
```

### 3. Run

```bash
./deploydeck --config config.yaml
```

### 4. Trigger from CI/CD

**GitHub Actions:**
```yaml
- name: Deploy
  run: |
    curl -X POST https://deploy.yourdomain.com/api/deploy/myapp \
      -H "X-DeployDeck-Secret: ${{ secrets.DEPLOYDECK_SECRET }}" \
      -H "Content-Type: application/json" \
      -d '{"image": "ghcr.io/user/myapp:latest"}'
```

**GitLab CI:**
```yaml
deploy:
  script:
    - |
      curl -X POST https://deploy.yourdomain.com/api/deploy/myapp \
        -H "X-GitLab-Token: $DEPLOYDECK_SECRET" \
        -H "Content-Type: application/json" \
        -d '{"image": "registry.gitlab.com/user/myapp:latest"}'
```

## Quick Start (Build Mode)

Build mode clones your repo and builds the image directly on the server. No registry needed.

### 1. Configure

```yaml
# config.yaml
services:
  myapp:
    mode: "build"
    branch: "main"
    repo: "https://github.com/user/myapp.git"       # fallback URL
    clone_token_file: "/run/secrets/github_token"     # for private repos
    compose_file: "docker-compose.yml"                # relative to repo root
    service_name: "myapp"
    working_dir: "/opt/builds/myapp"
    timeout: 15m
    prune_after_build: true
    health_check:
      enabled: true
      url: "http://localhost:8080/health"
    rollback:
      enabled: true
```

### 2. Set up GitHub Webhook

In your GitHub repo: **Settings > Webhooks > Add webhook**

- **Payload URL**: `https://deploy.yourdomain.com/api/deploy/myapp`
- **Content type**: `application/json`
- **Secret**: Your `webhook_secret`
- **Events**: Just the push event

Now every push to `main` triggers a build and deploy automatically.

## API Reference

### POST /api/deploy/:service

Trigger a deployment. Requires authentication header.

**Headers** (use one):
- `X-Hub-Signature-256: sha256=<hmac>` (GitHub)
- `X-GitLab-Token: <secret>` (GitLab)
- `X-DeployDeck-Secret: <secret>` (DeployDeck)

**Body (pull mode):**
```json
{"image": "ghcr.io/user/app:latest", "tag": "v1.2.3"}
```

**Body (build mode):** GitHub/GitLab push webhook payload (automatic).

**Response:**
```json
{"status": "pending", "deployment_id": "dep_123", "service": "myapp"}
```

**Response (branch filtered, build mode):**
```json
{"status": "skipped", "reason": "push to develop, expected main"}
```

### GET /api/deployments

List all deployments (in-memory, not persisted across restarts).

```json
{
  "deployments": [
    {
      "id": "dep_123",
      "service": "myapp",
      "status": "success",
      "mode": "pull",
      "image": "ghcr.io/user/app:latest",
      "rollback_tag": "myapp:rollback-1707750000",
      "started_at": "2026-02-12T10:00:00Z",
      "completed_at": "2026-02-12T10:01:15Z"
    }
  ]
}
```

Deployment statuses: `pending`, `running`, `success`, `failed`, `rolled_back`.

### POST /api/rollback/:service

Manual rollback endpoint (stub — will be fully implemented with persistent storage).

### GET /api/health

```json
{"status": "healthy", "version": "0.1.0", "uptime": "48h30m"}
```

## Configuration Reference

```yaml
server:
  port: 9000                          # default: 9000
  host: "0.0.0.0"                     # default: 0.0.0.0
  tls:
    enabled: false
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"

auth:
  webhook_secret: "your-secret"       # REQUIRED

dashboard:
  enabled: false                      # planned feature
  username: "admin"
  password: "change-me"

logging:
  level: "info"                       # debug, info, warn, error
  format: "text"                      # json, text

services:
  myapp:
    # Required
    compose_file: "/opt/apps/docker-compose.yml"
    service_name: "myapp"

    # Deploy mode
    mode: "pull"                      # "pull" (default) or "build"
    working_dir: "/opt/apps"          # working directory for compose commands

    # Build mode options
    branch: "main"                    # only deploy pushes to this branch (default: main)
    repo: "https://github.com/u/r"   # fallback clone URL if webhook lacks it
    clone_token: ""                   # auth token for private repos
    clone_token_file: ""              # read token from file (Docker Secrets)
    prune_after_build: false          # clean build cache after deploy

    # Timeouts
    timeout: 5m                       # default: 5m (pull), 10m (build)

    # Health check
    health_check:
      enabled: true
      url: "http://localhost:8080/health"
      timeout: 30s                    # total health check timeout
      interval: 2s                    # time between checks
      retries: 10                     # max attempts

    # Rollback
    rollback:
      enabled: true
      keep_images: 3                  # rollback snapshots to keep

    # Environment variables passed to docker compose
    env:
      DEPLOY_ENV: "production"
```

## Environment Variables

Environment variables override config file values:

| Variable | Overrides | Example |
|----------|-----------|---------|
| `DEPLOYDECK_PORT` | `server.port` | `9000` |
| `DEPLOYDECK_HOST` | `server.host` | `0.0.0.0` |
| `DEPLOYDECK_WEBHOOK_SECRET` | `auth.webhook_secret` | `abc123...` |
| `DEPLOYDECK_LOG_LEVEL` | `logging.level` | `debug` |
| `DEPLOYDECK_CLONE_TOKEN` | `clone_token` (all services) | `ghp_xxx...` |

**Precedence**: CLI flags > environment variables > config.yaml > defaults

## Docker Deployment

```yaml
# docker-compose.yml
services:
  deploydeck:
    image: ghcr.io/esteban-ams/deploydeck:latest
    container_name: deploydeck
    restart: unless-stopped
    ports:
      - "9000:9000"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - DEPLOYDECK_LOG_LEVEL=info
```

**Important:** DeployDeck needs access to the Docker socket to manage containers.

## Architecture

```
deploydeck/
├── cmd/deploydeck/
│   └── main.go                  # Entry point, CLI flags, server setup
├── internal/
│   ├── config/
│   │   └── config.go            # YAML parsing, env overrides, validation
│   ├── webhook/
│   │   ├── handler.go           # HTTP handlers (deploy, rollback, status, health)
│   │   ├── verify.go            # Auth verification (GitHub, GitLab, DeployDeck)
│   │   └── payload.go           # Push event parsing (GitHub/GitLab)
│   ├── deploy/
│   │   ├── deploy.go            # Deployment orchestration, rollback, state machine
│   │   └── health.go            # Health check polling
│   ├── docker/
│   │   └── docker.go            # Docker Compose CLI wrapper (pull, build, up, tag)
│   └── git/
│       └── git.go               # Git clone with token injection
├── config.example.yaml
├── Dockerfile
└── Makefile
```

**Request flow:**
```
HTTP Request → webhook/handler.go → webhook/verify.go (auth)
                     │
                     ▼
              deploy/deploy.go (orchestration)
                     │
           ┌─────────┼──────────┐
           ▼         ▼          ▼
      git/git.go  docker/    deploy/
      (clone)    docker.go   health.go
                (pull/build/  (health
                 up/tag)      check)
```

## Security

### Authentication

Three methods supported (use one per request):

| Method | Header | Format |
|--------|--------|--------|
| GitHub | `X-Hub-Signature-256` | `sha256=<hmac-sha256>` |
| GitLab | `X-GitLab-Token` | Plain token string |
| DeployDeck | `X-DeployDeck-Secret` | Plain string or `sha256=<hmac>` |

All methods use constant-time comparison to prevent timing attacks.

### Token Security for Private Repos

Clone tokens for private repos (build mode) can be provided via:

1. `clone_token` in config (least secure)
2. `clone_token_file` pointing to a file (Docker Secrets pattern, recommended)
3. `DEPLOYDECK_CLONE_TOKEN` environment variable (fallback)

**Priority**: config value > file > env var

Token injection is automatic per provider:
- GitHub: `https://x-access-token:<token>@github.com/...`
- GitLab: `https://oauth2:<token>@gitlab.com/...`

### Network Security

- Run behind a reverse proxy (Traefik, nginx) with HTTPS
- Use strong, randomly generated secrets: `openssl rand -hex 32`
- DeployDeck requires Docker socket access (runs as root or docker group)

## Development

### Prerequisites

- Go 1.22+
- Docker and Docker Compose

### Build & Run

```bash
make build          # Build binary
make build-linux    # Cross-compile for Linux amd64
make run            # Run with go run
make test           # Run all tests
make deps           # Download and tidy dependencies
make clean          # Remove build artifacts
```

### Run from source

```bash
cp config.example.yaml config.yaml
# Edit config.yaml
go run ./cmd/deploydeck --config config.yaml
```

### Test webhook

```bash
./test-webhook.sh http://localhost:9000 your-secret
```

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.
