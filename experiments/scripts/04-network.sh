#!/usr/bin/env bash
# 实验 04：veth pair + bridge + network namespace + NAT

set -euo pipefail
source "$(dirname "$0")/lib.sh"

readonly NETNS="gantry-lab-net"
readonly BR_NAME="gantry-br0"
readonly VETH_HOST="gantry-veth0"
readonly VETH_NS="gantry-veth1"
readonly SUBNET="10.200.1.0/24"
readonly IP_FORWARD_STATE="/tmp/gantry-lab-ip-forward"

cleanup() {
  iptables -t nat -D POSTROUTING -s "$SUBNET" -j MASQUERADE 2>/dev/null || true
  ip netns del "$NETNS" 2>/dev/null || true
  ip link del "$VETH_HOST" 2>/dev/null || true
  ip link del "$BR_NAME" 2>/dev/null || true
  if [[ -f "$IP_FORWARD_STATE" ]]; then
    original="$(cat "$IP_FORWARD_STATE")"
    [[ "$original" =~ ^[01]$ ]] && echo "$original" >/proc/sys/net/ipv4/ip_forward
    rm -f "$IP_FORWARD_STATE"
  fi
}
trap cleanup EXIT

log "=== 实验 04：veth + bridge + NAT ==="
ip netns add "$NETNS"
ip link add "$BR_NAME" type bridge
ip addr add 10.200.1.1/24 dev "$BR_NAME"
ip link set "$BR_NAME" up

ip link add "$VETH_HOST" type veth peer name "$VETH_NS"
ip link set "$VETH_HOST" master "$BR_NAME"
ip link set "$VETH_HOST" up
ip link set "$VETH_NS" netns "$NETNS"
ip -n "$NETNS" link set lo up
ip -n "$NETNS" link set "$VETH_NS" up
ip -n "$NETNS" addr add 10.200.1.2/24 dev "$VETH_NS"
ip -n "$NETNS" route add default via 10.200.1.1

cat /proc/sys/net/ipv4/ip_forward >"$IP_FORWARD_STATE"
echo 1 >/proc/sys/net/ipv4/ip_forward
iptables -t nat -A POSTROUTING -s "$SUBNET" -j MASQUERADE

output="$(ip netns exec "$NETNS" bash -c '
  echo "NS_IP=$(ip -o -4 addr show gantry-veth1 | awk "{print \$4}")"
  ping -c 1 -W 2 10.200.1.1 >/dev/null
  echo PING_BRIDGE=OK
')"
printf '%s\n' "$output"
assert_contains "$output" "NS_IP=10.200.1.2/24"
assert_contains "$output" "PING_BRIDGE=OK"

if ip netns exec "$NETNS" ping -c 1 -W 2 8.8.8.8 >/dev/null 2>&1; then
  log "可选检查通过：NAT 可访问外网"
else
  log "可选检查跳过：外网 ICMP 不可达，不影响 bridge 断言"
fi
log "=== 实验 04 完成 ==="
