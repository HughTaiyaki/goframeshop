# 任务 0019：结构化日志

状态：已完成。开发说明见 [`docs/development/logging.md`](../docs/development/logging.md)。按[详细设计](../docs/GoFrame微服务商城详细设计.md) §9.1 实现。可观测性的第二部分（第一部分是任务 0014 的链路追踪），后面是指标与告警（0020）、故障实验（0021）。目标是让日志诊断 Agent 能按字段查询，而不是在文本里猜。

## 新增了什么

| 类别 | 内容 |
| --- | --- |
| 格式 | 每行一个 JSON 对象，字段固定（见下）。所有进程的全部日志都是这个格式，包括 GoFrame 自己打的日志：不改原来的 87 处 `g.Log()` 调用，而是在 `telemetry.Start` 里装一个全局日志处理器 |
| 链路关联 | `trace_id`、`span_id` 单独成字段，和 Jaeger 里的一致 |
| 业务事件 | 设计 §9.1 列出的 7 个事件，每个带 `event.name` 和固定的附加字段 |
| 脱敏 | 每条消息在写出前去掉令牌、带账号密码的 URL / DSN、手机号 |
| 配置 | `logger.format`：默认 `json`；本地想看文本可以设成 `text` |

## 字段（设计 §9.1）

```json
{"timestamp":"2026-10-24T18:00:00.123+08:00","severity":"INFO",
 "service.name":"order-service","service.version":"1b54296","deployment.environment.name":"local",
 "event.name":"order.transition","trace_id":"…","span_id":"…",
 "operation_id":"","order_id":"O2026…","error.type":"",
 "message":"order O2026… PENDING_PAYMENT -> PAYMENT_CONFIRMING",
 "attributes":{"from":"PENDING_PAYMENT","to":"PAYMENT_CONFIRMING"}}
```

- 契约里的字段**总是出现**，没有值时是空字符串（"缺失关联字段必须显式为空"）。
- `trace_id`、`span_id` 只在这条链路**真的被记录**（采样）时填写。后台轮询运行在不采样的上下文里，它的链路 ID 在 Jaeger 里查不到，写进日志就等于伪造了一条链路，所以留空。
- `severity`：`DEBUG`、`INFO`、`WARN`、`ERROR`、`FATAL`。
- `attributes`：事件特有的字段，只放 ID、状态、错误码、次数这类东西。
- 错误日志的调用栈放在 `error.stack`。

## 业务事件

| `event.name` | 在哪里记录 | 关键字段 |
| --- | --- | --- |
| `order.transition` | 订单状态每次真正改变（条件 UPDATE 改到了行） | `order_id`；`from`、`to`、`reason` |
| `inventory.reserved` | goods 库存账本处理完预留 | `operation_id`、`order_id`；`outcome`、`replayed`（是否是重复调用） |
| `inventory.released` | goods 库存账本处理完释放 | 同上 |
| `payment.observed` | 订单服务确认了一笔付款（回调、主动查询、关单时发现已付） | `order_id`；`payment_id`、`outcome`（`CONFIRMING`、`LATE`、`DUPLICATE`） |
| `payment.refund.requested` | 向模拟器发起退款（技术退款和售后退款） | `order_id`；`refund_id`、`payment_id`、`kind`、`attempt`、`outcome`；失败时 `error.type` |
| `message.redelivered` | Inbox 发现同一个事件第二次到达 | `consumer`、`event_id`、`event_type` |
| `workflow.retry` | 恢复任务重试一个没有结果的步骤 | `order_id`；`step`（`reserve`、`commit`、`close`、`release`、`after_sale_refund`）、`attempt`（有时） |

`error.type` 用有限的取值：gRPC 状态码名（`Unavailable`、`DeadlineExceeded` 等），或者业务结果（`INSUFFICIENT_STOCK` 等）。

## 不能进日志的东西

DSN、带账号密码的连接 URL、支付凭证、JWT、手机号、地址（设计 §9.1）。代码本来就不该记录它们（`AGENTS.md`），日志处理器再兜一层底：

| 内容 | 处理 |
| --- | --- |
| `Bearer …`、JWT | `[REDACTED]`（和 span 脱敏用同一套规则） |
| `scheme://user:pass@host`、`mysql:user:pass@tcp(…)` | 账号密码换成 `[REDACTED]` |
| 11 位手机号 | `[REDACTED]` |

地址没法用规则识别，只能靠不记录。

## 范围外

- 日志收集（Loki、Elasticsearch 等）：这一轮只保证每个进程输出合格的 JSON，收集放到指标那一轮和 Collector 一起做。
- 日志采样、限流。

## 验收标准

- [x] `go test ./...`（44 个包，开启 `GOFRAMESHOP_IT=1`）、`go mod verify` 通过，`gofmt -l` 为空。
- [x] 日志处理器测试：契约字段齐全、没有值时为空字符串；采样的链路带 ID、`Quiet` 上下文的留空；级别映射（`Notice` 归入 `INFO`）；`Infof` 和多参数 `Info` 都能拼出消息；错误日志带 `error.stack`；令牌、JWT、URL 和 DSN 里的密码、手机号都被去掉，订单号和更长的数字串不受影响；事件字段不会串到下一行；不装处理器的日志器仍是文本。
- [x] 每个业务事件都有测试：订单从下单到付款的三次状态变化各记一次、重复回调不重复记；取消带原因；库存提交失败 → `EXCEPTION` → 技术退款 → `CLOSING`；恢复任务重试 `reserve`，到期关单不算重试；库存预留、重复预留（`replayed`）、库存不足、释放、操作号冲突；售后退款失败（带 `error.type`）后重试成功；Inbox 重复投递（真实 MySQL）。
- [x] 端到端（本地启动 7 个服务，走下单、付款、取消、售后（期间停掉模拟器）、发货、确认收货）：
  - 7 个进程共 149 行，全部是合法 JSON，契约字段一个不缺；60 行带链路 ID。
  - 出现了 6 个事件：`order.transition` 8 次、`inventory.reserved` 2 次、`inventory.released` 1 次、`payment.observed` 1 次、`payment.refund.requested` 2 次（一次 `DeadlineExceeded`）、`workflow.retry` 1 次。`message.redelivered` 只在 RabbitMQ 重复投递时出现，正常流程里没有，由集成测试覆盖。
  - `inventory.reserved` 那一行的 `trace_id` 在 Jaeger 里打开是一条 60 个 span 的链路，跨 gateway-h5、order、goods、user。
  - 日志里没有用户令牌、管理员令牌、用户和管理员的密码、`.env` 里的五个密码、收货电话和街道地址。
