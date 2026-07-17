# SupplyFlow Admin

SupplyFlow 是采购供应链运营后台，覆盖采购申请、供应商询价、比价审批、订单下发、到货验收、质检入库和结算对账。采购、仓库和财务按角色处理各自待办。

## 采购流程

1. 业务人员提交物料、数量和交期，生成 `待审批` 采购申请。
2. 采购员发起供应商询价并维护报价，负责人完成比价审批后下发采购订单。
3. 仓库登记到货、质检结果和入库数量，短收或异常会保留差异记录。
4. 财务根据验收结果处理结算，订单、库存和供应商事件可按时间线回放。
5. 所有写请求要求 `Idempotency-Key`，Redis 负责幂等结果和并发锁，MySQL 8.4 负责持久化。

```bash
# 一键启动 API + MySQL 8.4 + Redis 8（会自动加载合成演示数据）
docker compose -f deploy/docker-compose.yml up --build

# 或仅使用无外部依赖的内存模式运行 API
go run ./server

# 管理后台
cd web && npm install && npm run dev
```

前端默认请求 `/api/v1`，Vite 开发服务器会把 `/api` 和 `/healthz` 代理到 `http://localhost:8080`。部署到独立域名时，可在构建时设置 `VITE_API_BASE_URL=https://api.example.com`；客户端会自动补齐 `/api/v1`，所有创建、确认、状态推进和回访完成请求都会自动生成 `Idempotency-Key`。

后台的“采购单队列”“询价比价”“到货质检”“入库台账”按钮会优先调用真实 API；API 暂不可用时保留内置演示数据并提示当前数据来源。侧栏“移动端体验”提供采购员与仓库人员的窄屏视图，支持提交申请、查看报价、拍照验收和上报差异。

## API 示例

```bash
# 创建采购单（重复发送相同 Idempotency-Key 只会创建一次）
curl -X POST http://localhost:8080/api/v1/appointments \
  -H 'Content-Type: application/json' -H 'Idempotency-Key: demo-create-001' \
  -d '{"patient":"演示客户","department":"全科门诊","doctor":"林负责人","scheduledAt":"2026-07-16T09:00:00+08:00"}'

# 推进状态：待确认 -> 已确认 -> 候诊中 -> 处理中 -> 已完成（将 AP-1001 替换为上一步返回的 id）
curl -X POST http://localhost:8080/api/v1/appointments/AP-1001/checkin -H 'Idempotency-Key: demo-checkin-001'
curl -X POST http://localhost:8080/api/v1/appointments/AP-1001/status \
  -H 'Content-Type: application/json' -H 'Idempotency-Key: demo-waiting-001' -d '{"status":"候诊中"}'

# 查看审计事件
curl http://localhost:8080/api/v1/appointments/AP-1001/events

# 完成回访
curl -X POST http://localhost:8080/api/v1/followups/FW-0716-001/complete -H 'Idempotency-Key: demo-followup-001'
```

演示数据均为虚构数据；项目不得用于真实医疗诊断、处方、支付或客户隐私存储。

## 运行范围

SupplyFlow 覆盖申请、询价、审批、下单、到货、质检、入库和结算的采购操作；所有演示数据均为虚构，不接入真实财务、人事或客户隐私。

