# Case Study: FastShip en Produccion

> Documentacion del caso de exito de FastShip desplegado en produccion gestionando multiples servicios.

## Resumen

FastShip esta corriendo en produccion desde Enero 2026, gestionando deployments automatizados para 4 servicios en un servidor DigitalOcean. Reemplazo exitosamente a Watchtower, reduciendo el tiempo de deploy de ~5 minutos (polling) a ~76 segundos (webhook instantaneo).

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
   FastShip  Portfolio Komercia Komercia Metalurgica
    :9000     :5001     :8000   Landing   :5002
                                 :5003
```

### Servicios Gestionados

| Servicio | Stack | Imagen GHCR | Puerto |
|----------|-------|-------------|--------|
| Portfolio | FastHTML | `ghcr.io/esteban-ams/portafolio` | 5001 |
| Komercia | Django | `ghcr.io/esteban-ams/erp-market-django` | 8000 |
| Komercia Landing | FastHTML | `ghcr.io/esteban-ams/komercia-landing` | 5003 |
| Metalurgica | FastHTML | `ghcr.io/esteban-ams/metalurgica-spa` | 5002 |

---

## Configuracion

### config.yaml

```yaml
server:
  port: 9000
  host: "0.0.0.0"

auth:
  webhook_secret: "${FASTSHIP_SECRET}"

dashboard:
  enabled: true
  username: "admin"
  password: "${FASTSHIP_DASHBOARD_PASSWORD}"

logging:
  level: "info"
  format: "json"

services:
  portfolio:
    compose_file: "/infrastructure/docker-compose.yml"
    service_name: "portfolio"
    working_dir: "/infrastructure"
    health_check:
      enabled: true
      url: "http://portfolio:5001/"
      timeout: 30s
      interval: 2s
      retries: 10
    rollback:
      enabled: true
      keep_images: 3

  metalurgica:
    compose_file: "/infrastructure/docker-compose.yml"
    service_name: "metalurgica"
    working_dir: "/infrastructure"
    health_check:
      enabled: true
      url: "http://metalurgica:5002/"
      timeout: 30s
      interval: 2s
      retries: 10
    rollback:
      enabled: true
      keep_images: 3

  # ... mas servicios
```

### docker-compose.yml (FastShip service)

```yaml
fastship:
  image: ghcr.io/esteban-ams/fastship:latest
  container_name: fastship
  restart: unless-stopped
  environment:
    - FASTSHIP_SECRET=${FASTSHIP_SECRET}
    - FASTSHIP_DASHBOARD_PASSWORD=${FASTSHIP_DASHBOARD_PASSWORD}
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock
    - ./fastship/config.yaml:/app/config.yaml:ro
    - /opt/infrastructure:/infrastructure
  networks:
    - traefik-public
    - internal
  labels:
    - "traefik.enable=true"
    - "traefik.http.routers.fastship.rule=Host(`deploy.esteban-ams.cl`)"
    - "traefik.http.routers.fastship.entrypoints=websecure"
    - "traefik.http.routers.fastship.tls.certresolver=letsencrypt"
    - "traefik.http.services.fastship.loadbalancer.server.port=9000"
```

---

## Flujo de Deploy

### Diagrama

```
Developer          GitHub Actions         GHCR            FastShip          Docker
    |                    |                  |                 |                |
    |  git push          |                  |                 |                |
    |------------------->|                  |                 |                |
    |                    |  build image     |                 |                |
    |                    |----------------->|                 |                |
    |                    |  push to GHCR    |                 |                |
    |                    |----------------->|                 |                |
    |                    |                  |                 |                |
    |                    |  POST /api/deploy/metalurgica      |                |
    |                    |---------------------------------->|                |
    |                    |                  |                 |  docker pull   |
    |                    |                  |                 |--------------->|
    |                    |                  |                 |  health check  |
    |                    |                  |                 |--------------->|
    |                    |                  |                 |  [OK]          |
    |                    |                  |                 |<---------------|
    |                    |  200 OK          |                 |                |
    |                    |<----------------------------------|                |
    |                    |                  |                 |                |
```

### GitHub Actions Workflow

```yaml
# .github/workflows/build-and-push.yml
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

      # FastShip webhook - triggers automatic deploy
      - name: Deploy to production
        if: github.ref == 'refs/heads/master'
        run: |
          curl -sS -X POST https://deploy.esteban-ams.cl/api/deploy/metalurgica \
            -H "X-FastShip-Secret: ${{ secrets.FASTSHIP_SECRET }}" \
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
| **FastShip deploy** | ~34 segundos |

### Timeline detallado

```
04:23:17  Push a master (commit a7ebe8c)
04:23:17  GitHub Actions workflow iniciado
04:23:44  Imagen Docker construida y subida a GHCR
04:23:50  FastShip recibe webhook, inicia deployment
04:24:08  Pull de imagen completado
04:24:22  Nuevo contenedor iniciado
04:24:33  Health check pasado, deployment exitoso
```

### Comparacion Before/After

| Metrica | ANTES (Watchtower) | DESPUES (FastShip) |
|---------|--------------------|--------------------|
| Tiempo de deploy | ~5 min (polling) | ~76 seg (webhook) |
| Deteccion de cambio | Cada 5 min | Instantaneo |
| Health checks | No | Si |
| Rollback automatico | No | Si |
| Dashboard | No | Si |
| Visibilidad | Logs | API + Dashboard |

---

## Beneficios Observados

### 1. Velocidad
- Deploy 4x mas rapido en el peor caso (5 min vs 76 seg)
- Feedback inmediato al desarrollador

### 2. Confiabilidad
- Health checks previenen deploys rotos
- Rollback automatico si health falla
- Historial de deployments

### 3. Simplicidad
- Un archivo YAML de configuracion
- Sin SSH keys en CI/CD
- Sin exponer IPs del servidor

### 4. Seguridad
- Webhook autenticado con secret
- HTTPS con Let's Encrypt
- Sin acceso SSH desde GitHub Actions

---

## Lecciones Aprendidas

### Lo que funciono bien
1. **Webhooks > Polling**: Respuesta instantanea vs esperar al intervalo
2. **Health checks**: Evitan deployments de codigo roto
3. **GHCR publico**: Sin necesidad de auth para pull

### Mejoras identificadas
1. **Dashboard**: Falta UI para ver estado sin CLI
2. **Persistencia**: Historial se pierde al reiniciar
3. **Notifications**: No hay alertas a Slack/Discord
4. **Multi-replica**: Solo soporta un contenedor por servicio

---

## Reproducir este Setup

### Prerequisitos
- Servidor con Docker y Docker Compose
- Traefik configurado con SSL
- Cuenta de GitHub con GHCR habilitado

### Pasos

1. **Agregar FastShip al docker-compose.yml**
```yaml
fastship:
  image: ghcr.io/esteban-ams/fastship:latest
  # ... (ver config arriba)
```

2. **Crear config.yaml**
```yaml
# Ver ejemplo en QUICKSTART.md
```

3. **Agregar secret en GitHub**
```bash
gh secret set FASTSHIP_SECRET --body "$(openssl rand -hex 32)"
```

4. **Agregar step en GitHub Actions**
```yaml
- name: Deploy
  run: |
    curl -X POST https://your-domain.com/api/deploy/service \
      -H "X-FastShip-Secret: ${{ secrets.FASTSHIP_SECRET }}"
```

---

## Conclusion

FastShip demostro ser una solucion efectiva para automatizar deployments en un entorno de produccion real. La simplicidad del enfoque (webhook + docker compose) lo hace ideal para desarrolladores independientes y equipos pequenos que quieren CI/CD automatizado sin la complejidad de Kubernetes o plataformas managed.

**Estadisticas clave:**
- 4 servicios gestionados
- ~76 segundos de deploy end-to-end
- 0 deployments fallidos desde implementacion
- 0 downtime no planificado

---

## Links

- [Repositorio FastShip](https://github.com/esteban-ams/fastship)
- [Documentacion](./ARCHITECTURE.md)
- [Quick Start](../QUICKSTART.md)
