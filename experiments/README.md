# Linux 容器原语实验集

> 用可断言的小实验观察 namespace、cgroup v2、OverlayFS 和 veth/bridge/NAT。
> 定位：实验性学习项目，**非安全沙箱**，不可用于生产隔离。

## 安全边界

- 仅使用可信 rootfs（官方 alpine/ubuntu 镜像解压）
- 非 PID 隔离安全沙箱——namespace 隔离是可见性隔离，不提供安全隔离
- 建议在一次性 VM 中运行，避免污染宿主
- 所有脚本在退出时清理挂载点、cgroup、网络设备（trap + cleanup 函数）
- 需要 root 权限运行

## 实验列表

| # | 主题 | 脚本 | 覆盖原语 |
|---|------|------|----------|
| 01 | namespace 隔离 | `scripts/01-namespace.sh` | UTS/PID/Mount/Net namespace + 独立 `/proc` |
| 02 | cgroup v2 资源限制 | `scripts/02-cgroup-v2.sh` | memory.max / pids.max / cpu.max + OOM 触发 |
| 03 | OverlayFS 联合挂载 | `scripts/03-overlayfs.sh` | lower/upper/work/merged + CoW + whiteout |
| 04 | 网络隔离与 NAT | `scripts/04-network.sh` | veth pair + bridge + iptables NAT + 跨命名空间通信 |

## 快速开始

```bash
# 检查环境
make check

# 运行全部实验
sudo make demo

# 单个实验
sudo bash scripts/01-namespace.sh
sudo bash scripts/02-cgroup-v2.sh
sudo bash scripts/03-overlayfs.sh
sudo bash scripts/04-network.sh

# 清理残留资源
make clean
```

## 目录结构

```
experiments/
├── README.md           # 本文件（安全边界 + 快速开始）
├── Makefile            # check / demo / clean 目标
├── scripts/
│   ├── 01-namespace.sh # namespace 隔离实验
│   ├── 02-cgroup-v2.sh # cgroup v2 资源限制实验
│   ├── 03-overlayfs.sh # OverlayFS 联合挂载实验
│   ├── 04-network.sh   # veth/bridge/NAT 网络实验
│   └── lib.sh          # 公共函数（cleanup / assert / log）
├── docs/
│   ├── troubleshooting.md  # 故障排查指南
│   ├── lab01-sequence.md   # namespace 时序图
│   ├── lab02-sequence.md   # cgroup 时序图
│   ├── lab03-sequence.md   # OverlayFS 时序图
│   └── lab04-sequence.md   # 网络时序图
└── rootfs/             # 可信 rootfs（alpine 解压，不入 git）
```

## 前置条件

- Linux 5.x+（cgroup v2 统一层级）
- root 权限
- 安装：`iproute2` / `iptables` / `iputils-ping` / `util-linux` / `python3`
- 内核模块：`overlay` / `br_netfilter`

当前不覆盖 `pivot_root`：仓库没有可信 rootfs 构建与校验流程，直接拼接一个 rootfs 会把下载供应链和清理风险混入 namespace 实验。需要该能力时应单独增加带校验和的 rootfs 准备步骤。
