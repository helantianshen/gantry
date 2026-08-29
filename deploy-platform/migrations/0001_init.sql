-- 0001_init.sql — 5-table schema, verbatim from blueprint §2
-- (.omo/drafts/deploy-platform-design.md). MySQL 8 / InnoDB.

-- 应用注册
CREATE TABLE apps (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(64) NOT NULL UNIQUE,
  image_name VARCHAR(255) NOT NULL,           -- 如 nginx / registry/game-gateway
  healthcheck_path VARCHAR(255) NOT NULL DEFAULT '/healthz',
  healthcheck_timeout_sec INT NOT NULL DEFAULT 60,  -- 单次发布健康检查总预算
  current_version_id BIGINT NULL,             -- 当前运行版本（成功发布后更新）
  created_at DATETIME,
  updated_at DATETIME
);

-- 版本
CREATE TABLE versions (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  app_id BIGINT NOT NULL,
  tag VARCHAR(128) NOT NULL,                  -- v2.1.0
  description VARCHAR(255) DEFAULT '',
  created_at DATETIME,
  UNIQUE KEY uk_app_tag (app_id, tag)
);

-- 发布任务（核心状态机）
CREATE TABLE deployments (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  app_id BIGINT NOT NULL,
  version_id BIGINT NOT NULL,                 -- 目标版本
  from_version_id BIGINT NULL,                -- 回滚依据：发布开始时的当前版本
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  message_id VARCHAR(64) NOT NULL,            -- 与 MQ 消息关联
  lease_owner VARCHAR(64) NULL,               -- Worker 实例 ID
  lease_expires_at DATETIME NULL,
  attempt INT NOT NULL DEFAULT 0,             -- 重投次数
  fail_reason VARCHAR(512) DEFAULT '',
  created_at DATETIME, updated_at DATETIME,
  KEY idx_app_active (app_id, status),        -- 活跃任务检查
  KEY idx_lease (status, lease_expires_at)    -- reaper 扫描
);

-- 消费幂等挡板
CREATE TABLE idempotency_keys (
  message_id VARCHAR(64) PRIMARY KEY,
  deployment_id BIGINT NOT NULL,
  consumer VARCHAR(64) NOT NULL,
  consumed_at DATETIME
);

-- 审计事件溯源
CREATE TABLE events (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  deployment_id BIGINT NOT NULL,
  type VARCHAR(32) NOT NULL,                  -- state_changed / rollback_started / lease_reclaimed / ...
  from_status VARCHAR(16), to_status VARCHAR(16),
  actor VARCHAR(64) NOT NULL,                 -- api / worker-{id} / reaper
  detail JSON,
  created_at DATETIME,
  KEY idx_dep (deployment_id)
);
