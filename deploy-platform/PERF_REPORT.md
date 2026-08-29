# 最小并发验收

- 日期：2026-08-19
- 环境：WSL2，Docker Desktop Engine 29.6.2
- 命令：`make fault-e2e`
- 场景：同一应用、同一版本同时发起 20 个创建发布请求。
- 结果：1 个 `201`、19 个 `409`，请求在 1 秒内完成；唯一获准任务最终成功。
- 队列观测：Worker `/metrics` 已验证 `deploy_platform_queue_depth` 与 `deploy_platform_queue_consumers` 可抓取。

这是锁竞争正确性 smoke test，不是容量基准；没有据此声称 QPS、P95/P99 或生产容量。
