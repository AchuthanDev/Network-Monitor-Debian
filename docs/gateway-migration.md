# Gateway Migration Plan

This plan is intentionally staged. Do not enable gateway networking until simulation tests pass and a dry-run plan is reviewed.

## Stage 1: Read-Only Foundation

1. Keep `server_only` mode active.
2. Run gateway discovery.
3. Review detected WAN interface, candidate LAN interfaces, Docker networks, DHCP/DNS listeners, nftables availability, and IP forwarding state.
4. Generate a dry-run gateway plan.
5. Do not apply any network change.

## Stage 2: Hardware and Subnet Preparation

1. Choose a dedicated monitored LAN interface.
2. Prefer a USB Gigabit Ethernet adapter for the LAN side.
3. Verify Wi-Fi AP support before using Wi-Fi as a LAN interface.
4. Use a non-overlapping monitored LAN subnet, default recommendation `192.168.50.0/24`.
5. Confirm Docker networks do not overlap the monitored LAN subnet.

## Stage 3: Isolated Lab Validation

1. Use Linux network namespace simulation.
2. Prove LAN-only traffic does not count as Internet.
3. Prove public traffic counts for the correct client.
4. Prove NAT does not destroy attribution.
5. Prove the same flow is counted once.
6. Prove free/night and anytime buckets at boundaries.

## Stage 4: Single-Client Trial

1. Connect one disposable client to the monitored LAN.
2. Validate DHCP lease identity by MAC.
3. Validate DNS behavior.
4. Download a known-size public file.
5. Transfer a known-size LAN file.
6. Compare measured bytes and document error.

## Stage 5: Gradual Device Migration

Move devices in small groups. Keep the HG7 network available as fallback until monitoring, routing, DHCP, DNS, and Docker services are stable.

## Approval Gate

Gateway activation requires explicit approval after reviewing:

- generated plan,
- rollback plan,
- simulation results,
- single-client validation,
- Docker coexistence checks,
- SSH management path risk.
