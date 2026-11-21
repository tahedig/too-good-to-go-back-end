# Staging Infrastructure Monitoring

This directory contains the production-grade monitoring stack for the staging server using **Prometheus**, **Node Exporter**, and **Grafana**.

---

## Architecture Overview

```
                      +------------------------------------------+
                      |               Staging Host               |
                      |                                          |
                      |   [ Host Kernel: /proc, /sys, / (root) ] |
                      +--------------------+---------------------+
                                           | (read-only mounts)
                                           v
[ Team Browser ]  <--- (HTTPS / SSH Tunnel) ---> [ Grafana (127.0.0.1:3000) ]
                                                        |
                                                        v (Internal Docker Network)
                                                 [ Prometheus (:9090) ]
                                                        |
                                                        v (Internal Docker Network)
                                               [ Node Exporter (:9100) ]
```

- **Node Exporter (`quay.io/prometheus/node-exporter:v1.12.1`)**: Collects hardware and OS metrics from the host (CPU, memory, disk, network) using read-only host mounts (`/proc`, `/sys`, `/`).
- **Prometheus (`quay.io/prometheus/prometheus:v3.15.1`)**: Scrapes Node Exporter every 15 seconds, stores time-series data with a 15-day retention policy.
- **Grafana (`grafana/grafana:13.3.0`)**: Pre-configured with automatic data source and dashboard provisioning to visualize real staging metrics.

---

## Security Model

1. **Network Isolation**:
   - Prometheus (`9090`) and Node Exporter (`9100`) are **never published** to the host interface (`expose` only). They communicate strictly over the internal isolated bridge network (`monitoring`).
   - Grafana is explicitly bound to `127.0.0.1:${GRAFANA_PORT:-3000}`. It cannot be accessed directly from the public internet.
2. **Authentication**:
   - Anonymous access is disabled (`GF_AUTH_ANONYMOUS_ENABLED: "false"`).
   - User self-registration is disabled (`GF_USERS_ALLOW_SIGN_UP: "false"`).
   - Admin credentials must be explicitly provided in `.env` (`GF_SECURITY_ADMIN_PASSWORD`).

---

## Quick Start / Deployment on Staging

### 1. Configure Environment Variables
Copy `.env.example` to `.env`:
```bash
cp monitoring/.env.example monitoring/.env
```
Generate a strong random password for Grafana admin and set it in `.env`:
```bash
# Example generation:
openssl rand -hex 32
```
Edit `.env`:
```env
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=<your-strong-random-password>
GRAFANA_PORT=3000
PROMETHEUS_RETENTION=15d
```

### 2. Start the Monitoring Stack
From the repository root (or within `monitoring/`):
```bash
docker compose -f monitoring/docker-compose.yml --env-file monitoring/.env up -d
```

### 3. Verify Containers
```bash
docker compose -f monitoring/docker-compose.yml ps
```
All three services (`staging-prometheus`, `staging-node-exporter`, `staging-grafana`) should show `Up`.

---

## Accessing Grafana

### Option A: Secure SSH Port Forwarding (Recommended for maintenance)
From your local machine:
```bash
ssh -L 3000:127.0.0.1:3000 <user>@<staging-server-ip>
```
Open [http://localhost:3000](http://localhost:3000) in your browser and sign in with the admin credentials from `.env`.

### Option B: Nginx Reverse Proxy with TLS (For team access)
Configure an Nginx virtual host with SSL:
```nginx
server {
    listen 443 ssl http2;
    server_name monitoring.staging.yourdomain.com;

    ssl_certificate     /etc/letsencrypt/live/monitoring.staging.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/monitoring.staging.yourdomain.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## Dashboard Details: "Staging Server Overview"

The pre-provisioned dashboard (`staging-overview`) auto-refreshes every **15 seconds** and includes:
1. **CPU Utilization (%)**: Active CPU utilization computed from `node_cpu_seconds_total` across all cores.
2. **Memory Utilization (%)**: Used RAM percentage computed from `MemAvailable` vs `MemTotal`.
3. **Root Disk Utilization (%)**: Root partition (`/`) disk usage percentage, excluding virtual mount points.
4. **Network Receive (Bps)**: Inbound network rate on non-loopback/non-virtual interfaces.
5. **Network Transmit (Bps)**: Outbound network rate on non-loopback/non-virtual interfaces.
6. **Node Exporter Availability**: Instant health check (`UP` / `DOWN`).
7. **Monitored Hostname**: Host system identity reported by kernel uname.

---

## Verification & Troubleshooting

- **Check Prometheus scraping health**:
  ```bash
  docker compose -f monitoring/docker-compose.yml exec prometheus wget -qO- http://localhost:9090/api/v1/targets
  ```
  Verify that the `node-exporter` target has state `"health": "up"`.

- **Check Node Exporter metrics output**:
  ```bash
  docker compose -f monitoring/docker-compose.yml exec node-exporter wget -qO- http://localhost:9100/metrics | grep node_cpu_seconds_total
  ```

- **View Service Logs**:
  ```bash
  docker compose -f monitoring/docker-compose.yml logs -f grafana
  docker compose -f monitoring/docker-compose.yml logs -f prometheus
  docker compose -f monitoring/docker-compose.yml logs -f node-exporter
  ```
