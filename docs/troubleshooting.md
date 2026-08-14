# Troubleshooting

## Dashboard Shows Unavailable

In Phase 1 this is expected. Production metrics are intentionally unavailable until Phase 2 implements validated collection.

## Collector Cannot Start

Check:

```bash
docker compose -f deployments/docker/compose.yml logs network-monitor-collector
```

Common causes:

- Kernel or Docker runtime does not support declared capabilities.
- Docker Compose version does not recognize `BPF` or `PERFMON`.
- Docker socket mount is blocked.

## Go Commands Fail Locally

Install Go 1.23 or run tests in CI/Docker. The current server inspection showed Node/npm available but no local Go compiler.

## GitHub Push Fails

Check authentication for `https://github.com/AchuthanDev/Network-Monitor-Debian.git`. Never force-push `main`.
