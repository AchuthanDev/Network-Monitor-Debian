# Gateway Hardware Readiness

Status: preparation only. Gateway mode is not active.

## Current State

- WAN/management interface: `enp0s31f6`
- Current SSH client: `192.168.1.2`
- Existing management LAN: `192.168.1.0/24`
- Proposed monitored LAN: `192.168.50.0/24`
- Proposed gateway IP: `192.168.50.1`
- Proposed DHCP pool: `192.168.50.100-192.168.50.220`
- Dedicated monitored-LAN NIC: not connected
- IPv4 forwarding: already enabled
- Pi-hole owns host TCP/UDP port `53`

## Tooling Status

Approved install command:

```bash
sudo apt update
sudo apt install -y nftables iw
```

The command could not run from this Codex session because sudo requires a password:

```text
sudo: a password is required
```

After installing manually, run these read-only checks:

```bash
nft --version
sudo nft list ruleset
iw --version
iw list
```

Do not enable a new firewall policy, flush rules, restart Docker, change routing, configure Wi-Fi, or reboot.

## nftables Ruleset Review

Once `nft` is installed, review and record:

- Docker-created tables/chains
- host firewall tables/chains
- any existing project-owned table named `network_monitor`
- NAT/postrouting chains
- forward chains
- default policies

Expected project design remains isolated:

```text
table inet network_monitor
  chain forward_prenat_account
  chain gateway_forward
  chain wan_nat
```

The project must not modify Docker-owned chains directly.

## Wi-Fi AP Capability

Current Wi-Fi hardware:

- Interface: `wlp1s0`
- Chipset: Intel 8260
- Driver: `iwlwifi`

Run:

```bash
iw list
```

Check:

```text
Supported interface modes:
  * AP
```

Report `AP supported: yes` only if `* AP` appears. This is informational only. Do not configure AP mode.

## Production Topology

Production recommendation remains dedicated Ethernet plus external AP/switch:

```text
SLT Fibre
   |
Tenda HG7
192.168.1.1
   |
Debian WAN-side enp0s31f6
existing 192.168.1.0/24 management path
   |
Debian USB Gigabit Ethernet
192.168.50.1/24
   |
External AP / Switch
bridge/AP mode only
   |
Phone / TV / Laptop / IoT
```

The external AP must:

- run in access point/bridge mode only
- not perform NAT
- not run DHCP
- not run a second firewall/router mode for client traffic
- not use `192.168.1.0/24` on the monitored client side
- use a static management IP inside the monitored LAN, for example `192.168.50.2`
- hand client default gateway and DNS authority to Debian at `192.168.50.1`

Debian will eventually be the gateway and DHCP authority for `192.168.50.0/24`, but only after explicit approval.

## Automatic NIC Detection

Discovery now reports:

- interface name
- MAC address
- driver
- link state
- carrier
- negotiated speed
- duplex
- bridge/bond master, if the interface is enslaved
- IPv4 addresses
- IPv6 addresses
- per-interface routes
- default route ownership
- NetworkManager connection ownership
- monitored-LAN candidate status and reason

Expected USB Ethernet names may include:

```text
enx001122334455
enp...
```

The software must not assume the final interface name.

## LAN Candidate Requirements

When a second NIC appears, readiness should require:

- WAN and LAN are different interfaces
- LAN has link up
- LAN negotiates `1000 Mb/s` or faster unless `NETWORK_MONITOR_GATEWAY_ALLOW_SLOW_LAN=true` is explicitly set
- LAN is full duplex
- LAN has no default route
- LAN is not already part of another bridge or bond
- LAN is not Docker/bridge/veth/loopback
- LAN is not the management Wi-Fi fallback
- LAN subnet does not overlap current WAN LAN
- LAN subnet does not overlap Docker networks
- LAN subnet does not overlap active VPN routes
- no DHCP conflict on the monitored LAN
- DNS/Pi-hole plan is explicit
- nftables tooling is available

Recommended values:

```text
LAN subnet: 192.168.50.0/24
Gateway: 192.168.50.1
DHCP: 192.168.50.100-192.168.50.220
```

## Dry-Run Operator Commands

The preferred command shape for future live preparation is:

```bash
network-monitor gateway plan \
  --mode gateway \
  --wan enp0s31f6 \
  --lan <SECOND_INTERFACE> \
  --lan-cidr 192.168.50.0/24 \
  --gateway-ip 192.168.50.1 \
  --dhcp \
  --dhcp-start 192.168.50.100 \
  --dhcp-end 192.168.50.220 \
  --dns-mode pihole
```

`network-monitor gateway apply` and `network-monitor gateway rollback` remain dry-run-only on the current branch unless an explicit live activation phase is approved.

## Accounting Validation

Namespace tests use the same nftables generator as the production dry-run plan. The stress test records:

- expected payload bytes
- measured Internet bytes
- measured-minus-payload difference
- bidirectional overhead percentage
- download-side payload overhead percentage
- LAN-only bytes

Measured bytes are expected to be slightly above payload bytes because accounting includes TCP/IP framing, HTTP headers, connection setup/teardown, client ACK/control upload traffic, and any retransmissions at the selected measurement boundary. This is legitimate transport overhead, not a quota-classification error. LAN-only traffic must remain `0` Internet bytes.

## Activation Checklist

Gateway apply must remain disabled until all required checks pass:

```text
[ ] USB Ethernet connected
[ ] interface detected
[ ] 1 Gbps link confirmed
[ ] WAN unchanged
[ ] LAN subnet validated
[ ] nftables available
[ ] Pi-hole reachable
[ ] DHCP plan validated
[ ] DNS plan validated
[ ] SSH management path preserved
[ ] rollback tested
[ ] automatic rollback timer tested
[ ] CI green
[ ] explicit user approval
```

## No HG7 Changes Yet

Do not modify the Tenda HG7 yet.

Do not:

- disable DHCP
- bridge WAN
- change PPPoE
- change VLAN
- change PON settings
- change TR-069
- change Wi-Fi
- change routing
