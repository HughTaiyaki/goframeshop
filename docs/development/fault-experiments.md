# 故障实验

任务、决定和验收见 [`tasks/0021-fault-experiments.md`](../../tasks/0021-fault-experiments.md)。实验矩阵来自[详细设计](../GoFrame微服务商城详细设计.md) §8.4、§10。

## 1. 跑一遍

```bash
make infra-up
make fault-trade
```

- 实验程序自己构建并启动 user、goods、order、paysim 四个进程，结束后全部停掉。**本地正在运行这些服务时请先停掉**，否则会提示端口被占用并退出。
- 每个进程用的是你本地配置的**实验副本**：支付超时 8 秒、恢复任务每 1 秒一轮、`otel.environment=experiment`。你的配置文件不会被修改。
- 每个实验新建自己的商品和用户，不影响已有数据，可以反复运行。
- 结果在 `reports/fault/<时间>/`：`report.md`、`report.json`，以及 `work/logs/` 下每个进程的日志（JSON，任务 0019 的格式）。这个目录不进 git，因为实验配置副本里有密码。

因为环境是 `experiment`，实验期间的日志、指标和链路在 Loki、Prometheus、Jaeger 里都能按 `environment="experiment"`（Loki 里是 `deployment_environment_name`）单独筛出来。

## 2. 实验

| ID | 窗口 | 怎么造 | 验证什么 |
| --- | --- | --- | --- |
| T1 | 并发超卖 | 库存 10，100 个请求同时下单 | 最多 10 个订单拿到库存 |
| T2 | 重复提交 | 同一个 `requestId` 并发 20 次，再换参数 | 只有 1 个订单；换参数返回 `AlreadyExists` |
| T3 | 预留成功、响应丢失 | `goods.reserve.after-commit=error` | 订单先停在预留中，恢复任务用同一个操作号重试，只有 1 份预留 |
| T4 | 释放先于预留 | 直接调库存接口 | 迟到的预留被墓碑拒绝，库存不变 |
| T5 | 付款与关单竞争 | `order.payment.before-record=pause`，停住时取消订单，再放行 | 订单关闭、付款退回且只退一次、库存没有卖出 |
| T6 | 已支付订单收到到期消息 | 直接往到期队列投一条消息 | 订单仍是已支付，没有释放库存 |
| T7 | 消费提交后、ACK 前崩溃 | `eventbus.inbox.after-commit=exit`（goods） | 重启后重投，Inbox 去重，购物车只减一次 |
| T8 | 发出事件后、标记 SENT 前崩溃 | `eventbus.relay.after-publish=exit`（order） | 同一个 `event_id` 重发，消费方只处理一次 |
| T9 | 到期消息丢失 | 清空延迟队列 | 数据库扫描在到期后关单并释放库存 |
| T10 | 新旧 Protobuf | 和提交的快照对比 | 字段编号、类型、名字不变；删除的编号保留 |
| C1 | 订单写入后崩溃 | `order.create.after-insert=exit` | 重启后恢复任务接着预留，进入待支付 |

每个实验结束后检查不变量（`tests/fault/invariants.go`）：

- 库存 `available + reserved + sold` 等于开始时的量；库存行和预留记录对得上。
- 订单都停在可以停的状态（待支付、已支付、已发货、已完成、已关闭），没有卡在中间。
- 预留和订单状态一致：待支付对应 RESERVED，已支付对应 COMMITTED，已关闭对应 RELEASED 或没有预留。
- 一笔付款的退款总额不超过付款金额，技术退款最多一次。

T10 不需要启动进程，随 `go test ./...` 一起运行。有意新增字段或消息后，刷新快照：

```bash
go test ./tests/fault/ -run ProtoCompat -update-proto-snapshot
```

## 3. failpoint

`utility/failpoint` 在代码里放了 5 个命名的注入点。只有进程启动时设置了 `GOFRAMESHOP_FAILPOINTS` 才会生效，生产环境不会设置，这时每个注入点只是一次布尔判断。

```text
GOFRAMESHOP_FAILPOINTS="order.create.after-insert=exit"
GOFRAMESHOP_FAILPOINT_DIR=/path/for/barriers      # pause 用
```

| 动作 | 效果 |
| --- | --- |
| `exit` | 立刻结束进程，不跑 defer，等同于崩溃 |
| `error` | 这一步返回 `failpoint.ErrInjected`；goods 把它报成 `Unavailable`，调用方看到的就是"响应丢了" |
| `pause` | 写 `<dir>/<名字>.hit`，等 `<dir>/<名字>.release` 出现再继续（最多 2 分钟）：用屏障控制先后，不靠 `sleep` |

每个注入点在一个进程里**只触发一次**：同一进程里的重试会正常通过，重启后不带环境变量也就不再触发。

| 注入点 | 位置 |
| --- | --- |
| `order.create.after-insert` | `app/order/internal/logic/order/order.go`：订单写入后、预留前 |
| `order.payment.before-record` | `app/order/internal/logic/order/paymentflow.go`：确认付款、锁订单行前 |
| `goods.reserve.after-commit` | `app/goods/internal/logic/inventory/inventory.go`：预留事务提交后 |
| `eventbus.relay.after-publish` | `utility/eventbus/outbox.go`：RabbitMQ 确认后、标记 SENT 前 |
| `eventbus.inbox.after-commit` | `utility/eventbus/inbox.go`：消费事务提交后、ACK 前 |

新增注入点时：名字写成 `服务.步骤.位置`，在 `tasks/0021` 和这里的表里登记，并补一个用到它的实验。

## 4. 代码在哪

```text
utility/failpoint/            注入点
tests/fault/cluster.go        构建、实验配置、启动 / 杀掉 / 重启进程
tests/fault/shop.go           以真实调用方的方式下单、付款，直接往数据库放实验商品
tests/fault/invariants.go     不变量
tests/fault/experiments_test.go   T1–T9、C1
tests/fault/protocompat*.go   T10 和快照 testdata/proto_snapshot.json
tests/fault/report.go         报告
```
