# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DeployDeck is a lightweight Go webhook server that automates Docker Compose deployments. It supports two modes: **pull** (deploy pre-built images) and **build** (clone repo + build on server). It listens for webhooks from CI/CD pipelines (GitHub Actions, GitLab CI) or direct GitHub/GitLab push events and orchestrates container deployments with health checking and rollback support.

## Build & Development Commands

```bash
make build          # Build binary for current platform
make build-linux    # Cross-compile for Linux amd64
make run            # Run with go run (expects config.yaml)
make test           # Run all tests (go test -v ./...)
make deps           # Download and tidy dependencies
make install        # Install to GOPATH/bin
make clean          # Remove build artifacts
```

Run with flags: `./deploydeck --config config.yaml --port 8000 --version`

No linter is configured.

## Architecture

**Request flow**: Webhook HTTP request → Auth verification → Payload parsing (build mode) → Deployment engine → Git clone (build) or Docker pull → Docker Compose up → Health check → Success/Rollback

### Key packages under `internal/`:

- **config/** — YAML config parsing with environment variable overrides (`DEPLOYDECK_PORT`, `DEPLOYDECK_HOST`, `DEPLOYDECK_WEBHOOK_SECRET`, `DEPLOYDECK_LOG_LEVEL`, `DEPLOYDECK_CLONE_TOKEN`). Token resolution from files (Docker Secrets pattern). Config precedence: CLI flags > env vars > config.yaml > defaults.
- **webhook/** — Echo HTTP handlers, authentication (3 methods: GitHub HMAC, GitLab token, DeployDeck secret), and push event payload parsing (GitHub/GitLab). Branch filtering for build mode.
- **deploy/** — Deployment orchestration with per-service mutex. 7-step pipeline: save image → mode-specific (clone+build or pull) → compose up → health check → success → cleanup rollback tags → auto-prune. State machine: `pending → running → success/failed/rolled_back`. Rollback via image tagging. Per-service timeouts (default 5m pull, 10m build). In-memory storage only.
- **docker/** — Wraps `docker compose` CLI via `os/exec`. Methods: ComposePull, ComposeBuild, ComposeUp, GetCurrentImage, GetContainerName, TagImage, RemoveImage, ListImagesByFilter, BuilderPrune.
- **git/** — Git clone with shallow depth and automatic token injection per provider (GitHub: x-access-token, GitLab: oauth2).

### Entry point

`cmd/deploydeck/main.go` — Sets up Echo router, registers routes, and starts the HTTP server.

## API Endpoints

- `POST /api/deploy/:service` — Trigger deployment (requires auth header). Build mode parses push webhook payload; pull mode accepts `{"image": "...", "tag": "..."}`.
- `POST /api/rollback/:service` — Manual rollback (stub, requires auth)
- `GET /api/deployments` — List all deployments (in-memory)
- `GET /api/health` — Health check with version and uptime

## Configuration

Copy `config.example.yaml` to `config.yaml`. Services support two modes (`pull` and `build`) with compose file path, working directory, health check settings, rollback options, timeouts, branch filtering, and token security.

## CI/CD

GitHub Actions (`.github/workflows/build.yml`): tests on all PRs/pushes, builds multi-platform binaries on version tags, publishes Docker image to `ghcr.io/esteban-ams/deploydeck` on main push and tags.
