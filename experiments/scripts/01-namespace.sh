#!/usr/bin/env bash
# 实验 01：UTS/PID/Mount/Net namespace 隔离

set -euo pipefail
source "$(dirname "$0")/lib.sh"

log "=== 实验 01：namespace 隔离 ==="
host_hostname="$(hostname)"

output="$(unshare --uts --pid --mount --net --fork --mount-proc bash -c '
  hostname lab-container
  ip link set lo up
  echo "NS_HOSTNAME=$(hostname)"
  echo "NS_PID=$$"
  echo "NS_NET=$(ip -o link show lo)"
')"

printf '%s\n' "$output"
assert_contains "$output" "NS_HOSTNAME=lab-container"
assert_contains "$output" "NS_PID=1"
assert_contains "$output" "NS_NET="
assert_equals "$(hostname)" "$host_hostname"
log "=== 实验 01 完成 ==="
