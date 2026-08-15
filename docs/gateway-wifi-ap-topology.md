# Wi-Fi AP Gateway Topology

Status: evaluated for future testing only. Gateway mode is still disabled.

## Proposed Topology

```text
Tenda HG7
192.168.1.1
   |
Debian enp0s31f6
WAN / SSH management on 192.168.1.0/24
   |
Debian gateway
   |
wlp1s0 Intel 8260 Wi-Fi AP
192.168.50.1/24
   |
Phone / TV / Laptop
192.168.50.100-192.168.50.220
```

`enp0s31f6` remains the upstream interface and the management path. Gateway preparation must not remove its address, default route, or SSH reachability from `192.168.1.0/24`.

## AP Capability

Host inspection:

```text
iw version: 6.9
interface: wlp1s0
driver: iwlwifi
device family: Intel 8260
current mode: managed
current SSID: Achuthan
current channel: 1 / 2412 MHz, observed with 20/40 MHz width depending on current association state
```

`iw list` reports `* AP` under supported interface modes.

```text
AP mode supported: YES
```

Relevant limits:

- Supported modes include `managed`, `AP`, `AP/VLAN`, `monitor`, `P2P-client`, `P2P-GO`, and `P2P-device`.
- AP plus managed/P2P concurrency is limited to one channel.
- Because this topology uses `wlp1s0` as the monitored LAN, treat it as a dedicated AP during tests. Do not depend on keeping the same radio connected to the HG7 Wi-Fi as a managed client.
- 2.4 GHz channels 1-13 are available; channel 14 is disabled.
- Many 5 GHz channels are marked `no IR`, and DFS channels require radar detection. Initial testing should prefer a clean 2.4 GHz channel such as 1, 6, or 11 unless country/channel configuration is verified.

## Network Management

NetworkManager currently controls both physical interfaces:

- `enp0s31f6`: Ethernet, connected, connection `wan-main`
- `wlp1s0`: Wi-Fi, connected, connection `Achuthan`

Future AP activation must preserve `wan-main` and intentionally move only `wlp1s0` out of managed-client service. This must be done in an approved activation phase, with rollback.

## hostapd vs NetworkManager Hotspot

Recommended implementation for this project: `hostapd` for AP mode plus DHCP-only `dnsmasq`.

Reason:

- `hostapd` gives explicit AP configuration, channel selection, WPA settings, and interface binding.
- The project can keep NAT, forwarding, accounting, DHCP, and DNS behavior under its own dry-run/rollback plan.
- NetworkManager hotspot/shared mode is convenient, but it may automatically create NAT, DHCP/DNS service, and firewall behavior that conflicts with Pi-hole and the project-owned nftables accounting rules.

NetworkManager should still remain the host network manager for `enp0s31f6`. For AP mode, it should either leave `wlp1s0` unmanaged during the approved AP service window or use a controlled connection profile that does not enable NetworkManager's shared NAT/DNS behavior.

## DNS and DHCP

Use Pi-hole for DNS. Do not start another DNS listener on port 53.

Use DHCP-only `dnsmasq` bound to `wlp1s0` after AP mode is approved:

```text
interface=wlp1s0
port=0
bind-interfaces
dhcp-range=192.168.50.100,192.168.50.220,12h
dhcp-option=option:router,192.168.50.1
dhcp-option=option:dns-server,192.168.50.1
```

Pi-hole should remain reachable through the host-published DNS listener on `192.168.50.1:53` after that address is assigned in the approved activation phase.

## Safety Position

This topology is safe to prepare in software because it keeps the Ethernet management path independent from the monitored Wi-Fi LAN. It is not yet safe to activate until:

- The AP plan is reviewed.
- `wlp1s0` disconnection from the current `Achuthan` managed Wi-Fi client is accepted.
- hostapd and DHCP-only dnsmasq dry-run files are reviewed.
- nftables gateway rules are applied only through the project-owned table.
- SSH preservation and automatic rollback are tested immediately before live activation.

No AP, DHCP, NAT, nftables, or interface changes are applied by this document.
