# 任务 0014：链路追踪（OpenTelemetry + Jaeger）

状态：已完成。按[详细设计](../docs/GoFrame微服务商城详细设计.md) §9.1 实现，开发说明见 [`docs/development/tracing.md`](../docs/development/tracing.md)。这是可观测性的第一部分；结构化日志和指标在后面两轮。

## 新增了什么

| 类别 | 内容 |
| --- | --- |
| 基础设施 | OpenTelemetry Collector 0.130.0（OTLP `:4317` / `:4318`）、Jaeger 2.11.0（界面 `:16686`，数据只在内存里），`make infra-up` 一起启动 |
| 共享代码 | `utility/telemetry`：`Start`（每个进程的 main 第一行调用）、`Quiet`（后台轮询不产生链路）、`Root`（每一件后台工作一条新链路）、脱敏导出器 |
| 覆盖范围 | 11 个进程。HTTP 请求、gRPC 调用（两端）、SQL、Redis 由 GoFrame 自动记录；本仓库补上了 RabbitMQ 事件、后台任务和支付回调 |
| 事件 | `outbox_event` 新增 `traceparent` 列（迁移 013）。投递器在写入事件的那条链路里发布，消费者接着这条链路处理；延迟事件（订单到期）另起一条链路并用 Span Link 连回下单 |
| 后台任务 | 订单恢复任务每处理一个订单、每一笔技术退款、模拟器每发一次回调，各是一条独立链路；空转的轮询不产生任何链路 |
| 回调 | 模拟器发回调时带上 `traceparent`：一条链路里能看到 模拟器 → gateway-h5 → 订单服务 → 回头查询模拟器 → 提交库存 |
| 日志关联 | GoFrame 日志行里 `{...}` 那一段就是链路 ID，拿去 Jaeger 能直接查到同一条链路 |
| 依赖 | 新增 `go.opentelemetry.io/otel/sdk`、`otlptracegrpc` v1.38.0（与 GoFrame 使用的版本一致）。`google.golang.org/grpc` 随之从 v1.64.1 升到 v1.75.0，`protobuf` 升到 v1.36.8；GoFrame 仍为 v2.10.2 |

## 脱敏：这一轮最重要的部分

GoFrame 一旦接上真正的追踪后端，会把这些东西写进 span：

| 位置 | 内容 | 处理 |
| --- | --- | --- |
| HTTP 请求事件 `http.request.headers` | 全部请求头，包括 `Authorization`、`Cookie`、`X-Paysim-Signature` | 这几个头的值改成 `[REDACTED]`，其余保留 |
| gRPC 事件 `grpc.metadata.incoming/outgoing` | 全部元数据，包括转发的令牌 | `authorization` 改成 `[REDACTED]` |
| SQL span 属性 `db.statement` | **带值的完整 SQL**：姓名、手机号、地址、密码哈希、事件内容 | 所有字符串字面量换成 `'?'`，保留语句结构 |
| Redis 事件 `redis.execution.arguments` | 缓存的键和值 | 整个替换为 `[REDACTED]` |
| SQL 出错时的 span 状态 | `%+v` 的错误，可能带 SQL 和值 | 只保留 MySQL 错误号，如 `database error: Error 1062 (23000)` |
| 任何字符串 | 漏网的 `Bearer ...`、JWT | 正则兜底替换 |

两道防线：

1. **进程内**：`utility/telemetry` 包住 OTLP 导出器，span 离开进程前就改写好。
2. **Collector**：`deploy/otel/collector.yaml` 的 `transform/scrub` 用同样的规则再过一遍。万一某个进程没有经过第一道（比如以后新加的语言或工具），第二道仍然有效。

端到端验证时，SQL 的问题是**靠扫描真实链路发现的**：第一次扫描在订单插入语句里找到了测试手机号。修复后重新扫描了 5060 个 span，令牌、密码、手机号、收货人姓名、签名全部干净。

## 与常见做法的差异

目前贴过来的教程章节里没有链路追踪。GoFrame 最常见的接法是 `contrib/trace/otlpgrpc` 一行初始化、直接发给后端。这里的不同：

| 常见接法 | 本仓库 | 原因 |
| --- | --- | --- |
| 服务直接发给 Jaeger | 先发给 OTel Collector，再转给 Jaeger | 后端可替换；日志和指标以后走同一条路（详细设计 §3.2） |
| 原样导出 | 两道脱敏 | GoFrame 默认把令牌、带值的 SQL、缓存内容写进 span |
| 只有框架自动记录的 HTTP、gRPC、SQL、Redis | 还覆盖 RabbitMQ 事件、后台任务、支付回调 | 异步的部分正是最难排查的部分 |
| 后台任务共用启动时的链路 | 轮询不记录，每件工作一条新链路 | 否则所有后台查询都堆在一条永远不结束的链路里 |

## 范围外

- 结构化 JSON 日志（`trace_id`、`span_id` 作为字段）和业务事件日志（§9.1）：下一轮。
- 指标、Prometheus、告警（§9.2、§9.3）：再下一轮。
- search 的 binlog 同步、goods 缓存重试这类后台工作没有单独建链路（它们的轮询被静音了）。
- Jaeger 只存在内存里，重启就没了；实验环境够用。

## 验收标准

- 单元测试：
  - 脱敏：请求头、gRPC 元数据、Redis 参数、带值 SQL（含 `''` 和 `\'` 转义）、SQL 错误、无法解析的请求头、漏网的 Bearer 和 JWT，全部被改写，非敏感内容保留。
  - 导出器只交出脱敏后的 span。
  - `Quiet` 上下文有效但不采样，子 span 不记录。
  - 消费者 span：普通消息是发布者的子 span；延迟消息是新链路并链接到发布者；没有链路头时新起一条。
- 集成测试（`GOFRAMESHOP_IT=1`）：
  - 写事件时的链路 ID 存进 `outbox_event.traceparent`，并出现在消息头和信封里；投递器的 Producer span 在同一条链路里。
  - 绕过进程内脱敏直接发给 Collector 的 span，到 Jaeger 时令牌、SQL 值、Redis 参数都已去掉，非敏感部分还在。
- 本地端到端（10 个进程 + 基础设施），在 Jaeger 里确认：
  - 下单：gateway-h5 → order → goods（查价格、预留，含 Redis）、user（地址）→ 写事件的事务 → `publish order.payment_pending`。
  - 回调：`paysim.notify` → gateway-h5 → order → 查询模拟器 → 提交库存。
  - 后台改商品名：gateway-admin → goods → `publish goods.changed` → banner 的 `consume banner.goods-changed`。
  - 到期关单（恢复任务调到 60 秒）：`consume order.expiry` 是新链路，链接到下单链路里的发布 span，下面有关闭支付单和释放库存。
  - 恢复任务关单：每个订单一条 `order.recover` 链路；没有一条只含数据库轮询的"噪音"链路。
  - RabbitMQ 停机演练：下单链路里能看到 3 次失败的发布（带错误原因）和第 4 次成功。
  - 日志行里的链路 ID 能在 Jaeger 查到同一条链路。
  - 扫描 5060 个 span 没有任何敏感数据；支付端到端 27 项、事件端到端全部通过。
- `go test ./...`（含 `-race`）、`go vet ./...`、`go mod verify`、`docker compose config` 通过；`go mod tidy` 稳定；重新生成代码没有差异。
