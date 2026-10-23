# 任务 0008：库存预留与扣减、商品上下架

状态：已完成。库存部分按[详细设计](../docs/GoFrame微服务商城详细设计.md) 5.2–7.2 节实现，开发说明见 [`docs/development/goods-service.md`](../docs/development/goods-service.md#7-库存与上下架)。

## 新增了什么

| 类别 | 内容 |
| --- | --- |
| 迁移 | `006_inventory_and_shelf.sql`：新增 `inventory`（可用 / 预留 / 已售）、`reservation`、`inventory_operation` 三张表；`goods_info` 新增 `status`，删除 `stock`、`sale`（原数据迁入 `inventory`）。可重复执行，已有数据可以安全迁移 |
| gRPC | `InventoryService`：`ReserveInventory`、`CommitInventory`、`ReleaseInventory`、`QueryInventoryOperation`；`GoodsService` 新增 `SetGoodsShelf`、`AdjustStock` |
| 后台 HTTP | `PUT /backend/goods/shelf`、`POST /backend/goods/stock`；创建商品时填 `initialStock`；列表可按上下架筛选；详情显示 `onShelf`、`reserved` |
| 工具 | `scripts/inventory`：带管理员令牌直接调用库存接口，用于演示和排查 |
| 测试 | 详细设计 6.3 节状态表的每一格都有用例；另有连接 Compose MySQL 的并发集成测试（`GOFRAMESHOP_IT=1`） |

## 规则

```text
Reserve：available → reserved     Commit：reserved → sold
Release：reserved → available     已售库存永远不会被 Release 退回
```

| 当前状态 | Reserve（同一订单） | Commit | Release |
| --- | --- | --- | --- |
| 不存在 | 预留 | `NOT_READY`（不记录，可以用同一 ID 重试） | 写 RELEASED 墓碑，不加库存 |
| RESERVED | 不变 | → COMMITTED | → RELEASED，归还一次 |
| COMMITTED | 不变 | 不变 | `STATE_CONFLICT` |
| RELEASED | `ALREADY_RELEASED` | `STATE_CONFLICT` | 不变 |

- **幂等**：每次调用都带调用方给出的稳定 `operation_id`。操作结果和库存变动在同一个事务里提交。重试会返回最初的结果，不会重复执行；同一个 ID 带不同参数（按请求内容算出的 sha256 不同）返回 `AlreadyExists`。
- **全有或全无**：先检查所有商品，有任何一项不满足就什么都不改。库存不足、商品已下架这类业务拒绝也会被记录，作为结果返回，而不是 RPC 错误。
- **加锁顺序**：每个事务都先按 goods_id 顺序锁住库存行，再锁预留行。事务使用读已提交（READ COMMITTED）隔离级别：在可重复读级别下，查询还不存在的预留行会加间隙锁，相邻插入会互相死锁。偶发的死锁会把整个事务重试，最多 3 次。
- **上下架**：新商品默认下架。下架后前台查不到（和删除一样返回 NotFound），也不能再被预留；已经存在的预留仍然可以提交或释放。
- **库存展示**：库存数字不进 Redis 缓存，每次读取时按主键查询。否则每次预留都要清缓存，缓存就失去了意义。
- **管理员调整库存**：只改可用库存，用一条带条件的 UPDATE 完成，不会让库存变成负数，也不会和预留互相覆盖。

## 与原教程的差异

教程只有 `goods_info.stock` 一个字段，由后台表单直接覆盖，也没有上下架。这里把"能卖多少"交给库存账本，只有预留、提交、释放这几个事务能修改它；后台只能做增减调整。

## 范围外

- 订单服务以及订单的超时关单、服务身份：已在[任务 0010](0010-order-service.md) 实现。
- 幂等记录和墓碑的保留期清理（设计要求保留 30 天）。
- SKU（规格）：目前一个商品就是一个库存单位。

## 验收标准

- 逻辑测试（内存 fake）：
  - 状态表全部 12 种组合；全有或全无；重试返回原始结果，包括被拒绝的结果；同一 ID 不同参数被拒绝。
  - 竞争同一操作 ID 失败时，返回胜出方的结果。
  - 未就绪的提交不记录；空的商品列表被拒绝。
- 商品逻辑测试：新商品默认下架，前台看不到；上下架会清缓存；库存实时读取，不会写进共享缓存；管理员修改商品不影响库存；调整库存不会让库存变成负数。
- 集成测试（真实 MySQL，`-race` 下连跑 5 遍）：
  - 40 个并发买家抢 10 件库存，正好 10 个成功，没有超卖。
  - 20 个并发请求使用同一个操作 ID，只预留一次。
  - 多商品订单以不同顺序并发预留和释放，库存守恒，账本和预留记录一致。
- 端到端：用 `scripts/inventory` 走一遍预留、重试、提交、提交后释放、先释放后预留、下架后预留，结果全部符合上表。
