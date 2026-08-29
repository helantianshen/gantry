# 实验 01：namespace 隔离

```text
宿主 bash
  └─ unshare --uts --pid --mount --net --fork --mount-proc
       └─ namespace 内 bash（PID 1）
            ├─ 设置 hostname=lab-container
            ├─ 启用独立网络命名空间中的 lo
            └─ 从新挂载的 /proc 观察 PID 视图
```

脚本断言 namespace 内 hostname、PID 和 loopback 存在，并在退出后确认宿主 hostname 未变化。

当前不做 `pivot_root`：仓库没有可信 rootfs 的准备与校验步骤。该实验只证明 namespace 可见性隔离，不是安全沙箱。
