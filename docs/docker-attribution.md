# Docker Attribution

Docker attribution must not rely on `docker stats` network I/O. Docker NET I/O can include LAN traffic, miss host-network containers, and cannot distinguish Internet from Docker internal traffic by itself.

## Bridge Containers

For bridge-mode containers, Phase 2 and Phase 4 should map:

- Container ID and name from Docker API.
- Container network namespace.
- Container cgroup path.
- Container init PID.
- veth and bridge metadata as diagnostic data only.

Traffic is counted once at socket/cgroup level, then classified by remote IP.

## Host-Network Containers

Docker host-network mode shares the host network namespace. Docker documents that host-mode containers do not receive a separate container IP address and port publishing is ignored. This means network namespace alone is not enough.

Host-network containers must be attributed through:

- process PID tree,
- cgroup membership,
- Docker container init PID,
- executable/command metadata.

Plex and Home Assistant can therefore show traffic even when Docker NET I/O is zero.

## Attribution Confidence

Records should carry confidence:

- `exact`: socket/cgroup mapped directly to a container or process.
- `process`: process known, container unknown.
- `container_inferred`: container inferred from PID tree or cgroup path.
- `unknown`: traffic observed but not safely attributed.

Unknown attribution may still count as Internet if the remote IP is public, but UI must show the owner as unknown.
