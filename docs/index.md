# 文档索引

本项目采用渐进式披露：先从本页开始，只读取当前修改领域对应的文档。

## 当前状态

仓库具备 GoFrame MonoRepo、本地 MySQL/etcd 编排、可重复执行的建库脚本，以及三个已落地的进程：

- `app/user`（gRPC `:31001`，库 `user`）：注册、登录、查询和修改资料、改密码、收货地址 CRUD。密码用 Argon2id 哈希。它是**唯一持有 JWT 私钥**、能签发令牌的服务。
- `app/interaction`（gRPC `:31002`，库 `interaction`）：点赞、收藏、评论。只持有公钥；除评论列表外都需要登录。
- `app/gateway-h5`（HTTP `:8199`）：把 HTTP/JSON 翻译成 gRPC，经 etcd 发现后端，带 3 秒超时和 CORS。用公钥验签，默认拒绝，只放行白名单路由，令牌原样转发给后端再验一次。

后台另有三个进程，使用**另一对**密钥，商城用户的令牌在后台无效：

- `app/admin`（gRPC `:31003`，库 `admin`）：管理员登录、创建管理员。没有公开注册，第一个超级管理员用命令行创建。
- `app/file`（gRPC `:31004`，库 `file`）：记录上传文件的元数据，其他表通过 `file_id` 引用文件。
- `app/gateway-admin`（HTTP `:8299`）：后台入口。校验图片后写入七牛或本地目录，再交给 file 服务记录。

商品目录：

- `app/goods`（gRPC `:31005`，库 `goods`，Redis 缓存）：商品、商品图片、三级分类、热门推荐（`sort` 权重）、购物车（只接受商城用户令牌）。读接口公开，经 gateway-h5 开放给顾客；写接口只接受管理员令牌，经 gateway-admin 开放。商品详情和分类树走 Redis 缓存（缓存旁路，写后删除，删除失败时后台重试）。
- 库存账本（goods 服务内）：预留、提交、释放，幂等、全有或全无、并发安全；商品可以上下架，新商品默认下架。
- `app/search`（gRPC `:31006`）：Elasticsearch + IK 中文分词的商品搜索，经 gateway-h5 公开。启动时全量重建索引，之后通过 binlog 跟随变化。
- `app/order`（gRPC `:31007`，库 `orders`）：下单（服务端算价、地址快照、幂等）、预留库存、支付（验签回调、提交库存、晚到付款自动退款）、取消与超时关单（先关支付单再释放库存）、发货与确认收货（到期自动确认）、订单查询、售后申请与审批（按商品退款）。用**自己的服务身份**（第三对密钥）调用库存接口和支付模拟器。
- `app/paysim`（gRPC `:31009` + HTTP `:8399`，库 `paysim`）：支付模拟器，代替外部支付渠道。付款与关单在同一行事务里裁定；付款后带签名回调（第四对密钥），失败按退避重试；支持部分退款。
- `app/banner`（gRPC `:31008`，库 `banner`）：首页轮播图和手工位。列表公开；写接口只接受管理员令牌。
- Redis 7.4.2（`:6379`）：只做缓存，不持久化。Elasticsearch 8.11.0（`:9200`）：搜索索引，可以随时从 MySQL 重建。
- 结构化日志：所有进程每行一个 JSON 对象（详细设计 §9.1），带链路 ID 和 7 个业务事件，写出前脱敏。
- OpenTelemetry Collector（`:4317`）收集链路、指标、日志：链路进 Jaeger（界面 `:16686`），指标进 Prometheus（`:9090`，带告警规则），日志进 Loki（`:3100`）。全部进程接入，跨 HTTP、gRPC、RabbitMQ 和后台任务；令牌、带值 SQL 等敏感内容在导出前去掉。
- RabbitMQ 4.2.0（`:5672`，管理界面 `:15672`）：服务之间的事件。事件先和业务数据一起写进各服务的 `outbox_event`，再由投递器发出；消费方用 `inbox_event` 去重。现在有三条：订单到期（准时关单）、商品变更（banner 手工位跟随）、下单成功（从购物车移除已购商品）。

**交易闭环已完成**：浏览 → 热门 → 加购 → 下单 → 支付 → 发货 → 收货 → 售后审批与退款。

**尚未完成**：刷新令牌与吊销、角色权限、网关 HTTP 状态码映射、结构化日志、指标、日志诊断 Agent。

## 阅读路线

| 当前任务 | 需要阅读 |
| --- | --- |
| **第一次接触微服务 / 想搞懂在做什么** | [`tutorial/00-从单体到微服务.md`](tutorial/00-从单体到微服务.md) → [01](tutorial/01-一次请求的完整旅程.md) → [02](tutorial/02-代码分四层.md) → [03](tutorial/03-自己跑一遍.md) |
| 安装工具或启动本地依赖 | [`setup/local-environment.md`](setup/local-environment.md) |
| 了解项目如何从零创建及文件来源 | [`setup/project-origin.md`](setup/project-origin.md) |
| 理解对原始教程做出的修正 | [`setup/source-notes.md`](setup/source-notes.md) |
| 生成 DAO/Protobuf 或开发用户服务 | [`development/user-service.md`](development/user-service.md) |
| 登录、令牌、密钥、鉴权中间件与拦截器 | [`development/user-service.md`](development/user-service.md#7-鉴权) → [`tasks/0004-eddsa-and-gateway-auth.md`](../tasks/0004-eddsa-and-gateway-auth.md) |
| 点赞、收藏、评论 | [`development/interaction-service.md`](development/interaction-service.md) |
| 管理员、图片上传、七牛云 | [`development/admin-and-file.md`](development/admin-and-file.md) |
| 商品、分类、Redis 缓存、库存、上下架、热门推荐 | [`development/goods-service.md`](development/goods-service.md) |
| 购物车 | [`development/goods-service.md`](development/goods-service.md#9-购物车) → [`development/events.md`](development/events.md#53-下单成功orderplaced) |
| 修改资料、修改密码 | [`development/user-service.md`](development/user-service.md#8-修改资料与密码) |
| 搜索、Elasticsearch、IK、binlog | [`development/search-service.md`](development/search-service.md) |
| 下单、订单状态、关单、发货收货、售后审批与退款、服务身份 | [`development/order-service.md`](development/order-service.md) |
| 支付、支付模拟器、回调签名、晚到付款退款、部分退款、故障演练 | [`development/payment.md`](development/payment.md) |
| RabbitMQ、事件、Outbox / Inbox、延迟关单、手工位跟随商品、下单后清购物车 | [`development/events.md`](development/events.md) |
| 链路追踪、Jaeger、span 脱敏 | [`development/tracing.md`](development/tracing.md) |
| 日志格式、业务事件、日志脱敏 | [`development/logging.md`](development/logging.md) |
| 指标、告警、Prometheus、在 Loki 查日志 | [`development/metrics.md`](development/metrics.md) |
| 首页轮播图、手工位 | [`development/banner-service.md`](development/banner-service.md) |
| 开发或调试 H5 网关 | [`tutorial/01-一次请求的完整旅程.md`](tutorial/01-一次请求的完整旅程.md) |
| 新增建表脚本、补建本地库 | [`setup/local-environment.md`](setup/local-environment.md#5-启动-mysql-和-etcd)（`make db-init`） |
| 决定服务边界和实现顺序 | [`architecture/overview.md`](architecture/overview.md) |

## 最短启动路径

```bash
make env-check
docker compose config
make infra-up
make infra-ps
```

MySQL 进入健康状态后才能继续进行数据库代码生成。已有数据卷时，新增的建表脚本要用 `make db-init` 补执行。

## 端到端跑一遍

首次运行前，先复制三份 `config.yaml.example`，再用 `make jwt-keys` 生成密钥对：私钥填给 `user`，公钥填给另外两个进程（见 03 第 2 步）。

```bash
cd app/user        && go run .   # 终端 1，gRPC :31001
cd app/interaction && go run .   # 终端 2，gRPC :31002
cd app/gateway-h5  && go run .   # 终端 3，HTTP :8199
```

具体 curl 示例见 [`tutorial/03-自己跑一遍.md`](tutorial/03-自己跑一遍.md)。后台的启动方式见 [`development/admin-and-file.md`](development/admin-and-file.md#5-本地启动)。

## 已完成任务

按时间倒序的新增内容汇总见 [`changelog.md`](changelog.md)。

- [`tasks/0001-user-grpc-contract.md`](../tasks/0001-user-grpc-contract.md)：用户 gRPC 协议，实施报告见 [`reports/0001-user-grpc-contract.md`](reports/0001-user-grpc-contract.md)。
- [`tasks/0002-user-persistence-and-gateway.md`](../tasks/0002-user-persistence-and-gateway.md)：用户服务持久化与 H5 网关。
- [`tasks/0003-jwt-auth.md`](../tasks/0003-jwt-auth.md)：JWT 签发与 gRPC 鉴权拦截器，替换临时的 `X-User-Id`。
- [`tasks/0004-eddsa-and-gateway-auth.md`](../tasks/0004-eddsa-and-gateway-auth.md)：JWT 改为 Ed25519，网关鉴权中间件。
- [`tasks/0005-interaction-service.md`](../tasks/0005-interaction-service.md)：interaction 服务（点赞、收藏、评论）。
- [`tasks/0006-admin-file-upload.md`](../tasks/0006-admin-file-upload.md)：admin 服务、gateway-admin 与图片上传。
- [`tasks/0007-goods-service-and-cache.md`](../tasks/0007-goods-service-and-cache.md)：goods 商品服务与 Redis 缓存。
- [`tasks/0008-inventory-and-shelf.md`](../tasks/0008-inventory-and-shelf.md)：库存预留与扣减、商品上下架。
- [`tasks/0009-search-service.md`](../tasks/0009-search-service.md)：search 搜索服务。
- [`tasks/0010-order-service.md`](../tasks/0010-order-service.md)：order 订单服务。
- [`tasks/0011-banner-service.md`](../tasks/0011-banner-service.md)：banner 服务（轮播图、手工位）。
- [`tasks/0012-payment-simulator.md`](../tasks/0012-payment-simulator.md)：支付模拟器与订单支付。
- [`tasks/0013-rabbitmq-outbox-inbox.md`](../tasks/0013-rabbitmq-outbox-inbox.md)：RabbitMQ 与 Outbox / Inbox。
- [`tasks/0014-tracing.md`](../tasks/0014-tracing.md)：链路追踪（OpenTelemetry + Jaeger）。
- [`tasks/0015-frontend-api-and-hot-goods.md`](../tasks/0015-frontend-api-and-hot-goods.md)：前台改资料、改密码，商品热门推荐。
- [`tasks/0016-cart.md`](../tasks/0016-cart.md)：购物车，`order.placed` 事件；修复多商品订单无法预留。
- [`tasks/0017-shipping-and-receipt.md`](../tasks/0017-shipping-and-receipt.md)：发货、确认收货、到期自动确认。
- [`tasks/0018-after-sale-refunds.md`](../tasks/0018-after-sale-refunds.md)：售后审批、按商品退款、模拟器部分退款。
- [`tasks/0019-structured-logging.md`](../tasks/0019-structured-logging.md)：JSON 结构化日志、业务事件、日志脱敏。
- [`tasks/0020-metrics-and-alerts.md`](../tasks/0020-metrics-and-alerts.md)：指标、告警、日志集中收集到 Loki。

## 下一阶段

交易闭环和可观测性（链路、日志、指标与告警）都已完成，日志诊断 Agent 需要的三样信号都能集中查询。接下来：

1. **0021 故障实验**：把详细设计 §10 的 T1–T10 做成可以重复执行的测试。
2. 网关把 gRPC 状态码映射为 HTTP 状态码（现在所有业务错误都是 HTTP 200，只靠响应体 `code` 区分）；把集成测试扩展到其他 repository；事件的持久重试表与保留期清理。

优惠券不在第一版交易验收范围内（详细设计 §1）。如果要做，按订单状态机设计（下单锁券、支付核销、取消或退款退券），不照搬教程的 CRUD 加注册送券。
