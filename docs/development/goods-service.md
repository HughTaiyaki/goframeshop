# goods 商品服务开发

`app/goods` 负责商品、商品图片、三级分类、库存账本、上下架和购物车。它是一个 gRPC 服务，监听 `:31005`，使用 `goods` 库，前面有一层 Redis 缓存。任务、决定和验收见 [`tasks/0007-goods-service-and-cache.md`](../../tasks/0007-goods-service-and-cache.md)。

## 1. 谁能做什么

| 接口 | gateway-h5（前台） | gateway-admin（后台） | goods 服务端 |
| --- | --- | --- | --- |
| 商品列表、详情、分类列表、分类树 | 公开，只能看到上架商品 | 需要管理员令牌，也能看到下架商品和已预留数量 | `publicMethods` |
| 新增、修改、删除商品，挂图、删图，分类增删改，上下架，调整库存 | 不存在 | 需要管理员令牌 | 默认拒绝，只认管理员公钥 |
| 库存预留、提交、释放、查询 | 不存在 | 不存在（用 `scripts/inventory` 调用） | 管理员令牌，或订单服务的服务令牌（`jwtOrder` 公钥，见 [order-service.md](order-service.md#5-服务身份第三对密钥)） |
| 购物车 | 需要登录，只能操作自己的 | 不存在 | 只接受商城用户令牌（`jwtUser` 公钥），管理员和服务令牌返回 PermissionDenied |

goods 服务的 `jwt` 一节配置的是**管理员**公钥。商城用户访问读接口时，网关会转发他们的令牌，但读接口是公开的，令牌验不过也不影响。

## 2. 结构

```text
manifest/protobuf/{goods,category}/v1/*.proto   协议（手写）
internal/controller/{goods,category}/           薄 gRPC 层
internal/service/                               接口、领域类型、错误；Cache 接口也在这里
internal/logic/{goods,category}/                业务规则与缓存旁路
internal/repository/                            MySQL
internal/cache/                                 Redis 实现、空缓存、失败重试
internal/filelookup/                            调用 file 服务确认文件并取 URL
internal/testfake/                              逻辑测试共用的内存 fake
```

## 3. 数据规则

- **价格**：`price_cents`，单位分，必须大于 0。19.90 元填 `1990`。
- **库存和销量**：都在 `inventory` 表里，见[第 7 节](#7-库存与上下架)。后台表单不能填写；创建商品时给初始库存，之后只能通过"调整库存"接口增减。
- **分类链**：`level1CategoryId` 必填，且必须是一级分类。二级、三级可以不填（填 0）；如果填了，必须是上一级的子分类。不能只填三级而不填二级。
- **分类**：
  - 层级由父分类推导，最多三级。
  - 只能改名称、图标和排序，不能改父分类。
  - 还有子分类或商品时不能删除，返回 `code:9`。
- **图片**：先用 `POST /backend/upload/image` 上传拿到 `fileId`，再用 `POST /backend/goods/image` 挂到商品上。
  - goods 服务会带着同一个管理员令牌去 file 服务确认文件存在，并把 URL 复制进 `goods_images`。
  - 文件不存在返回 `code:3`；同一文件重复挂到同一商品返回 `code:6`。
  - 图片按 `sort` 升序排列，列表里的封面取排第一的那张。
- **推荐权重**：`goods_info.sort`，0–9999，0 表示不是热门，见[第 8 节](#8-热门推荐)。和图片的 `sort` 是两回事。

## 4. 缓存

```text
读详情：Redis 命中 → 直接返回
       "不存在"标记 → NotFound，不查库
       未命中 → MySQL（同一 ID 同时只查一次）→ 回填 Redis → 返回
写：    MySQL 成功 → 删除 Redis 中的键（失败则后台重试 5 次）
```

| 键 | 内容 | TTL |
| --- | --- | --- |
| `goods:v2:detail:{id}` | 商品及其全部图片、上下架状态、推荐权重（**不含库存数字**） | 1 小时 ±10% |
| `goods:v2:detail:{id}` = `__absent__` | 该 ID 不存在 | 60 秒 |
| `goods:v1:category:all` | 全部分类（扁平列表） | 24 小时 ±10% |

会删除缓存的操作：

| 写操作 | 删除的键 |
| --- | --- |
| 创建商品 | 新 ID 的详情键（清掉可能存在的"不存在"标记） |
| 修改、删除、上下架商品，加图、删图 | 该商品的详情键 |
| 创建、修改、删除分类 | 分类树键 |

库存数字**不缓存**：每次预留都会改变它，放进缓存就得每单都清一次缓存。读取详情时按主键实时查询 `inventory`；共享的缓存对象会先复制一份，再填入库存数字，不会被改动。

商品列表和按父分类分页的列表**不缓存**：它们的组合太多，删除时很难覆盖全，数据量也不大。

故障行为：
- 每次 Redis 调用最多等 100ms，出错就读 MySQL，并记一条 WARN 日志。
- 没配 `redis.default` 时使用空缓存，所有读都直接查库。
- 配了但连不上时，服务照常启动。

已知局限：先删缓存、后有一个并发读把查到的旧值写回去，这种情况仍可能发生，最长持续一个 TTL。另外，进程在后台重试完成前退出，也会留下旧数据。要彻底解决需要可靠消息（outbox 或订阅 binlog）。

## 5. 本地运行

```bash
make infra-up       # 现在包含 redis
make db-init        # 已有数据卷时补建 goods 库
cd app/goods
cp manifest/config/config.yaml.example manifest/config/config.yaml
# 把管理员密钥对的 JWT_PUBLIC_KEY 填进 jwt.publicKey（和 file、gateway-admin 用的是同一个）
go run .
```

配置里还要有 `rabbitmq.url`：改名、上下架、删除商品时，goods 在同一个事务里写一条 `goods.changed` 事件，由投递器发给 RabbitMQ（banner 用它同步手工位）。RabbitMQ 不可用时这些写操作照常成功，事件留在 `goods.outbox_event` 里稍后补发。见 [`events.md`](events.md#52-商品变更goodschanged)。

goods 服务依赖 file 服务（挂图片、设置分类图标时会调用它），所以后台演示需要 admin、file、goods 和 gateway-admin 四个进程一起运行，见 [`tutorial/03-自己跑一遍.md`](../tutorial/03-自己跑一遍.md#商品与分类)。

查看缓存：

```bash
docker compose exec redis redis-cli -a change-me --no-auth-warning -n 1 KEYS 'goods:*'
```

## 6. 踩过的坑

- **GoFrame 的事务选项**：`TransactionWithOptions` 必须显式设置 `Propagation`（比如 `gdb.PropagationRequired`），零值会报 `unsupported propagation behavior`。这是集成测试第一次运行时发现的，内存 fake 测不出来。
- **`gf gen pb` 会给新 RPC 生成"未实现"占位方法**：如果控制器里已经手写了实现，要删掉这些占位方法，否则会因方法重复定义而编译失败。
- **grpcx 客户端会改写错误**：下游返回的 gRPC 状态会被改成带同样数字码的 `gerror`，`status.Code(err)` 只能读到 `Unknown`。判断下游错误码要用 `utility.RPCCode(err)`。
- **在 zsh 脚本里不要用 `GID` 当变量名**：它是只读的系统变量，赋值会报 `failed to change group ID`。
- **Redis 镜像的匿名卷**：`redis` 镜像声明了 `VOLUME /data`，Compose 重建容器时会保留这个卷，Redis 启动时又会加载里面的旧快照。所以 Compose 里把 `/data` 挂成了 tmpfs。

## 7. 库存与上下架

任务和完整规则见 [`tasks/0008-inventory-and-shelf.md`](../../tasks/0008-inventory-and-shelf.md)，语义来自详细设计 5.2–7.2 节。

```text
inventory(goods_id, available, reserved, sold)
Reserve：available → reserved   Commit：reserved → sold   Release：reserved → available
```

- **结构**：业务规则在 `internal/logic/inventory`，事务和加锁在 `internal/repository/ledger.go`。
- **加锁**：每个事务都先按 goods_id 顺序锁库存行，再锁预留行；隔离级别为读已提交，遇到死锁自动重试。
- **幂等**：调用方给出稳定的 `operation_id`（按订单派生，比如 `reserve-<order_id>`）。结果和库存变动在同一个事务里提交，重试会返回最初的结果。
- **业务拒绝**（`INSUFFICIENT_STOCK`、`GOODS_UNAVAILABLE`、`ALREADY_RELEASED`、`STATE_CONFLICT`）作为结果返回，不是 RPC 错误；`NOT_READY` 不会被记录。
- **上下架**：新商品默认下架，用 `PUT /backend/goods/shelf` 上架。下架后前台查不到，也不能再被预留。
- **调整库存**：`POST /backend/goods/stock` 传 `delta`，正数入库、负数出库，只改可用库存，不会变成负数。

本地手动调用库存接口：

```bash
export ADMIN_TOKEN=...      # 后台登录拿到的令牌
go run ./scripts/inventory reserve -order o1 2:1      # 商品 2 预留 1 件
go run ./scripts/inventory commit  -order o1
go run ./scripts/inventory release -order o2 2:1      # 先释放：留下墓碑
go run ./scripts/inventory reserve -order o2 2:1      # 迟到的预留：ALREADY_RELEASED
go run ./scripts/inventory query   -op reserve-o1
```

测试：

```bash
go test ./app/goods/...
GOFRAMESHOP_IT=1 go test ./app/goods/internal/repository/ -run Integration -v   # 真实 MySQL 并发
```

集成测试会创建自己的商品和订单，结束时只删除这些数据。

## 8. 热门推荐

任务见 [`tasks/0015-frontend-api-and-hot-goods.md`](../../tasks/0015-frontend-api-and-hot-goods.md)。

- `goods_info.sort` 是运营填写的推荐权重：**0（默认）不是热门；大于 0 进入热门列表，越大越靠前**，相同时新商品在前。
- 前台：`GET /frontend/goods?isHot=1`，只有上架商品；可以和 `categoryId` 一起用。前台不返回权重本身。
- 后台：创建、修改商品时填 `sort`；`GET /backend/goods?isHot=1` 筛选；列表和详情返回 `sort`。修改商品是整体替换，**不填 `sort` 就等于 0**。
- gRPC：`ListGoodsRequest.hot_only`。仓库层是 `service.GoodsFilter{CategoryID, Shelf, HotOnly}`，热门时 `WHERE sort > 0 ORDER BY sort DESC, id DESC`，用索引 `idx_goods_hot (status, sort)`。
- 普通列表的顺序不变（新的在前）。热门列表和普通列表一样**不缓存**；详情缓存里带着 `sort`，修改商品后照常删除。
- search 不使用权重：`docQuery` 按列名查询，新增列不影响它。

## 9. 购物车

任务见 [`tasks/0016-cart.md`](../../tasks/0016-cart.md)。

| HTTP（gateway-h5，需要登录） | gRPC（`cart/v1.CartService`） | 规则 |
| --- | --- | --- |
| `GET /frontend/cart?page=&size=` | `ListCartItems` | 最新加入的在前；每行带商品当前的名称、价格、封面、可用库存、`available` |
| `POST /frontend/cart` `{goodsId, quantity}` | `AddCartItem` | 只能加购上架商品；已有时数量累加；累加后超过 99 返回 `code:9`，数量不变；每人最多 100 行 |
| `PUT /frontend/cart` `{cartItemId, quantity}` | `UpdateCartItem` | 数量 1–99；不是自己的返回 `code:5` |
| `DELETE /frontend/cart` `{cartItemIds:[...]}` | `DeleteCartItems` | 最多 100 个；不是自己的跳过；返回实际删除行数 |

- **身份**：goods 的 `jwtUser` 一节配置商城用户公钥，`cartMethods` 只允许 `RoleUser`。请求里没有用户 ID，`utility.UserIDFromContext` 是唯一来源；每条 UPDATE、DELETE 都带 `user_id` 条件。
- **加购是一条语句**：`INSERT ... AS new ON DUPLICATE KEY UPDATE`，靠 `(user_id, goods_id)` 唯一键，并发加购也只有一行。超过上限时两列都保持原值，影响行数为 0，返回 `ErrCartLimit`。注意赋值顺序：MySQL 从左到右执行，`updated_at` 必须写在 `quantity` 前面，判断时用的才是旧数量。
- **只存商品和数量**：名称、价格、库存每次列表时批量查（商品、封面、库存各一次查询），所以总是当前值。下单时订单服务会重新计价。
- **下架或删除的商品**留在购物车里，`available=false`，方便顾客自己删除；已删除的商品没有名称和价格。
- **下单后移除**：前端从购物车结算时下单带 `fromCart: true`。订单库存预留成功后，order 写入 `order.placed`，goods 的 `events.Cart` 消费，见 [events.md](events.md#53-下单成功orderplaced)。
- **时间精度**：`created_at`、`updated_at` 由服务写入，先截断到毫秒（列是 `DATETIME(3)`），避免 MySQL 四舍五入后比实际时间晚，影响和下单时间的比较。

