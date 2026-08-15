# Network Accounting

The central rule is simple: Internet usage is counted only when the server communicates with a public remote IP address. LAN, Docker internal, and loopback traffic are stored separately and never added to Internet quota totals.

## Traffic Classes

- `internet`: public remote IP traffic through the server's default route.
- `lan`: configured LAN CIDRs, RFC1918 private networks not identified as Docker internal, link-local, multicast, and other non-public local traffic.
- `docker_internal`: configured Docker bridge CIDRs such as `172.17.0.0/16`.
- `loopback`: `127.0.0.0/8` and `::1`.
- `unknown`: invalid or incomplete records. Unknown records do not count as Internet.

Docker CIDRs are checked before generic LAN CIDRs because Docker networks commonly live inside `172.16.0.0/12`.

## Why NIC Counters Are Not Enough

NIC RX/TX counters mix:

- Internet traffic.
- LAN transfers such as Plex or Samba.
- Broadcast and multicast traffic.
- Some Docker NAT paths.

They also cannot reliably identify process or container ownership. They are useful as a reconciliation signal, not as the source of truth for Internet accounting.

## Primary Collector Design

Phase 2 uses nftables counters as the authoritative total byte accounting layer. The collector creates a dedicated `inet network_monitor` table with non-verdict accounting chains on `input`, `output`, and `forward`. The rules count public-IP traffic on the detected default interface as Internet and RFC1918/private traffic as LAN. The chains use `policy accept` and do not block or redirect traffic.

Conntrack snapshot accounting remains as a fallback/diagnostic path. It is not the authoritative total counter because snapshot polling can undercount short-lived flows.

The first snapshot is treated as baseline. This prevents the system from counting bytes that were transferred before the collector started.

Temporary Phase 2 limitation: nftables totals are accurate for aggregate Internet/LAN usage, but they do not yet provide process/container/destination attribution. Phase 3 should add process attribution using eBPF/socket metadata while preserving nftables totals as a reconciliation source.

## Measurement Precision

Reports use measured network bytes, not application payload or downloaded file size. Gateway simulations record both download-side payload overhead and bidirectional Internet totals. Bidirectional totals include TCP/IP framing, HTTP metadata, connection setup/teardown, client ACK/control upload traffic, and any retransmissions visible at the accounting point.

The monitor must not subtract estimated protocol overhead to make values look closer to file sizes. ISP counters may measure at a different layer, so reports expose measurement metadata such as source, scope, classification method, and precision for future ISP comparison.

Requirements for conntrack fallback accounting:

- `/proc/net/nf_conntrack` must be readable by the collector.
- `net.netfilter.nf_conntrack_acct=1` must be enabled before flows are created, otherwise byte counters are not available.
- Collector must run with host PID/network visibility as declared in Docker Compose.

If nftables setup fails and conntrack requirements are not met, the collector health endpoint reports `accounting: unavailable` and does not fall back to NIC totals.

Future eBPF collection should use these layers:

1. eBPF socket/cgroup hooks record byte deltas at the process or cgroup boundary.
2. Socket metadata maps traffic to PID, command, UID, cgroup, and container when available.
3. Docker metadata maps container cgroups and init PIDs to container identity.
4. The classifier marks each record as Internet, LAN, Docker internal, loopback, or unknown.
5. The collector writes aggregated records, not packet payloads.

The preferred accounting point is the process or cgroup socket boundary. Counting only there avoids counting the same container packet again at veth, bridge, and host NIC layers.

## Fallback Collector Design

If eBPF support is unavailable, the fallback collector may use:

- `/proc/net/tcp*` and `/proc/net/udp*` for socket inventory.
- `/proc/<pid>/fd` socket inode matching for process ownership.
- conntrack snapshots for flow lifecycle and byte counters where available.
- nftables counters only for coarse reconciliation.

Fallback mode must display lower accuracy and should not claim reliable per-process byte attribution when the kernel cannot provide it.

## Double Counting Prevention

Every traffic record must include:

- accounting point: `socket`, `cgroup`, `conntrack`, or `interface`.
- direction: `rx` or `tx`.
- local endpoint.
- remote endpoint.
- process/container attribution fields.
- monotonic counter or interval key where available.

The production collector should select one authoritative accounting point per mode:

- Phase 2 mode: nftables host-interface accounting.
- Primary mode: socket/cgroup accounting.
- Reconciliation mode: interface counters only as comparison data.
- Fallback mode: conntrack accounting, with reduced attribution confidence.

The API must never sum authoritative records and reconciliation records together.

In Phase 2, double counting is avoided by tracking conntrack flow keys and storing only byte deltas from that flow's cumulative counters. A Docker packet may appear on veth, bridge, and host NIC interfaces, but interface counters are not used for Internet totals.

## Direction Semantics

For server-generated monitoring:

- Download means bytes received by the local process/container from the remote endpoint.
- Upload means bytes sent by the local process/container to the remote endpoint.

For a flow where the remote endpoint is public, these bytes affect Internet totals. For a remote endpoint such as `192.168.1.3`, they affect LAN totals only.

## ISP Time Buckets

Timestamps are stored in UTC. ISP period calculations convert to the configured display/package timezone, default `Asia/Colombo`.

Initial periods:

- Night: `00:00` to `07:00`.
- Anytime: `07:00` to `24:00`.

Traffic classification happens before ISP bucketing. LAN and Docker internal traffic are never added to Night or Anytime Internet quota.
