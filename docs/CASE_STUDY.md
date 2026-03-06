# Case Study: DeployDeck en Produccion

> Documentacion del caso de exito de DeployDeck desplegado en produccion gestionando multiples servicios.

## Resumen

DeployDeck esta corriendo en produccion desde Enero 2026, gestionando deployments automatizados para 4 servicios en un servidor DigitalOcean. Reemplazo exitosamente a Watchtower, reduciendo el tiempo de deploy de ~5 minutos (polling) a ~76 segundos (webhook instantaneo).

Desde Febrero 2026, DeployDeck soporta dos modos de deploy: **pull** (imagen precompilada) y **build** (desde codigo fuente), con rollback real basado en image tagging.

---

## Infraestructura

### Servidor
- **Proveedor**: DigitalOcean Droplet
- **Specs**: 2 vCPU, 4GB RAM
- **OS**: Ubuntu 22.04 LTS
- **Dominio**: `deploy.esteban-ams.cl`

### Stack
```
                    Internet
                        |
                   [ Traefik ]
                   (Reverse Proxy + SSL)
                        |
        +-------+-------+-------+-------+
        |       |       |       |       |
   DeployDeck  Portfolio Komercia Komercia Metalurgica
    :9000     :5001     :8000   Landing   :5002
                                 :5003
```

### Servicios Gestionados

| Servicio | Stack | Modo Deploy | Imagen/Repo | Puerto |
|----------|-------|-------------|-------------|--------|
| Portfolio | FastHTML | pull | `ghcr.io/esteban-ams/portafolio` | 5001 |
| Komercia | Django | pull | `ghcr.io/esteban-ams/erp-market-django` | 8000 |
| Komercia Landing | FastHTML | pull | `ghcr.io/esteban-ams/komercia-landing` | 5003 |
| Metalurgica | FastHTML | pull | `ghcr.io/esteban-ams/metalurgica-spa` | 5002 |

---

## Modos de Deploy

### Pull Mode (en uso en produccion)
CI/CD construye la imagen, la sube a GHCR, y DeployDeck la descarga:

```
GitHub Actions → build → push GHCR → webhook → DeployDeck pull → deploy
```

### Build Mode (disponible desde Feb 2026)
DeployDeck clona el repositorio y construye la imagen directamente en el servidor:

```
GitHub push → webhook → DeployDeck clone → docker compose build → deploy
```

**Cuando usar cada modo:**
| Aspecto | Pull Mode | Build Mode |
|---------|-----------|------------|
| Requiere GHCR/DockerHub | Si | No |
| Build en servidor | No | Si |
| Ideal para | Imagenes publicas | Repos privadas, sin registry |
| Velocidad | Rapido (solo pull) | Mas lento (build completo) |
| Ejemplo timeout | 5 min | 10 min |

---

## Configuracion

### config.yaml (Pull Mode)

```yaml
server:
  port: 9000
  host: "0.0.0.0"

auth:
  webhook_secret: "${DEPLOYDECK_SECRET}"

dashboard:
  enabled: true
  username: "admin"
  password: "${DEPLOYDECK_DASHBOARD_PASSWORD}"

services:
  metalurgica:
    mode: "pull"
    compose_file: "/infrastructure/docker-compose.yml"
    service_name: "metalurgica"
    working_dir: "/infrastructure"
    timeout: 5m
    health_check:
      enabled: true
      url: "http://metalurgica:5002/"
      timeout: 30s
      interval: 2s
      retries: 10
    rollback:
      enabled: true
      keep_images: 3
```

### config.yaml (Build Mode)

```yaml
services:
  myapp:
    mode: "build"
    branch: "main"
    repo: "https://github.com/user/myapp.git"
    clone_token_file: "/run/secrets/github_token"
    compose_file: "docker-compose.yml"
    service_name: "myapp"
    working_dir: "/opt/builds"
    timeout: 15m
    prune_after_build: true
    health_check:
      enabled: true
      url: "http://myapp:8080/health"
      timeout: 30s
    rollback:
      enabled: true
      keep_images: 3
```

### docker-compose.yml (DeployDeck service)

```yaml
deploydeck:
  image: ghcr.io/esteban-ams/deploydeck:latest
  container_name: deploydeck
  restart: unless-stopped
  environment:
    - DEPLOYDECK_SECRET=${DEPLOYDECK_SECRET}
    - DEPLOYDECK_DASHBOARD_PASSWORD=${DEPLOYDECK_DASHBOARD_PASSWORD}
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock
    - ./deploydeck/config.yaml:/app/config.yaml:ro
    - /opt/infrastructure:/infrastructure
  networks:
    - traefik-public
    - internal
  labels:
    - "traefik.enable=true"
    - "traefik.http.routers.deploydeck.rule=Host(`deploy.esteban-ams.cl`)"
    - "traefik.http.routers.deploydeck.entrypoints=websecure"
    - "traefik.http.routers.deploydeck.tls.certresolver=letsencrypt"
    - "traefik.http.services.deploydeck.loadbalancer.server.port=9000"
```

---

## Flujo de Deploy (7 pasos)

### Pull Mode

```
Developer          GitHub Actions         GHCR            DeployDeck          Docker
    |                    |                  |                 |                |
    |  git push          |                  |                 |                |
    |------------------->|                  |                 |                |
    |                    |  build image     |                 |                |
    |                    |----------------->|                 |                |
    |                    |                  |                 |                |
    |                    |  POST /api/deploy/metalurgica      |                |
    |                    |---------------------------------->|                |
    |                    |                  |                 |                |
    |                    |                  |                 | 1. Tag rollback|
    |                    |                  |                 |--------------->|
    |                    |                  |                 | 2. docker pull |
    |                    |                  |                 |--------------->|
    |                    |                  |                 | 3. compose up  |
    |                    |                  |                 |--------------->|
    |                    |                  |                 | 4. health check|
    |                    |                  |                 |<-------------->|
    |                    |                  |                 | 5. update state|
    |                    |                  |                 | 6. cleanup tags|
    |                    |                  |                 |--------------->|
    |                    |  200 OK          |                 |                |
    |                    |<----------------------------------|                |
```

### Build Mode

```
Developer          GitHub              DeployDeck              Docker
    |                 |                    |                     |
    |  git push       |                    |                     |
    |---------------->|                    |                     |
    |                 |  webhook payload   |                     |
    |                 |------------------>|                     |
    |                 |                    | 1. Tag rollback     |
    |                 |                    |-------------------->|
    |                 |                    | 2. Clone repo       |
    |                 |                    | 3. compose build    |
    |                 |                    |-------------------->|
    |                 |                    | 4. compose up       |
    |                 |                    |-------------------->|
    |                 |                    | 5. health check     |
    |                 |                    |<------------------->|
    |                 |                    | 6. cleanup tags     |
    |                 |                    |-------------------->|
    |                 |                    | 7. auto-prune cache |
    |                 |                    |-------------------->|
    |                 |  200 OK           |                     |
    |                 |<------------------|                     |
```

---

## Seguridad

### Autenticacion
- Webhook secret con HMAC-SHA256 o shared secret
- Soporte para headers de GitHub y GitLab

### Tokens para Repos Privadas
- `clone_token`: directo en YAML (no recomendado)
- `clone_token_file`: lee desde archivo (Docker Secrets pattern, recomendado)
- `DEPLOYDECK_CLONE_TOKEN`: variable de entorno (fallback)

**Inyeccion automatica por proveedor:**
| Proveedor | Formato |
|-----------|---------|
| GitHub | `x-access-token:<token>@github.com/...` |
| GitLab | `oauth2:<token>@gitlab.com/...` |
| Otros | `token:<token>@host/...` |

---

## Rollback

### Como funciona
1. Antes de cada deploy, DeployDeck etiqueta la imagen actual como `service:rollback-{timestamp}`
2. Si el health check falla, restaura automaticamente la imagen con el tag de rollback
3. Tags antiguos se eliminan automaticamente segun `keep_images` (default: 3)

### Rollback manual
```bash
curl -X POST https://deploy.esteban-ams.cl/api/rollback/metalurgica \
  -H "X-DeployDeck-Secret: $SECRET"
```

---

## GitHub Actions Workflow

```yaml
name: Build and Push to GHCR

on:
  push:
    branches: [master]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - uses: docker/metadata-action@v5
        id: meta
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=raw,value=latest,enable={{is_default_branch}}
            type=sha,prefix=,format=short

      - uses: docker/build-push-action@v5
        with:
          context: .
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

      - name: Deploy to production
        if: github.ref == 'refs/heads/master'
        run: |
          curl -sS -X POST https://deploy.esteban-ams.cl/api/deploy/metalurgica \
            -H "X-DeployDeck-Secret: ${{ secrets.DEPLOYDECK_SECRET }}" \
            -H "Content-Type: application/json" \
            -d '{"image": "${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:latest"}' \
            --fail --show-error
```

---

## Metricas de Produccion

### Deploy de Metalurgica (Enero 22, 2026)

| Metrica | Valor |
|---------|-------|
| **Tiempo total** | ~76 segundos |
| **Build image** | ~27 segundos |
| **Push to GHCR** | ~15 segundos |
| **DeployDeck deploy** | ~34 segundos |

### Timeline detallado

```
04:23:17  Push a master (commit a7ebe8c)
04:23:17  GitHub Actions workflow iniciado
04:23:44  Imagen Docker construida y subida a GHCR
04:23:50  DeployDeck recibe webhook, inicia deployment
04:24:08  Pull de imagen completado
04:24:22  Nuevo contenedor iniciado
04:24:33  Health check pasado, deployment exitoso
```

### Comparacion Before/After

| Metrica | ANTES (Watchtower) | DESPUES (DeployDeck) |
|---------|--------------------|--------------------|
| Tiempo de deploy | ~5 min (polling) | ~76 seg (webhook) |
| Deteccion de cambio | Cada 5 min | Instantaneo |
| Modos de deploy | Solo pull | Pull + Build |
| Health checks | No | Si |
| Rollback automatico | No | Si (image tagging) |
| Timeouts | No | Si (por servicio) |
| Dashboard | No | Planeado |
| Visibilidad | Logs | API + Dashboard |

---

## Beneficios Observados

### 1. Velocidad
- Deploy 4x mas rapido en el peor caso (5 min vs 76 seg)
- Feedback inmediato al desarrollador

### 2. Confiabilidad
- Health checks previenen deploys rotos
- Rollback automatico via image tagging si health falla
- Timeouts configurables evitan deploys colgados
- Historial de deployments

### 3. Flexibilidad
- Pull mode para imagenes precompiladas (GHCR, Docker Hub)
- Build mode para repos sin registry
- Branch filtering para deploy solo en la rama correcta

### 4. Simplicidad
- Un archivo YAML de configuracion
- Sin SSH keys en CI/CD
- Sin exponer IPs del servidor

### 5. Seguridad
- Webhook autenticado con HMAC-SHA256
- HTTPS con Let's Encrypt
- Tokens leidos desde archivos (Docker Secrets pattern)
- Sin acceso SSH desde GitHub Actions

---

## Lecciones Aprendidas

### Lo que funciono bien
1. **Webhooks > Polling**: Respuesta instantanea vs esperar al intervalo
2. **Health checks**: Evitan deployments de codigo roto
3. **GHCR publico**: Sin necesidad de auth para pull
4. **Image tagging para rollback**: Mas confiable que guardar metadata
5. **Timeouts por servicio**: Builds pesados necesitan mas tiempo

### Mejoras identificadas
1. **Dashboard**: Falta UI para ver estado sin CLI
2. **Persistencia**: Historial se pierde al reiniciar
3. **Notifications**: No hay alertas a Slack/Discord
4. **Multi-replica**: Solo soporta un contenedor por servicio

---

## Reproducir este Setup

### Prerequisitos
- Servidor con Docker y Docker Compose
- Traefik configurado con SSL (o cualquier reverse proxy)
- Cuenta de GitHub con GHCR habilitado (para pull mode)

### Pasos

1. **Agregar DeployDeck al docker-compose.yml**
```yaml
deploydeck:
  image: ghcr.io/esteban-ams/deploydeck:latest
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock
    - ./config.yaml:/app/config.yaml:ro
  environment:
    - DEPLOYDECK_SECRET=${DEPLOYDECK_SECRET}
```

2. **Crear config.yaml**
```yaml
server:
  port: 9000
auth:
  webhook_secret: "${DEPLOYDECK_SECRET}"
services:
  myapp:
    mode: "pull"  # o "build"
    compose_file: "/path/to/docker-compose.yml"
    service_name: "myapp"
    timeout: 5m
    health_check:
      enabled: true
      url: "http://myapp:8080/"
    rollback:
      enabled: true
      keep_images: 3
```

3. **Agregar secret en GitHub**
```bash
gh secret set DEPLOYDECK_SECRET --body "$(openssl rand -hex 32)"
```

4. **Agregar step en GitHub Actions**
```yaml
- name: Deploy
  run: |
    curl -sS -X POST https://your-domain.com/api/deploy/myapp \
      -H "X-DeployDeck-Secret: ${{ secrets.DEPLOYDECK_SECRET }}" \
      -H "Content-Type: application/json" \
      -d '{"image": "ghcr.io/user/myapp:latest"}' \
      --fail --show-error
```

---

## Conclusion

DeployDeck demostro ser una solucion efectiva para automatizar deployments en un entorno de produccion real. Con la adicion del build mode y rollback via image tagging, DeployDeck paso de ser un simple webhook que descarga imagenes a un orquestador de deployments completo.

**Estadisticas clave:**
- 4 servicios gestionados
- 2 modos de deploy (pull + build)
- ~76 segundos de deploy end-to-end
- 0 deployments fallidos desde implementacion
- 0 downtime no planificado

---

## Links

- [Repositorio DeployDeck](https://github.com/esteban-ams/deploydeck)
- [Documentacion](./ARCHITECTURE.md)
- [Website](https://esteban-ams.github.io/deploydeck/)
