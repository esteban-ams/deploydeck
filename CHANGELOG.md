# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Community files (CONTRIBUTING.md, issue templates, PR template)
- Consolidated documentation for open-source release

### Changed
- Rewrote README with complete feature documentation
- Updated ARCHITECTURE.md and CODE_OVERVIEW.md for Phase 1.5

### Removed
- Redundant documentation files (PROJECT_SUMMARY.md, STRUCTURE.md, BUILD.md)

## [0.1.0] - Unreleased

### Added

#### Core (Phase 1)
- HTTP webhook server built with Echo v4
- Webhook authentication: GitHub HMAC-SHA256, GitLab token, DeployDeck secret
- Docker Compose integration via `os/exec` (pull + up)
- Configurable health checks (URL, timeout, interval, retries)
- Automatic rollback on health check failure
- Per-service deployment serialization (mutex per service)
- In-memory deployment tracking
- YAML configuration with environment variable overrides
- API endpoints: deploy, rollback, list deployments, health
- CI/CD pipeline with GitHub Actions
- Docker image published to GHCR
- Systemd service file

#### Build Mode (Phase 1.5)
- Build mode: clone repo + `docker compose build` + deploy
- Webhook payload parsing for GitHub and GitLab push events
- Branch filtering (deploy only on configured branch)
- Rollback via image tagging (real snapshots before each deploy)
- Rollback cleanup with configurable `keep_images`
- Deployment timeouts (configurable per service, default 5m pull / 10m build)
- Token security: `clone_token_file` for Docker Secrets pattern
- Token injection for private repos (GitHub, GitLab, generic)
- Auto-prune Docker build cache (`prune_after_build` option)

[Unreleased]: https://github.com/esteban-ams/deploydeck/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/esteban-ams/deploydeck/releases/tag/v0.1.0
