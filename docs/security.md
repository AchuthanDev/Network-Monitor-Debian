# Security

The application is intended for LAN-only self-hosting. It should not be exposed publicly by default.

## Privilege Separation

- Collector: privileged host visibility, minimal capabilities, no public UI.
- API: unprivileged, talks to PostgreSQL.
- Web: static frontend, unprivileged.
- Database: private Docker network.

## Collector Permissions

Phase 1 Compose declares likely collector permissions:

- `pid: host`: required to map host and host-network container sockets to PIDs.
- `network_mode: host`: required for host-level network observation and collector health on the host namespace.
- `/proc:/host/proc:ro`: process and socket inode inspection.
- `/sys:/host/sys:ro`: cgroup and eBPF filesystem inspection.
- Docker socket read-only: container metadata.
- `NET_ADMIN`, `NET_RAW`, `BPF`, `PERFMON`, `SYS_RESOURCE`: expected for eBPF and network observer setup.
- `SYS_ADMIN`: compatibility fallback for older kernels that gate BPF operations broadly. This should be removed if target kernels allow narrower capabilities.

The collector must not modify firewall, routing, DNS, Docker daemon settings, or interface configuration.

## Data Privacy

The default collector must not store packet payloads, URLs, messages, passwords, or decrypted content. Stored data is limited to byte counts and metadata needed for accounting.

## Authentication Roadmap

Phase 1 has no production auth. Authentication is a blocking requirement before exposing the dashboard beyond a trusted LAN. Planned design:

- local admin account,
- Argon2id or bcrypt password hashing,
- secure session cookies,
- CSRF protection for cookie-authenticated state changes,
- rate limiting on login.
