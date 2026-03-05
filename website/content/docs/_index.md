---
title: "Documentation - DeployDeck"
description: "Complete documentation for DeployDeck, the lightweight webhook server for Docker Compose deployments."
layout: "docs"
type: "docs"
---

<section id="getting-started" class="docs-section">

## Getting Started

DeployDeck is a lightweight Go webhook server that automates Docker Compose deployments. It supports two modes: **pull** (deploy pre-built images) and **build** (clone repo + build on server). It listens for webhooks from CI/CD pipelines or direct GitHub/GitLab push events and orchestrates container deployments with health checking and rollback support.

### Installation

There are three ways to install DeployDeck:

#### Binary Download

Download the latest release for your platform:

```bash
# Linux (amd64)
curl -L https://github.com/esteban-ams/deploydeck/releases/latest/download/deploydeck-linux-amd64 \
  -o /usr/local/bin/deploydeck
chmod +x /usr/local/bin/deploydeck

# macOS (arm64)
curl -L https://github.com/esteban-ams/deploydeck/releases/latest/download/deploydeck-darwin-arm64 \
  -o /usr/local/bin/deploydeck
chmod +x /usr/local/bin/deploydeck

# Verify installation
deploydeck version
```

#### Docker

Run DeployDeck as a Docker container:

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

Then start it:

```bash
docker compose up -d
```

> **Note:** DeployDeck needs access to the Docker socket to manage containers.

#### From Source

Build from source requires Go 1.22+:

```bash
git clone https://github.com/esteban-ams/deploydeck.git
cd deploydeck
go mod download
go build -o deploydeck ./cmd/deploydeck
```

Or use the Makefile:

```bash
make build          # Build binary for current platform
make build-linux    # Cross-compile for Linux amd64
make install        # Install to GOPATH/bin
```

### Quick Start

Get DeployDeck running in 5 minutes with pull mode:

**1. Create a configuration file:**

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

**2. Start DeployDeck:**

```bash
./deploydeck --config config.yaml
```

**3. Test the health endpoint:**

```bash
curl http://localhost:9000/api/health
```

**4. Trigger a deployment:**

```bash
curl -X POST http://localhost:9000/api/deploy/myapp \
  -H "X-DeployDeck-Secret: your-secret-here" \
  -H "Content-Type: application/json" \
  -d '{"image": "your-image:latest"}'
```

### Configuration Basics

DeployDeck uses a YAML configuration file. The minimum configuration requires:

- A **server port** to listen on
- An **authentication secret** for webhook verification
- At least one **service** definition with a compose file path

Configuration values can be set via (in order of precedence):

1. **CLI flags** (`--port`, `--config`)
2. **Environment variables** (`DEPLOYDECK_*`)
3. **Config file** (`config.yaml`)
4. **Default values**

</section>

<section id="configuration" class="docs-section">

## Configuration

### Full config.yaml Reference

Below is a complete configuration file with all available options:

```yaml
server:
  port: 9000                          # Listen port (default: 9000)
  host: "0.0.0.0"                     # Bind address (default: 0.0.0.0)
  tls:
    enabled: false                    # Enable TLS
    cert_file: "/path/to/cert.pem"    # TLS certificate path
    key_file: "/path/to/key.pem"      # TLS private key path

auth:
  webhook_secret: "your-secret"       # REQUIRED - webhook auth secret

rate_limit:
  enabled: true                       # Enable per-IP rate limiting
  requests_per_minute: 10             # Sustained requests per IP per minute
  burst_size: 5                       # Maximum burst above steady rate

dashboard:
  enabled: false                      # Dashboard (planned feature)
  username: "admin"
  password: "change-me"

logging:
  level: "info"                       # Log level: debug, info, warn, error
  format: "text"                      # Output format: json, text

services:
  myapp:
    # Required fields
    compose_file: "/opt/apps/docker-compose.yml"
    service_name: "myapp"

    # Deploy mode
    mode: "pull"                      # "pull" (default) or "build"
    working_dir: "/opt/apps"          # Working directory for compose commands

    # Build mode options
    branch: "main"                    # Branch filter (default: main)
    repo: "https://github.com/u/r"   # Fallback clone URL
    clone_token: ""                   # Auth token for private repos
    clone_token_file: ""              # Read token from file (Docker Secrets)
    prune_after_build: false          # Clean build cache after deploy

    # Timeouts
    timeout: 5m                       # Default: 5m (pull), 10m (build)

    # Health check
    health_check:
      enabled: true
      url: "http://localhost:8080/health"
      timeout: 30s                    # Total health check timeout
      interval: 2s                    # Time between checks
      retries: 10                     # Max attempts

    # Rollback
    rollback:
      enabled: true
      keep_images: 3                  # Rollback snapshots to keep

    # Environment variables passed to docker compose
    env:
      DEPLOY_ENV: "production"
```

### Environment Variables

Environment variables override config file values:

| Variable | Overrides | Example |
|----------|-----------|---------|
| `DEPLOYDECK_PORT` | `server.port` | `9000` |
| `DEPLOYDECK_HOST` | `server.host` | `0.0.0.0` |
| `DEPLOYDECK_WEBHOOK_SECRET` | `auth.webhook_secret` | `abc123...` |
| `DEPLOYDECK_LOG_LEVEL` | `logging.level` | `debug` |
| `DEPLOYDECK_CLONE_TOKEN` | `clone_token` (all services) | `ghp_xxx...` |
| `DEPLOYDECK_RATE_LIMIT_RPM` | `rate_limit.requests_per_minute` | `10` |
| `DEPLOYDECK_RATE_LIMIT_BURST` | `rate_limit.burst_size` | `5` |

### Service Modes: Pull vs Build

DeployDeck supports two deployment modes:

#### Pull Mode (default)

In pull mode, your CI/CD pipeline builds the Docker image, pushes it to a registry, and then calls DeployDeck to pull and deploy it.

```
GitHub Actions --> build + push image --> POST /api/deploy/myapp --> DeployDeck
                                                                       |
                                                             docker compose pull
                                                             docker compose up -d
                                                             health check
```

**Configuration:**

```yaml
services:
  myapp:
    mode: "pull"                    # or omit (pull is default)
    compose_file: "/opt/apps/docker-compose.yml"
    service_name: "myapp"
    timeout: 5m                     # default for pull mode
```

**Trigger:**

```bash
curl -X POST https://deploy.example.com/api/deploy/myapp \
  -H "X-DeployDeck-Secret: your-secret" \
  -H "Content-Type: application/json" \
  -d '{"image": "ghcr.io/user/myapp:latest"}'
```

#### Build Mode

In build mode, DeployDeck receives a push webhook, clones the repository, and builds the image directly on the server. No registry is needed.

```
git push --> GitHub webhook --> DeployDeck --> git clone
                                              |
                                    docker compose build
                                    docker compose up -d
                                    health check
```

**Configuration:**

```yaml
services:
  myapp:
    mode: "build"
    branch: "main"
    repo: "https://github.com/user/myapp.git"
    compose_file: "docker-compose.yml"         # relative to repo root
    service_name: "myapp"
    working_dir: "/opt/builds/myapp"
    timeout: 15m
    prune_after_build: true
    clone_token_file: "/run/secrets/github_token"
```

### Health Check Configuration

Health checks verify that your service is running correctly after deployment. If a health check fails, DeployDeck can automatically roll back to the previous version.

```yaml
health_check:
  enabled: true                     # Enable health checking
  url: "http://localhost:8080/health"  # URL to poll
  timeout: 30s                      # Total time to wait for healthy
  interval: 2s                      # Time between poll attempts
  retries: 10                       # Maximum number of attempts
```

The health checker sends HTTP GET requests to the configured URL. Any 2xx response is considered healthy. The check runs in a loop: it attempts up to `retries` times, waiting `interval` between each attempt, with an overall `timeout` limit.

### Rollback Settings

Rollback uses Docker image tagging to snapshot the current state before each deployment. If the deployment fails (including health check failure), DeployDeck restores the snapshot.

```yaml
rollback:
  enabled: true                     # Enable automatic rollback
  keep_images: 3                    # Number of rollback snapshots to retain
```

**How it works:**

1. Before deploying, the current image is tagged as `service:rollback-<timestamp>`
2. The new image is pulled/built and deployed
3. If deployment fails, the rollback tag is restored and `docker compose up -d` brings back the previous version
4. Old rollback tags beyond `keep_images` are automatically cleaned up

Rollback gets its own 2-minute timeout, independent of the deployment timeout.

### Rate Limiting

DeployDeck supports per-IP rate limiting on the `/api/deploy` and `/api/rollback` endpoints using a token-bucket algorithm.

```yaml
rate_limit:
  enabled: true                     # Enable rate limiting
  requests_per_minute: 10           # Sustained rate per IP
  burst_size: 5                     # Max burst above steady rate
```

Each remote IP is tracked independently. When the limit is exceeded, the server returns `429 Too Many Requests`.

Environment variable overrides: `DEPLOYDECK_RATE_LIMIT_RPM` and `DEPLOYDECK_RATE_LIMIT_BURST`.

</section>

<section id="webhooks" class="docs-section">

## Webhooks

### GitHub Webhook Setup

To set up a GitHub webhook for build mode:

1. Go to your GitHub repository
2. Navigate to **Settings > Webhooks > Add webhook**
3. Configure:
   - **Payload URL:** `https://deploy.yourdomain.com/api/deploy/myapp`
   - **Content type:** `application/json`
   - **Secret:** Your `webhook_secret` from config.yaml
   - **Events:** Select "Just the push event"
4. Click **Add webhook**

GitHub signs every webhook payload with HMAC-SHA256 using the shared secret. DeployDeck verifies this signature automatically via the `X-Hub-Signature-256` header.

### GitLab Webhook Setup

To set up a GitLab webhook for build mode:

1. Go to your GitLab project
2. Navigate to **Settings > Webhooks**
3. Configure:
   - **URL:** `https://deploy.yourdomain.com/api/deploy/myapp`
   - **Secret token:** Your `webhook_secret` from config.yaml
   - **Trigger:** Check "Push events"
4. Click **Add webhook**

GitLab sends the secret token in the `X-GitLab-Token` header, which DeployDeck verifies using constant-time comparison.

### CI/CD Pipeline Integration

#### GitHub Actions Example

Add this step to your workflow after building and pushing the image:

```yaml
name: Build and Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Build and push Docker image
        run: |
          docker build -t ghcr.io/${{ github.repository }}:${{ github.sha }} .
          docker push ghcr.io/${{ github.repository }}:${{ github.sha }}

      - name: Deploy to production
        run: |
          curl -X POST https://deploy.yourdomain.com/api/deploy/myapp \
            -H "X-DeployDeck-Secret: ${{ secrets.DEPLOYDECK_SECRET }}" \
            -H "Content-Type: application/json" \
            -d '{"image": "ghcr.io/${{ github.repository }}:${{ github.sha }}"}'
```

#### GitLab CI Example

Add a deploy stage to your `.gitlab-ci.yml`:

```yaml
stages:
  - build
  - deploy

build:
  stage: build
  script:
    - docker build -t registry.gitlab.com/$CI_PROJECT_PATH:$CI_COMMIT_SHA .
    - docker push registry.gitlab.com/$CI_PROJECT_PATH:$CI_COMMIT_SHA

deploy:
  stage: deploy
  script:
    - |
      curl -X POST https://deploy.yourdomain.com/api/deploy/myapp \
        -H "X-GitLab-Token: $DEPLOYDECK_SECRET" \
        -H "Content-Type: application/json" \
        -d '{"image": "registry.gitlab.com/$CI_PROJECT_PATH:$CI_COMMIT_SHA"}'
  only:
    - main
```

### Authentication Methods

DeployDeck supports three authentication methods. Use one per request:

| Method | Header | Format | Use Case |
|--------|--------|--------|----------|
| **GitHub HMAC** | `X-Hub-Signature-256` | `sha256=<hmac-sha256>` | GitHub webhooks |
| **GitLab Token** | `X-GitLab-Token` | Plain token string | GitLab webhooks |
| **DeployDeck Secret** | `X-DeployDeck-Secret` | Plain string or `sha256=<hmac>` | CI/CD pipelines, manual triggers |

All methods use constant-time comparison (`hmac.Equal()`) to prevent timing attacks.

**Authentication check order:**

1. GitHub `X-Hub-Signature-256` header (HMAC-SHA256 verification)
2. GitLab `X-GitLab-Token` header (token comparison)
3. DeployDeck `X-DeployDeck-Secret` header (HMAC or token)

**Generating a secure secret:**

```bash
openssl rand -hex 32
```

</section>

<section id="api-reference" class="docs-section">

## API Reference

### POST /api/deploy/:service

Trigger a deployment for the specified service. Requires authentication.

**URL Parameters:**
- `:service` - The service name as defined in your config.yaml

**Request Headers** (use one):
- `X-Hub-Signature-256: sha256=<hmac>` (GitHub)
- `X-GitLab-Token: <secret>` (GitLab)
- `X-DeployDeck-Secret: <secret>` (DeployDeck)
- `Content-Type: application/json`

**Request Body (pull mode):**

```json
{
  "image": "ghcr.io/user/myapp:latest",
  "tag": "v1.2.3"
}
```

**Request Body (build mode):** GitHub or GitLab push webhook payload (sent automatically by the webhook).

**Response (success):**

```json
{
  "status": "pending",
  "deployment_id": "dep_123",
  "service": "myapp"
}
```

**Response (branch filtered, build mode):**

```json
{
  "status": "skipped",
  "reason": "push to develop, expected main"
}
```

**Response (auth failure):**

```json
{
  "error": "authentication failed"
}
```

**Status codes:**
- `200` - Deployment initiated (or skipped for wrong branch)
- `401` - Authentication failed
- `404` - Service not found in configuration
- `429` - Rate limit exceeded

### POST /api/rollback/:service

Manually trigger a rollback for the specified service. Requires authentication.

> **Note:** This endpoint is currently a stub and will be fully implemented with persistent storage in a future release.

**URL Parameters:**
- `:service` - The service name as defined in your config.yaml

**Request Headers:** Same as deploy endpoint.

### GET /api/deployments

List all deployments stored in memory. Deployments are not persisted across restarts.

**Response:**

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

**Deployment statuses:**

| Status | Description |
|--------|-------------|
| `pending` | Created, not yet started |
| `running` | Deployment in progress |
| `success` | Completed successfully |
| `failed` | Failed (no rollback available or rollback disabled) |
| `rolled_back` | Failed and rolled back to previous version |

### GET /api/health

Health check endpoint returning server status, version, and uptime.

**Response:**

```json
{
  "status": "healthy",
  "version": "0.1.0",
  "uptime": "48h30m"
}
```

</section>

<section id="deployment" class="docs-section">

## Deployment

### Running with systemd

For production servers, run DeployDeck as a systemd service:

**1. Install the binary:**

```bash
curl -L https://github.com/esteban-ams/deploydeck/releases/latest/download/deploydeck-linux-amd64 \
  -o /usr/local/bin/deploydeck
chmod +x /usr/local/bin/deploydeck
```

**2. Create the configuration directory:**

```bash
mkdir -p /opt/deploydeck
# Copy and edit your config.yaml to /opt/deploydeck/config.yaml
```

**3. Create the systemd service file:**

```ini
# /etc/systemd/system/deploydeck.service
[Unit]
Description=DeployDeck - Container Deployment Server
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/deploydeck
ExecStart=/usr/local/bin/deploydeck --config /opt/deploydeck/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**4. Enable and start:**

```bash
systemctl daemon-reload
systemctl enable deploydeck
systemctl start deploydeck
```

**5. Check status and logs:**

```bash
systemctl status deploydeck
journalctl -u deploydeck -f
```

### Running with Docker Compose

Run DeployDeck itself as a container:

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

> **Important:** The Docker socket mount (`/var/run/docker.sock`) is required for DeployDeck to manage containers on the host.

Start it:

```bash
docker compose up -d
docker compose logs -f
```

### Production Tips

**Use HTTPS in production.** Either enable TLS in DeployDeck config or put it behind a reverse proxy:

```yaml
# Option A: TLS in DeployDeck
server:
  tls:
    enabled: true
    cert_file: "/etc/letsencrypt/live/deploy.example.com/fullchain.pem"
    key_file: "/etc/letsencrypt/live/deploy.example.com/privkey.pem"
```

```nginx
# Option B: Nginx reverse proxy
server {
    listen 443 ssl;
    server_name deploy.example.com;

    ssl_certificate /etc/letsencrypt/live/deploy.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/deploy.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

**Generate strong secrets:**

```bash
openssl rand -hex 32
```

**Restrict network access** to the DeployDeck port:

```bash
# Allow only your CI/CD IP
ufw allow from YOUR_CI_IP to any port 9000
```

**Use token files for private repos** instead of inline tokens (Docker Secrets pattern):

```yaml
services:
  myapp:
    clone_token_file: "/run/secrets/github_token"
```

**Monitor with the health endpoint:**

```bash
curl -s https://deploy.example.com/api/health | jq .
```

</section>

<section id="cli-reference" class="docs-section">

## CLI Reference

DeployDeck provides a CLI with several subcommands for managing and inspecting your deployment server.

### deploydeck

The main command starts the DeployDeck webhook server.

```bash
deploydeck [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--config` | Path to configuration file | `config.yaml` |
| `--port` | Server listen port | `9000` |
| `--version` | Print version and exit | - |

**Examples:**

```bash
# Start with default config
deploydeck

# Start with custom config and port
deploydeck --config /opt/deploydeck/config.yaml --port 8080

# Print version
deploydeck --version
```

### deploydeck doctor

Checks the system environment and reports any issues that might prevent DeployDeck from running correctly.

```bash
deploydeck doctor
```

**Checks performed:**
- Docker is installed and accessible
- Docker Compose is available
- Configuration file is valid
- Required directories exist
- Docker socket is accessible

### deploydeck status

Shows the current status of the DeployDeck server and recent deployments.

```bash
deploydeck status
```

**Output includes:**
- Server running state
- Number of configured services
- Recent deployment history
- Current service states

### deploydeck version

Prints the DeployDeck version, build date, and commit hash.

```bash
deploydeck version
```

**Example output:**

```
DeployDeck v0.1.0
Build date: 2026-02-12
Commit: a1b2c3d
```

</section>
