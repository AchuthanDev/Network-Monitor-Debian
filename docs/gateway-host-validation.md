# Gateway Host Validation

Status: read-only validation. Gateway mode is not active.

## Tooling

- `nftables`: `v1.1.3`
- `iw`: `6.9`
- Plain `nft` and `iw` may be missing from an unprivileged shell `PATH`; `/usr/sbin/nft` and `/usr/sbin/iw` are present. Discovery checks `/usr/sbin` explicitly.

## Current nftables Structure

The host ruleset contains Docker-managed compatibility tables:

- `table ip raw`
- `table ip filter`
- `table ip nat`
- `table ip6 filter`
- `table ip6 nat`

The `ip filter` and `ip nat` tables are marked as managed by `iptables-nft`. Network Monitor must not edit these tables directly.

Docker-owned chains observed:

- `DOCKER`
- `DOCKER-USER`
- `DOCKER-FORWARD`
- `DOCKER-BRIDGE`
- `DOCKER-CT`
- `DOCKER-INTERNAL`

Docker NAT is active in `table ip nat`, including per-bridge masquerade rules for `172.17.0.0/16` through `172.25.0.0/16` and published container DNAT rules.

## Existing Project Table

The host already has:

```text
table inet network_monitor
```

This table is used by current server-only accounting:

- `input_account`
- `output_account`
- `forward_account`
- `internet_download`
- `internet_upload`
- `lan_download`
- `lan_upload`

Gateway deployment must not delete this table during rollback because it belongs to the working server-monitoring feature.

## Pre-existing Gateway-looking Rules

The read-only ruleset output shows non-Docker gateway-looking rules in `table ip filter` and `table ip nat`, including:

- FORWARD accepts for `enx00e04c680278` to/from `enp0s31f6`
- DNS reject rules involving `192.168.50.1`
- VPN/public-DNS blocking rules
- broad `oifname "enp0s31f6" masquerade`

`enx00e04c680278` is not currently present in `ip addr`. Treat these as stale or manually-created rules until proven otherwise. Do not remove them automatically. Before live activation, review whether they should be preserved, migrated, or manually removed by the operator.

## Final Gateway nftables Design

Use a separate project-owned gateway table:

```text
table inet network_monitor_gateway
```

Gateway-owned chains:

- `nm_gateway_prenat_account`: `type filter hook forward priority -150`; counts pre-NAT client upload/download and LAN-only traffic once.
- `nm_gateway_forward`: `type filter hook forward priority 0`; allows monitored LAN to WAN forwarding and stateful return traffic.
- `nm_gateway_nat`: `type nat hook postrouting priority srcnat`; masquerades `192.168.50.0/24` to `enp0s31f6`.

This coexists with Docker because it does not modify Docker/iptables-nft tables or chains. Rollback deletes only `inet network_monitor_gateway`.

## Wi-Fi AP Capability

Hardware:

- Interface: `wlp1s0`
- Driver: `iwlwifi`
- Device family: Intel 8260

`iw list` reports `* AP` under supported interface modes.

Result:

```text
AP mode supported: YES
```

Relevant constraints:

- Supports `managed`, `AP`, `AP/VLAN`, `monitor`, `P2P-client`, `P2P-GO`, and `P2P-device`.
- Valid AP concurrency includes one managed interface and one AP/P2P role, with `total <= 3`.
- AP plus managed mode is limited to `#channels <= 1` in the listed combination.
- 2.4 GHz channels 1-13 are available; channel 14 is disabled.
- Many 5 GHz channels are marked `no IR`, and DFS channels require radar detection.

Recommendation remains conservative: use Intel Wi-Fi AP mode only for testing/fallback. Production should use USB Gigabit Ethernet plus an external AP/switch.

## Network Management

NetworkManager is active.

Observed connections:

- `enp0s31f6`: Ethernet, connected, NetworkManager connection `wan-main`
- `wlp1s0`: Wi-Fi, connected, NetworkManager connection `Achuthan`
- Docker bridges: externally managed by Docker
- systemd-networkd: inactive
- `/etc/network/interfaces`: present
- `/etc/netplan`: absent

Future monitored-LAN configuration should use NetworkManager or a compatible mechanism for interface ownership. For the accepted one-Ethernet test topology, `wlp1s0` may be used as a dedicated Wi-Fi AP only after AP mode is confirmed and the current managed Wi-Fi client connection is intentionally released during an approved activation phase.

## IPv4 Forwarding Source

IPv4 forwarding is enabled:

```text
net.ipv4.ip_forward = 1
```

Persistent sources found:

```text
/etc/sysctl.conf:net.ipv4.ip_forward=1
/etc/sysctl.d/99-ipforward.conf:net.ipv4.ip_forward=1
/etc/sysctl.d/99-examshield.conf:net.ipv4.ip_forward = 1
```

This appears to be persistent local configuration, not only Docker runtime behavior. Do not change it during gateway preparation.

## Pi-hole DNS Design

Pi-hole is a Docker container on the `media` network:

- Container IP: `172.20.0.5`
- Host DNS publishing: `0.0.0.0:53/tcp` and `0.0.0.0:53/udp`
- Web UI publishing: `0.0.0.0:8082->80/tcp`

Future monitored-LAN clients should receive DNS as `192.168.50.1` only if the host-published Pi-hole listener remains reachable on that address after the LAN IP is assigned. Do not start another DNS listener on port 53.

Preferred DNS/DHCP split:

- Pi-hole: DNS only, continuing to own TCP/UDP port 53.
- dnsmasq: DHCP only, bound only to the monitored LAN interface, with DNS option pointing clients to the Debian gateway address where Pi-hole is published.

Do not bind dnsmasq DNS to port 53. Do not run competing DNS services on the monitored LAN.

## DHCP Recommendation

Use dnsmasq DHCP-only for the monitored LAN:

```text
interface=<MONITORED_LAN_INTERFACE>
port=0
bind-interfaces
dhcp-range=192.168.50.100,192.168.50.220,12h
dhcp-option=option:router,192.168.50.1
dhcp-option=option:dns-server,192.168.50.1
```

Pi-hole DHCP is not recommended for initial gateway activation because Pi-hole is already Docker-published for DNS and changing it increases rollback risk. Kea is more powerful than needed for this home gateway phase.

## SSH Preservation

Current management network:

```text
192.168.1.0/24
```

Current management NIC:

```text
enp0s31f6
```

Gateway activation must not remove the `192.168.1.0/24` address, default route, or SSH reachability on `enp0s31f6`.

## Current Readiness Summary

Read-only gateway readiness with the current host state:

- WAN interface `enp0s31f6`: pass
- Monitored LAN interface: `wlp1s0` is AP-capable and can be selected for a future Wi-Fi AP test; otherwise no dedicated Ethernet LAN is connected
- nftables availability: pass
- IPv4 forwarding: pass
- Docker subnet overlap with `192.168.50.0/24`: pass
- VPN subnet overlap with `192.168.50.0/24`: pass
- DHCP conflict: pass
- Pi-hole DNS: pass, expected DNS owner
- DNS conflict: warning, expected because Pi-hole owns port 53
- Existing gateway-looking rules: warning, review stale/manual `enx00e04c680278` and `192.168.50.1` rules before activation
- Accounting simulation: pass
- Rollback plan: pass
- Automatic rollback timer design: pass

Activation remains blocked until an approved monitored-LAN interface is selected. `wlp1s0` is technically viable for a controlled AP test, but live activation still requires explicit approval and rollback.

## USB Ethernet Requirement

Required:

- USB 3.0 or newer
- Gigabit Ethernet, 10/100/1000 Mbps
- Full duplex capable
- Linux-supported driver in the Debian kernel
- No vendor-specific userspace software requirement

Preferred: Linux-friendly Realtek or ASIX USB Gigabit controllers supported by the installed Debian kernel.

When connected, discovery must inspect:

- interface name
- MAC
- driver
- speed
- duplex
- link state
- IPv4/IPv6 addresses
- routes/default route ownership
- NetworkManager ownership
- bridge/bond membership
