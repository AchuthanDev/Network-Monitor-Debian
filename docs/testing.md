# Testing

## Commands

```bash
make test
make lint
make build
docker compose -f deployments/docker/compose.yml config
```

## Critical Accounting Tests

Required before Phase 2 is considered complete:

- Server to `192.168.1.3`: LAN increases, Internet remains zero.
- Server to `8.8.8.8` or another public endpoint: Internet increases.
- Bridge container to public endpoint: correct container, counted once.
- Bridge container to bridge peer: Docker internal increases, Internet remains zero.
- Host-network container to public endpoint: attributed by PID/cgroup, counted once.
- Loopback transfer: loopback increases, Internet remains zero.

Gateway model tests live in `features/gateway`. They define behavior before gateway mode can be implemented against live networking:

- Client to LAN destination: Internet count remains zero.
- Client to public Internet: counted for the MAC-backed device.
- Two clients using Internet: counted separately.
- Same forwarded flow observed at multiple hooks: counted only once.
- Post-NAT observation: rejected for authoritative per-device accounting.
- Night/free window: `06:59` is free and `07:00` is anytime for the configured timezone.
- DHCP IP change: history follows MAC-backed identity.
- IP-only device identity: marked ephemeral.
- Docker bridge traffic: not attributed to a LAN device.
- Server host traffic: remains `host/server`.

Current host inspection on 2026-08-14 showed `net.netfilter.nf_conntrack_acct=0`. In that state Phase 2 must report accounting unavailable. Enabling conntrack byte accounting is an explicit deployment action, not something the application silently changes.

Validation after enabling `nf_conntrack_acct=1` showed that conntrack snapshot polling undercounted controlled downloads. Phase 2 now uses nftables counters for authoritative aggregate totals and keeps conntrack as a fallback/diagnostic path.

## Validation Procedure

Controlled tests should record expected byte sizes and measured results:

1. LAN test: transfer a known 1 GiB file to a LAN client.
2. Internet test: download a known-size file from a public endpoint.
3. Container test: perform the same public download from a test bridge container.
4. Host-network test: perform the same public download from a host-network process/container.

The report should include error percentage and explain expected protocol overhead.
