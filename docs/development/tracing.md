# 链路追踪：OpenTelemetry + Jaeger

一次下单会经过网关、订单、商品、用户四个服务，十几条 SQL，一次 Redis，还可能在 30 秒后由 RabbitMQ 触发关单。出了问题时，日志分散在四个进程里。**链路追踪**把一次请求经过的每一步串成一棵树，按时间画出来。

任务、决定和验收见 [`tasks/0014-tracing.md`](../../tasks/0014-tracing.md)。

## 1. 先看一眼

```bash
make infra-up    # 现在包含 otel-collector 和 jaeger
```

各服务启动后随便下一单，打开 Jaeger：<http://127.0.0.1:16686>，Service 选 `gateway-h5`，点 Find Traces。下单那一条展开是这样的（去掉了 SQL）：

```text
/frontend/order                                   gateway-h5       70ms
  order.v1.OrderService/CreateOrder               gateway-h5（客户端）
    order.v1.OrderService/CreateOrder             order-service（服务端）
      goods.v1.GoodsService/GetGoods              order-service → goods-service
        Redis.GET                                 goods-service
      consignee.v1.ConsigneeService/GetConsignee  order-service → user-service
      inventory.v1.InventoryService/ReserveInventory  order-service → goods-service
      DB.Begin（写 PENDING_PAYMENT 和事件的事务）
        publish order.payment_pending             order-service（约 200ms 后，由投递器发出）
```

## 2. 几个词

| 词 | 意思 |
| --- | --- |
| trace（链路） | 一次请求引起的全部工作，有一个 32 位十六进制的 trace id |
| span | 其中一步：一次 HTTP 请求、一次 gRPC 调用、一条 SQL。有开始时间、耗时、父 span |
| 上下文传播 | 调用别的服务时，把 trace id 和当前 span id 放进请求头（W3C `traceparent`），对方接着记 |
| Span Link | 两条不同链路之间的"相关"关系，用于很久以后才发生的异步工作 |
| 采样 | 记不记录这条链路。实验环境全记（`sampleRatio: 1`） |

## 3. 数据怎么走

```text
11 个进程 ──OTLP gRPC :4317──▶ OTel Collector ──▶ Jaeger（内存，界面 :16686）
                                    │
                                    └ transform/scrub：第二道脱敏
```

服务不直接连 Jaeger：换后端、以后加日志和指标，都只改 Collector 的配置。

每个进程的 `main` 第一行：

```go
ctx, stopTracing := telemetry.Start(gctx.GetInitCtx(), "order-service")
defer stopTracing()   // 退出时把还没发出的 span 发完
```

配置（每个服务的 `config.yaml.example` 都有）：

```yaml
otel:
  endpoint: "127.0.0.1:4317"   # 留空就关闭追踪，也可以用环境变量 OTEL_ENDPOINT
  sampleRatio: 1.0
  environment: "local"
```

## 4. 谁在记录什么

| 来源 | 记录方式 |
| --- | --- |
| HTTP 请求（两个网关、模拟器收银台） | GoFrame 自动；从请求头接上上游的链路 |
| gRPC 调用 | grpcx 自动，客户端和服务端各一个 span，通过 metadata 传播 |
| SQL、事务 | GoFrame 自动，`DB.Begin`、`DB.QueryContext`、`TX.Commit` |
| Redis | GoFrame 自动 |
| 事件发布、消费 | 本仓库（`utility/eventbus`、`utility/rabbitmq`），见第 5 节 |
| 后台任务 | 本仓库（`telemetry.Root`），见第 6 节 |
| 支付回调 | 模拟器发回调时带 `traceparent`，网关接着记 |

## 5. 事件也能追：链路穿过 RabbitMQ

事件是异步的：订单在事务里写下事件，几百毫秒后投递器才发出去，消费者又在另一个进程里处理。链路靠这几步接起来：

1. `eventbus.Append` 把当前链路的 `traceparent` 存进 `outbox_event.traceparent`。
2. 投递器发布时，在**这条链路**里开一个 `publish <事件类型>` span（Producer），即使是几秒后、或者 RabbitMQ 恢复以后才发。
3. 发布者把 `traceparent` 写进消息头（也写进信封）。
4. 消费者从消息头取出来，开一个 `consume <队列>` span（Consumer）：
   - **普通事件**：作为发布 span 的子 span，同一条链路。后台改商品名的链路里，能一直看到 banner 更新手工位。
   - **延迟事件**（订单到期，15 分钟后）：**新开一条链路**，用 Span Link 指回发布 span。否则下单那条链路会被拉长到 15 分钟，没法看。在 Jaeger 里点 consume span 的 References 就能跳回下单。

发布失败的每一次尝试都是一个带错误原因的 span。RabbitMQ 停机演练后，下单链路里能看到：

```text
publish order.payment_pending  attempt 1  error  dial tcp 127.0.0.1:5672: connect: connection refused
publish order.payment_pending  attempt 2  error  Exception (501) ...
publish order.payment_pending  attempt 3  error  Exception (501) ...
publish order.payment_pending  attempt 4  ok
```

## 6. 后台任务：一件事一条链路

进程启动时 GoFrame 给初始上下文分配了一个链路 ID。如果后台循环直接用它，投递器每 500ms 一次的轮询、恢复任务每 2 秒一次的扫描，全都会堆进**同一条永远不结束的链路**。

所以：

- `telemetry.Start` 返回的是 **Quiet** 上下文：链路 ID 有效（日志里照样有），但标记为"不采样"。在它下面发生的 SQL 全部不记录，空转的轮询不产生任何 span。
- 真正干活时用 `telemetry.Root` 开**新链路**：

| 后台工作 | span 名 | 属性 |
| --- | --- | --- |
| 恢复任务推进一个订单 | `order.recover` | `order.no`、`order.status` |
| 一笔技术退款 | `order.tech_refund` | `refund.id`、`refund.reason`、`refund.attempt` |
| 模拟器发一次回调 | `paysim.notify` | `payment.id`、`notify.attempt` |
| 投递器发一条事件 | `publish <类型>` | 接在写事件的链路里（没有就新开） |
| 消费一条事件 | `consume <队列>` | 见第 5 节 |

在 Jaeger 里按标签查：Service `order-service`，Operation `order.recover`，Tags `order.no=O2026...`。

## 7. 日志和链路怎么对上

GoFrame 的每一行日志都带着当前的链路 ID：

```text
2026-10-21T19:54:36.879+08:00 [INFO] {5c087640125dd9a983f0f59709cf974a} order O...: payment time is up; closing (expiry message)
```

`{...}` 里就是 trace id。打开 `http://127.0.0.1:16686/trace/5c087640125dd9a983f0f59709cf974a` 就是同一件事的完整链路；反过来，在所有服务的日志里 grep 这个 ID，能找到这条链路上每个进程说过的话。

## 8. 脱敏

GoFrame 默认会把请求头（含令牌）、gRPC 元数据（含转发的令牌）、**带值的 SQL**（姓名、手机号、地址）、Redis 的缓存内容写进 span。这些都不能进 Jaeger，以后更不能进日志诊断 Agent 的输入。

| 内容 | 处理后 |
| --- | --- |
| `Authorization`、`Cookie`、`Set-Cookie`、`X-Paysim-Signature` 请求头 | `[REDACTED]`，其他头保留 |
| gRPC `authorization` 元数据 | `[REDACTED]` |
| `db.statement` | `INSERT INTO order_info(...) VALUES('?',26,'?','?',...)`：字符串全部换成 `'?'`，数字保留 |
| Redis 参数 | `[REDACTED]`（命令名保留） |
| SQL 错误状态 | `database error: Error 1062 (23000)` |
| 其他状态描述 | 只留第一行，最多 300 字 |
| 任何地方漏网的 `Bearer xxx`、JWT | 正则替换 |

两道防线：进程内的导出器（`utility/telemetry/redact.go`），以及 Collector 的 `transform/scrub`。两者都有测试：单元测试覆盖每一条规则；集成测试绕过第一道，直接往 Collector 发一个带密钥的 span，确认 Jaeger 里看不到。

**以后新加 span 属性时**：不要放令牌、密码、手机号、地址、支付凭证；如果 GoFrame 或新库会自动记录新的敏感字段，先在 `redact.go` 加规则和测试。

## 9. 故障演练

| 想看什么 | 怎么做 | 在 Jaeger 里应该看到 |
| --- | --- | --- |
| 一次下单经过了哪些服务 | 下单 | Service `gateway-h5`，Operation `/frontend/order` |
| 慢在哪里 | 看最长的那一段 | 时间轴上最宽的 span；SQL 也是单独的 span |
| 回调走了哪条路 | 付款 | Service `payment-simulator`，Operation `paysim.notify` |
| 事件的去向 | 后台改商品名 | `/backend/goods` 下面有 `publish goods.changed` → `consume banner.goods-changed` |
| 到期关单的来龙去脉 | 把 `recoveryInterval` 调到 `60s`，下单不付 | `consume order.expiry`，References 指回下单链路 |
| 消息系统故障 | `docker compose stop rabbitmq` 后下单，再启动 | 下单链路里多次带错误的 `publish`，最后一次成功 |
| 下游服务挂了 | 停掉 goods 再下单 | order-service 调用 goods 的客户端 span 标为错误，下单链路在那里中断 |

## 10. 代码在哪

```text
utility/telemetry/telemetry.go     Init、Start、Quiet、Root
utility/telemetry/redact.go        脱敏规则（改之前先读这个）
utility/rabbitmq/trace.go          消息头传播、消费 span、延迟消息的 Link
utility/eventbus/outbox.go         Append 存 traceparent；投递器的 publish span
app/*/main.go                      telemetry.Start
app/order/internal/logic/order/    order.recover、order.tech_refund
app/paysim/internal/notify/        paysim.notify，回调请求带 traceparent
deploy/otel/collector.yaml         Collector 管道和第二道脱敏
deploy/mysql/init/013_event_trace_schema.sql
```

## 11. 测试

```bash
go test ./utility/telemetry/ ./utility/rabbitmq/
GOFRAMESHOP_IT=1 go test ./utility/telemetry/ ./utility/eventbus/ -v
```

## 12. 已知局限

- Jaeger 存在内存里，重启就清空。
- 日志还是 GoFrame 的文本格式；JSON 结构化日志（`trace_id`、`span_id` 单独成字段）是下一轮。
- search 的 binlog 同步、goods 的缓存删除重试没有单独的链路（轮询被静音了，工作本身也没有 Root）。
- 以 `sampleRatio: 1` 记录全部链路，只适合实验环境。
