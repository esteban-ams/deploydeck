# FastShip Roadmap

> Plan de desarrollo para convertir FastShip en un proyecto open source maduro y atractivo.

## Vision

FastShip es un servidor de webhooks ligero para deployments automatizados con Docker Compose. La meta es ser la alternativa simple a Watchtower, Coolify, y Kamal para desarrolladores independientes y equipos pequenos.

**Tagline**: "Deploy on push. No polling. No complexity."

---

## Estado Actual

```
Phase 1: Core               ████████████████████ 100% DONE
```

### Completado
- Servidor HTTP con Echo
- Webhook auth (HMAC, simple secret, GitLab)
- Integracion con Docker Compose
- Health checks configurables
- Rollback automatico si health falla
- API REST: `/api/deploy`, `/api/health`, `/api/deployments`
- CI/CD con GitHub Actions
- Documentacion basica

---

## Roadmap por Fases

### PHASE A: DX & Community Ready
> Prioridad #1 - Primera impresion importa

| Tarea | Esfuerzo | Estado |
|-------|----------|--------|
| `install.sh` (curl \| bash) descarga binario correcto | 2h | Pending |
| CLI: colores con lipgloss | 2h | Pending |
| CLI: `fastship doctor` (verifica docker, config) | 2h | Pending |
| CLI: `fastship status` (tabla con estado servicios) | 1h | Pending |
| GitHub: issue templates (bug, feature) | 30min | Pending |
| GitHub: PR template | 15min | Pending |
| CONTRIBUTING.md | 1h | Pending |
| README: agregar GIF de demo | 1h | Pending |
| Release v0.1.0 con goreleaser | 2h | Pending |

**Entregable**: Cualquiera puede `curl -fsSL fastship.dev/install | bash` e instalar

---

### PHASE B: Persistencia Simple
> Historial que sobrevive reinicios

| Tarea | Esfuerzo | Estado |
|-------|----------|--------|
| SQLite con `modernc.org/sqlite` (pure Go, no CGO) | 30min | Pending |
| Schema: `deployments(id, service, image, status, created_at)` | 1h | Pending |
| Schema: `images(id, service, tag, sha, created_at)` | 30min | Pending |
| Store interface: `Save`, `List`, `GetByService` | 2h | Pending |
| Migrar codigo actual a usar store | 2h | Pending |
| Rollback a imagen especifica (no solo anterior) | 2h | Pending |
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
| Vista: dashboard overview (servicios + estado) | 3h | Pending |
| Vista: historial de deployments (tabla paginada) | 2h | Pending |
| HTMX: boton deploy con confirmacion | 2h | Pending |
| HTMX: boton rollback con selector de version | 2h | Pending |
| Settings: ver/rotar token | 1h | Pending |
| CSS: Tailwind o Pico para estilos | 2h | Pending |
| Responsive: mobile friendly | 1h | Pending |

**Entregable**: Dashboard funcional en `/ui` con auth

---

### PHASE D: Tests Criticos
> Confianza sin over-engineering

| Tarea | Esfuerzo | Estado |
|-------|----------|--------|
| Tests: `internal/config` (parsing, validation) | 1h | Pending |
| Tests: `internal/webhook` (auth verification) | 2h | Pending |
| Tests: `internal/deploy` (health check logic) | 2h | Pending |
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
| **Persistencia** | SQLite (modernc, pure Go) | Sin CGO, portable, suficiente para el uso |
| **Dashboard** | Templ + HTMX | Go-idiomatico, sin build JS separado |
| **Testing** | Solo paths criticos | Realista para side project |
| **Auth** | Token simple | Sin complejidad de usuarios/roles |
| **Orquestador** | Docker Compose only | Cubre 90% de casos target |

---

## Comparacion con Competencia

| Feature | FastShip | Watchtower | Kamal | Coolify |
|---------|----------|------------|-------|---------|
| Instalacion | 1 comando | Docker | Ruby gems | Docker |
| Config | 1 YAML | Labels | YAML + secrets | UI |
| Dashboard | Si (simple) | No | No | Si (complejo) |
| Webhooks | Si | No (polling) | No | Si |
| Health checks | Si | No | Si | Si |
| Rollback auto | Si | No | Si | Si |
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

---

## Contributing

Ver [CONTRIBUTING.md](./CONTRIBUTING.md) para guias de contribucion.

## License

MIT
