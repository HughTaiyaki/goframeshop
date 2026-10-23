# order 订单服务开发

`app/order` 负责下单、支付、取消、超时关单、订单查询和售后申请。它是一个 gRPC 服务，监听 `:31007`，使用 `orders` 库。任务、决定和验收见 [`tasks/0010-order-service.md`](../../tasks/0010-order-service.md)；支付部分见 [`payment.md`](payment.md) 和 [`tasks/0012-payment-simulator.md`](../../tasks/0012-payment-simulator.md)。

本文先用一次下单把整个过程讲清楚，再说明规则和运维方法。

## 1. 一次下单发生了什么

顾客在前台点"提交订单"：

```text
浏览器 ──POST /frontend/order──▶ gateway-h5 ──CreateOrder──▶ order 服务
        { requestId, items:[{goodsId, quantity}], consigneeId, remark }
```

order 服务依次做这些事：

| 步 | 做什么 | 为什么 |
| --- | --- | --- |
| 1 | 检查参数，合并重复商品，按商品 ID 排序 | `{A:1, A:1}` 和 `{A:2}` 是同一个订单 |
| 2 | 用 `(用户, requestId)` 查找已有订单；找到就直接返回它 | 网络断了、用户连点两下，都不会生成第二个订单 |
| 3 | 向 goods 服务查询每件商品的**当前价格和名称** | 价格绝不信任客户端：教程让客户端自己填金额，等于可以 1 分钱下单 |
| 4 | 带着**顾客自己的令牌**向 user 服务查询收货地址 | 只能用自己的地址；地址被复制进订单（快照），以后改地址不影响旧订单 |
| 5 | 在一个事务里写入订单和订单商品，状态为 `RESERVING`（预留中） | 订单从此存在，后面任何一步失败都能被找回来继续处理 |
| 6 | 带着**订单服务自己的服务令牌**，请 goods 服务预留库存，操作号是 `reserve-<订单号>` | 预留成功才会让顾客付款，所以不会超卖 |
| 7 | 根据结果更新订单状态，见下表 | |

| 预留结果 | 订单变成 | 说明 |
| --- | --- | --- |
| 成功 | `PENDING_PAYMENT`（待支付），`expiresAt` = 现在 + 15 分钟 | 库存已经为这个订单留好 |
| 库存不足 / 商品已下架 | `CLOSED`（已关闭），原因写在 `closeReason` | 没有预留到任何库存，所以不需要释放 |
| 订单已被取消（预留请求慢了一步） | `CLOSING` → 释放 → `CLOSED` | 详见第 3 节 |
| **超时或 goods 不可用，结果未知** | **保持 `RESERVING`** | 不知道到底预留没有，就不能乱改状态；后台任务会用**同一个操作号**再问一次 |

返回给前端的就是这时的订单。前端如果看到 `RESERVING`，过一两秒再查一次即可。

## 2. 订单状态

```text
            预留成功                    付款确认                      库存提交成功
RESERVING ─────────▶ PENDING_PAYMENT ─────────▶ PAYMENT_CONFIRMING ────────────▶ PAID
    │                      │                            │ 库存提交被拒
    │                      │ 超时 / 取消                 ▼
    │                      ▼                        EXCEPTION ──退款完成──┐
    │                   CLOSING ◀──────────────────────────────────────────┘
    │                      ▲  │ 支付单已关闭（或已付款并安排退款）、库存已还
    │                      │  ▼
    │                      │ CLOSED
    │ 库存不足 / 已下架 ─────┼──────────▶ CLOSED
    └ 超过 60 秒仍未预留成功 / 取消 ┘
```

| 状态 | 含义 | 谁让它离开这个状态 |
| --- | --- | --- |
| `RESERVING` | 正在向 goods 预留库存 | 下单请求本身，或者后台任务重试 |
| `PENDING_PAYMENT` | 库存已留好，等待付款 | 付款确认；顾客取消；到期后由后台任务关单 |
| `PAYMENT_CONFIRMING` | 钱到了，正在把预留的库存变成已售 | 提交成功 → `PAID`；没有结果由后台任务重试 |
| `PAID` | 已支付 | 终态（发货以后再做） |
| `EXCEPTION` | 钱到了，库存却提交不了 | 自动退款成功后 → `CLOSING` |
| `CLOSING` | 正在关闭支付单、把库存还回去 | 都完成后进入 `CLOSED`；失败的话后台任务一直重试 |
| `CLOSED` | 结束，`closeReason` 写明原因 | 终态 |

支付相关的三个状态详见 [`payment.md`](payment.md#3-订单状态新增部分)。

关闭原因：`USER_CANCELLED`（顾客取消）、`PAYMENT_TIMEOUT`（超时未付）、`RESERVE_TIMEOUT`（预留一直没有结果）、`INSUFFICIENT_STOCK`、`GOODS_UNAVAILABLE`、`RESERVE_CONFLICT`（取消和预留同时发生）、`INVENTORY_CONFLICT`（付款后库存提交不了，已退款）。

## 3. 三个保证，以及它们是怎么做到的

### 不会多扣，也不会漏还库存

- 每次调用库存接口，操作号都由订单号派生：`reserve-<订单号>`、`release-<订单号>`。goods 服务对同一个操作号只执行一次，重试时返回第一次的结果。所以"不知道成功没有就再问一次"永远是安全的。
- 释放时，如果预留请求还没到，goods 会留下一条"已释放"的墓碑；之后迟到的预留会被拒绝。所以"先取消、后预留到达"也不会留下被占住的库存。

### 取消和超时同时发生，只处理一次

每次改状态都是一条**带条件的 UPDATE**：

```sql
UPDATE order_info SET status='CLOSING', close_reason=... WHERE id=? AND status IN ('RESERVING','PENDING_PAYMENT')
```

只有真正改到行的那一方（受影响行数为 1）才会去释放库存。顾客取消和后台超时同时到达时，只有一方能成功，库存只释放一次。测试里专门让两者并发跑过。

### 服务挂了也能接着处理

订单在第 5 步就写进了数据库。之后不管哪一步失败（goods 超时、进程重启），订单都停在某个中间状态。**后台恢复任务**每隔几秒扫描一次：

| 找到的订单 | 处理 |
| --- | --- |
| `RESERVING`，还没到 60 秒 | 用同一个操作号重试预留 |
| `RESERVING`，超过 60 秒 | 改为 `CLOSING`，再释放 |
| `PENDING_PAYMENT`，已经过期 | 改为 `CLOSING`（原因 `PAYMENT_TIMEOUT`），再关闭 |
| `PAYMENT_CONFIRMING` | 用同一个操作号重试提交库存 |
| `CLOSING` | 先请支付模拟器关闭支付单，再释放库存（见 [`payment.md` 规则五](payment.md#规则五关单先处理支付再还库存)） |
| 没完成的技术退款 | 用同一个退款单号重试 |

因为每一步都满足上面两个保证，同时运行多个订单服务实例也是安全的。

**到期关单其实不用等扫描**：订单进入"待支付"时，同一个事务里还写了一条延迟事件 `order.payment_pending`，到期那一刻经 RabbitMQ 送回订单服务，订单当场关闭（`Settle`）。扫描是兜底：消息丢了、晚了，订单照样会被关。见 [`events.md`](events.md#51-订单到期orderpayment_pending)。

## 4. 谁能调用什么

订单服务同时接受两种令牌，按接口区分：

| 接口 | 谁能调 | 经过哪个网关 |
| --- | --- | --- |
| 下单、取消、去支付、我的订单（列表、详情）、申请售后、我的售后（列表、详情） | 商城用户令牌 | gateway-h5：`/frontend/order*`、`/frontend/refund*` |
| 全部订单（列表、详情）、全部售后 | 管理员令牌 | gateway-admin：`/backend/order*`、`/backend/refund` |
| 支付结果通知 `NotifyPayment` | 不需要令牌，靠支付模拟器的签名 | gateway-h5：`/frontend/payment/notify` |

- 用户令牌调用管理员接口返回 `PermissionDenied`（`code:7`），反过来也一样。没有令牌返回 `Unauthenticated`（`code:16`）。
- 顾客只能看到自己的订单。别人的订单号返回 `NotFound`，不会让你知道这个单号存在。教程里前台订单列表可以按任意 `user_id` 查询，等于能看到所有人的订单。

订单服务调用其他服务时用的身份：

| 调用 | 用什么身份 |
| --- | --- |
| goods `GetGoods`（查价格） | 不需要身份（公开接口） |
| user `GetConsignee`（查地址） | 转发顾客自己的令牌：只能拿到这个顾客自己的地址 |
| goods 库存接口 | **订单服务自己的服务令牌**（见下节） |
| 支付模拟器（创建、关闭、查询、退款） | 同一个服务令牌 |

## 5. 服务身份：第三对密钥

库存接口不能让顾客直接调用，又不应该借用管理员令牌。所以订单服务有**自己的**一对 Ed25519 密钥：

```text
order 服务：持有私钥（jwtService），每 5 分钟签一个服务令牌，过半就换新
goods 服务：持有对应公钥（jwtOrder），只允许这种令牌调用库存的 4 个接口
```

goods 的权限规则因此变成：

| goods 的接口 | 允许谁 |
| --- | --- |
| 商品、分类的读接口 | 所有人 |
| 库存的预留、提交、释放、查询 | 管理员、订单服务 |
| 其余写接口 | 只有管理员 |

服务令牌拿去调用 goods 的其他写接口会被拒绝（`PermissionDenied`）。

这套"多种身份、按接口授权"的机制在 `utility/auth.go` 的 `AuthPolicy` 里，订单服务和 goods 服务都在用。

## 6. 售后

- 顾客只能对**自己的、已支付的**（`PAID`）订单申请；商品必须在这个订单里；同一订单的同一件商品只能申请一次。其他状态的订单返回 `code:9`。
- 售后的审批（同意、拒绝）和按商品退款还没做；目前只有系统自动发起的整单技术退款（见 [`payment.md`](payment.md)）。

## 7. 本地运行

```bash
make db-init                     # 008：建 orders 库
make jwt-keys                    # 第三对密钥：私钥给 order，公钥给 goods
cd app/order
cp manifest/config/config.yaml.example manifest/config/config.yaml
```

在 `app/order/manifest/config/config.yaml` 里填写：

| 配置 | 填什么 |
| --- | --- |
| `jwtUser.publicKey` | 商城用户那对密钥的公钥（和 gateway-h5 一样） |
| `jwtAdmin.publicKey` | 管理员那对密钥的公钥（和 gateway-admin 一样） |
| `jwtService.privateKey` | 刚生成的第三对密钥的**私钥** |
| `paysim.publicKey` | 支付模拟器回调密钥（第四对）的**公钥**，见 [`payment.md`](payment.md#7-本地运行) |
| `rabbitmq.url` | RabbitMQ 地址（延迟关单事件），见 [`events.md`](events.md#6-本地运行) |

再在 goods 的配置里加上 `jwtOrder.publicKey`（第三对的**公钥**），然后重启 goods。

想在本地看到超时关单，可以把 `order.paymentTimeout` 改成 `30s`。

完整演示见 [`tutorial/03-自己跑一遍.md`](../tutorial/03-自己跑一遍.md#下单取消与超时关单)。

## 8. 代码在哪

```text
app/order/
  manifest/protobuf/{order,refund,payment}/v1/   协议
  internal/logic/order/                  状态机、下单、取消、恢复任务（核心，先读这个）
  internal/logic/order/paymentflow.go    去支付、回调、提交库存、技术退款
  internal/logic/refund/                 售后规则
  internal/repository/                   MySQL：带条件的状态变更、订单与商品一起写入、锁订单行记账
  internal/clients/                      调用 goods（价格、库存）、user（地址）和支付模拟器
  internal/cmd/cmd.go                    权限规则、后台恢复任务的定时器
deploy/mysql/init/008_order_schema.sql   建表
deploy/mysql/init/011_order_payment_schema.sql   payment、tech_refund、paid_at
```

## 9. 测试

```bash
go test ./app/order/...
```

- `logic/order`：价格由服务端计算；重复提交返回同一个订单；内容不同就拒绝；不能用别人的地址；库存不足时直接关闭；结果未知时由后台任务重试（并且用同一个操作号）；预留超时关单；支付超时关单（只释放一次）；取消失败后由后台任务补完；取消和超时并发时只释放一次；已支付订单不能取消；列表按用户隔离；金额溢出被拒绝。
- `logic/refund`：只能对自己已支付的订单申请；商品必须在订单里；不能重复申请。
- `logic/order/paymentflow_test.go`：支付相关的用例，清单见 [`tasks/0012`](../../tasks/0012-payment-simulator.md#验收标准)。
- `cmd`：用户、管理员、陌生人、旧的 `x-user-id` 分别调用每个接口，结果都符合第 4 节的表；`NotifyPayment` 不需要令牌。
- 集成测试（`GOFRAMESHOP_IT=1`）：`repository` 里记账与关单在真实 MySQL 上并发。

## 10. 已知局限

- 订单号的唯一性靠数据库唯一键保证；同一秒内两个订单号碰撞的概率约为一亿分之一，碰撞时那次下单会失败，需要重新提交。
- 后台任务每次最多处理 100 个订单；订单量大时应该分批，并给 `Due` 查询加上更合适的索引。
- 历史订单、墓碑记录的清理没有做。
