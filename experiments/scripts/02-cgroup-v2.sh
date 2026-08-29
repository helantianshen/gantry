#!/usr/bin/env bash
# 实验 02：cgroup v2 memory/pids/cpu 控制器

set -euo pipefail
source "$(dirname "$0")/lib.sh"

readonly CGPATH="/sys/fs/cgroup/gantry-lab-cgroup"

cleanup() {
  if [[ -d "$CGPATH" ]]; then
    [[ -f "$CGPATH/cgroup.kill" ]] && echo 1 >"$CGPATH/cgroup.kill" 2>/dev/null || true
    rmdir "$CGPATH" 2>/dev/null || true
  fi
}
trap cleanup EXIT

log "=== 实验 02：cgroup v2 资源限制 ==="
mkdir "$CGPATH"
echo 16777216 >"$CGPATH/memory.max"
echo 5 >"$CGPATH/pids.max"
echo "50000 100000" >"$CGPATH/cpu.max"
assert_equals "$(cat "$CGPATH/cpu.max")" "50000 100000"

oom_before="$(awk '$1 == "oom_kill" {print $2}' "$CGPATH/memory.events")"
set +e
bash -c "echo \$\$ >'$CGPATH/cgroup.procs'; exec python3 -c 'import time; x = bytearray(64 * 1024 * 1024); time.sleep(1)'" \
  >/dev/null 2>&1
oom_status=$?
set -e
oom_after="$(awk '$1 == "oom_kill" {print $2}' "$CGPATH/memory.events")"
if (( oom_status == 0 || oom_after <= oom_before )); then
  log "断言失败：memory.max 未触发 OOM kill"
  exit 1
fi
log "断言通过：memory.events oom_kill ${oom_before}→${oom_after}"

pids_before="$(awk '$1 == "max" {print $2}' "$CGPATH/pids.events")"
set +e
bash -c "
  echo \$\$ >'$CGPATH/cgroup.procs'
  for _ in {1..10}; do sleep 2 & done
  wait
" >/dev/null 2>&1
set -e
pids_after="$(awk '$1 == "max" {print $2}' "$CGPATH/pids.events")"
if (( pids_after <= pids_before )); then
  log "断言失败：pids.max 未拒绝并发 fork"
  exit 1
fi
log "断言通过：pids.events max ${pids_before}→${pids_after}"
log "=== 实验 02 完成 ==="
