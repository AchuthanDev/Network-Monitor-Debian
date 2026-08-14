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

## Validation Procedure

Controlled tests should record expected byte sizes and measured results:

1. LAN test: transfer a known 1 GiB file to a LAN client.
2. Internet test: download a known-size file from a public endpoint.
3. Container test: perform the same public download from a test bridge container.
4. Host-network test: perform the same public download from a host-network process/container.

The report should include error percentage and explain expected protocol overhead.
