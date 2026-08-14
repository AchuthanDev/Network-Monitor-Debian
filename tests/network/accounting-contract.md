# Accounting Contract Fixtures

These fixtures define non-negotiable behavior for the collector and API.

| Local endpoint | Remote endpoint | Expected class | Counts as Internet |
| --- | --- | --- | --- |
| 192.168.1.7 | 192.168.1.3 | lan | no |
| 192.168.1.7 | 8.8.8.8 | internet | yes |
| 172.20.0.4 | 172.20.0.5 | docker_internal | no |
| 127.0.0.1 | 127.0.0.1 | loopback | no |

Any regression that counts LAN, Docker internal, or loopback bytes as Internet is a release blocker.
