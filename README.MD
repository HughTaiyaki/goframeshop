# GoFrame Shop

这是一个基于 GoFrame MonoRepo 的分布式商城实验项目。它先实现一个可观测、能复现故障的商城后端，再把这个后端作为日志诊断 Agent 的实验环境。

目前已完成用户服务、interaction 服务（点赞、收藏、评论）、goods 商品服务（商品、图片、三级分类、库存预留与扣减、上下架，带 Redis 缓存）、search 搜索服务（Elasticsearch + IK，binlog 同步）、order 订单服务（下单、库存闭环、支付、超时关单、晚到付款自动退款、售后申请）、paysim 支付模拟器、banner 服务（轮播图、手工位）、H5 网关，以及后台的 admin 服务、file 服务、gateway-admin（管理员登录、图片上传、商品管理）。用户、管理员、订单服务身份、支付回调各用一对 Ed25519 密钥。可观测性已有链路追踪，结构化日志和指标还没有做。

## 当前基线

- Go 1.24.5 或更高版本。
- GoFrame CLI 与框架 v2.10.2（`go mod tidy` 后要用 `go list -m` 复核，它可能悄悄降级）。
- Protobuf 编译器及 Go gRPC 代码生成插件。
- 通过 Docker Compose 运行 MySQL 8.4.11、etcd 3.6.1、Redis 7.4.2（只做缓存）、Elasticsearch 8.11.0（带 IK 分词，首次 `make infra-up` 时构建镜像）、RabbitMQ 4.2.0（服务之间的事件，Outbox / Inbox），以及 OpenTelemetry Collector + Jaeger（链路追踪，界面 <http://127.0.0.1:16686>）。

| 进程 | 地址 | 职责 |
| --- | --- | --- |
| `app/user` | gRPC `:31001` | 注册、登录、资料、改密码、收货地址 CRUD；Argon2id 存密码；唯一持有 JWT 私钥 |
| `app/interaction` | gRPC `:31002` | 点赞、收藏、评论；只持有公钥；评论列表公开，其余需要登录 |
| `app/gateway-h5` | HTTP `:8199` | HTTP/JSON 转 gRPC，经 etcd 发现后端；用公钥验签，默认拒绝；自身不碰数据库 |
| `app/admin` | gRPC `:31003` | 管理员登录、创建管理员；持有**管理员**密钥对的私钥；没有公开注册 |
| `app/file` | gRPC `:31004` | 记录上传文件的元数据；只接受管理员令牌 |
| `app/goods` | gRPC `:31005` | 商品、商品图片、三级分类、库存账本、上下架；读接口公开并走 Redis 缓存，写接口只接受管理员令牌 |
| `app/order` | gRPC `:31007` | 下单、支付、取消、超时关单、订单查询、售后申请；用自己的服务身份调用库存接口和支付模拟器 |
| `app/paysim` | gRPC `:31009`、HTTP `:8399` | 支付模拟器：商户接口只给订单服务用；收银台页面给付款人；付款后带签名回调；有自己的库 |
| `app/banner` | gRPC `:31008` | 首页轮播图、手工位；列表公开，写接口只接受管理员令牌 |
| `app/search` | gRPC `:31006` | 商品搜索（Elasticsearch + IK）；公开、只读；启动时重建索引，之后跟随 binlog |
| `app/gateway-admin` | HTTP `:8299` | 后台入口；校验图片后写入七牛或本地目录，再交给 file 服务记录 |

## 快速开始

```bash
make env-check
make infra-up
make infra-ps        # 等 mysql 和 etcd 都 healthy
```

执行 `make infra-up` 前必须启动 Docker Desktop。已有数据卷时，先执行 `make db-init` 补建 `interaction` 库。

每个服务都需要一份只在本地使用的配置，还需要一对 Ed25519 密钥：

```bash
for svc in user interaction gateway-h5; do
  cp app/$svc/manifest/config/config.yaml.example app/$svc/manifest/config/config.yaml
done
make jwt-keys   # 私钥 → app/user 的 jwt.privateKey；公钥 → 另外两个的 jwt.publicKey
```

然后在三个终端里分别启动：

```bash
cd app/user && go run .
cd app/interaction && go run .
cd app/gateway-h5 && go run .    # Swagger 在 http://127.0.0.1:8199/swagger
```

完整的 curl 演示见 [自己跑一遍](docs/tutorial/03-自己跑一遍.md)。后台（admin、file、gateway-admin）需要**另一对**密钥和一个用命令行创建的超级管理员，见 [后台开发说明](docs/development/admin-and-file.md)。

## 文档入口

- [文档索引](docs/index.md)：根据当前任务按需阅读文档。
- [变更记录](docs/changelog.md)：每个任务新增了什么、需要手动执行什么。
- [微服务入门教程](docs/tutorial/00-从单体到微服务.md)：第一次接触本项目从这里开始。
- [从零搭建与文件来源](docs/setup/project-origin.md)：了解执行过的命令，以及每个目录是手写还是生成的。
- [本地环境](docs/setup/local-environment.md)：安装工具和启动依赖。
- [用户服务开发](docs/development/user-service.md)：DAO、pbentity、gRPC 开发约束与鉴权。
- [interaction 服务开发](docs/development/interaction-service.md)：点赞、收藏、评论的规则。
- [后台开发](docs/development/admin-and-file.md)：管理员、图片上传、七牛云配置。
- [商品服务开发](docs/development/goods-service.md)：商品、分类规则、Redis 缓存、库存与上下架。
- [搜索服务开发](docs/development/search-service.md)：Elasticsearch、IK 分词、binlog 同步。
- [订单服务开发](docs/development/order-service.md)：一次下单的完整过程、订单状态、服务身份。
- [支付](docs/development/payment.md)：支付模拟器、一次付款的完整过程、回调验签、付款与关单的竞争、故障演练。
- [事件](docs/development/events.md)：RabbitMQ、Outbox / Inbox、订单准时关单、手工位跟随商品。
- [链路追踪](docs/development/tracing.md)：在 Jaeger 里看一次请求经过的每个服务、每条 SQL；事件和后台任务的链路；脱敏。
- [banner 服务开发](docs/development/banner-service.md)：轮播图、手工位。
- [AGENTS.md](AGENTS.md)：AI Coding 的渐进式披露规则。

## 安全边界

- Compose 中的账号和密码只能用于本地开发。
- `config.yaml` 和 `.env` 已被 Git 忽略，只提交示例文件。JWT 私钥只放在 `app/user`、`app/admin` 和 `app/order`（服务身份）各自的本地 `config.yaml` 或环境变量里，支付回调的签名私钥只放在 `app/paysim`，其他进程只拿公钥。七牛凭据同理，而且必须是你自己账号的，不能用教程里贴出的。
- 当前阶段不接入真实支付，不保存生产凭据，也不允许 Agent 执行写操作。
- 不要手工修改带有 `DO NOT EDIT` 标记的生成文件。
