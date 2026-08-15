# One-Device Wi-Fi Gateway Test

This is a preparation document only. It does not activate `wlp1s0`, DHCP, NAT, forwarding rules, Pi-hole changes, or hostapd.

## Current Hardware State

- WAN and SSH management: `enp0s31f6`, `192.168.1.7` management path.
- Wi-Fi candidate: `wlp1s0`, Intel 8260, `iwlwifi`.
- Current Wi-Fi role: NetworkManager managed client, SSID `Achuthan`, address `192.168.1.6`.
- Current Wi-Fi fallback route: `192.168.1.1` via `wlp1s0`, metric `600`.
- AP capability: confirmed by `iw list`.
- AP plus managed concurrency: supported on one shared channel only; the test treats `wlp1s0` as a dedicated AP and releases the current managed profile.

## Prepared Test Network

```text
SSID:       NetworkMonitor-Test
Interface:  wlp1s0
Gateway:    192.168.50.1/24
DHCP:       192.168.50.100-192.168.50.150
DNS:        192.168.50.1, Pi-hole on the Debian host
Band:       2.4 GHz
Channel:    6
```

The generated hostapd template uses `${NETWORK_MONITOR_AP_PASSPHRASE}`. The value must be supplied through a protected service environment or secret store and must never be committed.

## Future Activation Sequence

1. Confirm a current SSH session through `enp0s31f6` and record the rollback deadline.
2. Confirm the exact dry-run hostapd, DHCP-only dnsmasq, and project nftables output.
3. Intentionally disconnect only NetworkManager profile `Achuthan` from `wlp1s0`.
4. Assign `192.168.50.1/24` to `wlp1s0` and start hostapd.
5. Start DHCP-only dnsmasq with `port=0`; do not create a DNS listener.
6. Apply only `inet network_monitor_gateway` and start the 120-second safety timer.
7. Connect one test phone and verify DHCP, gateway, DNS, Internet, Pi-hole correlation, and dashboard identity.
8. Test a known-size download, upload, and local Plex access. Local Plex must remain LAN traffic with zero ISP bytes.
9. Confirm the test device's free/Anytime buckets and classification evidence.
10. Confirm the change or run rollback. On failure, rollback removes only project-owned state and leaves `enp0s31f6`, SSH, HG7, Docker, and Pi-hole untouched.

## Management Policy

The monitored subnet may use Internet, Pi-hole DNS, and explicitly selected local services. SSH, database ports, Portainer, internal APIs, and other management ports are denied by default. This policy remains a generated plan until a separate approved activation change.

HG7 Wi-Fi remains enabled throughout the test. Existing household devices stay on HG7; only the selected phone joins `NetworkMonitor-Test`.
