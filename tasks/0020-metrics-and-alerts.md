# 任务 0020：指标、告警与日志集中收集

状态：已完成。开发说明见 [`docs/development/metrics.md`](../docs/development/metrics.md)。按[详细设计](../docs/GoFrame微服务商城详细设计.md) §9.2、§9.3 实现。可观测性的第三部分：做完以后，日志诊断 Agent 需要的三样信号都能集中查询——链路在 Jaeger（0014），日志在 Loki，指标和告警在 Prometheus。

## 新增了什么

```text
                    OTLP（:4317）                      抓取（每 5 秒）
各服务 ──链路、指标、日志──▶ OTel Collector ──链路──▶ Jaeger（:16686）
                                       ├──指标──▶ :8889 ◀── Prometheus（:9090）── 告警规则
                                       └──日志──▶ Loki（:3100）
```

| 类别 | 内容 |
| --- | --- |
| 指标 | 设计 §9.2 的指标，用 OpenTelemetry 指标 SDK 记录，每 5 秒经 OTLP 推给 Collector；Collector 以 Prometheus 格式暴露在 `:8889` |
| 告警 | Prometheus（`:9090`）每 5 秒抓取，按设计 §9.3 的两条规则告警；告警可以在 Prometheus 的界面和 `/api/v1/alerts` 查到 |
| 日志 | 任务 0019 的 JSON 日志除了写到标准输出，也经 OTLP 发给 Collector，再存进 Loki，按字段查询 |
| 基础设施 | Compose 新增 Prometheus、Loki；Collector 新增 metrics、logs 两条流水线 |

服务不直接连 Prometheus 或 Loki：和链路一样只认 Collector，后端可以换。

## 指标（设计 §9.2）

每个数据点都带 `service`、`environment` 两个标签；其余标签只能是下表列出的、取值有限的那些。订单号、用户 ID、链路 ID、商品 ID **不能**做标签。

| 指标 | 类型 | 其他标签 | 谁记录、怎么算 |
| --- | --- | --- | --- |
| `trade_order_create_total` | counter | `outcome` | order 每次处理下单请求：`pending_payment`（已预留库存）、`reserving`（预留还没有结果）、`rejected`（库存不足、商品下架）、`duplicate`（同一个 `requestId` 重试）、`invalid`、`error` |
| `trade_order_pending_payment` | gauge | — | order：现在处于待支付的订单数 |
| `trade_workflow_backlog` | gauge | `step` | order、goods：等待后台处理的工作数量 |
| `trade_workflow_oldest_age_seconds` | gauge（秒） | `step` | 其中最老的一件等了多久；没有积压时为 0 |
| `trade_rpc_duration_seconds` | histogram（秒） | `peer`、`method`、`outcome` | 所有 gRPC 客户端调用（网关和服务之间），在共用的超时拦截器里记录 |
| `trade_payment_callback_delay_seconds` | histogram（秒） | — | order：从模拟器记录的付款时间到订单服务确认这笔付款 |
| `trade_db_pool_in_use` / `trade_db_pool_limit` | gauge（连接数） | `pool` | 有数据库的服务：连接池正在用的连接数和上限 |
| `trade_db_acquire_timeout_total` | counter | `pool` | 见下面的说明 |

`step` 的取值：`reserve`（预留中）、`commit`（付款确认中）、`close`（关闭中）、`tech_refund`（未完成的技术退款）、`after_sale_refund`（已同意未退款的售后）、`outbox`（未发出的事件，order 和 goods 各自统计）。

`outcome`（RPC）的取值：`ok`；`rejected`（业务拒绝：参数错误、找不到、状态不允许、未登录、无权限、已存在）；`error`（技术错误：不可用、超时、内部错误等）。§9.3 的"技术错误率"只看 `error`。

**`trade_db_acquire_timeout_total` 的说明**：Go 的 `database/sql` 连接池没有"获取连接超时"这个概念，拿不到连接的调用会一直等到它自己的 context 超时，然后报 `context deadline exceeded`，和查询本身超时分不开。所以这个计数器记的是**等过连接的次数**（`sql.DBStats.WaitCount` 的增量）：连接池不够用时它会增长，正常时是 0。另外加一个 `trade_db_acquire_wait_seconds_total`（`WaitDuration`）。

## 告警（设计 §9.3）

| 告警 | 条件 | 持续 |
| --- | --- | --- |
| `TradeRpcErrorRateHigh` | 某个 `service` 调某个 `peer`，最近 1 分钟技术错误率超过 5%，且请求数至少 20 | 30 秒 |
| `TradeWorkflowStuck` | 某个 `step` 最老的待处理工作超过 30 秒 | 15 秒 |

告警带服务、环境、开始时间和指标摘要（当前的错误率或等待时间），**不**带任何故障注入的名字（设计 §9.3）。阈值是演示用的初值，不是生产 SLO。

## 日志集中收集

- `telemetry.Start` 在装 JSON 日志处理器的同时，配置了 `otel.endpoint` 就把同一份日志也经 OTLP 发出去。
- 每条日志在 OTLP 里是一条 log record：正文是 `message`，契约字段（`event.name`、`trace_id`、`order_id` 等）和 `attributes` 都是属性，严重级别和时间都对应上。
- Loki 把 `service.name` 作为标签，其余字段作为可以过滤的结构化元数据。例如查一个订单：

```logql
{service_name="order-service"} | order_id="O2026…"
```

- 和标准输出一样经过脱敏，脱敏在进程里完成。

## 范围外

- Alertmanager（把告警发到邮件、IM）：Agent 直接查 Prometheus 的告警接口就够了。
- Grafana 看板。
- 服务端（被调用方）的 RPC 指标：调用方的指标已经覆盖所有经网关和服务之间的调用。
- 长期存储：Prometheus 和 Loki 的数据都在 Compose 卷里，只用于实验。

## 实现中发现并修正的问题

- **`peer` 标签一开始是空的**：grpcx 拨号的目标是 `service:///service/default/default/goods/latest/`，服务名在第四段，后面还有版本和斜杠。`PeerName` 按这个格式解析，测试里用的是真实格式。
- **多出来的标签**：Collector 的导出器会加 `otel_scope_name`，抓取会加 `job`、`instance`。Prometheus 在抓取时去掉它们（`metric_relabel_configs`），只留下契约里的标签。
- **指标读数据库会产生链路**：积压、待支付、连接池每 5 秒查一次数据库，默认会在 Jaeger 里每次生成一条链路。这些回调改为在 `telemetry.Quiet` 上下文里运行。

## 验收标准

- [x] `go test ./...`（44 个包，开启 `GOFRAMESHOP_IT=1`）、`go mod verify` 通过，`gofmt -l` 为空；GoFrame 仍为 v2.10.2，依赖只新增了 OpenTelemetry 的指标和日志模块。
- [x] 指标测试（OpenTelemetry 内存读取器）：每个指标的名字、类型、单位、桶边界，以及只有契约允许的标签；回调延迟为负时按 0 计；积压的最老等待时间；RPC `outcome` 的划分；`PeerName` 对真实目标格式的解析；下单 `outcome` 的 8 种情况。
- [x] 积压和待支付查询的 Compose 集成测试（真实 MySQL）。
- [x] `docker compose config` 通过；`promtool check config` 通过，2 条规则。
- [x] 端到端（本地启动 7 个服务、Prometheus、Loki）：
  - 10 个指标都能在 Prometheus 查到，来自预期的服务；新数据只带契约里的标签。
  - RPC 指标的 `peer` 正确：gateway-h5 → goods / user / order，order → goods / user / payment-simulator；查询不存在的商品记为 `rejected`，不算技术错误。
  - 下单按 `outcome` 计数（`pending_payment`、`duplicate`、`rejected`）；付款回调平均延迟约 0.65 秒；连接池指标来自 5 个有数据库的服务。
  - 设计 §9.2 的 `rpc_latency_p95` 模板在持续流量下返回 0.00475 秒。
  - `TradeWorkflowStuck`：停掉模拟器后取消一个订单，订单停在 CLOSING，46 秒时告警 firing（摘要"the oldest close work has waited 46.71s"）；模拟器恢复后解除。
  - `TradeRpcErrorRateHigh`：模拟器停掉期间，订单服务的恢复任务调用它 96% 失败，告警 firing；goods 停掉期间，网关调用 goods 100% 失败，告警 firing（摘要"gateway-h5 → goods-service: 100% of calls failed in the last minute"）；两次都在服务恢复后解除。goods 这次停机是测试脚本造成的意外（重启时旧进程还没释放端口，新进程启动失败），不是计划内的步骤，但告警的触发和解除都是对的。
  - 在 Loki 按 `order_id` 查一个订单：goods 和 order 两个服务共 5 条事件按顺序出现，两个 `trace_id` 都能在 Jaeger 打开（43、31 个 span）。
