# 事件：RabbitMQ 与 Outbox / Inbox

服务之间除了 gRPC 调用，现在还可以发**事件**："某件事已经发生了，谁关心谁处理"。这一篇讲事件怎么做到不丢、不重复生效，以及现在有哪两条事件。

任务、决定和验收见 [`tasks/0013-rabbitmq-outbox-inbox.md`](../../tasks/0013-rabbitmq-outbox-inbox.md)。

## 1. 为什么不能"提交后发消息"

原教程的写法：

```go
tx.Commit()                       // ① 订单写进数据库
go rabbitmq.Publish(ctx, msg)     // ② 再发消息
```

① 和 ② 之间任何时候崩溃，订单存在，消息却永远没发。反过来先发消息、后提交，事务回滚了，消息却已经发出去了。两个系统（MySQL、RabbitMQ）没法放进同一个事务里。

## 2. 解决办法：Outbox（发件箱）

把"要发的消息"也当成业务数据，**和业务修改写在同一个事务里**：

```text
BEGIN
  UPDATE order_info SET status='PENDING_PAYMENT' ...
  INSERT INTO outbox_event (event_id, type, payload, state='PENDING') ...
COMMIT
```

要么两者都在，要么都不在。然后由**投递器（Relay）**在后台把 PENDING 的事件发出去：

```text
Relay 每 500ms：
  ① 短事务领取一批 PENDING 事件（SELECT ... FOR UPDATE SKIP LOCKED），写上自己的名字和租约（15 秒）
  ② 事务外逐条发布，等 RabbitMQ 确认（publisher confirm）
  ③ 确认了 → SENT；失败或超时 → 记下错误，1s、2s、4s…（最长 1 分钟）后再试
```

| 情况 | 结果 |
| --- | --- |
| RabbitMQ 挂了 | 事件留在表里，恢复后补发，一条不丢 |
| 发出去了，标记 SENT 前崩溃 | 下次再发一遍，**同一个 event_id** |
| 两个 Relay 同时运行 | `SKIP LOCKED` 让它们领到不同的事件；租约过期前别人不会抢 |
| 没有队列接收（路由不到） | RabbitMQ 退回消息，当作失败重试，不会悄悄丢掉 |

所以：**事件一定会送达，但可能送达不止一次。**

## 3. 消费方：Inbox（收件箱）

"可能不止一次"由消费方解决。每个消费者在处理事件时，把 `(消费者名, event_id)` 插进 `inbox_event`，**和业务修改在同一个事务里**：

```text
BEGIN
  INSERT IGNORE INTO inbox_event (consumer, event_id, payload_hash)
  -- 插入了：第一次见到，执行业务修改
  -- 没插入：以前处理过，什么都不做
COMMIT
ACK（告诉 RabbitMQ 可以删掉这条消息了）
```

| 情况 | 结果 |
| --- | --- |
| 同一个事件来了两次 | 第二次只 ACK，不再执行 |
| 提交后、ACK 前崩溃 | RabbitMQ 重投，Inbox 认出来，只 ACK |
| 业务步骤失败（数据库不可用） | 整个事务回滚（包括 Inbox 记录），不 ACK，1 秒后重投 |
| 消息格式错误，或者同一个 event_id 内容却不同 | 写进 `quarantine_event`，然后 ACK，不让它堵住队列 |

## 4. 事件长什么样

```json
{
  "schema_version": 1,
  "event_id": "869ef2c2-da2e-434e-a3c6-66e6ef36573a",
  "type": "goods.changed",
  "aggregate_id": "2",
  "aggregate_version": 1532,
  "occurred_at": "2026-10-21T11:09:24Z",
  "payload": {"goodsId": 2, "name": "电视 X5", "onShelf": true, "deleted": false}
}
```

- `event_id` 在写入 outbox 时生成，重发时不变。
- `aggregate_version` 是 outbox 的自增 id。同一个商品（或订单）的事件是在锁住那一行时写入的，所以版本号越大，变化越晚。
- `type` 同时也是路由键。

**事件是事实，不是命令**：消费方要按自己看到的当前状态判断，而不是事件说什么就做什么。

## 5. 现在有哪些事件

所有交换机、队列、绑定都写在 `utility/rabbitmq/topology.go` 一个文件里，生产者和消费者启动时都会声明一遍（重复声明没有副作用），谁先启动都行。

```text
                          交换机 shop.events（topic）
order ──order.payment_pending──▶ order.expiry.delay（没人消费，消息带 TTL）
                                      │ TTL 到了，死信转发
                                      ▼ order.expiry.due
                                 order.expiry ──▶ order 服务：到期关单

goods ──goods.changed──────────▶ banner.goods-changed ──▶ banner 服务：更新手工位
```

### 5.1 订单到期：`order.payment_pending`

订单进入"待支付"时（同一个事务）写一条事件，送达时间 `expiresAt + 1 秒`（多 1 秒，防止毫秒取整让消息提前一点点到）。

**延迟怎么实现**：不用延迟交换机插件（它的延迟消息只存在单个节点上），而是用普通的持久队列：

1. Relay 发布时给消息设置 TTL = 送达时间 − 现在。
2. 消息进入 `order.expiry.delay`，这个队列**没有消费者**。
3. TTL 到了，RabbitMQ 把消息"死信"转发到路由键 `order.expiry.due`，进入 `order.expiry` 队列。
4. 订单服务消费：

```text
Inbox 事务：记录事件 + 把订单从 PENDING_PAYMENT 改成 CLOSING（条件：数据库里的 expires_at 确实已过）
提交，然后马上 Settle：关闭支付单、释放库存（和恢复任务做的是同一件事）
ACK
```

- 进入 `CLOSING` 本身就是"剩下的活"的持久记录：Settle 失败或进程崩溃，恢复任务会接着做完。
- 消息提前到了（时钟偏差），条件不成立，什么都不改；到期时恢复任务会关单。
- 消息丢了，恢复任务照样会关单。**消息负责准时，扫描负责兜底。**

TTL 的一个特点：RabbitMQ 只在消息排到队首时检查是否过期。所有到期消息的延迟都差不多（都是 `paymentTimeout`），按时间顺序进队，所以不会互相耽误。

### 5.2 商品变更：`goods.changed`

goods 改名、上下架、删除时（同一个事务）写一条事件，内容是改完之后的商品：名字、是否在售、是否已删除。

banner 收到后更新所有引用这个商品的手工位：

```sql
UPDATE position_info SET goods_name=?, goods_on_shelf=?, goods_version=?
WHERE goods_id=? AND goods_version < ?     -- 旧事件晚到，不会覆盖新的
```

- 前台（gateway-h5）只列 `goods_on_shelf = 1` 的手工位：商品下架或删除后，首页不再出现一个点进去是"商品不存在"的位置。
- 后台照样列出全部，并显示 `goodsOnShelf: false`，方便管理员处理。
- 管理员重新保存手工位时，会重新检查商品是否在售。

这修复了任务 0011 留下的已知局限（手工位复制的商品名不会跟着改）。

## 6. 本地运行

```bash
make infra-up        # 现在包含 rabbitmq
make db-init         # 012：事件表
```

order、goods、banner 的配置里加上：

```yaml
rabbitmq:
  url: "amqp://shop:change-me@127.0.0.1:5672/"   # 也可以用环境变量 RABBITMQ_URL
outbox:
  interval: "500ms"   # order、goods：Relay 多久看一次
```

管理界面：<http://127.0.0.1:15672>（本地账号 `shop` / `change-me`），能看到每个队列里有多少消息。

### 故障演练

| 想看什么 | 怎么做 | 应该看到 |
| --- | --- | --- |
| 消息准时关单 | 把 order 的 `recoveryInterval` 调到 `60s`，下单不付款 | 第 `paymentTimeout + 1` 秒左右关单，order 日志 `closing (expiry message)` |
| RabbitMQ 挂了 | `docker compose stop rabbitmq` 后下单 | 下单照常成功；`orders.outbox_event` 里事件是 PENDING，`attempts` 在涨；order 日志 `outbox event ... attempt N failed` |
| RabbitMQ 恢复 | `docker compose start rabbitmq` | 几秒到一分钟内补发（看退避到哪一步）；两个消费者日志 `consumer ...: connected` |
| 手工位跟随商品 | 后台给手工位上的商品改名、下架 | 前台 `/frontend/position` 名字跟着变、手工位消失；banner 日志 `goods N changed ... slot(s) updated` |
| 看积压 | 管理界面 Queues | `order.expiry.delay` 里是还没到期的订单 |

## 7. 代码在哪

```text
utility/rabbitmq/topology.go      所有交换机、队列、绑定（先读这个）
utility/rabbitmq/rabbitmq.go      发布（确认 + 退回）、消费循环（手动 ACK、断线重连）
utility/eventbus/outbox.go        Append、Relay
utility/eventbus/inbox.go         Handle：Inbox 去重、隔离
utility/eventbus/contracts.go     跨服务的事件内容
app/order/internal/events/        到期消费者
app/order/internal/logic/order/   reserve() 写事件；Settle
app/goods/internal/repository/goods.go   changeGoods：改商品 + 写事件
app/banner/internal/events/       商品变更消费者
deploy/mysql/init/012_events_schema.sql
```

## 8. 测试

```bash
go test ./app/order/... ./app/banner/... ./app/goods/...
GOFRAMESHOP_IT=1 go test ./utility/eventbus/ ./app/order/internal/events/ ./app/banner/internal/events/ ./app/goods/internal/repository/ -v
```

用例清单见任务文件的"验收标准"。最能说明问题的是：两个 Relay 同时投递 60 条事件，每条恰好发一次；同一事件的 10 个并发副本只执行一次；版本 20 先到、版本 10 后到，结果仍是版本 20。

## 9. 已知局限

- 消费失败靠 RabbitMQ 重投（间隔 1 秒），没有持久的重试表和 24 小时后转人工。两个消费者的业务步骤都只是本地数据库操作，暂时够用。
- 事件表没有清理，会一直增长。
- 手工位刚创建时版本号是 0：如果一条早于创建的旧事件恰好那时才送达，会被应用一次（下一次商品变更会纠正）。
- ~~事件里还没有 `traceparent`~~：任务 0014 起事件带链路上下文，Jaeger 里能从下单一路看到消费者，见 [`tracing.md`](tracing.md#5-事件也能追链路穿过-rabbitmq)。
