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

## Collector Reports Conntrack Accounting Unavailable

Check:

```bash
cat /proc/sys/net/netfilter/nf_conntrack_acct
```

Expected for byte accounting:

```text
1
```

If the value is `0`, conntrack may expose flows without byte counters. The collector will not estimate Internet usage from NIC counters because that would mix LAN and Internet traffic.

## Go Commands Fail Locally

Install Go 1.23 or run tests in CI/Docker. The current server inspection showed Node/npm available but no local Go compiler.

## GitHub Push Fails

Check authentication for `https://github.com/AchuthanDev/Network-Monitor-Debian.git`. Never force-push `main`.
