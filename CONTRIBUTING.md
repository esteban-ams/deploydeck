# Contributing to DeployDeck

Thanks for your interest in contributing to DeployDeck! This guide will help you get started.

## Development Setup

### Prerequisites

- Go 1.22+
- Docker and Docker Compose
- Git

### Getting Started

```bash
git clone https://github.com/esteban-ams/deploydeck.git
cd deploydeck
go mod download
make build
```

### Running Locally

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your services
make run
```

### Running Tests

```bash
make test
```

## How to Contribute

### Reporting Bugs

- Use the [Bug Report](https://github.com/esteban-ams/deploydeck/issues/new?template=bug_report.yml) issue template
- Include your DeployDeck version, OS, and deploy mode
- Provide relevant config (redact secrets) and logs

### Requesting Features

- Use the [Feature Request](https://github.com/esteban-ams/deploydeck/issues/new?template=feature_request.yml) issue template
- Explain the problem you're trying to solve
- Suggest a solution if you have one

### Submitting Pull Requests

1. Fork the repository
2. Create a branch from `main` (`git checkout -b feat/my-feature`)
3. Make your changes
4. Run tests (`make test`)
5. Run formatting (`gofmt -w .`)
6. Commit using the [commit convention](#commit-convention)
7. Push and open a PR

## Code Style

- Run `gofmt` on all Go files
- Run `go vet ./...` before committing
- Use table-driven tests
- Wrap errors with context: `fmt.Errorf("doing thing: %w", err)`
- Keep functions focused and short
- Document exported functions

## Commit Convention

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add notification support
fix: handle empty webhook payload
docs: update API reference
test: add config validation tests
refactor: extract health check logic
chore: update dependencies
```

## Project Structure

```
cmd/deploydeck/        # Application entry point
internal/
  config/            # YAML config + env overrides
  webhook/           # HTTP handlers + auth + payload parsing
  deploy/            # Deployment engine + health checks
  docker/            # Docker CLI wrapper
  git/               # Git clone for build mode
```

See [docs/CODE_OVERVIEW.md](docs/CODE_OVERVIEW.md) for a detailed codebase tour.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
