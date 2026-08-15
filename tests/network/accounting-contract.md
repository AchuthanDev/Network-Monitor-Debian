# Accounting Contract Fixtures

These fixtures define non-negotiable behavior for the collector and API.

| Local endpoint | Remote endpoint | Expected class | Counts as Internet |
| --- | --- | --- | --- |
| 192.168.1.7 | 192.168.1.3 | lan | no |
| 192.168.1.7 | 8.8.8.8 | internet | yes |
| 172.20.0.4 | 172.20.0.5 | docker_internal | no |
| 127.0.0.1 | 127.0.0.1 | loopback | no |

Any regression that counts LAN, Docker internal, or loopback bytes as Internet is a release blocker.

## Gateway Contract Additions

Gateway mode only claims per-device accuracy when the Debian server is the actual default gateway for the monitored LAN.

| Scenario | Expected behavior |
| --- | --- |
| Client `192.168.50.21` transfers to LAN server `192.168.50.10` | LAN only, Internet remains zero |
| Client `192.168.50.21` transfers to public `8.8.8.8` | Internet counted for the client's MAC-backed device |
| Two clients transfer to public destinations | Each device receives only its own bytes |
| Same flow appears at LAN ingress, pre-NAT forward, and WAN egress | Only the authoritative pre-NAT forward record counts |
| Post-NAT tuple source is Debian WAN IP | Not accepted as authoritative per-device accounting |
| Device DHCP IP changes but MAC is the same | History follows the same device ID |
| Client has IP but no MAC/lease evidence | Identity is ephemeral, not permanent |
| Docker bridge traffic appears near gateway accounting | Docker internal, not a LAN client |
| Large LAN Plex stream | LAN only, Internet remains zero |
