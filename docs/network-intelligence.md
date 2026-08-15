# Network Intelligence

Status: prepared. This does not enable live gateway mode.

## Goals

Raw byte accounting remains authoritative. Classification is a separate metadata layer:

```text
accurate accounting
  -> device identity
  -> destination metadata
  -> classification engine
  -> reports, alerts, and investigation views
```

If classification fails, byte totals remain correct and the category is reported as `unknown_https`, `other`, or another generic metadata-only class.

## Privacy Boundary

The system must not:

- intercept HTTPS
- install certificates on client devices
- decrypt TLS
- inspect passwords
- capture packet payloads
- store browsing page contents

Allowed evidence:

- DNS correlation from the existing Pi-hole deployment
- SNI when available without decryption
- destination IP and port metadata
- protocol metadata
- locally maintained provider rules

## Confidence Model

Every classification includes:

```json
{
  "service": "YouTube",
  "category": "video_streaming",
  "confidence": "high",
  "evidence": ["dns_or_sni:rr1---sn.googlevideo.com"]
}
```

Confidence levels:

- `high`: DNS query for the same client resolved to the destination IP within the correlation window.
- `medium`: domain/SNI matched a provider rule, but IP correlation is weaker.
- `low`: protocol-only metadata such as UDP/443 as QUIC.
- `unknown`: no reliable provider evidence.

CDN IP ownership by itself is not enough to claim a final service.

## Pi-hole Integration

Pi-hole remains the DNS service. The monitor does not replace it.

Preferred ingestion:

1. Read Pi-hole query data from the existing Pi-hole FTL SQLite database through a read-only path.
2. Import only the fields needed for correlation: timestamp, client IP, query domain, resolved IP when available, and source.
3. Store imported DNS observations with privacy-conscious retention.
4. Correlate a later flow only when client IP, destination IP, and time window match.

The Pi-hole documentation describes the FTL long-term query database as SQLite-backed and updated periodically. The monitor should either use Pi-hole's supported SQLite access or a read-only copy/mount of that database, never by stopping Pi-hole or changing its DNS binding.

Current server evidence:

- Pi-hole owns host TCP/UDP port `53`.
- Container: `pihole`
- Process: `/usr/bin/pihole-FTL no-daemon`
- Host PID observed during discovery: `707675`
- Host bindings: `0.0.0.0:53/tcp` and `0.0.0.0:53/udp`

## Provider Architecture

The implementation lives in:

```text
features/traffic-classification/
  domain/
  application/
  infrastructure/
    dns/
    providers/
```

Provider modules expose a common interface and can be changed independently from accounting. Initial provider families:

- YouTube
- Netflix
- Prime Video
- Meta/Facebook/Instagram
- TikTok
- X
- Snapchat
- Downloads
- Software updates
- Generic HTTP/HTTPS/QUIC/DNS fallback

## Reports

Prepared APIs:

- `GET /api/v1/destinations`
- `GET /api/v1/reports/device`
- `GET /api/v1/investigation/hour`
- `GET /api/v1/reports/daily`
- `GET /api/v1/classification/catalog`
- `GET /api/v1/alerts/policy`

Gateway mode must write verified device rows before these endpoints show per-client production data.

## Alerts

Default policies:

- Anytime usage greater than 2 GB per device per day.
- Social/video greater than 2 GB per device per day.
- Unknown Internet traffic greater than 2 GB per device per day.
- 1 GB transferred within 10 minutes.
- Unusual upload spike.
- New device detected.
- Previously inactive device significant usage.

Alert deduplication supports threshold tiers such as 2 GB, 5 GB, and 10 GB plus cooldowns.

## Retention

Recommended defaults are represented in schema:

- DNS observations: 30 days.
- minute aggregates: 30 days.
- hourly aggregates: 1 year.
- daily aggregates: long-term.
- destination hourly data: 1 year.

DNS/domain data uses shorter retention because it is more privacy-sensitive than byte counters.

## Tooling Status

Requested install:

```bash
sudo apt update
sudo apt install -y nftables iw
```

Attempt result in this Codex session:

```text
sudo: a terminal is required to read the password
sudo: a password is required
```

Because installation did not complete:

- `nft --version` is still unavailable from this session.
- `iw --version` is still unavailable from this session.
- `iw list` could not verify Intel 8260 AP mode support.

After installing manually, run:

```bash
nft --version
sudo nft list ruleset
iw --version
iw list
```

Do not enable gateway NAT, DHCP, AP mode, or interface addressing until a monitored LAN interface is selected and explicit approval is given.
