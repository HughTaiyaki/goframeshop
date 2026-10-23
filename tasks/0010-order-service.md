# 任务 0010：order 订单服务（下单、库存闭环、售后申请）

状态：已完成，支付不在本任务范围内。来源是原教程的"订单服务-事务操作"和"售后订单接口"两节，按[详细设计](../docs/GoFrame微服务商城详细设计.md) 6.1–6.3 节重写。开发说明见 [`docs/development/order-service.md`](../docs/development/order-service.md)。

## 新增了什么

| 类别 | 内容 |
| --- | --- |
| 进程 | `app/order`（gRPC `:31007`，库 `orders`） |
| 建表 | `008_order_schema.sql`：`order_info`、`order_goods`、`refund_info` |
| gRPC | `OrderService`（下单、取消、我的订单、全部订单），`RefundService`（申请、我的售后、全部售后）；user 服务新增 `GetConsignee` |
| 前台 HTTP | `POST /frontend/order`、`POST /frontend/order/cancel`、`GET /frontend/order`、`GET /frontend/order/detail`、`POST /frontend/refund`、`GET /frontend/refund`、`GET /frontend/refund/detail` |
| 后台 HTTP | `GET /backend/order`、`GET /backend/order/detail`、`GET /backend/refund` |
| 身份 | 第三对密钥：订单服务的**服务身份**。`utility.AuthPolicy` 支持一个服务同时接受多种身份，并按接口授权 |
| 后台任务 | 订单服务里的恢复任务：重试预留、预留超时关单、支付超时关单、重试释放 |
| 共享代码 | `AuthPolicy`、`LoadTokenVerifierAt` / `LoadTokenSignerAt`、`ServiceToken`、`GrpcClientTimeoutOf`；`TimeFromTimestamp` 改为输出本地时间 |

## 与原教程的差异

| 原教程 | 本仓库 | 原因 |
| --- | --- | --- |
| 金额（`price`、`actual_price`）由客户端填写 | 服务端向 goods 查询价格并计算总额 | 否则可以用任意价格下单 |
| 收货人姓名、电话、地址由客户端填写 | 客户端只传自己的地址 ID，服务端用顾客自己的令牌查询并保存快照 | 地址必须属于本人；之后改地址不影响旧订单 |
| 下单不碰库存 | 先预留库存，待支付期间占住；取消或超时后释放 | 防止超卖，未付款的订单也不会永久占用库存 |
| 没有幂等，连点两次会生成两个订单 | 客户端传 `requestId`，同一个请求号只会生成一个订单 | 网络重试是常态 |
| 前台订单列表可以按任意 `user_id` 查询，详情不检查归属 | 前台只能看到自己的订单，别人的订单号返回 NotFound | 原写法会泄露所有人的订单和地址 |
| 订单状态只有数字 1–5，没有关闭 | 按详细设计的状态机：RESERVING、PENDING_PAYMENT、CLOSING、CLOSED | 能表达"库存正在还回去"这类中间状态，崩溃后能接着处理 |
| 售后直接写入请求内容，`user_id` 甚至可以由客户端传 | 只能对自己的已支付订单、订单里的商品申请，每件商品一次；用户来自令牌 | 原写法可以替任何人、对任何订单发起售后 |
| 订单号用时间加 4 位 `math/rand` | 时间加 8 位 `crypto/rand`，并有唯一键 | 4 位随机数在高并发下很容易重复 |
| 库名 `order` | 库名 `orders` | `order` 是 SQL 保留字，每处都要加反引号 |
| 返回 `pbentity` | 独立的协议消息 | 见原教程修订说明 |

## 已采用的决定

- **服务身份用第三对密钥，而不是借用管理员令牌**：订单服务被攻破时，只能调用库存接口，不能改商品、删分类。
- **结果未知时不改状态**：库存调用超时，可能已经预留，也可能没有。订单保持原样，由后台任务用同一个操作号重试，比"猜一个结果"安全。
- **状态变更全部是带条件的 UPDATE**，只有改到行的一方继续下一步；所以取消和超时并发时只释放一次，多实例运行也安全。
- **超时时间来自配置**：`order.reserveTimeout`（60s）、`order.paymentTimeout`（15m）、`order.recoveryInterval`（5s），默认值取自详细设计。

## 范围外

- 支付模拟器和 `PAID` 及之后的状态，以及支付成功后的库存提交（`CommitInventory` 已经可用）。下一轮做。
- 售后审批和真正的退款。
- 优惠券（教程的 `coupon_price`）：详细设计明确首版不接受优惠券。
- 商品规格（教程的 `goods_options_id`）：目前一个商品就是一个库存单位。
- 网关把 gRPC 错误码映射为 HTTP 状态码：仍然统一返回 HTTP 200，靠 `code` 区分。

## 验收标准

- 单元测试见开发说明第 9 节，`-race` 下通过。
- goods 鉴权测试：服务令牌只能调用库存接口，调用其他写接口返回 `PermissionDenied`。
- 本地端到端（9 个进程）：
  - 请求里伪造的金额被忽略；同一请求号返回同一订单，内容不同返回 `code:6`。
  - 别人的订单查看、取消都返回 `code:5`；用别人的地址返回 `code:3`。
  - 库存不足时订单直接关闭；取消后库存恢复；30 秒未付的订单被后台任务关闭，库存恢复。
  - 已关闭的订单申请售后返回 `code:9`；商品下架后无法下单。
  - 两个网关都拒绝用错的令牌；日志里没有令牌和密码。
- `go test ./...`、`go vet ./...`、`go mod verify`、`docker compose config` 通过；GoFrame 仍为 v2.10.2；重新生成代码没有差异；ES 和 MySQL 集成测试仍然通过。
