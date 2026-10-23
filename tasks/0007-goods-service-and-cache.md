# 任务 0007：goods 商品服务与 Redis 缓存

状态：已完成。来源是原教程的"goods 商品服务开发""商品服务_详情缓存"和"商品分类接口_缓存"三节。

> 后续变更：`goods_info.stock` / `sale` 已在[任务 0008](0008-inventory-and-shelf.md) 迁入库存账本，库存数字也不再进缓存；商品还新增了上下架状态。下文中关于 `stock`、`sale` 的描述保留为历史记录。开发说明见 [`docs/development/goods-service.md`](../docs/development/goods-service.md)。

## 新增了什么

| 类别 | 内容 |
| --- | --- |
| 进程 | `app/goods`（gRPC `:31005`，库 `goods`）：商品、商品图片、三级分类 |
| 基础设施 | Compose 新增 `redis:7.4.2`（`:6379`，有密码，不持久化）；`make infra-up` 一并启动 |
| 建表 | `deploy/mysql/init/005_goods_schema.sql`：`goods_info`、`goods_images`、`category_info` |
| 后台接口（gateway-admin，需管理员令牌） | `GET/POST/PUT/DELETE /backend/goods`、`GET /backend/goods/detail`、`POST/DELETE /backend/goods/image`、`GET/POST/PUT/DELETE /backend/category`、`GET /backend/category/all` |
| 前台接口（gateway-h5，公开） | `GET /frontend/goods`、`GET /frontend/goods/detail`、`GET /frontend/category`、`GET /frontend/category/all` |
| 缓存 | 商品详情（含图片）、全量分类树；写操作后删除缓存，删除失败时后台重试 |
| 共享代码 | `utility.RPCCode`（识别 grpcx 改写后的下游错误码）；`GrpcClientForwardAuth` 也支持服务间调用时转发令牌 |

## 已采用的决定

| 决定 | 原因 |
| --- | --- |
| 读接口公开，写接口只接受管理员令牌，由服务端拦截器默认拒绝 | 不靠每个 handler 自己检查身份 |
| 价格字段叫 `price_cents`，单位分，`BIGINT UNSIGNED` | 字段名直接说明单位；整数避免浮点误差 |
| `sale`（销量）不能由管理员填写 | 销量应来自订单，教程允许在创建、修改时随意设置 |
| 分类 `level` 由父分类推导，最多三级；修改分类不能改父分类 | 教程让客户端自己填 level，示例数据里出现了 level 62 这样的脏数据；移动分类会让商品上已存的分类链失效 |
| 商品的 level1 → level2 → level3 必须构成真实的父子链 | 教程不校验，可以把商品挂到不相关的分类组合下 |
| 删除分类时，若还有子分类或商品则拒绝（`code:9`） | 否则子分类和商品会挂在一个不存在的分类下 |
| 挂图片时，goods 服务带着管理员令牌去 file 服务确认文件存在，并把 URL 复制进 `goods_images.url` | 只有真实存在的文件能被引用；读商品时不需要再调用其他服务（这就是教程"读多写少"冗余 URL 的思路） |
| 不加 `goods_info.images` JSON 列 | 和 `goods_images` 表重复，两份数据会不一致；读性能交给缓存解决 |
| 同一文件不能重复挂到同一商品（唯一键） | 防止重复图片 |
| 不提供全局"商品图片列表"接口 | 教程的 `GoodsImages.GetList` 会把所有商品的图片混在一起分页，没有用处；图片随商品详情返回 |
| 缓存旁路：读先查缓存，未命中再查库并回填；写先写库，再删缓存 | 删除比覆盖简单，下一次读取自然拿到新数据 |
| 不存在的 ID 缓存 60 秒的"不存在"标记；**创建商品时也删除对应缓存键** | 防缓存穿透；教程没处理"某个 ID 先被查过、之后才被创建"的情况，新商品会被旧标记挡住 60 秒 |
| 同一商品的并发未命中只查一次库（singleflight） | 热门商品缓存过期时，不会让 MySQL 被同时打满 |
| TTL 加 ±10% 随机抖动 | 同一时刻写入的缓存不会在同一秒集中过期 |
| 每次 Redis 操作单独设 100ms 超时；Redis 出错时直接读 MySQL | 缓存只是优化，不能让它的故障变成整个服务的故障 |
| 删缓存失败时后台重试 5 次（约 30 秒） | 实测：Redis 暂停期间修改商品，删缓存失败，旧数据会一直留到 TTL 过期 |
| Redis 不持久化，并且 `/data` 挂成 tmpfs | 实测：镜像自带的匿名卷在重建容器后保留了旧快照，重启后又读回已失效的缓存 |
| 缓存层报错只保留操作名、键和根因 | 实测：GoFrame 的 Redis 报错会带上 SET 的完整参数，等于把缓存内容写进日志 |
| 缓存键带版本号：`goods:v1:detail:{id}`、`goods:v1:category:all` | 改缓存结构时升级版本号即可，不会把旧格式的数据解码成新结构 |
| 没配 Redis 时使用空缓存；配了但连不上时照常启动 | 教程在 Redis 连不上时直接让服务退出 |

## 范围外

- 库存预留、扣减和销量更新：属于结算链路，见[架构概览](../docs/architecture/overview.md)。
- 商品搜索、多条件筛选、排序方式选择。
- 商品上下架状态。
- 缓存一致性的强保证：删除缓存和并发读回填之间仍有很小的时间窗口，可能写回旧值，最长持续一个 TTL。另外，进程在后台重试完成前退出也会留下旧数据。彻底解决需要可靠消息（outbox 或 binlog 订阅），属于后续可靠性设计。
- 教程"resource 网关"一节里按 `uploader_type` 区分用户上传和管理员上传：目前只有管理员能上传。

## 验收标准

- 逻辑测试（内存 fake）：
  - 缓存命中时不读 MySQL；不存在的 ID 只读一次库。
  - 创建商品会清掉旧的"不存在"标记；修改、删除、加图、删图都会清掉缓存。
  - Redis 故障时读写都正常；50 个并发未命中只读一次库。
  - 分类链校验、销量不被覆盖、图片按 sort 排序、封面取第一张；分类层级推导和三级上限、分类不可移动、使用中的分类不能删除。
- 缓存测试（miniredis）：编解码往返、TTL 在 ±10% 范围内、"不存在"标记很快过期、损坏的数据当作未命中、Redis 挂掉时快速失败、错误信息不含缓存内容；失败的删除会在后台重试，且不受请求结束影响。
- 鉴权测试：读接口不需要令牌；8 个写接口在无令牌或商城用户令牌下返回 `Unauthenticated`，管理员令牌可以通过；嵌套的 `GoodsInput` 字段也会被校验。
- 本地端到端：
  - 后台建三级分类 → 上传图片 → 建商品 → 挂图 → 前台不带令牌浏览列表、详情、分类树。
  - 直接改 MySQL 后接口仍返回缓存值；后台修改后立即返回新值。
  - 挂不存在的文件返回 `code:3`。
  - Redis 停止或暂停期间读写正常，恢复后后台重试删掉了旧缓存。
- `go test ./...`、`go vet ./...`、`go mod verify`、`docker compose config` 通过；`gf gen pb` / `gf gen ctrl` 重新生成无差异；GoFrame 仍为 v2.10.2。
