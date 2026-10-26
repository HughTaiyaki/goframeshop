# 结构化日志

所有进程的日志都是**每行一个 JSON 对象**，字段按[详细设计](../GoFrame微服务商城详细设计.md) §9.1 固定。任务、决定和验收见 [`tasks/0019-structured-logging.md`](../../tasks/0019-structured-logging.md)。链路追踪见 [`tracing.md`](tracing.md)。

## 1. 一行日志长什么样

```json
{"timestamp":"2026-10-24T17:58:23.412+08:00","severity":"INFO",
 "service.name":"goods-service","service.version":"1b542963530a","deployment.environment.name":"local",
 "event.name":"inventory.reserved","trace_id":"8a6ca1c31a7c92190228a1856ecef99f","span_id":"…",
 "operation_id":"reserve-O2026102409582318719721","order_id":"O2026102409582318719721","error.type":"",
 "message":"inventory.reserved reserve-O2026… for order O2026…: SUCCEEDED",
 "attributes":{"lines":2,"outcome":"SUCCEEDED","replayed":false}}
```

| 字段 | 说明 |
| --- | --- |
| `timestamp` | RFC 3339，毫秒，带时区 |
| `severity` | `DEBUG`、`INFO`、`WARN`、`ERROR`、`FATAL` |
| `service.name`、`service.version`、`deployment.environment.name` | 和 span 的资源属性相同：`telemetry.Start` 的服务名、构建时的 Git 版本、配置 `otel.environment` |
| `event.name` | 业务事件名，见第 3 节；普通日志为空 |
| `trace_id`、`span_id` | 当前链路，能直接在 Jaeger 打开。**只有被记录的链路才填**：后台轮询运行在不采样的上下文里，它的 ID 在 Jaeger 里查不到，所以留空 |
| `operation_id`、`order_id`、`error.type` | 业务事件的关联字段；没有值时是空字符串，不会缺字段 |
| `message` | 人看的文本，写出前已经脱敏（第 4 节） |
| `attributes` | 事件特有的字段，只放 ID、状态、错误码、次数 |
| `error.stack` | 错误日志的调用栈 |

## 2. 怎么接进来的

`telemetry.Start` 第一步调用 `InstallLogging`：给 GoFrame 的所有日志器装一个全局处理器（`glog.SetDefaultHandler`）。原有的 `g.Log().Infof(...)` 不用改，GoFrame 自己的日志也是同样的格式。

配置：`logger.format` 默认是 `json`；本地想看文本可以设成 `text`。

查日志用 `jq`，比如看某个订单经历了什么：

```bash
jq -c 'select(.order_id=="O2026102409582318719721") | [.timestamp, ."service.name", ."event.name", .message]' order.log goods.log
```

或者从一条链路出发：所有日志里 `trace_id` 相同的行，就是 Jaeger 里那棵树上每个进程说过的话。

## 3. 业务事件

用 `telemetry.Event(ctx, 名字, telemetry.Fields{...}, 消息格式, 参数...)` 记录。有 `ErrorType` 时是 `WARN`，否则是 `INFO`。名字定义在 `utility/telemetry/log.go`，**只能加，不能改**：日志诊断 Agent 按名字查询。

| `event.name` | 谁记录 | 什么时候 | 关键字段 |
| --- | --- | --- | --- |
| `order.transition` | order | 订单状态真正改变（条件 UPDATE 改到了行）；没改到行的一方不记 | `order_id`；`from`、`to`、`reason` |
| `inventory.reserved` | goods | 预留事务提交后；被拒绝的参数不记 | `operation_id`、`order_id`；`outcome`、`replayed`、`lines` |
| `inventory.released` | goods | 释放事务提交后 | 同上 |
| `payment.observed` | order | 确认一笔付款（回调、主动查询、关单时发现已付） | `order_id`；`payment_id`、`outcome`（`CONFIRMING`、`LATE`、`DUPLICATE`） |
| `payment.refund.requested` | order | 每次向模拟器发起退款 | `order_id`；`refund_id`、`payment_id`、`kind`（`technical`、`after_sale`）、`attempt`、`outcome`；失败时 `error.type` 是 gRPC 状态码名 |
| `message.redelivered` | 所有消费者（`eventbus.Handle`） | 同一个事件第二次到达，只确认不执行 | `consumer`、`event_id`、`event_type` |
| `workflow.retry` | order | 恢复任务重试没有结果的步骤 | `order_id`；`step`（`reserve`、`commit`、`close`、`tech_refund`、`after_sale_refund`）、`attempt` |

`order.transition` 覆盖了所有改订单状态的地方：`transition` 辅助函数、付款确认（`PENDING_PAYMENT → PAYMENT_CONFIRMING`）、库存提交失败（`→ EXCEPTION`）、技术退款完成（`EXCEPTION → CLOSING`）、到期消息关单、发货和确认收货。

## 4. 不能进日志的东西

令牌、密码、连接串里的账号密码、支付凭证、手机号、地址（设计 §9.1、`AGENTS.md`）。代码本来就不该记录它们，处理器再兜一层底：

| 内容 | 处理 |
| --- | --- |
| `Bearer …`、JWT | `[REDACTED]`（和 span 脱敏共用规则） |
| `amqp://user:pass@host` 这类 URL | `amqp://[REDACTED]@host` |
| `mysql:user:pass@tcp(…)` | `mysql:[REDACTED]@tcp(…)` |
| 11 位手机号（前后不是数字） | `[REDACTED]`；订单号和更长的数字串不受影响 |

地址没法用规则识别，只能靠不记录。`attributes` 里同样只能放 ID、状态、错误码和次数。

## 5. 代码在哪

```text
utility/telemetry/log.go          处理器、字段、事件名、Event、脱敏
utility/telemetry/logcapture/     测试用：把 g.Log() 的输出解码成行，按事件名筛选
app/order/internal/logic/order/   order.transition、payment.observed、技术退款、workflow.retry
app/order/internal/logic/refund/  售后退款的 payment.refund.requested、workflow.retry
app/order/internal/events/        到期消息关单的 order.transition
app/goods/internal/logic/inventory/  inventory.reserved / released
utility/eventbus/inbox.go         message.redelivered
```

## 6. 测试

```bash
go test ./utility/telemetry/ ./app/order/internal/logic/... ./app/goods/internal/logic/inventory/
GOFRAMESHOP_IT=1 go test ./utility/eventbus/ -run Inbox   # message.redelivered，需要 Compose
```

## 7. 已知局限

- ~~日志只写到每个进程的标准输出~~：任务 0020 起同一份日志也经 OTLP 进 Loki，查询方法见 [`metrics.md`](metrics.md#4-查日志loki)。
- `workflow.retry` 的 `attempt` 只是参考：管理员同意售后和恢复任务可能同时去退同一笔款（退款单号固定，模拟器只退一次），这时两边读到的次数可能相同。
