# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Rate limiting middleware (per-IP token bucket via `golang.org/x/time/rate`)
- Authentication required on `GET /api/deployments`
- Landing page with Hugo static site and custom theme
- Full documentation page at `/docs/` with sidebar navigation
- GitHub Pages deployment workflow
- Cobra CLI with `doctor`, `status`, and `version` subcommands
- GoReleaser for automated cross-platform binary builds

### Changed
- Renamed project from FastShip to DeployDeck
- Renamed auth header from `X-FastShip-Secret` to `X-DeployDeck-Secret`
- Renamed environment variables from `FASTSHIP_*` to `DEPLOYDECK_*`
- Redesigned landing page with refined industrial aesthetic (DM Serif Display + coral accent)
- Improved error messages across all packages with actionable suggestions
- Updated README with Mermaid sequence diagrams and flowcharts

### Fixed
- Nav links now work correctly across pages (docs → landing page sections)
- Smooth scroll with `scroll-margin-top` for fixed navbar offset

## [0.1.0] - 2026-01-22

### Added

#### Core
- HTTP webhook server built with Echo v4
- Webhook authentication: GitHub HMAC-SHA256, GitLab token, shared secret
- Docker Compose integration via `os/exec` (pull + up)
- Configurable health checks (URL, timeout, interval, retries)
- Automatic rollback on health check failure via image tagging
- Per-service deployment serialization (mutex per service)
- In-memory deployment tracking
- YAML configuration with environment variable overrides
- API endpoints: deploy, rollback, list deployments, health
- CI/CD pipeline with GitHub Actions
- Docker image published to GHCR
- Systemd service file

#### Build Mode
- Clone repo + `docker compose build` + deploy
- Webhook payload parsing for GitHub and GitLab push events
- Branch filtering (deploy only on configured branch)
- Rollback via image tagging (snapshots before each deploy)
- Rollback cleanup with configurable `keep_images`
- Deployment timeouts (configurable per service, default 5m pull / 10m build)
- Token security: `clone_token_file` for Docker Secrets pattern
- Token injection for private repos (GitHub, GitLab, generic)
- Auto-prune Docker build cache (`prune_after_build` option)

#### Community
- CONTRIBUTING.md with development guide
- Issue templates (bug report, feature request)
- PR template
- Architecture and code overview documentation
- Production case study

[Unreleased]: https://github.com/esteban-ams/deploydeck/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/esteban-ams/deploydeck/releases/tag/v0.1.0
