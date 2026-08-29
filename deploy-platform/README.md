# Gantry 发布平台

单机 Docker 发布平台：API 将任务写入 MySQL 并投递 RabbitMQ，Worker 通过 Redis 应用锁串行执行容器切换。

## 自动验收

前置条件：Go、Docker Engine（含 Compose 插件）和 curl。

```bash
make demo-e2e
```

该命令会启动隔离的 MySQL、RabbitMQ、Redis，构建 API/Worker，并依次断言：

- 健康镜像发布成功；
- 相同 `message_id` 的重复消息不会重复执行；
- 新镜像健康检查失败时保留原容器并进入 `rolled_back`；
- Worker 的 Prometheus 回滚指标可抓取。

脚本退出时清理本次创建的容器、镜像、进程和 Compose 卷。

完整故障验收约需 2–3 分钟，并使用真实 90 秒 lease：

```bash
make fault-e2e
```

它在基础 E2E 之外验证双 Worker 并发与重复消息、Redis 锁 token、防 `kill -9` 后孤儿容器阻塞重投、API 崩溃窗口 pending 补偿，以及 RabbitMQ 宕机后的 `publish_failed`。

最小并发结果见 [PERF_REPORT.md](PERF_REPORT.md)。

## 手动启动

复制本地配置并替换其中的占位密码，`.env` 已被 Git 忽略：

```bash
cp .env.example .env
```

随后分别启动基础设施和两个 Go 进程：

```bash
make infra
make api
# 另开终端
make worker
```

Makefile 会把本地 `.env` 导出给 API 和 Worker。默认地址：API `:8080`，Worker metrics `:9090`，MySQL `:13306`，RabbitMQ `:5673` / 管理台 `:15673`，Redis `:16379`。基础设施端口只绑定到 `127.0.0.1`。

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:9090/metrics
```

## 核心语义

- 7 状态、8 条合法边；仓储层同时校验合法边并用 CAS 更新。
- RabbitMQ 使用 durable topology、持久化消息、publisher confirm、manual ack 和 `prefetch=1`。
- `message_id` 数据库唯一键记录重复投递，`queued → running` CAS 决定唯一执行者；提前到达的 pending 消息短暂重排。
- Worker 同时续 Redis 锁和 MySQL lease；Reaper 回收超时任务并限制重投次数。
- Worker 指标包含 RabbitMQ 队列深度和消费者数。
- 同一应用的任务创建通过 MySQL 行锁串行化；超过发布窗口的 pending 任务由 Reaper 标记失败。
- 新容器通过健康检查后才停止旧容器；失败时删除新容器，旧服务保持运行。

当前范围仅覆盖单机 Docker，不包含多集群、金丝雀、审批流或告警系统。

## 管理页面

管理台使用 Vue 3、TypeScript、Vite 和 Arco Design Vue。Vite 8 需要 Node.js 20.19+，本机通过 NVM 临时切换到 Node.js 26：

```bash
nvm use 26
cd web
npm install
npm run dev
```

浏览器访问 `http://127.0.0.1:5173`。开发服务器会把 `/api` 和 `/healthz` 代理到 `http://127.0.0.1:8080`，因此需要同时运行 API。

生产构建使用：

```bash
nvm use 26
make web-build
```

当前管理台面向本机开发环境，API 尚未加入身份认证，不应直接暴露到公网。

## 代码导读

按真实发布链路、断点和故障推演深入理解项目，见 [CODE_GUIDE.md](CODE_GUIDE.md)。

## 开发检查

```bash
make test
make vet
make lint
```
