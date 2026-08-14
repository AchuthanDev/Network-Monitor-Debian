# Deployment

Phase 1 deployment is for skeleton validation only.

```bash
cp .env.example .env
docker compose -f deployments/docker/compose.yml up -d --build
```

Services:

- `network-monitor-db`: PostgreSQL.
- `network-monitor-api`: Go API.
- `network-monitor-collector`: Go collector skeleton.
- `network-monitor-web`: static React UI served by nginx.

Default host ports are chosen to avoid common home-server media stack collisions:

- Web: `18000`
- API: `18080`
- Collector health: `19091`
- PostgreSQL localhost binding for the host-network collector: `127.0.0.1:15432`

## Do Not Change Host Networking

Phase 1 does not require route, firewall, DHCP, DNS, Docker daemon, or interface changes. Any future change to those areas must be handled as a separate reviewed deployment step.

## Backup

Initial local database backup command:

```bash
docker compose -f deployments/docker/compose.yml exec network-monitor-db pg_dump -U network_monitor network_monitor > network-monitor.sql
```

Automated local backup scheduling is planned for a later phase.
