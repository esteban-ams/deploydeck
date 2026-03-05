# DeployDeck TODO

> Ver [ROADMAP.md](./ROADMAP.md) para el plan completo y timeline.
> Ver [docs/CASE_STUDY.md](./docs/CASE_STUDY.md) para el caso de exito en produccion.

---

## Phase 1: Core (DONE)
- [x] Configuration parsing with YAML
- [x] Environment variable overrides
- [x] Webhook authentication (HMAC, simple secret, GitLab)
- [x] HTTP server with Echo
- [x] Deploy endpoint
- [x] Health check system
- [x] Rollback on failure
- [x] Docker Compose integration
- [x] Per-service deployment serialization
- [x] In-memory deployment tracking
- [x] API endpoints (deploy, health, list deployments)

## Phase 1.5: Core Extended (DONE - Feb 2026)
- [x] Build mode: clone repo + docker compose build + deploy
- [x] Pull mode: docker compose pull + deploy
- [x] Webhook payload parsing (GitHub push, GitLab push)
- [x] Branch filtering (deploy only on configured branch)
- [x] Rollback via image tagging (real snapshots before deploy)
- [x] Rollback cleanup (keep_images configurable)
- [x] Deployment timeouts (configurable per service, default 5m/10m)
- [x] Token security: clone_token_file (Docker Secrets pattern)
- [x] Token injection for private repos (GitHub, GitLab, generic)
- [x] Auto-prune build cache (prune_after_build option)
- [x] New Docker commands: ComposeBuild, TagImage, RemoveImage, ListImagesByFilter, BuilderPrune

---

## Phase A: DX & Community Ready (PRIORITY #1)
> Primera impresion importa. Hacer DeployDeck facil de instalar y usar.

### Installer
- [ ] `install.sh` script para curl | bash
- [ ] Detectar OS y arquitectura automaticamente
- [ ] Descargar binario correcto de GitHub Releases
- [ ] Verificar checksum

### CLI Improvements
- [ ] Colores con lipgloss o similar
- [ ] `deploydeck doctor` - verifica docker, config, permisos
- [ ] `deploydeck status` - tabla bonita con estado de servicios
- [ ] Mensajes de error humanos (no stack traces)
- [ ] Flag `--json` para output parseable

### GitHub Community
- [ ] Issue templates (bug report, feature request)
- [ ] PR template
- [ ] CONTRIBUTING.md
- [ ] CODE_OF_CONDUCT.md
- [ ] CHANGELOG.md (o usar Release notes)

### Documentation
- [ ] README con GIF de demo
- [ ] Badges (build, version, license)
- [ ] Quickstart de 2 minutos (pull mode y build mode)

### Release
- [ ] Setup goreleaser
- [ ] GitHub Actions para release automatico
- [ ] Release v0.1.0

---

## Phase B: Persistencia Simple (PRIORITY #2)
> Historial que sobrevive reinicios.

### SQLite
- [ ] Usar `modernc.org/sqlite` (pure Go, sin CGO)
- [ ] Schema: `deployments(id, service, mode, image, status, started_at, finished_at, rollback_tag, error)`
- [ ] Schema: `images(id, service, tag, sha, pulled_at)`
- [ ] Auto-create database si no existe

### Store Package
- [ ] Interface: `Save()`, `List()`, `GetByService()`, `GetByID()`
- [ ] Implementacion SQLite
- [ ] Migrar codigo actual para usar store
- [ ] Cleanup: eliminar registros antiguos (configurable)

### Rollback Mejorado
- [ ] Listar rollback tags disponibles por servicio
- [ ] Rollback a version especifica (seleccionar tag)
- [ ] API: `GET /api/services/:name/images`

### Tests
- [ ] Tests para store package

---

## Phase C: Dashboard (PRIORITY #3)
> UI visual para gestionar deploys.

### Setup
- [ ] Instalar templ
- [ ] Configurar air para live reload
- [ ] Embed templates en binario

### Layout
- [ ] Base layout (header, nav, footer)
- [ ] CSS con Tailwind o Pico
- [ ] Responsive basico

### Auth
- [ ] Middleware auth con token
- [ ] Login page simple
- [ ] Logout

### Vistas
- [ ] Dashboard overview (servicios + estado + modo pull/build)
- [ ] Lista de deployments (tabla paginada con modo, rollback_tag)
- [ ] Detalle de deployment (logs, duracion, etc)
- [ ] Settings (ver/rotar token)

### Acciones
- [ ] Boton deploy manual con confirmacion
- [ ] Boton rollback con selector de rollback tags
- [ ] HTMX para updates sin refresh

---

## Phase D: Tests Criticos (PRIORITY #4)
> Confianza sin over-engineering.

### Unit Tests
- [ ] `internal/config` - parsing, validation, defaults, deploy modes
- [ ] `internal/webhook` - HMAC verification, secret auth, payload parsing
- [ ] `internal/deploy` - health check logic, timeout handling, rollback flow
- [ ] `internal/docker` - tag, prune, build commands
- [ ] `internal/store` - CRUD operations

### Integration
- [ ] CI: run `go test` en GitHub Actions
- [ ] Badge de coverage en README

---

## Future (Post v0.3.0)

### Observability
- [ ] Structured logging con zerolog
- [ ] Prometheus metrics endpoint
- [ ] Metricas: deployments_total, deployment_duration_seconds

### Notifications
- [ ] Slack webhooks
- [ ] Discord webhooks
- [ ] Custom webhook (user-defined URL)

### Security
- [ ] Rate limiting per IP
- [ ] API keys per service
- [ ] IP whitelist

### Advanced
- [ ] Preview deploys (branch -> URL temporal)
- [ ] GitHub App (zero-config)
- [ ] Multi-server support
- [ ] Blue-green deployments
- [ ] Deployment approvals

---

## Known Issues
- [ ] No authentication on GET /api/deployments (fix in Phase A)
- [ ] Deployment history lost on restart (fix in Phase B)
- [ ] No dashboard (fix in Phase C)
- [ ] No rate limiting (defer to Future)
- [ ] No graceful shutdown (add in Phase A)

---

## Quick Wins
Small tasks that can be done anytime:

- [ ] Add deployment duration tracking (30 min)
- [ ] Add graceful shutdown (1 hour)
- [ ] Improve error messages (2 hours)
- [ ] Add version command to CLI (15 min)
- [ ] Add `--config` flag to specify config path (30 min)
