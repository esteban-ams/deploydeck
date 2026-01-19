# FastShip

A lightweight webhook server for automated Docker Compose deployments. Receive webhooks from your CI/CD pipeline and automatically deploy your containers.

## The Problem

When you push code to GitHub, your CI/CD builds a new Docker image and pushes it to a registry (GHCR, Docker Hub, etc.). But how do you tell your server to pull the new image?

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│  Developer  │      │   GitHub    │      │   Server    │
│             │      │   Actions   │      │             │
│  git push ──┼─────►│  build &    │      │ containers  │
│             │      │  push image │      │ (old image) │
└─────────────┘      └──────┬──────┘      └─────────────┘
                            │
                            │  How to trigger deploy?
                            ▼
                         ¿¿¿???
```

### Current Options (and their problems)

| Option | Problem |
|--------|---------|
| SSH manually | You have to log in every time |
| SSH from GitHub Actions | Exposes server IP + SSH keys in secrets |
| Watchtower | Polls every X minutes, not instant, unmaintained |
| Coolify/Portainer | Overkill for simple deployments |

## The Solution

FastShip runs on your server and listens for webhooks. When your CI/CD calls it, FastShip pulls the new image and restarts the container.

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│  Developer  │      │   GitHub    │      │   Server    │
│             │      │   Actions   │      │             │
│  git push ──┼─────►│  build &    │      │  FastShip   │
│             │      │  push image │      │  listening  │
└─────────────┘      └──────┬──────┘      └──────┬──────┘
                            │                     │
                            │ POST /deploy/myapp  │
                            │ (with HMAC secret)  │
                            └────────────────────►│
                                                  │
                                                  ▼
                                          docker compose pull
                                          docker compose up -d
```

## Features

- **Simple**: One binary, one config file
- **Secure**: HMAC-SHA256 webhook verification
- **Universal**: Works with GitHub, GitLab, Bitbucket, or any CI/CD
- **Dashboard**: Web UI to view deployment history and trigger manual deploys
- **Rollback**: Automatic rollback on failed health checks
- **Health Checks**: Verify deployments succeeded before marking complete

## Quick Start

### 1. Download FastShip

```bash
# Download the latest release
curl -L https://github.com/esteban-ams/fastship/releases/latest/download/fastship-linux-amd64 -o fastship
chmod +x fastship
```

### 2. Create Configuration

```yaml
# config.yaml
server:
  port: 9000
  host: "0.0.0.0"

auth:
  # Generate with: openssl rand -hex 32
  webhook_secret: "your-secret-here"

# Optional: Dashboard authentication
dashboard:
  enabled: true
  username: "admin"
  password: "change-me"

services:
  myapp:
    compose_file: "/opt/apps/docker-compose.yml"
    service_name: "myapp"
    health_check:
      enabled: true
      url: "http://localhost:8080/health"
      timeout: 30s
    rollback:
      enabled: true
      keep_images: 3
```

### 3. Run FastShip

```bash
./fastship --config config.yaml
```

### 4. Configure Your CI/CD

**GitHub Actions:**

```yaml
- name: Deploy to production
  run: |
    curl -X POST https://deploy.yourdomain.com/api/deploy/myapp \
      -H "X-FastShip-Secret: ${{ secrets.FASTSHIP_SECRET }}" \
      -H "Content-Type: application/json" \
      -d '{"image": "ghcr.io/user/myapp:latest"}'
```

**GitLab CI:**

```yaml
deploy:
  script:
    - |
      curl -X POST https://deploy.yourdomain.com/api/deploy/myapp \
        -H "X-FastShip-Secret: $FASTSHIP_SECRET" \
        -H "Content-Type: application/json" \
        -d '{"image": "registry.gitlab.com/user/myapp:latest"}'
```

## API Reference

### POST /api/deploy/:service

Trigger a deployment for a configured service.

**Headers:**
- `X-FastShip-Secret`: HMAC-SHA256 signature or shared secret

**Body (optional):**
```json
{
  "image": "ghcr.io/user/app:latest",
  "tag": "v1.2.3"
}
```

**Response:**
```json
{
  "status": "deploying",
  "deployment_id": "dep_abc123",
  "service": "myapp"
}
```

### GET /api/deployments

List recent deployments.

**Response:**
```json
{
  "deployments": [
    {
      "id": "dep_abc123",
      "service": "myapp",
      "status": "success",
      "started_at": "2024-01-15T10:30:00Z",
      "completed_at": "2024-01-15T10:30:45Z",
      "image": "ghcr.io/user/app:sha-abc1234"
    }
  ]
}
```

### POST /api/rollback/:service

Rollback to the previous image.

**Response:**
```json
{
  "status": "rolling_back",
  "deployment_id": "dep_xyz789",
  "service": "myapp",
  "target_image": "ghcr.io/user/app:sha-previous"
}
```

### GET /api/health

Health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "48h30m"
}
```

## Configuration Reference

```yaml
# config.yaml - Full reference

server:
  port: 9000                    # Port to listen on
  host: "0.0.0.0"              # Host to bind to
  tls:
    enabled: false             # Enable HTTPS
    cert_file: "/path/to/cert"
    key_file: "/path/to/key"

auth:
  webhook_secret: "secret"     # Shared secret for webhook verification

dashboard:
  enabled: true                # Enable web dashboard
  username: "admin"            # Dashboard login
  password: "password"         # Dashboard password

logging:
  level: "info"                # debug, info, warn, error
  format: "json"               # json or text

services:
  # Service name (used in API endpoints)
  myapp:
    # Path to docker-compose.yml
    compose_file: "/opt/apps/docker-compose.yml"

    # Service name in docker-compose.yml
    service_name: "myapp"

    # Working directory for docker compose commands
    working_dir: "/opt/apps"

    # Health check configuration
    health_check:
      enabled: true
      url: "http://localhost:8080/health"
      timeout: 30s
      interval: 2s
      retries: 10

    # Rollback configuration
    rollback:
      enabled: true
      keep_images: 3           # Number of old images to keep

    # Environment variables for deployment
    env:
      DEPLOY_ENV: "production"

  # Multiple services supported
  another-app:
    compose_file: "/opt/another/docker-compose.yml"
    service_name: "web"
```

## Docker Deployment

FastShip can run as a Docker container:

```yaml
# docker-compose.yml
services:
  fastship:
    image: ghcr.io/esteban-ams/fastship:latest
    container_name: fastship
    restart: unless-stopped
    ports:
      - "9000:9000"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - FASTSHIP_CONFIG=/app/config.yaml
```

**Important:** FastShip needs access to the Docker socket to manage containers.

## Security Considerations

### Webhook Verification

FastShip supports HMAC-SHA256 verification (same as GitHub webhooks):

```go
// How verification works
signature := request.Header.Get("X-FastShip-Secret")
expectedMAC := hmac.New(sha256.New, []byte(secret))
expectedMAC.Write(requestBody)
expected := hex.EncodeToString(expectedMAC.Sum(nil))

if !hmac.Equal([]byte(signature), []byte("sha256="+expected)) {
    return Unauthorized
}
```

### Network Security

- Run FastShip behind a reverse proxy (Traefik, nginx) with HTTPS
- Consider restricting access to known CI/CD IP ranges
- Use strong, randomly generated secrets

### Docker Socket Access

FastShip requires access to the Docker socket. This gives it full control over Docker on the host. Consider:

- Running FastShip as a dedicated user
- Using Docker socket proxies for additional security
- Limiting which services can be deployed via configuration

## Architecture

```
fastship/
├── cmd/
│   └── fastship/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # YAML configuration
│   ├── docker/
│   │   └── docker.go            # Docker/Compose operations
│   ├── deploy/
│   │   ├── deploy.go            # Deployment orchestration
│   │   ├── health.go            # Health checking
│   │   └── rollback.go          # Rollback logic
│   ├── webhook/
│   │   ├── handler.go           # Webhook handlers
│   │   └── verify.go            # HMAC verification
│   └── store/
│       └── store.go             # SQLite storage (optional)
├── web/
│   ├── templates/               # templ components
│   │   ├── layout.templ
│   │   ├── dashboard.templ
│   │   └── deployments.templ
│   ├── static/
│   │   └── styles.css
│   └── handlers.go              # Web routes
├── config.example.yaml
├── Dockerfile
└── README.md
```

## Development

### Prerequisites

- Go 1.22+
- templ (`go install github.com/a-h/templ/cmd/templ@latest`)
- Docker (for testing)

### Building

```bash
# Generate templ files
templ generate

# Build
go build -o fastship ./cmd/fastship

# Run
./fastship --config config.yaml
```

### Testing

```bash
go test ./...
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions welcome! Please read our [Contributing Guide](CONTRIBUTING.md) first.
