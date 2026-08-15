# Device Accounting

Device accounting is a gateway-mode feature. It depends on client traffic traversing the Debian gateway.

## Identity Model

Stable identity is MAC-first:

```text
id
mac_address
current_ip
hostname
friendly_name
device_type
manufacturer
first_seen
last_seen
status
```

MAC address is the stable identity. IP address is temporary DHCP state. Duplicate IPs must not merge different MAC-backed devices.

## Accounting Event Contract

Collectors must preserve pre-NAT metadata:

```text
timestamp
src_ip
src_mac
dst_ip
src_port
dst_port
protocol
direction
bytes
interface
traffic_class
pre_nat
flow_id
```

The collector must aggregate in memory and flush deltas. It must not write one database row per packet.

## Traffic Classes

- `internet`: public destination/source on the gateway path.
- `lan`: monitored LAN to LAN/private destination.
- `server_local`: client to Debian gateway/server service.
- `docker_internal`: Docker bridge traffic.
- `unknown`: invalid or insufficient evidence.

Only `internet` contributes to measured ISP usage.

## ISP Windows

Default package window:

```text
Timezone: Asia/Colombo
Free/Night: 00:00-07:00
Anytime: 07:00-24:00
```

The bucket is calculated from each transfer timestamp after conversion from UTC to the configured timezone. The same logic applies to server traffic today and device traffic later.

## Classification Honesty

Service/category classification must be evidence-backed. DNS correlation, SNI when available without interception, ASN/provider data, and protocol metadata can increase confidence. The system must never claim all TCP/443 traffic is a specific application.

Allowed confidence labels:

- `high`
- `medium`
- `low`
- `unknown`

When uncertain, display `Encrypted HTTPS / Unknown`.
