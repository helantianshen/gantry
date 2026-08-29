# Gantry

> 龙门架 / 发射塔架：让服务的发布过程可执行、可恢复、可观察

Gantry 是一个面向单机 Docker 的轻量发布平台。API 将任务持久化到 MySQL 并投递 RabbitMQ，Worker 通过 Redis 应用锁、数据库 lease 和 Docker SDK 串行执行容器切换，Vue 管理台用于管理应用、版本和发布记录。

## 项目结构

```text
gantry/
├── deploy-platform/  # Go 发布平台、Vue 管理台和端到端验收
└── experiments/      # namespace、cgroup、overlayfs 和网络实验
```

## 已实现能力

- 应用、版本、发布任务和审计事件管理
- RabbitMQ publisher confirm、持久化消息、手动确认与幂等消费
- Redis 应用锁、MySQL lease、CAS 执行权和超时任务恢复
- 新容器健康后再停止旧容器，失败时保留旧服务
- Prometheus 发布、回滚、队列和 Worker 指标
- Vue 3 + TypeScript 管理台及响应式发布状态轨道
- Docker 真实发布、重复投递和故障回滚 E2E

## 快速开始

完整启动、验收和管理台使用方法见 [deploy-platform/README.md](deploy-platform/README.md)。

代码阅读路径和故障推演见 [deploy-platform/CODE_GUIDE.md](deploy-platform/CODE_GUIDE.md)。

## 当前边界

当前版本定位为单机 Docker 发布平台 MVP，尚未提供稳定流量网关、身份认证、Outbox、多主机调度、金丝雀或审批流，不应直接暴露到公网。
