#!/bin/sh
set -eu

SIM_A_MB="${SIM_A_MB:-4}"
SIM_B_MB="${SIM_B_MB:-6}"
LAN_CIDR="10.0.50.0/24"
LAN_IFACE="br-mon"
WAN_IFACE="veth-gw-wan"

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

ip link add "$WAN_IFACE" type veth peer name veth-wan
ip link set "$WAN_IFACE" netns nm-gw
ip link set veth-wan netns nm-wan

ip -n nm-c1 addr add 10.0.50.2/24 dev veth-c1
ip -n nm-c1 link set veth-c1 up
ip -n nm-c1 route add default via 10.0.50.1

ip -n nm-c2 addr add 10.0.50.3/24 dev veth-c2
ip -n nm-c2 link set veth-c2 up
ip -n nm-c2 route add default via 10.0.50.1

ip -n nm-gw link add "$LAN_IFACE" type bridge
ip -n nm-gw link set "$LAN_IFACE" up
ip -n nm-gw link set veth-gw-c1 master "$LAN_IFACE"
ip -n nm-gw link set veth-gw-c2 master "$LAN_IFACE"
ip -n nm-gw addr add 10.0.50.1/24 dev "$LAN_IFACE"
ip -n nm-gw addr add 198.51.100.1/24 dev "$WAN_IFACE"
ip -n nm-gw link set veth-gw-c1 up
ip -n nm-gw link set veth-gw-c2 up
ip -n nm-gw link set "$WAN_IFACE" up
ip netns exec nm-gw sh -c 'echo 1 > /proc/sys/net/ipv4/ip_forward'

ip -n nm-wan addr add 198.51.100.2/24 dev veth-wan
ip -n nm-wan link set veth-wan up

go run ./apps/gatewayctl/cmd/network-monitor-gateway nftables \
  --wan "$WAN_IFACE" \
  --lan "$LAN_IFACE" \
  --lan-cidr "$LAN_CIDR" \
  --client-counter client_a=10.0.50.2 \
  --client-counter client_b=10.0.50.3 \
  >/tmp/network-monitor-gateway.nft
ip netns exec nm-gw nft -f /tmp/network-monitor-gateway.nft

ip netns exec nm-gw nft -f /tmp/network-monitor-gateway.nft
ip netns exec nm-gw nft delete table inet network_monitor_gateway
if ip netns exec nm-gw nft list table inet network_monitor_gateway >/dev/null 2>&1; then
  echo "FAIL: rollback removal left project-owned nftables table behind" >&2
  exit 1
fi

ip netns exec nm-gw nft -f /tmp/network-monitor-gateway.nft
( sleep 1; ip netns exec nm-gw nft delete table inet network_monitor_gateway ) &
SAFETY_PID="$!"
wait "$SAFETY_PID"
if ip netns exec nm-gw nft list table inet network_monitor_gateway >/dev/null 2>&1; then
  echo "FAIL: safety timer rollback left project-owned nftables table behind" >&2
  exit 1
fi

ip netns exec nm-gw nft -f /tmp/network-monitor-gateway.nft

ip netns exec nm-wan sh -c "mkdir -p /tmp/www && dd if=/dev/zero of=/tmp/www/body.bin bs=1M count=$SIM_A_MB >/dev/null 2>&1 && printf 'HTTP/1.1 200 OK\r\nContent-Length: %s\r\nConnection: close\r\n\r\n' $((SIM_A_MB * 1024 * 1024)) > /tmp/www/response.http && cat /tmp/www/body.bin >> /tmp/www/response.http"
ip netns exec nm-wan socat TCP-LISTEN:8080,bind=198.51.100.2,reuseaddr,fork SYSTEM:'cat /tmp/www/response.http' >/tmp/nm-socat.log 2>&1 &
SERVER_PID="$!"
sleep 2
ip netns exec nm-wan ss -ltn | grep -q ':8080' || {
  echo "FAIL: test HTTP server is not listening in WAN namespace" >&2
  exit 1
}

counter_bytes() {
  ip netns exec nm-gw nft list counter inet network_monitor_gateway "$1" | awk '{ for (i = 1; i <= NF; i++) if ($i == "bytes") print $(i + 1) }'
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

lan_before="$(counter_bytes nm_gateway_client_lan_upload)"
assert_zero "$lan_before" "LAN-to-local before Internet tests"

ip netns exec nm-c1 curl -fsS http://198.51.100.2:8080/blob.bin >/dev/null
ip netns exec nm-c2 curl -fsS http://198.51.100.2:8080/blob.bin >/dev/null
ip netns exec nm-c2 curl -fsS http://198.51.100.2:8080/blob.bin >/dev/null

client_a_up="$(counter_bytes nm_gateway_client_a_internet_upload)"
client_a_down="$(counter_bytes nm_gateway_client_a_internet_download)"
client_b_up="$(counter_bytes nm_gateway_client_b_internet_upload)"
client_b_down="$(counter_bytes nm_gateway_client_b_internet_download)"
client_a=$((client_a_up + client_a_down))
client_b=$((client_b_up + client_b_down))
lan_after="$(counter_bytes nm_gateway_client_lan_upload)"

assert_gt_zero "$client_a_down" "client A download"
assert_gt_zero "$client_a_up" "client A upload"
assert_gt_zero "$client_b_down" "client B download"
assert_gt_zero "$client_b_up" "client B upload"
assert_zero "$lan_after" "LAN/client-to-local Internet accounting"

if [ "$client_a" -eq "$client_b" ]; then
  echo "FAIL: client A and B usage should be separately measured and different" >&2
  exit 1
fi

payload_a=$((SIM_A_MB * 1024 * 1024))
payload_b=$((SIM_A_MB * 2 * 1024 * 1024))
assert_not_double_counted "$client_a" "$payload_a" "client A"
assert_not_double_counted "$client_b" "$payload_b" "client B"

percent_over_payload() {
  measured="$1"
  payload="$2"
  awk "BEGIN { printf \"%.3f\", (($measured - $payload) / $payload) * 100 }"
}

difference_bytes() {
  measured="$1"
  payload="$2"
  echo $((measured - payload))
}

echo "PASS: isolated gateway simulation"
echo "nftables_table=inet network_monitor_gateway"
echo "rollback_removal=pass"
echo "safety_timer_rollback=pass"
echo "client_a_internet_bytes=$client_a"
echo "client_a_download_bytes=$client_a_down"
echo "client_a_upload_bytes=$client_a_up"
echo "client_a_payload_bytes=$payload_a"
echo "client_a_internet_difference_bytes=$(difference_bytes "$client_a" "$payload_a")"
echo "client_a_download_difference_bytes=$(difference_bytes "$client_a_down" "$payload_a")"
echo "client_a_bidirectional_over_payload_percent=$(percent_over_payload "$client_a" "$payload_a")"
echo "client_a_download_over_payload_percent=$(percent_over_payload "$client_a_down" "$payload_a")"
echo "client_b_internet_bytes=$client_b"
echo "client_b_download_bytes=$client_b_down"
echo "client_b_upload_bytes=$client_b_up"
echo "client_b_payload_bytes=$payload_b"
echo "client_b_internet_difference_bytes=$(difference_bytes "$client_b" "$payload_b")"
echo "client_b_download_difference_bytes=$(difference_bytes "$client_b_down" "$payload_b")"
echo "client_b_bidirectional_over_payload_percent=$(percent_over_payload "$client_b" "$payload_b")"
echo "client_b_download_over_payload_percent=$(percent_over_payload "$client_b_down" "$payload_b")"
echo "client_a_lan_bytes=$lan_after"
