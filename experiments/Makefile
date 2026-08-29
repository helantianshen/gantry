.PHONY: check demo clean

check:
	@echo "=== 环境检查 ==="
	@test $$(id -u) -eq 0 || { echo "需要 root 权限"; exit 1; }
	@command -v unshare >/dev/null 2>&1 || { echo "缺少 unshare（util-linux）"; exit 1; }
	@command -v ip >/dev/null 2>&1 || { echo "缺少 ip（iproute2）"; exit 1; }
	@command -v iptables >/dev/null 2>&1 || { echo "缺少 iptables"; exit 1; }
	@command -v python3 >/dev/null 2>&1 || { echo "缺少 python3"; exit 1; }
	@command -v ping >/dev/null 2>&1 || { echo "缺少 ping"; exit 1; }
	@mount | grep -q "cgroup2" || { echo "cgroup v2 未挂载"; exit 1; }
	@grep -qw memory /sys/fs/cgroup/cgroup.controllers || { echo "cgroup memory controller 不可用"; exit 1; }
	@grep -qw pids /sys/fs/cgroup/cgroup.controllers || { echo "cgroup pids controller 不可用"; exit 1; }
	@grep -qw cpu /sys/fs/cgroup/cgroup.controllers || { echo "cgroup cpu controller 不可用"; exit 1; }
	@echo "环境检查通过"

demo: check
	@echo "=== 运行全部实验 ==="
	@bash scripts/01-namespace.sh
	@bash scripts/02-cgroup-v2.sh
	@bash scripts/03-overlayfs.sh
	@bash scripts/04-network.sh
	@echo "=== 全部实验完成 ==="

clean:
	@echo "=== 清理残留资源 ==="
	@test $$(id -u) -eq 0 || { echo "需要 root 权限"; exit 1; }
	-bash scripts/lib.sh cleanup_all
	@echo "清理完成"
