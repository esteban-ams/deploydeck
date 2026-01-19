# FastShip Quick Start Guide

Get FastShip running in 5 minutes.

## Prerequisites

- Docker and Docker Compose installed
- A service running with docker-compose.yml
- Go 1.22+ (if building from source)

## Option 1: Run from Source (Development)

### 1. Clone and Build

```bash
git clone https://github.com/esteban-ams/fastship.git
cd fastship
go mod download
go build -o fastship ./cmd/fastship
```

### 2. Create Configuration

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml`:

```yaml
server:
  port: 9000
  host: "0.0.0.0"

auth:
  webhook_secret: "my-super-secret-key"

services:
  myapp:
    compose_file: "/path/to/your/docker-compose.yml"
    service_name: "myapp"
    health_check:
      enabled: true
      url: "http://localhost:8080/health"
```

### 3. Run FastShip

```bash
./fastship --config config.yaml
```

### 4. Test It

```bash
# Test health endpoint
curl http://localhost:9000/api/health

# Trigger a deployment
curl -X POST http://localhost:9000/api/deploy/myapp \
  -H "X-FastShip-Secret: my-super-secret-key" \
  -H "Content-Type: application/json" \
  -d '{"image": "your-image:latest"}'
```

## Option 2: Run with Docker

### 1. Create Configuration

```bash
mkdir fastship
cd fastship
wget https://raw.githubusercontent.com/esteban-ams/fastship/main/config.example.yaml -O config.yaml
wget https://raw.githubusercontent.com/esteban-ams/fastship/main/docker-compose.yml
```

Edit `config.yaml` with your services.

### 2. Start FastShip

```bash
docker compose up -d
```

### 3. Check Logs

```bash
docker compose logs -f
```

## Option 3: Install as System Service

### 1. Download Binary

```bash
# Download latest release
curl -L https://github.com/esteban-ams/fastship/releases/latest/download/fastship-linux-amd64 -o /usr/local/bin/fastship
chmod +x /usr/local/bin/fastship
```

### 2. Setup Configuration

```bash
mkdir -p /opt/fastship
cd /opt/fastship
wget https://raw.githubusercontent.com/esteban-ams/fastship/main/config.example.yaml -O config.yaml
# Edit config.yaml
```

### 3. Install Systemd Service

```bash
wget https://raw.githubusercontent.com/esteban-ams/fastship/main/fastship.service -O /etc/systemd/system/fastship.service
systemctl daemon-reload
systemctl enable fastship
systemctl start fastship
```

### 4. Check Status

```bash
systemctl status fastship
journalctl -u fastship -f
```

## Configure Your CI/CD

### GitHub Actions

Add to your workflow:

```yaml
- name: Deploy to production
  run: |
    curl -X POST https://your-server.com:9000/api/deploy/myapp \
      -H "X-FastShip-Secret: ${{ secrets.FASTSHIP_SECRET }}" \
      -H "Content-Type: application/json" \
      -d '{"image": "ghcr.io/${{ github.repository }}:${{ github.sha }}"}'
```

### GitLab CI

Add to your `.gitlab-ci.yml`:

```yaml
deploy:
  stage: deploy
  script:
    - |
      curl -X POST https://your-server.com:9000/api/deploy/myapp \
        -H "X-GitLab-Token: $FASTSHIP_SECRET" \
        -H "Content-Type: application/json" \
        -d '{"image": "registry.gitlab.com/$CI_PROJECT_PATH:$CI_COMMIT_SHA"}'
```

## Security Tips

### 1. Generate a Strong Secret

```bash
openssl rand -hex 32
```

Add this to your config.yaml and CI/CD secrets.

### 2. Use HTTPS in Production

Either:
- Enable TLS in FastShip config
- Put FastShip behind a reverse proxy (Traefik, nginx)

### 3. Restrict Access

Configure your firewall to only allow connections from your CI/CD IPs:

```bash
# Example with ufw
ufw allow from CI_IP_ADDRESS to any port 9000
```

## Troubleshooting

### "service not found"

Check your config.yaml - the service name in the URL must match a key in the `services:` section.

### "authentication failed"

- Verify the secret in config.yaml matches the header value
- Check for typos in the header name (`X-FastShip-Secret`)

### "deployment failed at pull phase"

- Verify the compose_file path is correct
- Ensure the service_name matches your docker-compose.yml
- Check Docker has access to pull the image (login if private registry)

### "health check timeout"

- Verify the health_check URL is correct
- Increase the timeout in config.yaml
- Check the service is actually starting

## Next Steps

- Read the full [README.md](README.md) for detailed documentation
- Check [ARCHITECTURE.md](docs/ARCHITECTURE.md) to understand internals
- See [BUILD.md](BUILD.md) for development setup

## Getting Help

- GitHub Issues: https://github.com/esteban-ams/fastship/issues
- Documentation: https://github.com/esteban-ams/fastship/docs
