# FastShip Project Structure

```
fastship/
├── .github/
│   └── workflows/
│       └── build.yml              # CI/CD pipeline for releases
│
├── cmd/
│   └── fastship/
│       └── main.go                # Application entry point
│
├── internal/
│   ├── config/
│   │   └── config.go              # YAML config parsing & validation
│   │
│   ├── webhook/
│   │   ├── verify.go              # HMAC/secret authentication
│   │   └── handler.go             # HTTP request handlers
│   │
│   ├── deploy/
│   │   ├── deploy.go              # Deployment orchestration
│   │   └── health.go              # Health check polling
│   │
│   └── docker/
│       └── docker.go              # Docker Compose operations
│
├── docs/
│   └── ARCHITECTURE.md            # System architecture details
│
├── config.example.yaml            # Example configuration
├── docker-compose.yml             # Run FastShip as container
├── docker-compose.example.yml     # Example service to deploy
├── fastship.service               # Systemd service file
├── Dockerfile                     # Container image definition
├── Makefile                       # Build & development commands
├── test-webhook.sh                # Webhook testing script
├── go.mod                         # Go module definition
├── .gitignore                     # Git ignore rules
├── .env.example                   # Environment variables example
├── LICENSE                        # MIT License
│
└── Documentation/
    ├── README.md                  # Main user documentation
    ├── BUILD.md                   # Build instructions
    ├── QUICKSTART.md              # Quick start guide
    ├── PROJECT_SUMMARY.md         # Implementation summary
    ├── CODE_OVERVIEW.md           # Code walkthrough
    ├── TODO.md                    # Future enhancements
    └── STRUCTURE.md               # This file
```

## File Descriptions

### Core Application Code

| File | Lines | Purpose |
|------|-------|---------|
| `cmd/fastship/main.go` | 97 | Entry point, CLI, server setup |
| `internal/config/config.go` | 163 | Configuration management |
| `internal/webhook/verify.go` | 68 | Authentication verification |
| `internal/webhook/handler.go` | 179 | HTTP endpoint handlers |
| `internal/deploy/deploy.go` | 230 | Deployment engine |
| `internal/deploy/health.go` | 76 | Health checking |
| `internal/docker/docker.go` | 125 | Docker operations |

**Total Go Code: 1,077 lines**

### Configuration Files

| File | Purpose |
|------|---------|
| `config.example.yaml` | Example configuration with all options |
| `.env.example` | Environment variable examples |
| `go.mod` | Go dependencies |

### Deployment Files

| File | Purpose |
|------|---------|
| `Dockerfile` | Multi-stage container build |
| `docker-compose.yml` | Run FastShip as container |
| `docker-compose.example.yml` | Example service configuration |
| `fastship.service` | Systemd service definition |

### Development Tools

| File | Purpose |
|------|---------|
| `Makefile` | Build, test, and run commands |
| `test-webhook.sh` | Test all webhook endpoints |
| `.gitignore` | Git ignore rules |

### CI/CD

| File | Purpose |
|------|---------|
| `.github/workflows/build.yml` | Build and release pipeline |

### Documentation

| File | Purpose |
|------|---------|
| `README.md` | Main user documentation |
| `ARCHITECTURE.md` | System design and architecture |
| `BUILD.md` | Build and development setup |
| `QUICKSTART.md` | 5-minute getting started guide |
| `PROJECT_SUMMARY.md` | What was implemented |
| `CODE_OVERVIEW.md` | Code walkthrough for developers |
| `TODO.md` | Future enhancements roadmap |
| `STRUCTURE.md` | This file - project structure |

## Package Dependencies

```
github.com/esteban-ams/fastship
├── cmd/fastship                   (imports all internal packages)
│
├── internal/config                (no internal dependencies)
│   └── gopkg.in/yaml.v3
│
├── internal/webhook
│   ├── internal/config
│   ├── internal/deploy
│   └── github.com/labstack/echo/v4
│
├── internal/deploy
│   ├── internal/config
│   ├── internal/docker
│   └── (no external dependencies)
│
└── internal/docker
    └── (no external dependencies, uses os/exec)
```

## Data Flow

```
HTTP Request
    │
    ▼
webhook/handler.go (API endpoints)
    │
    ├─► webhook/verify.go (authentication)
    │
    └─► deploy/deploy.go (orchestration)
        │
        ├─► docker/docker.go (Docker operations)
        │
        └─► deploy/health.go (health checks)
```

## Build Artifacts

When you build the project, you'll get:

```
fastship/
├── fastship                       # Binary (macOS/Linux)
├── fastship-linux-amd64          # Linux binary
├── fastship-darwin-amd64         # macOS binary
├── fastship-windows-amd64.exe    # Windows binary
└── config.yaml                   # Your configuration (not tracked)
```

## Development Workflow

```
1. Edit code in internal/
2. Run: go run ./cmd/fastship --config config.yaml
3. Test: ./test-webhook.sh
4. Build: make build
5. Deploy: ./fastship
```

## Production Deployment Options

### Option 1: Binary

```
/usr/local/bin/fastship           # Binary
/opt/fastship/config.yaml         # Configuration
/etc/systemd/system/fastship.service  # Service
```

### Option 2: Docker

```
docker-compose.yml                # Service definition
config.yaml                       # Configuration
/var/run/docker.sock              # Docker socket (mounted)
```

### Option 3: Kubernetes (Future)

```
fastship-deployment.yaml          # K8s Deployment
fastship-service.yaml             # K8s Service
fastship-configmap.yaml           # Configuration
```

## File Size Estimates

| Component | Size |
|-----------|------|
| Go binary | ~15-20 MB |
| Docker image | ~50-60 MB |
| Source code | ~100 KB |
| Documentation | ~150 KB |

## Key Directories Explained

### `/cmd/fastship/`
Contains the application entry point. In Go, `cmd/` is the standard location for main packages that produce executables.

### `/internal/`
Private application code that cannot be imported by external projects. This is a Go convention enforced by the compiler.

### `/docs/`
Architecture documentation and design decisions. User-facing docs are at the root level.

### `/.github/`
GitHub-specific files including CI/CD workflows and issue templates.

## Configuration Precedence

```
1. CLI Flags (--port, --config)
   ↓ (overrides)
2. Environment Variables (FASTSHIP_*)
   ↓ (overrides)
3. config.yaml
   ↓ (overrides)
4. Default Values
```

## Logging Output

```
/var/log/fastship/              # If using systemd
journalctl -u fastship          # Systemd journal
docker logs fastship            # If using Docker
stdout/stderr                   # If running directly
```

## Next Steps

1. **To build**: See BUILD.md
2. **To run**: See QUICKSTART.md
3. **To understand**: See CODE_OVERVIEW.md
4. **To contribute**: See TODO.md
5. **To deploy**: See README.md

---

*Generated: 2024-01-19*
*FastShip v1.0.0*
