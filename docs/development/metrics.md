# 指标、告警与日志查询

任务、决定和验收见 [`tasks/0020-metrics-and-alerts.md`](../../tasks/0020-metrics-and-alerts.md)。指标和告警的契约在[详细设计](../GoFrame微服务商城详细设计.md) §9.2、§9.3；日志格式见 [`logging.md`](logging.md)，链路见 [`tracing.md`](tracing.md)。

## 1. 数据怎么走

```text
                    OTLP（:4317）                      抓取（每 5 秒）
各服务 ──链路、指标、日志──▶ OTel Collector ──链路──▶ Jaeger（:16686）
                                       ├──指标──▶ :8889 ◀── Prometheus（:9090）── 告警规则
                                       └──日志──▶ Loki（:3100）
```

- 服务只认 Collector（`otel.endpoint`）。不配置时什么都不导出：日志照样写到标准输出，指标和链路关闭。
- 指标每 5 秒推一次（`telemetry.MetricsInterval`），Prometheus 每 5 秒抓一次 Collector。
- `make infra-up` 一起启动 Prometheus 和 Loki。

| 界面 / 接口 | 地址 |
| --- | --- |
| Prometheus 查询、告警 | <http://127.0.0.1:9090>，告警在 `/alerts`，接口 `/api/v1/alerts` |
| Collector 暴露的原始指标 | <http://127.0.0.1:8889/metrics> |
| Loki 查询接口 | `http://127.0.0.1:3100/loki/api/v1/query_range` |
| Jaeger | <http://127.0.0.1:16686> |

## 2. 指标

所有数据点都带 `service`（`order-service` 等，和链路里的服务名一致）、`environment`（配置 `otel.environment`）。名字和标签严格按设计 §9.2；Collector 配置为不加后缀、不把资源属性变成标签，所以代码里的名字就是 Prometheus 里的名字。

| 指标 | 类型 | 其他标签 | 记录位置 |
| --- | --- | --- | --- |
| `trade_order_create_total` | counter | `outcome`：`pending_payment`、`reserving`、`rejected`、`duplicate`、`invalid`、`error` | order：`logic/order` 的 `Create` |
| `trade_order_pending_payment` | gauge | — | order：每次采集查一次数据库 |
| `trade_workflow_backlog` | gauge | `step` | order：`reserve`、`commit`、`close`、`tech_refund`、`after_sale_refund`、`outbox`；goods：`outbox` |
| `trade_workflow_oldest_age_seconds` | gauge | `step` | 同上；没有积压时为 0 |
| `trade_rpc_duration_seconds` | histogram | `peer`、`method`、`outcome`：`ok`、`rejected`、`error` | 所有 gRPC 客户端：共用的 `utility.GrpcClientTimeout` / `GrpcClientTimeoutOf` |
| `trade_payment_callback_delay_seconds` | histogram | — | order：模拟器记录的付款时间 → 订单服务确认付款 |
| `trade_db_pool_in_use`、`trade_db_pool_limit` | gauge | `pool` | 所有有数据库的服务：`telemetry.Start` 按配置里的 `database.<组名>` 自动注册 |
| `trade_db_acquire_timeout_total` | counter | `pool` | 同上：**等过连接的次数**（见下） |
| `trade_db_acquire_wait_seconds_total` | counter | `pool` | 同上：等连接的总时间（契约之外的补充） |

几点说明：

- **`outcome` 的划分**：`rejected` 是被调用方给出的业务答复（参数错误、找不到、状态不允许、未登录、无权限、已存在），`error` 是技术错误（不可用、超时、内部错误等）。告警只看 `error`。
- **`trade_db_acquire_timeout_total`**：Go 的 `database/sql` 连接池没有"获取连接超时"，拿不到连接的调用会一直等到自己的 context 结束。所以这里记的是 `sql.DBStats.WaitCount`：连接池不够用时它会增长，正常时是 0。
- **积压的"最老"**：`reserve` 按下单时间，`commit`、`close` 按订单最后一次变化的时间（进入当前状态的时间），技术退款按创建时间，售后退款按同意时间，outbox 按事件写入时间。
- **读数据库的指标**（积压、待支付、连接池）在采集时查询，运行在 `telemetry.Quiet` 上下文里，不会每 5 秒在 Jaeger 里产生一条链路。
- **订单号、用户 ID、链路 ID、商品 ID 都不能做标签**：标签组合会无限增长。需要逐个查看时用日志。

设计 §9.2 的查询模板可以直接执行：

```promql
histogram_quantile(0.95,
  sum by (le) (rate(trade_rpc_duration_seconds_bucket{
    service="order-service",environment="local",peer="goods-service"
  }[2m])))
```

（实验环境把 `otel.environment` 设为 `experiment`，模板里的 `environment` 跟着改。）

## 3. 告警

规则在 `deploy/prometheus/rules/trade.yml`，修改后用 `promtool` 检查：

```bash
docker run --rm -v "$PWD/deploy/prometheus:/etc/prometheus:ro" --entrypoint /bin/promtool prom/prometheus:v3.5.0 check config /etc/prometheus/prometheus.yml
```

| 告警 | 条件 | 持续 | 摘要 |
| --- | --- | --- | --- |
| `TradeRpcErrorRateHigh` | 某个 `service` 调某个 `peer`：最近 1 分钟技术错误率 > 5%，且至少 20 次调用 | 30 秒 | 服务、被调方、错误率 |
| `TradeWorkflowStuck` | 某个 `step` 最老的待处理工作 > 30 秒 | 15 秒 | 服务、步骤、已等待的时间 |

- 告警带 `service`、`environment`、开始时间（`activeAt`）和摘要（`annotations.summary`），不带任何故障注入的名字（设计 §9.3）。
- 阈值是演示用的初值，不是生产 SLO。
- 没有 Alertmanager：告警只在 Prometheus 里查看，Agent 直接读 `/api/v1/alerts`。

## 4. 查日志（Loki）

每个服务的 JSON 日志也经 OTLP 发到 Loki。`service.name` 变成标签 `service_name`；其余字段是可以过滤的结构化元数据，名字里的点变成下划线（`event.name` → `event_name`）。

```logql
# 某个服务的全部日志
{service_name="order-service"}

# 某个订单经过的状态
{service_name="order-service"} | event_name="order.transition" | order_id="O2026…"

# 所有服务里某条链路的日志
{service_name=~".+"} | trace_id="8a6ca1c31a7c92190228a1856ecef99f"
```

接口：

```bash
curl -s --get http://127.0.0.1:3100/loki/api/v1/query_range \
  --data-urlencode 'query={service_name="order-service"} | event_name="order.transition"' \
  --data-urlencode "start=$(date -v-15M +%s)000000000"
```

- 日志在进程里已经脱敏，发往 Loki 的和写到标准输出的是同一份。
- 和标准输出一样，只有被记录的链路才带 `trace_id`。
- Loki 保留 7 天（`deploy/loki/loki.yaml`）。

## 5. 代码在哪

```text
utility/telemetry/metrics.go          指标名、记录函数、可观测指标、连接池、RPC outcome 划分
utility/telemetry/logexport.go        日志经 OTLP 发出
utility/telemetry/telemetry.go        Init：链路、指标、日志三个导出一起启动
utility/grpc.go                       timedCall：所有 gRPC 客户端调用都在这里计时
app/order/internal/repository/metrics.go   order 的积压和待支付查询
utility/eventbus/outbox.go            Backlog：outbox 积压，order 和 goods 共用
deploy/otel/collector.yaml            metrics、logs 流水线
deploy/prometheus/                    抓取配置和告警规则
deploy/loki/loki.yaml                 Loki 配置
```

## 6. 测试

```bash
go test ./utility/telemetry/ ./app/order/internal/logic/order/
GOFRAMESHOP_IT=1 go test ./app/order/internal/repository/ -run Backlog   # 积压查询，需要 Compose
```

单元测试用 OpenTelemetry 的内存读取器检查导出的内容：名字、单位、桶边界，以及每个指标只有契约允许的标签。

## 7. 已知局限

- 只有调用方记录 RPC 指标：经网关和服务之间的调用都覆盖了，但没有网关的直接调用（比如脚本直连 gRPC）不会被计入。
- Prometheus 和 Loki 的数据在 Compose 卷里，只用于实验；没有高可用。
- Collector 的 Prometheus 导出器 5 分钟没有新数据就丢掉一个序列（`metric_expiration`）：进程停掉 5 分钟后，它的指标会从查询结果里消失。
