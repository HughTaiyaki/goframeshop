# 变更记录

每完成一个任务，在最上面加一节：新增了什么、改了什么、有哪些需要手动执行的步骤。细节和决定见对应的任务文件。

## 0017 · 发货与收货

[任务文件](../tasks/0017-shipping-and-receipt.md) · [开发说明](development/order-service.md#11-发货与收货)

- **订单新增两个状态**：`PAID ──发货──▶ SHIPPED ──确认收货 / 到期自动确认──▶ COMPLETED`。
- **新增接口**
  - 后台：`POST /backend/order/ship`（物流公司、运单号）。同一运单号重复提交直接返回；换运单号返回 `code:9`。
  - 前台：`POST /frontend/order/confirm` 确认收货。
  - order 服务：`ShipOrder`（只接受管理员）、`ConfirmReceipt`（只接受商城用户）。
  - 订单视图新增 `carrier`、`trackingNo`、`shippedAt`、`confirmDeadline`、`completedAt`、`completeReason`；前后台列表可以按 `SHIPPED`、`COMPLETED` 筛选。
- **自动确认收货**：恢复任务把超过 `confirm_deadline` 的已发货订单改为已完成。新增配置 `order.autoConfirm`，默认 `240h`（10 天）。
- **行为变化**：已发货、已完成的订单也可以申请售后（以前只有 `PAID`）。
- **新增列** `orders.order_info` 的 `carrier`、`tracking_no`、`shipped_at`、`confirm_deadline`、`completed_at`、`complete_reason`，索引 `idx_order_confirm`。
- **需要手动执行**
  - `make db-init`（016）。
  - 需要时在 `app/order` 的配置里加 `order.autoConfirm`（不加就是 10 天）。
  - 重启 order、gateway-h5、gateway-admin。

## 0016 · 购物车

[任务文件](../tasks/0016-cart.md) · [开发说明](development/goods-service.md#9-购物车)

- **新增接口**
  - 前台（需要登录）：`GET/POST/PUT/DELETE /frontend/cart`（列表、加购、改数量、批量删除）。
  - 下单接口新增 `fromCart`：从购物车结算时填 `true`。
  - goods 服务：`cart/v1.CartService`，只接受商城用户令牌。
- **新增事件** `order.placed`：订单库存预留成功时写入 outbox；goods 消费，从购物车里减去已购数量（只处理 `fromCart` 的订单，下单之后改过的行不动）。新增队列 `goods.cart-order-placed`。
- **新增表和列** `goods.cart_item`（`(user_id, goods_id)` 唯一）；`orders.order_info.from_cart`。订单的 `created_at` 改为由订单服务写入，和事件里的下单时间一致。
- **修复：两种以上商品的订单一直卡在"预留中"**。goods 库存接口的校验规则把商品列表当成 JSON 文本量长度，两行就超过 50 个字符被拒绝；订单服务当成"结果未知"一直重试，直到超时关单。见任务文件。
- **需要手动执行**
  - `make db-init`（015）。
  - `app/goods` 的配置里加上 `jwtUser` 一节，填**商城用户**的公钥（和 gateway-h5 的 `jwt.publicKey` 相同，见 `config.yaml.example`）。不加的话购物车不可用，其他功能照常。
  - 重启 goods、order、gateway-h5。

## 0015 · 前台接口补齐与热门推荐

[任务文件](../tasks/0015-frontend-api-and-hot-goods.md) · [开发说明](development/goods-service.md#8-热门推荐)

- **新增接口**
  - 前台（需要登录）：`PUT /frontend/user/password` 修改密码（要当前密码），`PUT /frontend/user/profile` 修改头像、性别、签名。
  - 前台（公开）：`GET /frontend/goods?isHot=1` 热门推荐，按推荐权重从高到低。
  - 后台：创建、修改商品可以填 `sort`（0–9999，0 表示不是热门）；商品列表可以按 `isHot` 筛选。
  - user 服务：`UpdateProfile`。
- **新增列** `goods.goods_info.sort` 和索引 `idx_goods_hot`。
- **行为变化**
  - 修改商品是整体替换，**不填 `sort` 就等于 0**，商品会退出热门列表。
  - 注册时也检查头像（空或 http/https 绝对地址）、性别（0–2）、签名（最多 255 字）。之前不合法的值能写进库。
  - 商品详情缓存键改为 `goods:v2:detail:{id}`，旧键 1 小时内自然过期。
- **教程中已经完成的部分**：分类接口、`pic_url`（已有 `coverUrl`）、从令牌取用户 ID，这次不用再做。
- **需要手动执行**
  - `make db-init`（014）。
  - 重启 user、goods、gateway-h5、gateway-admin。

## 0014 · 链路追踪（OpenTelemetry + Jaeger）

[任务文件](../tasks/0014-tracing.md) · [开发说明](development/tracing.md)

- **新增基础设施** OpenTelemetry Collector（`:4317`）和 Jaeger（界面 <http://127.0.0.1:16686>），`make infra-up` 一起启动。
- **全部 11 个进程接入**：HTTP、gRPC、SQL、Redis 由 GoFrame 自动记录；本仓库补上 RabbitMQ 事件（发布、消费，延迟事件用 Span Link 连回下单）、后台任务（每个订单、每笔退款、每次回调一条链路）、支付回调（模拟器 → 网关 → 订单 → 模拟器）。
- **后台轮询不再产生噪音**：轮询在不采样的上下文里运行，空转时一条链路都不产生。修复了任务 0012 留下的"后台任务共用一个链路 ID"。
- **脱敏**：请求头和 gRPC 元数据里的令牌、签名，带值的 SQL（姓名、手机号、地址），Redis 缓存内容，SQL 错误详情，都在离开进程前去掉；Collector 再过一遍。
- **日志关联**：日志行里 `{...}` 的链路 ID 可以直接在 Jaeger 打开。
- **依赖升级** `google.golang.org/grpc` v1.64.1 → v1.75.0、`protobuf` → v1.36.8（OTLP 导出器 v1.38.0 需要）。GoFrame 仍为 v2.10.2，全部测试通过。
- **新增列** 三个库的 `outbox_event.traceparent`。
- **需要手动执行**
  - `make infra-up`（拉取 Collector 和 Jaeger 镜像）。
  - `make db-init`（013）。
  - 每个服务的配置里加上 `otel` 一节（见各自的 `config.yaml.example`；不加就是关闭追踪，服务照常运行）。
  - 重启所有服务。

## 0013 · RabbitMQ 与 Outbox / Inbox

[任务文件](../tasks/0013-rabbitmq-outbox-inbox.md) · [开发说明](development/events.md)

- **新增基础设施** RabbitMQ 4.2.0（`:5672`，管理界面 `:15672`），`make infra-up` 一起启动。
- **事件不会丢**：事件和业务修改写在同一个事务里（`outbox_event`），由投递器（Relay）发出，收到 RabbitMQ 确认才算发出；RabbitMQ 挂了就留在表里，恢复后补发。
- **重复只生效一次**：消费方在同一个事务里记录 `inbox_event`，同一个事件第二次到达只确认、不执行。坏消息进 `quarantine_event`，不堵队列。
- **订单准时关单**：订单进入"待支付"时写一条延迟事件，到期那一刻订单服务收到并关单、关闭支付单、释放库存。数据库扫描仍然保留，作为兜底。
- **手工位跟随商品**：goods 改名、上下架、删除时发 `goods.changed`；banner 自动更新手工位的商品名，商品不在售时前台不再显示（修复任务 0011 的已知局限）。后台手工位列表多了 `goodsOnShelf`。
- **共享代码** `utility/rabbitmq`（拓扑、带确认的发布、消费循环）、`utility/eventbus`（Outbox、Relay、Inbox）。
- **新增表** `orders`、`goods`、`banner` 各一套 `outbox_event`、`inbox_event`、`quarantine_event`；`banner.position_info` 新增 `goods_on_shelf`、`goods_version`。
- **需要手动执行**
  - `make infra-up`（拉取 RabbitMQ 镜像）。
  - `make db-init`（012）。
  - 在 `app/order`、`app/goods`、`app/banner` 的配置里加上 `rabbitmq.url`（见各自的 `config.yaml.example`），order 和 goods 再加 `outbox.interval`。
  - 重启 order、goods、banner、gateway-h5、gateway-admin。

## 0012 · 支付模拟器与订单支付

[任务文件](../tasks/0012-payment-simulator.md) · [开发说明](development/payment.md)

- **新增进程** `app/paysim` 支付模拟器：gRPC :31009（商户接口，只给订单服务用），HTTP :8399（付款人的收银台页面），库 `paysim`。
- **订单能付款了**：`PENDING_PAYMENT` → `PAYMENT_CONFIRMING`（提交库存）→ `PAID`。新增状态 `EXCEPTION`：付了款但库存提交不了，自动退款后关单。
- **新增接口**
  - 前台：`POST /frontend/order/pay`（去支付，返回收银台地址；已付款时直接确认）。
  - 前台（公开，靠签名）：`POST /frontend/payment/notify`，模拟器的回调入口。
  - 订单详情多了 `paidAt`；前后台订单列表可以按新状态筛选。
- **规则**
  - 回调必须验签，而且只当提示：订单服务会去模拟器查询，核对订单号、金额、币种后才记账。
  - 付款和关单同时发生时只有一个成功；钱到了但订单已经在关闭，就自动退款，退且只退一次。
  - 关单先关闭支付单，模拟器联系不上就先不还库存。
  - 库存在付款后才从"预留"变成"已售"。
- **售后申请可以用了**（已支付的订单）。审批和按商品退款还没做。
- **新增身份** 第四对密钥：模拟器给回调签名，订单服务验签。模拟器的商户接口只接受订单服务的服务令牌。
- **新增表** `paysim.payment`、`paysim.refund`；`orders.payment`、`orders.tech_refund`；`orders.order_info.paid_at`。
- **共享代码** `utility/paysign.go`（回调签名与验签）。
- **需要手动执行**
  - `make db-init`（010、011）。
  - `make jwt-keys` 再生成一对：私钥填进 `app/paysim` 的 `notify.privateKey`，公钥填进 `app/order` 的 `paysim.publicKey`。
  - 准备 `app/paysim` 的配置：`jwtOrder.publicKey` 填订单服务那对的公钥（和 goods 的 `jwtOrder` 相同）。
  - 启动 `app/paysim`，重启 `app/order`、`app/gateway-h5`、`app/gateway-admin`。

## 0011 · banner 服务（轮播图、手工位）

[任务文件](../tasks/0011-banner-service.md) · [开发说明](development/banner-service.md)

- **新增进程** `app/banner`（gRPC :31008，库 `banner`）。
- **新增接口**
  - 后台：轮播图、手工位的增删改查。
  - 前台（公开）：轮播图列表、手工位列表。
- **规则**：图片用 `fileId` 引用 file 服务里的文件；链接只接受 http/https（拒绝 `javascript:` 等）；手工位的商品名从商品服务读取；固定按 `sort` 升序。
- **没有照做的部分** 教程同一章的 RabbitMQ 延迟消息关单没有实现：它会丢消息（详细设计 B1），而订单超时关单已经由任务 0010 的恢复任务完成。RabbitMQ 将在支付那一轮按 Outbox/Inbox 方式引入。
- **需要手动执行**
  - `make db-init`（009）。
  - 准备 `app/banner` 的配置（填管理员公钥）。

## 0010 · order 订单服务（下单、库存闭环、售后申请）

[任务文件](../tasks/0010-order-service.md) · [开发说明](development/order-service.md)

- **新增进程** `app/order`（gRPC :31007，库 `orders`）。
- **下单**：价格和地址由服务端查询，不信任客户端；`requestId` 防止重复下单；下单时预留库存，进入"待支付"。
- **关单**：顾客可以取消；15 分钟未支付由后台任务自动关单；两种情况都会释放库存，并且只释放一次。
- **恢复任务**：库存调用超时、服务重启都不会丢订单，后台任务用同一个操作号继续处理。
- **售后**：只能对自己的已支付订单申请。支付下一轮才做，所以现在会返回 `code:9`。
- **新增接口**
  - 前台：下单、取消、我的订单、售后申请与查询。
  - 后台：全部订单、全部售后。
  - user 服务：`GetConsignee`。
- **新增身份** 第三对密钥：订单服务的服务身份。goods 的库存接口只接受管理员令牌和订单服务的服务令牌。
- **共享代码** `AuthPolicy`（多种身份、按接口授权），以及 `ServiceToken`、`GrpcClientTimeoutOf`、`LoadTokenVerifierAt`、`LoadTokenSignerAt`。
- **行为修正** 网关返回的时间改为服务器本地时间。之前是 UTC 却没有标出时区，在东八区看起来晚了 8 小时。
- **需要手动执行**
  - `make db-init`（008）。
  - `make jwt-keys` 再生成一对：私钥填进 `app/order` 的 `jwtService`，公钥填进 `app/goods` 的 `jwtOrder`，然后重启 goods。
  - 准备 `app/order` 的配置（用户公钥、管理员公钥、服务私钥）。

## 0009 · search 搜索服务（Elasticsearch + IK + binlog）

[任务文件](../tasks/0009-search-service.md) · [开发说明](development/search-service.md)

- **新增进程** `app/search`（gRPC :31006）：公开、只读、不持有密钥。
- **新增基础设施** Elasticsearch 8.11.0，镜像构建时装好 IK 分词器（锁定 sha256）；Kibana 可选（`--profile tools`，中文界面）。
- **新增接口** 前台 `GET /frontend/search/goods`：关键词、分类、品牌、价格、5 种排序，结果带安全的高亮。
- **同步**：启动时全量重建（别名原子切换），之后跟随 binlog；binlog 只用来通知哪个商品变了，再按列名回查 MySQL；ES 故障时一直重试，不丢变更。
- **新增账号** `search_sync`（只能读 goods 库和 binlog）。
- **共享代码** `UnaryServerAuth` 支持不配置验签器（只放行公开方法）。
- **需要手动执行**
  - `make infra-up`（首次会构建 ES 镜像，需要联网）。
  - `make db-init`（007）。
  - 给 `app/search` 准备配置。

## 0008 · 库存预留与扣减、商品上下架

[任务文件](../tasks/0008-inventory-and-shelf.md) · [开发说明](development/goods-service.md#7-库存与上下架)

- **新增表** `inventory`、`reservation`、`inventory_operation`；`goods_info` 增加 `status`，`stock`、`sale` 迁入库存账本后删除（迁移可以重复执行）。
- **新增 gRPC** `InventoryService`（预留、提交、释放、查询，带幂等记录和墓碑）；`SetGoodsShelf`、`AdjustStock`。
- **后台接口** 上下架、调整库存；创建商品时填初始库存；列表可按上下架筛选。
- **行为变化**
  - 新商品默认下架。
  - 前台看不到下架商品。
  - 修改商品时不能再改库存。
  - 库存数字不进缓存。
- **新增工具** `scripts/inventory`。
- **新增测试** 连接 Compose MySQL 的并发集成测试（`GOFRAMESHOP_IT=1`）。
- **需要手动执行** `make db-init`（006，会迁移已有的库存数据）。

## 0007 · goods 商品服务与 Redis 缓存

[任务文件](../tasks/0007-goods-service-and-cache.md) · [开发说明](development/goods-service.md)

- **新增进程** `app/goods`（gRPC `:31005`，库 `goods`）：商品、商品图片、三级分类。
- **新增基础设施** Redis 7.4.2（`:6379`），只做缓存：有密码、不持久化、`/data` 挂 tmpfs。
- **新增接口**
  - 后台：商品增删改查、挂图删图、分类增删改查、全部分类。
  - 前台（公开）：商品列表、商品详情、分类列表、全部分类。
- **缓存**：商品详情和分类树使用缓存旁路；缓存不存在的 ID；同一 ID 的并发未命中只查一次库；TTL 加抖动；写后删除缓存，失败时后台重试；Redis 故障时回退到 MySQL。
- **共享代码**：`utility.RPCCode`；`GrpcClientForwardAuth` 也能转发服务间调用收到的令牌。
- **需要手动执行**
  - `make infra-up`（启动 Redis）。
  - `make db-init`（已有数据卷时补建 goods 库）。
  - 给 `app/goods` 准备配置，填**管理员**公钥。

## 0006 · admin、file 服务与图片上传

[任务文件](../tasks/0006-admin-file-upload.md) · [开发说明](development/admin-and-file.md)

- **新增进程** `app/admin`（:31003）、`app/file`（:31004）、`app/gateway-admin`（:8299）。
- **两套身份**：管理员使用第二对 Ed25519 密钥；没有公开注册，用 `create-admin` 命令创建超级管理员。
- **图片上传**：按文件内容判断类型；对象 key 随机生成；七牛和本地目录两种存储可切换；记录失败时删除已上传的对象。
- **共享代码**：密码哈希移到 `utility/password.go`；`utility.IsDuplicateKey`。
- **需要手动执行**
  - `make jwt-keys`（再生成一对，给管理员用）。
  - `make db-init`。
  - 启用七牛时，用你自己的凭据。

## 0005 · interaction 服务

[任务文件](../tasks/0005-interaction-service.md) · [开发说明](development/interaction-service.md)

- **新增进程** `app/interaction`（:31002）：点赞、收藏、评论；评论列表公开。
- **新增** `make db-init`，用于补执行建表脚本。

## 0004 · JWT 改为 Ed25519，网关鉴权中间件

[任务文件](../tasks/0004-eddsa-and-gateway-auth.md)

- 签名改为非对称：只有签发方持有私钥。
- 网关中间件默认拒绝，只放行白名单路由；令牌自动转发给后端，后端再验一次。
- **新增** `make jwt-keys`。

## 0003 · JWT 签发与 gRPC 鉴权拦截器

[任务文件](../tasks/0003-jwt-auth.md)

- 登录签发访问令牌；删除临时的 `X-User-Id`。

## 0002 · 用户服务持久化与 H5 网关

[任务文件](../tasks/0002-user-persistence-and-gateway.md)

- 用户与收货地址写入 MySQL，密码用 Argon2id；新增 gateway-h5（:8199）和 etcd 服务发现。

## 0001 · 用户 gRPC 协议

[任务文件](../tasks/0001-user-grpc-contract.md) · [报告](reports/0001-user-grpc-contract.md)

- 定义 `user/v1`、`consignee/v1` 协议并生成代码。
