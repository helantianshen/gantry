#!/usr/bin/env bash
set -euo pipefail

export NO_PROXY="127.0.0.1,localhost${NO_PROXY:+,$NO_PROXY}"
export no_proxy="$NO_PROXY"

for command in docker curl go; do
  command -v "$command" >/dev/null || { echo "缺少命令: $command" >&2; exit 1; }
done

run_id="$(date +%s)-$$"
project="gantry-e2e-$run_id"
image="gantry-e2e-$run_id"
app_name="gantry-e2e-$run_id"
run_dir="$(mktemp -d)"
api_pid=""
worker_pid=""
worker2_pid=""
app_id=""
api="http://127.0.0.1:18080"

# E2E 只使用本次隔离环境的临时凭据，不读取开发者本机 .env
export GANTRY_MYSQL_ROOT_PASSWORD="e2e-root-$run_id"
export GANTRY_MYSQL_DATABASE="gantry"
export GANTRY_MYSQL_USER="gantry"
export GANTRY_MYSQL_PASSWORD="e2e-mysql-$run_id"
export GANTRY_RABBITMQ_USER="gantry"
export GANTRY_RABBITMQ_PASSWORD="e2e-rabbit-$run_id"
export MYSQL_DSN="$GANTRY_MYSQL_USER:$GANTRY_MYSQL_PASSWORD@tcp(127.0.0.1:13306)/$GANTRY_MYSQL_DATABASE?parseTime=true"
export RABBITMQ_URL="amqp://$GANTRY_RABBITMQ_USER:$GANTRY_RABBITMQ_PASSWORD@127.0.0.1:5673/"
export REDIS_URL="redis://127.0.0.1:16379/0"

cleanup() {
  status=$?
  set +e
  if (( status != 0 )); then
    [[ -f "$run_dir/api.log" ]] && { echo "=== API 日志 ===" >&2; tail -n 40 "$run_dir/api.log" >&2; }
    [[ -f "$run_dir/worker.log" ]] && { echo "=== Worker 日志 ===" >&2; tail -n 40 "$run_dir/worker.log" >&2; }
    [[ -f "$run_dir/worker2.log" ]] && { echo "=== Worker 2 日志 ===" >&2; tail -n 40 "$run_dir/worker2.log" >&2; }
  fi
  for pid in "$api_pid" "$worker_pid" "$worker2_pid"; do
    [[ "$pid" =~ ^[0-9]+$ ]] && kill "$pid" 2>/dev/null
  done
  for pid in "$api_pid" "$worker_pid" "$worker2_pid"; do
    [[ "$pid" =~ ^[0-9]+$ ]] && wait "$pid" 2>/dev/null
  done
  while read -r container_id; do
    [[ -n "$container_id" ]] && docker rm -f "$container_id" >/dev/null
  done < <(docker ps -aq --filter "label=gantry.instance=$project")
  docker image rm "$image:good" "$image:bad" "$image:slow" >/dev/null 2>&1
  COMPOSE_PROJECT_NAME="$project" docker compose down -v --remove-orphans >/dev/null 2>&1
  [[ -n "$run_dir" && -d "$run_dir" ]] && rm -rf "$run_dir"
}
trap cleanup EXIT INT TERM

number_field() {
  local json="$1" key="$2"
  printf '%s' "$json" | sed -n "s/.*\"$key\":\([0-9][0-9]*\).*/\1/p"
}

string_field() {
  local json="$1" key="$2"
  printf '%s' "$json" | sed -n "s/.*\"$key\":\"\([^\"]*\)\".*/\1/p"
}

wait_for_api() {
  local deadline=$((SECONDS + 60))
  until curl -fsS "$api/healthz" >/dev/null; do
    (( SECONDS >= deadline )) && { echo "API 未就绪" >&2; return 1; }
    sleep 1
  done
}

wait_for_status() {
  local deployment_id="$1" expected="$2" deadline=$((SECONDS + ${3:-90})) json current
  while (( SECONDS < deadline )); do
    json="$(curl -fsS "$api/api/v1/deployments/$deployment_id")"
    current="$(string_field "$json" status)"
    [[ "$current" == "$expected" ]] && return 0
    case "$current" in
      success|failed|rolled_back|failed_rollback)
        echo "任务 $deployment_id 终态为 $current，期望 $expected: $json" >&2
        return 1
        ;;
    esac
    sleep 1
  done
  echo "等待任务 $deployment_id 进入 $expected 超时" >&2
  return 1
}

start_api() {
  LISTEN_ADDR=:18080 "$run_dir/gantry-api" >"$run_dir/api.log" 2>&1 &
  api_pid=$!
}

start_worker() {
  local pid_var="$1" worker_id="$2" metrics_addr="$3" log_name="$4"
  GANTRY_INSTANCE="$project" WORKER_ID="$worker_id" METRICS_ADDR="$metrics_addr" \
    "$run_dir/gantry-worker" >"$run_dir/$log_name" 2>&1 &
  printf -v "$pid_var" '%s' "$!"
}

mysql_query() {
	COMPOSE_PROJECT_NAME="$project" docker compose exec -T mysql \
		mysql -N -B -u"$GANTRY_MYSQL_USER" -p"$GANTRY_MYSQL_PASSWORD" "$GANTRY_MYSQL_DATABASE" -e "$1" 2>/dev/null
}

wait_for_db_status() {
  local deployment_id="$1" expected="$2" deadline=$((SECONDS + ${3:-45})) current
  while (( SECONDS < deadline )); do
    current="$(mysql_query "SELECT status FROM deployments WHERE id=$deployment_id")"
    [[ "$current" == "$expected" ]] && return 0
    sleep 1
  done
  echo "等待数据库任务 $deployment_id 进入 $expected 超时，当前为 $current" >&2
  return 1
}

echo "[1/5] 启动隔离的 MySQL、RabbitMQ、Redis"
COMPOSE_PROJECT_NAME="$project" docker compose up -d --wait

go build -o "$run_dir/gantry-api" ./cmd/api
go build -o "$run_dir/gantry-worker" ./cmd/worker
start_api
start_worker worker_pid fault-worker-1 :19090 worker.log
wait_for_api

echo "[2/5] 准备健康与不健康的本地镜像"
docker pull nginx:alpine >/dev/null
docker tag nginx:alpine "$image:good"
docker build -q -t "$image:bad" - >/dev/null <<'DOCKERFILE'
FROM nginx:alpine
RUN rm -f /usr/share/nginx/html/index.html
DOCKERFILE

app_json="$(curl -fsS -X POST "$api/api/v1/apps" -H 'Content-Type: application/json' \
  -d "{\"name\":\"$app_name\",\"image_name\":\"$image\",\"healthcheck_path\":\"/\",\"healthcheck_timeout_sec\":8}")"
app_id="$(number_field "$app_json" id)"
[[ "$app_id" =~ ^[0-9]+$ ]] || { echo "创建应用失败: $app_json" >&2; exit 1; }

good_version_json="$(curl -fsS -X POST "$api/api/v1/apps/$app_id/versions" -H 'Content-Type: application/json' -d '{"tag":"good"}')"
good_version_id="$(number_field "$good_version_json" id)"
good_deployment_json="$(curl -fsS -X POST "$api/api/v1/deployments" -H 'Content-Type: application/json' \
  -d "{\"app_id\":$app_id,\"version_id\":$good_version_id}")"
good_deployment_id="$(number_field "$good_deployment_json" id)"

echo "[3/5] 验证成功发布"
wait_for_status "$good_deployment_id" success
old_id="$(docker ps -q --filter "label=gantry.instance=$project" --filter "label=app=$app_id")"
[[ "$(printf '%s\n' "$old_id" | sed '/^$/d' | wc -l)" -eq 1 ]] || { echo "成功发布后容器数不为 1" >&2; exit 1; }

echo "[4/5] 验证重复消息被幂等吸收"
deployment_json="$(curl -fsS "$api/api/v1/deployments/$good_deployment_id")"
message_id="$(string_field "$deployment_json" message_id)"
publish_body="$(printf '{"properties":{"delivery_mode":2},"routing_key":"deploy.run","payload":"{\\"message_id\\":\\"%s\\",\\"deployment_id\\":%s,\\"app_id\\":%s,\\"version_id\\":%s,\\"attempt\\":0}","payload_encoding":"string"}' \
  "$message_id" "$good_deployment_id" "$app_id" "$good_version_id")"
publish_result="$(curl -fsS -u "$GANTRY_RABBITMQ_USER:$GANTRY_RABBITMQ_PASSWORD" -H 'Content-Type: application/json' -d "$publish_body" \
  http://127.0.0.1:15673/api/exchanges/%2F/deploy.exchange/publish)"
[[ "$publish_result" == *'"routed":true'* ]] || { echo "重复消息未路由: $publish_result" >&2; exit 1; }
sleep 2
[[ "$(string_field "$(curl -fsS "$api/api/v1/deployments/$good_deployment_id")" status)" == success ]] || exit 1
[[ "$(docker ps -q --filter "label=gantry.instance=$project" --filter "label=app=$app_id" | wc -l)" -eq 1 ]] || exit 1

echo "[5/5] 验证坏版本回滚且旧容器不中断"
bad_version_json="$(curl -fsS -X POST "$api/api/v1/apps/$app_id/versions" -H 'Content-Type: application/json' -d '{"tag":"bad"}')"
bad_version_id="$(number_field "$bad_version_json" id)"
bad_deployment_json="$(curl -fsS -X POST "$api/api/v1/deployments" -H 'Content-Type: application/json' \
  -d "{\"app_id\":$app_id,\"version_id\":$bad_version_id}")"
bad_deployment_id="$(number_field "$bad_deployment_json" id)"
wait_for_status "$bad_deployment_id" rolled_back
[[ "$(docker ps -q --filter "label=gantry.instance=$project" --filter "label=app=$app_id")" == "$old_id" ]] || { echo "回滚后旧容器未保留" >&2; exit 1; }
metrics_body="$(curl -fsS http://127.0.0.1:19090/metrics)"
grep -q '^deploy_platform_rollback_total{result="success"} 1' <<<"$metrics_body"
grep -q '^deploy_platform_queue_depth ' <<<"$metrics_body"
grep -q '^deploy_platform_queue_consumers ' <<<"$metrics_body"

if [[ "${GANTRY_FAULT_E2E:-0}" != 1 ]]; then
  echo "PASS: 成功发布、重复投递、健康失败回滚均通过"
  exit 0
fi

echo "[6/10] 验证双 Worker 下 20 并发创建仅一条成功"
start_worker worker2_pid fault-worker-2 :19091 worker2.log
docker build -q -t "$image:slow" - >/dev/null <<'DOCKERFILE'
FROM nginx:alpine
CMD ["sh", "-c", "sleep 15; exec nginx -g 'daemon off;'"]
DOCKERFILE
concurrent_app_json="$(curl -fsS -X POST "$api/api/v1/apps" -H 'Content-Type: application/json' \
  -d "{\"name\":\"$app_name-concurrent\",\"image_name\":\"$image\",\"healthcheck_path\":\"/\",\"healthcheck_timeout_sec\":60}")"
concurrent_app_id="$(number_field "$concurrent_app_json" id)"
concurrent_version_json="$(curl -fsS -X POST "$api/api/v1/apps/$concurrent_app_id/versions" -H 'Content-Type: application/json' -d '{"tag":"slow"}')"
concurrent_version_id="$(number_field "$concurrent_version_json" id)"
submit_body="{\"app_id\":$concurrent_app_id,\"version_id\":$concurrent_version_id}"
submit_started=$SECONDS
submit_pids=()
for i in $(seq 1 20); do
  curl -sS -o "$run_dir/submit$i.json" -w '%{http_code}' -X POST "$api/api/v1/deployments" -H 'Content-Type: application/json' -d "$submit_body" >"$run_dir/code$i" &
  submit_pids+=("$!")
done
for pid in "${submit_pids[@]}"; do
  wait "$pid"
done
success_count="$(grep -h '^201$' "$run_dir"/code* | wc -l)"
conflict_count="$(grep -h '^409$' "$run_dir"/code* | wc -l)"
[[ "$success_count" -eq 1 && "$conflict_count" -eq 19 ]] || { echo "并发创建状态码错误: 201=$success_count 409=$conflict_count" >&2; exit 1; }
for i in $(seq 1 20); do
  if [[ "$(<"$run_dir/code$i")" == 201 ]]; then
    concurrent_deployment_json="$(<"$run_dir/submit$i.json")"
    break
  fi
done
echo "20 个并发请求在 $((SECONDS - submit_started)) 秒内完成：201=1，409=19"
concurrent_deployment_id="$(number_field "$concurrent_deployment_json" id)"
wait_for_status "$concurrent_deployment_id" success

echo "[7/10] 验证 Redis 锁竞争与旧 token 防误删"
go test ./internal/lock -run TestAppLockIntegration -count=1

echo "[8/10] 验证 Worker kill -9 后 lease 回收并完成"
slow_version_id="$concurrent_version_id"
slow_deployment_json="$(curl -fsS -X POST "$api/api/v1/deployments" -H 'Content-Type: application/json' \
  -d "{\"app_id\":$concurrent_app_id,\"version_id\":$slow_version_id}")"
slow_deployment_id="$(number_field "$slow_deployment_json" id)"
slow_message_id="$(string_field "$(curl -fsS "$api/api/v1/deployments/$slow_deployment_id")" message_id)"
slow_publish_body="$(printf '{"properties":{"delivery_mode":2},"routing_key":"deploy.run","payload":"{\\"message_id\\":\\"%s\\",\\"deployment_id\\":%s,\\"app_id\\":%s,\\"version_id\\":%s,\\"attempt\\":0}","payload_encoding":"string"}' \
  "$slow_message_id" "$slow_deployment_id" "$concurrent_app_id" "$slow_version_id")"
for _ in 1 2; do
	curl -fsS -u "$GANTRY_RABBITMQ_USER:$GANTRY_RABBITMQ_PASSWORD" -H 'Content-Type: application/json' -d "$slow_publish_body" \
    http://127.0.0.1:15673/api/exchanges/%2F/deploy.exchange/publish | grep -q '"routed":true'
done
wait_for_status "$slow_deployment_id" running
deadline=$((SECONDS + 20))
until docker ps -q --filter "label=gantry.instance=$project" --filter "label=deployment-id=$slow_deployment_id" | grep -q .; do
  (( SECONDS >= deadline )) && { echo "待杀 Worker 尚未启动容器" >&2; exit 1; }
  sleep 1
done
sleep 2
[[ "$(docker ps -q --filter "label=gantry.instance=$project" --filter "label=deployment-id=$slow_deployment_id" | wc -l)" -eq 1 ]] || { echo "重复消息触发了多个容器" >&2; exit 1; }
lease_owner="$(string_field "$(curl -fsS "$api/api/v1/deployments/$slow_deployment_id")" lease_owner)"
case "$lease_owner" in
  fault-worker-1)
    kill -9 "$worker_pid"
    wait "$worker_pid" 2>/dev/null || true
    worker_pid=""
    ;;
  fault-worker-2)
    kill -9 "$worker2_pid"
    wait "$worker2_pid" 2>/dev/null || true
    worker2_pid=""
    ;;
  *)
    echo "未知 lease owner: $lease_owner" >&2
    exit 1
    ;;
esac
wait_for_status "$slow_deployment_id" success 160
slow_result="$(curl -fsS "$api/api/v1/deployments/$slow_deployment_id")"
[[ "$(number_field "$slow_result" attempt)" == 1 ]] || { echo "lease 重投 attempt 错误: $slow_result" >&2; exit 1; }
curl -fsS "$api/api/v1/deployments/$slow_deployment_id/events" | grep -q 'lease_reclaimed'

echo "[9/10] 验证 API 崩溃窗口遗留 pending 被补偿"
pending_app_json="$(curl -fsS -X POST "$api/api/v1/apps" -H 'Content-Type: application/json' \
  -d "{\"name\":\"$app_name-pending\",\"image_name\":\"$image\"}")"
pending_app_id="$(number_field "$pending_app_json" id)"
pending_version_json="$(curl -fsS -X POST "$api/api/v1/apps/$pending_app_id/versions" -H 'Content-Type: application/json' -d '{"tag":"good"}')"
pending_version_id="$(number_field "$pending_version_json" id)"
kill -9 "$api_pid"
wait "$api_pid" 2>/dev/null || true
api_pid=""
pending_deployment_id="$(mysql_query "INSERT INTO deployments (app_id,version_id,status,message_id,attempt,created_at,updated_at) VALUES ($pending_app_id,$pending_version_id,'pending','pending-$run_id',0,NOW()-INTERVAL 31 SECOND,NOW()); SELECT LAST_INSERT_ID();")"
wait_for_db_status "$pending_deployment_id" failed
pending_reason="$(mysql_query "SELECT fail_reason FROM deployments WHERE id=$pending_deployment_id")"
[[ "$pending_reason" == publish_timeout ]] || { echo "pending 补偿原因错误: $pending_reason" >&2; exit 1; }
start_api
wait_for_api

echo "[10/10] 验证 RabbitMQ 宕机时任务进入 publish_failed"
mq_app_json="$(curl -fsS -X POST "$api/api/v1/apps" -H 'Content-Type: application/json' \
  -d "{\"name\":\"$app_name-mq\",\"image_name\":\"$image\"}")"
mq_app_id="$(number_field "$mq_app_json" id)"
mq_version_json="$(curl -fsS -X POST "$api/api/v1/apps/$mq_app_id/versions" -H 'Content-Type: application/json' -d '{"tag":"good"}')"
mq_version_id="$(number_field "$mq_version_json" id)"
COMPOSE_PROJECT_NAME="$project" docker compose stop rabbitmq >/dev/null
mq_code="$(curl -sS -o "$run_dir/mq-failure.json" -w '%{http_code}' -X POST "$api/api/v1/deployments" -H 'Content-Type: application/json' \
  -d "{\"app_id\":$mq_app_id,\"version_id\":$mq_version_id}")"
[[ "$mq_code" == 500 ]] || { echo "MQ 宕机响应码错误: $mq_code $(<"$run_dir/mq-failure.json")" >&2; exit 1; }
mq_result="$(mysql_query "SELECT CONCAT(status,'|',fail_reason) FROM deployments WHERE app_id=$mq_app_id ORDER BY id DESC LIMIT 1")"
[[ "$mq_result" == failed\|publish_failed:* ]] || { echo "MQ 宕机任务状态错误: $mq_result" >&2; exit 1; }

echo "PASS: 基础 E2E、20 并发创建、Redis 锁、双消费者幂等、lease 恢复、pending 补偿和 MQ 宕机均通过"
