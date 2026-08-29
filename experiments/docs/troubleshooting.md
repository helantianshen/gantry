# 故障排查指南

> 第三方可复现：每条命令可在全新 Linux 上独立执行验证。

## 工具速查

| 工具 | 安装 | 用途 |
|------|------|------|
| `strace` | `strace` 包 | 追踪系统调用（clone/setns/mount） |
| `nsenter` | `util-linux` | 进入已有命名空间执行命令 |
| `ip` | `iproute2` | 查看/操作网络设备、命名空间 |
| `nft` / `iptables` | `nftables` / `iptables` | 查看/操作 NAT 规则 |
| `cat /sys/fs/cgroup/.../cgroup.events` | 内核 | 查看 cgroup OOM 事件 |
| `unshare` | `util-linux` | 创建新命名空间 |
| `mount` | `util-linux` | 查看/操作挂载点 |

---

## 实验 01：namespace 排查

### 问题：unshare 报 "Operation not permitted"

**原因**：用户命名空间未启用或 seccomp 限制。

**排查**：
```bash
# 检查内核是否允许非特权用户使用 user namespace
cat /proc/sys/kernel/unprivileged_userns_clone
# 0 = 禁止非特权用户，1 = 允许

# 如果是 0 且需要非特权用户使用
echo 1 | sudo tee /proc/sys/kernel/unprivileged_userns_clone

# 如果仍然失败，检查 seccomp
cat /proc/self/status | grep Seccomp
# 0=disabled 1=strict 2=filter
```

**解决**：使用 root 权限运行（`sudo make demo`），或开启 unprivileged_userns_clone。

### 问题：hostname 在 unshare 后没有变化

**原因**：未进入 UTS namespace 或 hostname 命令未在新 namespace 中执行。

**排查**：
```bash
# 确认 unshare 包含 --uts
unshare --uts bash -c 'hostname test123; hostname'
# 应输出 test123

# 在宿主检查
hostname
# 应恢复原 hostname
```

### 问题：PID=1 但 ps aux 看到宿主进程

**原因**：未挂载新的 /proc。`ps` 从 /proc 读取进程列表，宿主的 /proc 包含所有进程。

**解决**：
```bash
unshare --pid --mount --fork bash -c '
	mount -t proc proc /proc
	ps aux  # 此时应只看到命名空间内进程
'
```

---

## 实验 02：cgroup v2 排查

### 问题：创建 cgroup 报 "Read-only file system"

**原因**：cgroup v2 未挂载在 /sys/fs/cgroup。

**排查**：
```bash
mount | grep cgroup2
# 应看到：cgroup2 on /sys/fs/cgroup type cgroup2

# 如果没有
mount -t cgroup2 none /sys/fs/cgroup
```

### 问题：进程未被 OOM 杀死

**原因**：①进程未加入 cgroup；②memory.max 设置无效；③内核的 OOM killer 延迟。

**排查**：
```bash
# 1. 确认进程在 cgroup 中
cat /sys/fs/cgroup/gantry-lab-cgroup/cgroup.procs
# 应包含目标 PID

# 2. 确认 memory.max 已设置
cat /sys/fs/cgroup/gantry-lab-cgroup/memory.max
# 应输出数字（字节）

# 3. 查看 OOM 事件
cat /sys/fs/cgroup/gantry-lab-cgroup/memory.events
# oom_kill 的计数增加说明内核杀死了超限进程
```

### 问题：pids.max 不生效

**原因**：子进程没有继承目标 cgroup，或测试进程是串行执行，未同时超过上限。

**排查**：每个 fork 的子进程必须在 `cgroup.procs` 中写入自己的 PID。bash 的 `$$` 是当前 shell PID，但 `&` 后台进程可能有不同 PID。

```bash
# 正确做法：在子 shell 中写入
bash -c 'echo $$ > /sys/fs/cgroup/gantry-lab-cgroup/cgroup.procs; for i in {1..10}; do sleep 2 & done; wait'
cat /sys/fs/cgroup/gantry-lab-cgroup/pids.events
```

---

## 实验 03：OverlayFS 排查

### 问题：mount overlay 报 "overlayfs: missing 'lowerdir'"

**原因**：mount 选项格式错误。OverlayFS 要求 `lowerdir`、`upperdir`、`workdir` 全部指定。

**排查**：
```bash
# 正确格式
mount -t overlay overlay \
	-o "lowerdir=/tmp/lower,upperdir=/tmp/upper,workdir=/tmp/work" \
	/tmp/merged
```

### 问题：修改文件后 lower 层也变了

**原因**：挂载时未指定 `upperdir`（只读 overlay）。或 lower 和 upper 指向同一目录。

**排查**：
```bash
# 确认挂载选项
mount | grep overlay
# 应包含 upperdir=

# 确认 upper 和 lower 不是同一目录
[ "$(realpath /tmp/lower)" = "$(realpath /tmp/upper)" ] && echo "冲突" || echo "正常"
```

### 问题：whiteout 文件残留

**原因**：删除文件时 OverlayFS 在 upper 层创建 whiteout 字符设备。

**排查**：
```bash
# 查看 upper 层中的 whiteout
ls -la /tmp/upper/
# whiteout 显示为 c--------- 文件（字符设备，主次设备号 0/0）
```

---

## 实验 04：网络排查

### 问题：veth pair 创建失败

**原因**：非 root 或缺少 iproute2。

```bash
id -u  # 必须是 0
which ip  # 必须存在
```

### 问题：ping bridge IP 不通

**原因**：veth 未接入 bridge 或 bridge 未 up。

**排查**：
```bash
# 查看 bridge 下挂的端口
ip link show master gantry-br0
# 应看到 gantry-veth0

# 确认 bridge 状态
ip link show gantry-br0
# 应有 UP 标志

# 查看ARP表
ip neigh show dev gantry-br0
```

### 问题：ping 外网不通（NAT 未生效）

**原因**：ip_forward 未开或 NAT 规则未添加。

**排查**：
```bash
# 1. 确认 IP 转发开启
cat /proc/sys/net/ipv4/ip_forward
# 必须是 1

# 2. 确认 NAT 规则
iptables -t nat -L POSTROUTING -n
# 应有 MASQUERADE 规则，源 10.200.1.0/24

# 3. 确认子命名空间有默认路由
# 在命名空间内执行
ip route show
# 应有 default via 10.200.1.1
```

### 问题：命名空间内 ip link show 只看到 lo

**原因**：veth 另一端未移入命名空间。

```bash
# 在命名空间内
ip link show
# 应看到 lo + gantry-veth1

# 如果只有 lo，说明 veth 未移入
# 检查宿主侧
ip link show gantry-veth1
# 如果在宿主侧，手动移入
ip link set gantry-veth1 netns gantry-lab-net
```

---

## 残留资源清理

如果实验中途失败，可能残留挂载点、cgroup、网络设备。手动清理：

```bash
# OverlayFS 残留挂载
mount | grep lab-overlay | awk '{print $3}' | xargs -r umount

# cgroup 残留
rmdir /sys/fs/cgroup/gantry-lab-cgroup 2>/dev/null

# 网络设备残留
ip netns del gantry-lab-net 2>/dev/null
ip link del gantry-br0 2>/dev/null
ip link del gantry-veth0 2>/dev/null

# iptables 残留规则
iptables -t nat -D POSTROUTING -s 10.200.1.0/24 -j MASQUERADE 2>/dev/null

# /tmp 残留目录
rm -rf /tmp/lab-*
```

或直接运行：
```bash
make clean
```
