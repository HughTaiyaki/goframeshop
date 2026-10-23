# 支付：支付模拟器与订单支付

这一轮新增了两样东西：

- `app/paysim`：**支付模拟器**，代替微信、支付宝这类外部支付渠道。
- 订单服务里的**支付流程**：去支付、收回调、提交库存、超时关单时处理支付、自动退款。

任务、决定和验收见 [`tasks/0012-payment-simulator.md`](../../tasks/0012-payment-simulator.md)。订单本身的状态机见 [`order-service.md`](order-service.md)。

## 1. 为什么要一个"模拟器"

真实支付渠道是别人的系统，它有几个让人头疼的特点：

| 特点 | 后果 |
| --- | --- |
| 调用会超时 | 不知道支付单到底建没建成 |
| 回调会重复，也会丢 | 不能"收到回调就加一次钱"，也不能"没收到回调就当没付" |
| 回调地址是公开的 | 谁都能伪造一个"支付成功" |
| 付款和关单可能同时发生 | 顾客在第 14 分 59 秒付款，后台在第 15 分钟关单 |

模拟器把这些特点原样搬到本地：它有自己的库 `paysim`，订单服务**只能通过它的接口**打交道，不能直接查它的表。这样写出来的订单代码，换成真实渠道时规则不用变。

## 2. 一次付款发生了什么

```text
① 顾客         POST /frontend/order/pay {orderNo}
② 订单服务      记下支付单（本地 payment 表，CREATED）
③ 订单服务 ──▶  模拟器 CreatePayment(P<订单号>, 金额, 截止时间)
④ 顾客 ◀──      { paymentId, state: CREATED, payUrl: http://127.0.0.1:8399/checkout/P<订单号> }
⑤ 顾客         打开 payUrl，点"支付"
⑥ 模拟器        同一事务里：CREATED → SUCCEEDED，生成交易号，安排回调
⑦ 模拟器 ──▶    POST /frontend/payment/notify（带签名）──▶ gateway-h5 ──▶ 订单服务
⑧ 订单服务      验签 → 向模拟器查询支付单 → 核对订单号、金额、币种
⑨ 订单服务      同一事务里：记下 SUCCEEDED，订单 PENDING_PAYMENT → PAYMENT_CONFIRMING
⑩ 订单服务 ──▶  goods CommitInventory(commit-<订单号>)：预留的库存变成已售
⑪ 订单服务      PAYMENT_CONFIRMING → PAID
⑫ 订单服务 ──▶  回复 code:0，模拟器不再回调
```

顾客看到的订单状态变化：`PENDING_PAYMENT` → `PAYMENT_CONFIRMING`（一般一闪而过）→ `PAID`。

## 3. 订单状态（新增部分）

```text
PENDING_PAYMENT ──付款确认──▶ PAYMENT_CONFIRMING ──库存提交成功──▶ PAID
       │                            │
       │ 超时 / 取消                 └ 库存提交被拒 ──▶ EXCEPTION ──退款完成──┐
       ▼                                                                    ▼
    CLOSING ──支付单已关闭（或已付款并安排退款）、库存已还──▶ CLOSED ◀── CLOSING
```

| 状态 | 含义 | 谁让它离开 |
| --- | --- | --- |
| `PAYMENT_CONFIRMING` | 钱已经到了，正在把预留的库存变成已售 | 提交成功 → `PAID`；没有结果就由后台任务重试 |
| `PAID` | 已支付，库存已售出 | 终态（发货以后再做） |
| `EXCEPTION` | 钱到了，但库存提交不了（预留已经没了） | 自动退款成功后 → `CLOSING` → `CLOSED`，关闭原因 `INVENTORY_CONFLICT` |

## 4. 五条规则，以及它们防住了什么

### 规则一：支付单号由订单号决定

支付单号永远是 `P` + 订单号，库存提交的操作号是 `commit-<订单号>`，技术退款单号是 `TR` + 支付单号。

所以"不知道成功没有就再来一次"永远安全：模拟器、goods 都按这些号去重，重复调用返回第一次的结果。

### 规则二：付款和关单，模拟器只让一个成功

模拟器每次改支付单，都先 `SELECT ... FOR UPDATE` 锁住那一行，在同一个事务里判断：

| 支付单现在 | 付款（顾客点"支付"） | 关单（订单服务） |
| --- | --- | --- |
| 不存在 | 404 | 写一条"已关闭"**墓碑** |
| `CREATED` | → `SUCCEEDED`（过了截止时间则拒绝） | → `CLOSED` |
| `SUCCEEDED` | 不变（不会扣两次钱） | **不变**，回答"已付款" |
| `CLOSED` | 拒绝（409） | 不变 |

墓碑的作用：订单先关了、支付单还没建（顾客恰好那一刻在点"去支付"），之后迟到的创建请求看到墓碑，返回"已关闭"，不会再生出一张能付款的单。

### 规则三：回调只是"提示"，结果以查询为准

`/frontend/payment/notify` 不需要登录，谁都能往里发请求。所以订单服务：

1. 先用模拟器的**公钥**验签（Ed25519，签的是原始请求体，网关原样转发，不做任何改动）。
2. 回调时间离现在超过 5 分钟的也拒绝，防止截获旧回调重放。
3. 验签通过也**不相信回调内容**，而是拿着支付单号去模拟器**查询**。
4. 核对查询结果：订单号、金额、币种（CNY）都必须和订单一致，否则记错误日志、不记账，留给人工处理。

回调签名验不过返回 `code:16`；处理过程中出错（比如模拟器暂时查不到）返回非 0，模拟器会稍后再发。

### 规则四：记账和改订单在同一个事务里

`ConfirmPaid` 先锁住订单行，再记支付、再改订单。后台关单用的是同一行上的条件 UPDATE，所以两者一定一先一后：

| 拿到锁时订单是 | 做什么 |
| --- | --- |
| `PENDING_PAYMENT` | 订单 → `PAYMENT_CONFIRMING`，记下付款时间，然后去提交库存 |
| `PAYMENT_CONFIRMING` / `PAID` / `EXCEPTION` | 什么都不做（重复回调） |
| `CLOSING` / `CLOSED` | 订单**保持关闭**，安排一笔技术退款（`LATE_PAYMENT`）；已经安排过就不再安排 |

"钱收了订单却关了"不会被忽略，"同一笔钱让订单走两遍"也不会发生。

### 规则五：关单先处理支付，再还库存

订单进入 `CLOSING` 后（超时、取消都一样），后台任务：

1. 如果这张订单曾经可以付款（有支付截止时间），先请模拟器关闭支付单。
   - 模拟器联系不上：**停在 `CLOSING`，库存先不还**，过几秒再试。
   - 模拟器说已关闭：记下来。
   - 模拟器说已付款：按规则四记账，安排退款。
2. 然后释放库存，订单 → `CLOSED`。

退款由后台任务单独重试，直到模拟器确认。

## 5. 回调丢了怎么办

模拟器发回调失败（网关挂了、返回非 0），会按 2s、4s、8s……（最长 5 分钟）重试，最多 10 次。

10 次都失败，或者付款时选了"不通知"，订单服务就不知道已经付款了。这时顾客只要再点一次"去支付"：订单服务向模拟器创建支付单（幂等），发现已经是 `SUCCEEDED`，当场确认，订单变成 `PAID`。

如果顾客一直不点，订单会在截止时间被关单：关单时模拟器回答"已付款"，钱会被自动退回。钱不会丢，只是这单没买成。

## 6. 身份与密钥

这一轮新增**第四对**密钥。四对密钥各管各的：

| 密钥对 | 私钥在 | 公钥在 | 用途 |
| --- | --- | --- | --- |
| 用户 | user | 网关、各服务 | 顾客令牌 |
| 管理员 | admin | gateway-admin、各服务 | 管理员令牌 |
| 订单服务身份 | order（`jwtService`） | goods、**paysim**（都叫 `jwtOrder`） | 订单服务调用库存接口、模拟器的商户接口 |
| **模拟器回调** | **paysim**（`notify.privateKey`） | **order**（`paysim.publicKey`） | 回调签名 |

模拟器的商户接口（创建、关闭、查询、退款）只接受订单服务的服务令牌；收银台页面不需要令牌。

## 7. 本地运行

```bash
make db-init                      # 010、011：paysim 库，orders 库的 payment、tech_refund 表
make jwt-keys                     # 第四对：私钥给 paysim 的 notify，公钥给 order 的 paysim
cd app/paysim
cp manifest/config/config.yaml.example manifest/config/config.yaml
```

| 配置 | 填什么 |
| --- | --- |
| `app/paysim` 的 `jwtOrder.publicKey` | 订单服务那对密钥的公钥（和 goods 的 `jwtOrder` 一样） |
| `app/paysim` 的 `notify.privateKey` | 第四对的**私钥** |
| `app/paysim` 的 `notify.url` | 默认 `http://127.0.0.1:8199/frontend/payment/notify`（gateway-h5） |
| `app/order` 的 `paysim.publicKey` | 第四对的**公钥** |

然后启动 paysim，重启 order 和 gateway-h5（新路由）、gateway-admin（新状态）。完整演示见 [`tutorial/03-自己跑一遍.md`](../tutorial/03-自己跑一遍.md#支付)。

### 故障演练

模拟器专门留了几个"制造故障"的办法，方便观察系统怎么自愈（也是以后日志诊断 Agent 的练习素材）：

| 想看什么 | 怎么做 | 应该看到 |
| --- | --- | --- |
| 回调丢失 | `curl -X POST '<payUrl>/pay?notify=0'` | 订单停在 `PENDING_PAYMENT`；再调一次 `/frontend/order/pay` 变成 `PAID` |
| 回调重试 | 停掉 gateway-h5 再付款 | paysim 日志：`attempt 1 failed, retry at ...`，间隔翻倍；重启网关后 `acknowledged` |
| 晚到的付款 | `notify=0` 付款后立刻取消订单 | 订单 `CLOSED`；order 日志 `arrived after the order started closing; refund ... scheduled`；几秒后 `tech_refund` 为 `SUCCEEDED`，模拟器支付单为 `REFUNDED` |
| 超时关单 | 去支付后什么都不做，等 `order.paymentTimeout` | 订单 `CLOSED`（`PAYMENT_TIMEOUT`），模拟器支付单 `CLOSED`，再付款返回 409 |
| 伪造回调 | 往 `/frontend/payment/notify` 发任意内容 | `code:16`，order 日志 `bad signature`，订单不变 |

## 8. 代码在哪

```text
app/paysim/
  manifest/protobuf/provider/v1/        商户接口协议
  internal/logic/provider/              付款、关单、退款的裁定规则（核心，先读这个）
  internal/repository/                  行锁事务：每次改支付单都锁行后在同一事务里判断
  internal/notify/                      签名回调与重试
  internal/checkout/                    收银台页面
app/order/
  manifest/protobuf/payment/v1/         PayOrder、NotifyPayment
  internal/logic/order/paymentflow.go   去支付、处理回调、提交库存、技术退款（核心）
  internal/logic/order/order.go         close()：关单时先处理支付再还库存
  internal/repository/payment.go        ConfirmPaid 等：锁订单行后记账并改状态
  internal/clients/clients.go           调用模拟器
utility/paysign.go                      回调签名与验签
deploy/mysql/init/010_*.sql, 011_*.sql  建库建表
```

## 9. 测试

```bash
go test -race ./app/paysim/... ./app/order/...
GOFRAMESHOP_IT=1 go test ./app/paysim/internal/repository/ ./app/order/internal/repository/ -run Integration -v
```

用例清单见任务文件的"验收标准"。其中最重要的是两个并发测试：

- 订单逻辑：付款和超时关单同时发生 100 次，结果只能是"已支付、没有退款"或"已关闭、付了钱就退一次"。
- 真实 MySQL：记账和关单同时发生 30 次，结果和"谁先拿到订单行锁"完全一致。

## 10. 已知局限

- 只支持整单退款。售后（按商品退款、审批）下一步做。
- ~~后台任务每一轮的日志用同一个链路 ID~~：任务 0014 起每个订单、每次回调各是一条链路，见 [`tracing.md`](tracing.md#6-后台任务一件事一条链路)。
- 技术退款失败会一直重试，没有退避，也没有告警；`attempts` 字段记录了次数，可以据此排查。
- 模拟器没有付款人身份，拿到收银台地址的人都能付款。
