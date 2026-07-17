# LegalFlow Admin

LegalFlow 是法务案件与合规协同运营后台，覆盖案件队列、律师排班、当事人档案和合规任务。它只做案件运营，不接入真实当事人隐私。

## 案件流程

1. 前台创建案件（`待立案`）。
2. 当事人提交后进入 `已立案`，运营人员推进到 `待办理`、`办理中`。
3. 案件结案后进入 `已结案`；每次状态变化写入 `appointment_events`，可从详情页回放。
4. 运营人员创建合规任务，执行完成后由 `待完成` 变为 `已完成`。
5. 所有写请求要求 `Idempotency-Key`，Redis 负责幂等结果和并发锁，MySQL 8.4 负责持久化。

```bash
# 一键启动 API + MySQL 8.4 + Redis 8（会自动加载合成演示数据）
docker compose -f deploy/docker-compose.yml up --build

# 或仅使用无外部依赖的内存模式运行 API
go run ./server

# 管理后台
cd web && npm install && npm run dev
```

前端默认请求 `/api/v1`，Vite 开发服务器会把 `/api` 和 `/healthz` 代理到 `http://localhost:8080`。部署到独立域名时，可在构建时设置 `VITE_API_BASE_URL=https://api.example.com`；客户端会自动补齐 `/api/v1`，所有创建、签到、状态推进和合规完成请求都会自动生成 `Idempotency-Key`。

后台的“案件队列”和“合规任务”按钮会优先调用真实 API；API 暂不可用时保留内置演示数据并提示当前数据来源。侧栏“移动端体验”提供同一套接口的窄屏当事人视图，支持创建演示案件、立案、办理完成和合规完成，便于用手机浏览器联调。

## API 示例

```bash
# 创建案件（重复发送相同 Idempotency-Key 只会创建一次）
curl -X POST http://localhost:8080/api/v1/appointments \
  -H 'Content-Type: application/json' -H 'Idempotency-Key: demo-create-001' \
  -d '{"patient":"案卷 C001","department":"合同纠纷","doctor":"林律师","scheduledAt":"2026-07-16T09:00:00+08:00"}'

# 推进状态：待立案 -> 已立案 -> 待办理 -> 办理中 -> 已结案（将 LG-1001 替换为上一步返回的 id）
curl -X POST http://localhost:8080/api/v1/appointments/LG-1001/checkin -H 'Idempotency-Key: demo-checkin-001'
curl -X POST http://localhost:8080/api/v1/appointments/LG-1001/status \
  -H 'Content-Type: application/json' -H 'Idempotency-Key: demo-waiting-001' -d '{"status":"待办理"}'

# 查看审计事件
curl http://localhost:8080/api/v1/appointments/LG-1001/events

# 完成合规
curl -X POST http://localhost:8080/api/v1/followups/LG-0716-001/complete -H 'Idempotency-Key: demo-followup-001'
```

演示数据均为虚构数据；项目不得用于真实医疗诊断、处方、支付或当事人隐私存储。
