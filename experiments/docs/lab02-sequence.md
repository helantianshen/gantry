# 实验 02：cgroup v2 资源限制

```text
创建 /sys/fs/cgroup/gantry-lab-cgroup
  ├─ memory.max = 16 MiB
  │    └─ Python 分配 64 MiB → memory.events 的 oom_kill 增加
  ├─ pids.max = 5
  │    └─ 同一 shell 并发启动 10 个 sleep → pids.events 的 max 增加
  └─ cpu.max = 50000 100000
       └─ 回读断言为 50% 单核配额配置
```

OOM 和 PID 限制都通过内核事件计数断言，不根据错误文本猜测。CPU 本实验只验证配额写入；没有把运行时间差包装成稳定性能基准。
