# 任务 0017：发货与收货

状态：已完成。开发说明见 [`docs/development/order-service.md`](../docs/development/order-service.md#11-发货与收货)。按[详细设计](../docs/GoFrame微服务商城详细设计.md) §1、§6.1 实现"简化发货收货"：订单状态在 `PAID` 之后增加 `SHIPPED → COMPLETED`。这是交易闭环的第三步；原教程没有这一节。

## 新增了什么

| 类别 | 内容 |
| --- | --- |
| 订单状态 | `PAID ──后台发货──▶ SHIPPED ──顾客确认收货 / 到期自动确认──▶ COMPLETED` |
| 建表 | `016_order_shipping.sql`：`order_info` 增加 `carrier`、`tracking_no`、`shipped_at`、`confirm_deadline`、`completed_at`、`complete_reason`，以及索引 `(status, confirm_deadline)` |
| order 服务 | `ShipOrder`（只接受管理员令牌）、`ConfirmReceipt`（只接受商城用户令牌，只能确认自己的订单）；恢复任务自动确认到期的订单 |
| 后台 HTTP | `POST /backend/order/ship`：填物流公司和运单号 |
| 前台 HTTP | `POST /frontend/order/confirm`：确认收货 |
| 订单视图 | 新增 `carrier`、`trackingNo`、`shippedAt`、`confirmDeadline`、`completedAt`、`completeReason`；列表可以按 `SHIPPED`、`COMPLETED` 筛选 |
| 配置 | `order.autoConfirm`：发货后多久自动确认收货，默认 `240h`（10 天） |

## 规则

- **状态只能用带条件的 UPDATE 改**（和其他状态一样）：发货是 `WHERE status = 'PAID'`，确认收货是 `WHERE status = 'SHIPPED'`。后台重复点"发货"、顾客确认和自动确认同时发生，都只有一方生效。
- **发货**：
  - 物流公司 1–32 个字符；运单号 1–64 位，只能是字母、数字和 `-`。
  - 只有 `PAID` 的订单能发货，其他状态返回 `FailedPrecondition`。
  - 幂等：已经发货（或已完成）的订单，用**同样的**物流公司和运单号再发一次，直接返回当前订单；换了运单号则返回 `FailedPrecondition`，不会悄悄改掉已发出的运单。
  - 发货时写入 `confirm_deadline = shipped_at + autoConfirm`。
- **确认收货**：
  - 只能确认自己的订单；别人的订单返回 NotFound（和查询一致，不泄露订单号是否存在）。
  - 只有 `SHIPPED` 的订单能确认；已经 `COMPLETED` 时直接返回（幂等），其他状态返回 `FailedPrecondition`。
  - `complete_reason`：`USER_CONFIRMED` 或 `AUTO_CONFIRMED`。
- **自动确认**：恢复任务扫描 `SHIPPED` 且 `confirm_deadline` 已过的订单，改为 `COMPLETED`（`AUTO_CONFIRMED`）。不用延迟消息：10 天的精度不需要准到秒，数据库扫描就够了，也少一个会丢的东西。
- **售后**：以前只有 `PAID` 的订单能申请售后；现在 `SHIPPED`、`COMPLETED` 也可以（不然发货就等于取消了售后资格）。售后审批、退款，以及"有售后申请时能不能发货"，在任务 0018 里定。

## 范围外

- 物流轨迹查询、对接真实快递接口、拆单发货（一个订单多个包裹）。
- 发货、收货事件（`order.shipped` 等）：现在没有消费者，等需要时再加。
- 售后时限（比如完成后 7 天内）：任务 0018。

## 验收标准

- [x] `go test ./...`（44 个包，开启 `GOFRAMESHOP_IT=1`）、`go mod verify` 通过，`gofmt -l` 为空。
- [x] 逻辑测试：发货只对 `PAID` 生效，未付款、不存在的订单被拒；同运单号重复发货幂等、换运单号被拒；物流公司和运单号格式校验，被拒时订单不变；确认收货只对自己的 `SHIPPED` 订单生效、重复确认幂等、别人的订单 NotFound；自动确认在截止时间前一秒不动、到点完成、晚发货的订单不受影响；顾客确认与自动确认并发 50 轮（`-race`）每轮都恰好完成一次；`SHIPPED`、`COMPLETED` 可以申请售后，`CLOSED` 不行。
- [x] order 鉴权测试（真实 gRPC 链路）：`ShipOrder` 只接受管理员，`ConfirmReceipt` 只接受商城用户。
- [x] Compose 集成测试：两个并发发货只有一个生效；发货字段写入并读回；`Due` 在截止时间前一秒不包含、到点包含、完成后不再包含；完成只生效一次。
- [x] `016` 迁移连续执行两次无报错。
- [x] 端到端（本地启动 user、goods、order、paysim、admin、gateway-h5、gateway-admin，新建测试管理员 `it0017_admin`）：
  - 未发货时确认收货返回 `code:9`；用商城用户令牌调后台发货返回 `code:16`；运单号带空格返回 `code:51`。
  - 发货后 `confirmDeadline` 为 10 天后；同运单号重复发货返回同一结果；换运单号返回 `code:9`。
  - 前台确认收货后 `COMPLETED`、`USER_CONFIRMED`。
  - 第二个订单发货后，把 `confirm_deadline` 改到过去，恢复任务几秒内把它改为 `COMPLETED`、`AUTO_CONFIRMED`。
  - 按 `COMPLETED` 筛选得到这两个订单；已完成的订单可以申请售后。
