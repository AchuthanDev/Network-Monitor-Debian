#!/bin/sh
set -eu

SIM_A_MB="${SIM_A_MB:-4}"
SIM_B_MB="${SIM_B_MB:-6}"

cleanup() {
  if [ "${SERVER_PID:-}" != "" ]; then
    kill "$SERVER_PID" 2>/dev/null || true
  fi
  for ns in nm-c1 nm-c2 nm-gw nm-wan; do
    ip netns del "$ns" 2>/dev/null || true
  done
}
trap cleanup EXIT

cleanup

ip netns add nm-c1
ip netns add nm-c2
ip netns add nm-gw
ip netns add nm-wan

for ns in nm-c1 nm-c2 nm-gw nm-wan; do
  ip -n "$ns" link set lo up
done

ip link add veth-c1 type veth peer name veth-gw-c1
ip link set veth-c1 netns nm-c1
ip link set veth-gw-c1 netns nm-gw

ip link add veth-c2 type veth peer name veth-gw-c2
ip link set veth-c2 netns nm-c2
ip link set veth-gw-c2 netns nm-gw

ip link add veth-gw-wan type veth peer name veth-wan
ip link set veth-gw-wan netns nm-gw
ip link set veth-wan netns nm-wan

ip -n nm-c1 addr add 10.0.1.2/24 dev veth-c1
ip -n nm-c1 link set veth-c1 up
ip -n nm-c1 route add default via 10.0.1.1

ip -n nm-c2 addr add 10.0.2.2/24 dev veth-c2
ip -n nm-c2 link set veth-c2 up
ip -n nm-c2 route add default via 10.0.2.1

ip -n nm-gw addr add 10.0.1.1/24 dev veth-gw-c1
ip -n nm-gw addr add 10.0.2.1/24 dev veth-gw-c2
ip -n nm-gw addr add 198.51.100.1/24 dev veth-gw-wan
ip -n nm-gw link set veth-gw-c1 up
ip -n nm-gw link set veth-gw-c2 up
ip -n nm-gw link set veth-gw-wan up
ip netns exec nm-gw sh -c 'echo 1 > /proc/sys/net/ipv4/ip_forward'

ip -n nm-wan addr add 198.51.100.2/24 dev veth-wan
ip -n nm-wan link set veth-wan up

ip netns exec nm-gw nft -f - <<'NFT'
table ip nmtest {
  counter client_a_internet {}
  counter client_b_internet {}
  counter client_a_lan {}

  chain forward_prenat {
    type filter hook forward priority -150; policy accept;
    ip saddr 10.0.1.2 ip daddr 198.51.100.2 counter name client_a_internet
    ip daddr 10.0.1.2 ip saddr 198.51.100.2 counter name client_a_internet
    ip saddr 10.0.2.2 ip daddr 198.51.100.2 counter name client_b_internet
    ip daddr 10.0.2.2 ip saddr 198.51.100.2 counter name client_b_internet
    ip saddr 10.0.1.2 ip daddr 10.0.1.0/24 counter name client_a_lan
  }

  chain wan_nat {
    type nat hook postrouting priority srcnat; policy accept;
    oifname "veth-gw-wan" ip saddr { 10.0.1.0/24, 10.0.2.0/24 } masquerade
  }
}
NFT

ip netns exec nm-wan sh -c "mkdir -p /tmp/www && dd if=/dev/zero of=/tmp/www/body.bin bs=1M count=$SIM_A_MB >/dev/null 2>&1 && printf 'HTTP/1.1 200 OK\r\nContent-Length: %s\r\nConnection: close\r\n\r\n' $((SIM_A_MB * 1024 * 1024)) > /tmp/www/response.http && cat /tmp/www/body.bin >> /tmp/www/response.http"
ip netns exec nm-wan socat TCP-LISTEN:8080,bind=198.51.100.2,reuseaddr,fork SYSTEM:'cat /tmp/www/response.http' >/tmp/nm-socat.log 2>&1 &
SERVER_PID="$!"
sleep 2
ip netns exec nm-wan ss -ltn | grep -q ':8080' || {
  echo "FAIL: test HTTP server is not listening in WAN namespace" >&2
  exit 1
}

counter_bytes() {
  ip netns exec nm-gw nft list counter ip nmtest "$1" | awk '{ for (i = 1; i <= NF; i++) if ($i == "bytes") print $(i + 1) }'
}

assert_zero() {
  value="$1"
  name="$2"
  if [ "$value" -ne 0 ]; then
    echo "FAIL: $name expected 0 bytes, got $value" >&2
    exit 1
  fi
}

assert_gt_zero() {
  value="$1"
  name="$2"
  if [ "$value" -le 0 ]; then
    echo "FAIL: $name expected >0 bytes, got $value" >&2
    exit 1
  fi
}

assert_not_double_counted() {
  value="$1"
  payload="$2"
  name="$3"
  limit=$((payload * 2))
  if [ "$value" -ge "$limit" ]; then
    echo "FAIL: $name appears double-counted: value=$value limit=$limit" >&2
    exit 1
  fi
}

lan_before="$(counter_bytes client_a_lan)"
assert_zero "$lan_before" "LAN-to-local before Internet tests"

ip netns exec nm-c1 curl -fsS http://198.51.100.2:8080/blob.bin >/dev/null
ip netns exec nm-c2 curl -fsS http://198.51.100.2:8080/blob.bin >/dev/null
ip netns exec nm-c2 curl -fsS http://198.51.100.2:8080/blob.bin >/dev/null

client_a="$(counter_bytes client_a_internet)"
client_b="$(counter_bytes client_b_internet)"
lan_after="$(counter_bytes client_a_lan)"

assert_gt_zero "$client_a" "client A Internet"
assert_gt_zero "$client_b" "client B Internet"
assert_zero "$lan_after" "LAN/client-to-local Internet accounting"

if [ "$client_a" -eq "$client_b" ]; then
  echo "FAIL: client A and B usage should be separately measured and different" >&2
  exit 1
fi

payload_a=$((SIM_A_MB * 1024 * 1024))
payload_b=$((SIM_A_MB * 2 * 1024 * 1024))
assert_not_double_counted "$client_a" "$payload_a" "client A"
assert_not_double_counted "$client_b" "$payload_b" "client B"

echo "PASS: isolated gateway simulation"
echo "client_a_internet_bytes=$client_a"
echo "client_b_internet_bytes=$client_b"
echo "client_a_lan_bytes=$lan_after"
