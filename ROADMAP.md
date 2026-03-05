# DeployDeck Roadmap

> Plan de desarrollo para convertir DeployDeck en un proyecto open source maduro y atractivo.

## Vision

DeployDeck es un servidor de webhooks ligero para deployments automatizados con Docker Compose. La meta es ser la alternativa simple a Watchtower, Coolify, y Kamal para desarrolladores independientes y equipos pequenos.

**Tagline**: "Deploy on push. No polling. No complexity."

---

## Estado Actual

```
Phase 1: Core               ████████████████████ 100% DONE
Phase 1.5: Core Extended    ████████████████████ 100% DONE
```

### Phase 1: Core (Completado)
- Servidor HTTP con Echo
- Webhook auth (HMAC, simple secret, GitLab)
- Integracion con Docker Compose (modo pull)
- Health checks configurables
- Rollback automatico si health falla
- API REST: `/api/deploy`, `/api/health`, `/api/deployments`, `/api/rollback`
- CI/CD con GitHub Actions
- Documentacion basica

### Phase 1.5: Core Extended (Completado - Feb 2026)
- **Build mode**: Clone repo + `docker compose build` + deploy (ademas de pull)
- **Rollback via image tagging**: Snapshots reales de imagenes antes de cada deploy
- **Deployment timeouts**: Configurables por servicio (default 5m pull, 10m build)
- **Token security**: Lectura de tokens desde archivos (Docker Secrets pattern)
- **Auto-prune**: Limpieza automatica de build cache despues de builds
- **Webhook parsing**: Deteccion automatica de payloads GitHub/GitLab
- **Branch filtering**: Deploy solo en push a la rama configurada

---

## Dos Modos de Deploy

DeployDeck ahora soporta dos modos de despliegue:

```
PULL MODE                           BUILD MODE
(imagen precompilada)               (construye desde codigo)

GitHub Actions → GHCR               GitHub push → DeployDeck
       ↓                                   ↓
  DeployDeck webhook                   Clone repo
       ↓                                   ↓
  docker compose pull               docker compose build
       ↓                                   ↓
  docker compose up                  docker compose up
       ↓                                   ↓
  Health check                       Health check
       ↓                                   ↓
  Success / Rollback                 Success / Rollback
                                           ↓
                                     Auto-prune cache
```

---

## Flujo de Deploy (7 pasos)

```
1. Guardar imagen actual → tag rollback snapshot
2. Pull imagen O clone + build (segun modo)
3. docker compose up -d
4. Health check loop
5. Actualizar estado (success/rolled_back)
6. Limpiar rollback tags antiguos (keep_images)
7. Auto-prune build cache (solo build mode)
```

---

## Roadmap por Fases

### PHASE A: DX & Community Ready
> Prioridad #1 - Primera impresion importa

| Tarea | Esfuerzo | Estado |
|-------|----------|--------|
| `install.sh` (curl \| bash) descarga binario correcto | 2h | Pending |
| CLI: colores con lipgloss | 2h | Pending |
| CLI: `deploydeck doctor` (verifica docker, config) | 2h | Pending |
| CLI: `deploydeck status` (tabla con estado servicios) | 1h | Pending |
| GitHub: issue templates (bug, feature) | 30min | Pending |
| GitHub: PR template | 15min | Pending |
| CONTRIBUTING.md | 1h | Pending |
| README: agregar GIF de demo | 1h | Pending |
| Release v0.1.0 con goreleaser | 2h | Pending |

**Entregable**: Cualquiera puede `curl -fsSL deploydeck.dev/install | bash` e instalar

---

### PHASE B: Persistencia Simple
> Historial que sobrevive reinicios

| Tarea | Esfuerzo | Estado |
|-------|----------|--------|
| SQLite con `modernc.org/sqlite` (pure Go, no CGO) | 30min | Pending |
| Schema: `deployments(id, service, mode, image, status, started_at, finished_at, rollback_tag, error)` | 1h | Pending |
| Schema: `images(id, service, tag, sha, created_at)` | 30min | Pending |
| Store interface: `Save`, `List`, `GetByService` | 2h | Pending |
| Migrar codigo actual a usar store | 2h | Pending |
| Rollback a imagen especifica (con listado de tags) | 2h | Pending |
| Tests para store | 1h | Pending |

**Entregable**: `GET /api/deployments` muestra historial persistido

---

### PHASE C: Dashboard (Templ + HTMX)
> UI visual para gestionar deploys

| Tarea | Esfuerzo | Estado |
|-------|----------|--------|
| Setup Templ + air (live reload) | 1h | Pending |
| Layout base: header, nav, footer | 2h | Pending |
| Auth: middleware con token simple | 1h | Pending |
| Login page simple | 1h | Pending |
| Vista: dashboard overview (servicios + estado + modo pull/build) | 3h | Pending |
| Vista: historial de deployments (tabla paginada) | 2h | Pending |
| HTMX: boton deploy con confirmacion | 2h | Pending |
| HTMX: boton rollback con selector de version (image tags) | 2h | Pending |
| Settings: ver/rotar token | 1h | Pending |
| CSS: Tailwind o Pico para estilos | 2h | Pending |
| Responsive: mobile friendly | 1h | Pending |

**Entregable**: Dashboard funcional en `/ui` con auth

---

### PHASE D: Tests Criticos
> Confianza sin over-engineering

| Tarea | Esfuerzo | Estado |
|-------|----------|--------|
| Tests: `internal/config` (parsing, validation, modes) | 1h | Pending |
| Tests: `internal/webhook` (auth verification, payload parsing) | 2h | Pending |
| Tests: `internal/deploy` (health check logic, timeout, rollback) | 2h | Pending |
| Tests: `internal/docker` (tag, prune, build commands) | 1h | Pending |
| Tests: `internal/store` (CRUD operations) | 1h | Pending |
| CI: agregar `go test` en GitHub Actions | 30min | Pending |
| Badge: coverage en README | 30min | Pending |

**Entregable**: Tests pasan en CI, paths criticos cubiertos

---

## Timeline

```
Semana:  1    2    3    4    5    6    7    8
         |----+----|----+----|----+----+----|----|
         | PHASE A | PHASE B |   PHASE C    | D  |
         |   DX    | SQLite  |  Dashboard   |Test|
              ^                      ^         ^
           v0.1.0                 v0.2.0    v0.3.0
```

**Nota**: Timeline estimado para dedicacion part-time (side project).

---

## Releases

| Version | Contenido | Mensaje |
|---------|-----------|---------|
| **v0.1.0** | Phase A completada | "Easy install, beautiful CLI" |
| **v0.2.0** | + Phase B + C | "Dashboard & persistence" |
| **v0.3.0** | + Phase D | "Production ready" |

---

## Decisiones Tecnicas

| Aspecto | Decision | Razon |
|---------|----------|-------|
| **Deploy modes** | Pull + Build | Cubre tanto imagenes precompiladas como builds desde codigo |
| **Persistencia** | SQLite (modernc, pure Go) | Sin CGO, portable, suficiente para el uso |
| **Dashboard** | Templ + HTMX | Go-idiomatico, sin build JS separado |
| **Testing** | Solo paths criticos | Realista para side project |
| **Auth** | Token simple | Sin complejidad de usuarios/roles |
| **Rollback** | Image tagging | Snapshots reales, rollback a cualquier version |
| **Orquestador** | Docker Compose only | Cubre 90% de casos target |

---

## Comparacion con Competencia

| Feature | DeployDeck | Watchtower | Kamal | Coolify |
|---------|----------|------------|-------|---------|
| Instalacion | 1 comando | Docker | Ruby gems | Docker |
| Config | 1 YAML | Labels | YAML + secrets | UI |
| Modo Pull | Si | Si (polling) | Si | Si |
| Modo Build | Si | No | Si | Si |
| Dashboard | Planeado | No | No | Si (complejo) |
| Webhooks | Si | No (polling) | No | Si |
| Health checks | Si | No | Si | Si |
| Rollback auto | Si (image tags) | No | Si | Si |
| Timeouts | Si (por servicio) | No | Si | Si |
| Token files | Si (Docker Secrets) | No | Si | Si |
| Complejidad | Baja | Muy baja | Media | Alta |

---

## Target Audience

- Desarrolladores independientes (indie hackers)
- Equipos pequenos (1-5 personas)
- Side projects y MVPs
- Quien tenga 1-3 servidores propios
- Quien no quiera pagar Vercel/Railway pero quiera DX similar

---

## Future Ideas (Post v0.3.0)

- Preview deploys: PR -> URL temporal automatica
- GitHub App: Zero-config, sin secrets manuales
- Notifications: Slack/Discord/Telegram
- Multi-server: Deploy a multiples hosts
- Metrics: Tiempo de deploy, uptime basico
- Prometheus endpoint
- Deployment approvals
- Blue-green deployments

---

## Contributing

Ver [CONTRIBUTING.md](./CONTRIBUTING.md) para guias de contribucion.

## License

MIT
