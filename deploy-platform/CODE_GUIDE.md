# Gantry 代码掌握指南

目标不是记住每个函数，而是能够从入口独立推导状态变化、并发执行权、容器切换和故障恢复

## 1. 先建立职责边界

```text
cmd        组装依赖并管理进程生命周期
api        解析 HTTP，请求校验，映射响应码
worker     解析 RabbitMQ delivery，执行 Ack/Nack
service    编排发布、执行和恢复业务
repo       保存数据并实现事务、CAS 和租约条件
mq         RabbitMQ 连接、拓扑、发布和消费
executor   Docker 操作与健康检查
lock       Redis 应用锁
status     状态机合法边
model      数据结构
```

判断代码应该放哪一层时，只问一个问题：它是在处理协议、业务规则、数据持久化，还是外部系统

## 2. 按一条成功发布链路阅读

不要按目录逐个文件阅读，按下面顺序跟踪同一条 deployment：

1. `migrations/0001_init.sql`：先看五张表保存了什么事实
2. `internal/status/status.go`：记住 `pending → queued → running → success`
3. `cmd/api/main.go`：观察 Repo、Publisher、DeploymentService、Handler 如何组装
4. `internal/api/deployment.go:createDeployment`：HTTP 层只剩输入输出
5. `internal/service/deployment.go:CreateDeployment`：找到版本归属校验、回滚快照、建任务、发消息和状态推进
6. `internal/repo/deployment.go:CreateDeployment`：看应用行锁如何挡住同应用并发创建
7. `internal/mq/publisher.go:Publish`：区分 publisher confirm 与消费确认
8. `cmd/worker/main.go`：观察 Consumer、DeploymentRunner、Worker 如何组装
9. `internal/worker/worker.go:handleDelivery`：看 delivery 如何变成 Ack、Requeue 或 Reject
10. `internal/service/runner.go:Handle`：找到真正获得执行权的 `queued → running` CAS
11. `internal/service/rollout.go:rollout`：看新容器健康后才停止旧容器
12. `internal/repo/deployment.go:FinalizeDeployment`：看终态、当前版本和事件如何在一个事务内完成

读完后，自己画一遍下面这条链，不看文档也能补齐每个箭头上的状态：

```text
HTTP → MySQL → RabbitMQ → Worker → Redis/MySQL 执行权 → Docker → MySQL 终态 → Ack
```

## 3. 用断点获得代码掌控感

启动依赖和两个进程：

```bash
docker compose up -d --wait
go run ./cmd/api
go run ./cmd/worker
```

在 VS Code 依次给这些位置打断点：

- `api.createDeployment`
- `service.DeploymentService.CreateDeployment`
- `mq.Publisher.Publish`
- `worker.Worker.handleDelivery`
- `service.DeploymentRunner.Handle`
- `service.DeploymentRunner.rollout`
- `repo.Repo.FinalizeDeployment`

发起一次发布，每停一次只记录四项：当前 deployment 状态、message_id、lease_owner、下一步外部副作用

## 4. 分五次做故障推演

每次先写出预期，再运行 `make demo-e2e` 或 `make fault-e2e` 验证

### 重复消息

回答：幂等键记录了什么，为什么真正的唯一执行权仍由 `queued → running` CAS 决定

### 消息提前到达

回答：publisher confirm 已返回但数据库仍是 pending 时，Worker 为什么 Requeue

### 新版本不健康

回答：旧容器为什么还在，新容器在哪一步删除，最终为什么是 rolled_back 而不是 failed

### Worker 崩溃

回答：Redis 锁、MySQL lease 和 Reaper 分别解决哪一部分，attempt 如何限制重投

### RabbitMQ 不可用

回答：数据库事务与 MQ 发布之间为什么仍有窗口，当前为什么选择 `publish_failed`，outbox 会改变什么

## 5. 掌握程度自测

不看代码回答以下问题，并给出负责该规则的层：

- 同一应用为什么只能创建一个活跃任务
- 重复消息在哪一层被记录，在哪一层被真正挡住
- Worker 在哪个 SQL 条件更新成功后获得执行权
- publisher confirm、consumer Ack、数据库终态三者的先后关系
- 新旧容器并存为什么不冲突端口
- lease 过期后旧 Worker 为什么不能覆盖新一轮数据库终态
- Reaper 为什么必须在更新瞬间再次检查 lease 过期条件
- 哪些失败会 Requeue，哪些失败会 Reject，哪些状态直接 Ack

能够独立讲清这些问题，并在故障脚本失败时定位到对应层，就已经从“看懂代码”进入“掌握设计”

## 6. 推荐的二次实现练习

不要立即加新功能，先做三个不改变行为的小练习：

1. 给 `CreateDeployment` 画时序图，并逐行标出每个错误对应的 HTTP 状态码
2. 给 `DeploymentRunner.Handle` 写一张状态与 ProcessingResult 对照表，再与测试核对
3. 手工构造一次健康检查失败，查询 deployments、events、idempotency_keys 三张表解释结果

完成后再尝试设计 outbox，但先只写方案：新增哪些表字段、谁投递、何时标记完成、如何处理重复，不急着实现
