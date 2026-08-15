# Gateway Rollback Plan

Rollback must only remove Network Monitor-owned configuration. It must not flush Docker or unrelated firewall rules.

## Generated Rollback Items

Every dry-run/apply plan must include:

```text
nft delete table inet network_monitor_gateway
systemctl stop network-monitor-dnsmasq.service
ip addr del <gateway_ip> dev <lan_interface>
sysctl -w net.ipv4.ip_forward=<previous-value>
```

The real apply command must capture previous values before changing anything.

## Manual Device Recovery

1. Move clients back to the HG7 Wi-Fi/LAN.
2. Renew DHCP leases on clients.
3. Verify clients use HG7 as default gateway.
4. Verify DNS resolves through the previous DNS path.

## Server Recovery Checks

1. Confirm SSH remains reachable.
2. Confirm default route points to HG7.
3. Confirm Docker containers are running.
4. Confirm published services remain reachable.
5. Confirm Network Monitor returns to `server_only` mode.

## Safety Rules

- Do not reboot automatically.
- Do not delete non-Network-Monitor nftables tables.
- Do not stop Pi-hole, Docker, or existing services unless the user explicitly approves.
- Do not run DHCP on `192.168.1.0/24` while HG7 DHCP is active.

## Automatic Safety Timer

Future live apply must start a rollback timer before changes become permanent:

```text
apply dry-run reviewed
  -> apply live changes
  -> start 120-second timer
  -> verify management SSH path
  -> require explicit confirmation
  -> cancel timer only after confirmation
```

If confirmation is not received, rollback must remove only project-owned gateway state such as `inet network_monitor_gateway`, the project DHCP service, and the monitored-LAN address. Namespace simulation validates this flow without touching the host network.
