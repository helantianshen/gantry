#!/usr/bin/env bash
set -euo pipefail

readonly PREFIX="[lab]"

log() { echo "$PREFIX $*"; }

assert_contains() {
  [[ "$1" == *"$2"* ]] || { log "断言失败：不包含 '$2'"; exit 1; }
  log "断言通过：包含 '$2'"
}

assert_equals() {
  [[ "$1" == "$2" ]] || { log "断言失败：'$1' != '$2'"; exit 1; }
  log "断言通过：'$1' == '$2'"
}

cleanup_all() {
  umount /tmp/lab-overlayfs/merged 2>/dev/null || true
  rm -rf /tmp/lab-overlayfs /tmp/lab-namespace
  if [[ -d /sys/fs/cgroup/gantry-lab-cgroup ]]; then
    [[ -f /sys/fs/cgroup/gantry-lab-cgroup/cgroup.kill ]] && echo 1 >/sys/fs/cgroup/gantry-lab-cgroup/cgroup.kill 2>/dev/null || true
    rmdir /sys/fs/cgroup/gantry-lab-cgroup 2>/dev/null || true
  fi
  iptables -t nat -D POSTROUTING -s 10.200.1.0/24 -j MASQUERADE 2>/dev/null || true
  ip netns del gantry-lab-net 2>/dev/null || true
  ip link del gantry-veth0 2>/dev/null || true
  ip link del gantry-br0 2>/dev/null || true
  if [[ -f /tmp/gantry-lab-ip-forward ]]; then
    original="$(cat /tmp/gantry-lab-ip-forward)"
    [[ "$original" =~ ^[01]$ ]] && echo "$original" >/proc/sys/net/ipv4/ip_forward
    rm -f /tmp/gantry-lab-ip-forward
  fi
  log "全局清理完成"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  [[ "${1:-}" == "cleanup_all" ]] || { echo "usage: $0 cleanup_all" >&2; exit 2; }
  cleanup_all
fi
