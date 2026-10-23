# AGENTS.md

## 项目目标

本仓库是一个 GoFrame 商城实验环境。先构建规模可控、可观测的分布式系统；只有当服务能够产生可信的日志、链路、指标和确定性故障后，才开始实现日志诊断 Agent。

## 当前阶段

已完成：本地基础设施、用户服务、interaction 服务、H5 网关、Ed25519 JWT 鉴权，后台的 admin、file 服务和 gateway-admin 图片上传，带 Redis 缓存的 goods 商品服务、库存预留与上下架，Elasticsearch 搜索服务，order 订单服务，banner 服务，支付模拟器与订单支付，RabbitMQ 事件（Outbox / Inbox），链路追踪，以及前台改资料、改密码和热门推荐（任务 0001–0015）。下一步见 `docs/index.md` 的"下一阶段"。

在验收测试存在并通过之前，不得声称结算、支付、导购推荐或故障诊断已经实现。

## 渐进式文档加载

每次任务必须先读：

1. `docs/index.md`
2. `tasks/` 下当前生效的任务文件（如果存在）

只有任务涉及对应领域时才继续读取：

- 环境、Docker、开发工具：`docs/setup/local-environment.md`
- 项目搭建过程与文件来源：`docs/setup/project-origin.md`
- 用户服务、Protobuf、DAO 生成、鉴权：`docs/development/user-service.md`
- 点赞、收藏、评论：`docs/development/interaction-service.md`
- 管理员、图片上传、七牛云：`docs/development/admin-and-file.md`
- 商品、分类、Redis 缓存、库存、上下架：`docs/development/goods-service.md`
- 搜索、Elasticsearch、binlog：`docs/development/search-service.md`
- 订单、订单状态机、服务身份：`docs/development/order-service.md`
- 首页轮播图、手工位：`docs/development/banner-service.md`
- 支付、支付模拟器、回调签名、技术退款：`docs/development/payment.md`
- RabbitMQ、事件、Outbox / Inbox：`docs/development/events.md`
- 链路追踪、span 脱敏：`docs/development/tracing.md`
- 服务边界或开发顺序：`docs/architecture/overview.md`
- 原始教程与本项目决策的差异：`docs/setup/source-notes.md`
- 网关、HTTP 转 gRPC、服务发现、令牌传递：`docs/tutorial/01-一次请求的完整旅程.md`
- 本地端到端演示：`docs/tutorial/03-自己跑一遍.md`

处理局部修改时不要一次性加载全部文档。只有当前任务跨越边界时，才继续追踪相关链接。

## Go 开发流程

进行任何 Go 编码、评审、调试、排错或环境配置前，先加载 `samber/cc-skills-golang@golang-how-to` skill，由它选择当前任务需要的其他 Go skills。

使用仓库锁定的模块版本。功能开发期间禁止执行宽泛的 `go get -u ./...` 升级。依赖变更后执行 `go mod tidy`、`go mod verify` 和相关测试。

## 常用命令

```bash
make env-check
make infra-up
make infra-ps
make db-init     # 已有数据卷时补执行建表脚本
make jwt-keys    # 打印本地开发用的 Ed25519 密钥对
make verify
GOFRAMESHOP_IT=1 go test ./app/goods/internal/repository/ ./app/search/internal/index/ -run Integration   # 需要 Compose
```

`make infra-reset` 会删除本地 MySQL 和 etcd 数据卷。只有任务明确要求重建本地环境时才能执行。新增建表脚本后，用 `make db-init` 补执行（脚本必须可重复执行）。

## 工程约束

- 传输层处理器与业务逻辑分离。
- Protobuf API 按领域和版本组织，例如 `app/user/manifest/protobuf/user/v1`。
- 每个 RPC 都使用明确的请求和响应消息。
- 传递 `context.Context`，客户端必须设置超时。
- 返回有意义的 gRPC 状态码，不泄露数据库内部错误。
- 禁止记录密码、令牌、支付数据、密保答案和原始提示词。
- 新增 gRPC 方法和 HTTP 路由默认需要登录。只有确实公开的接口才能加入对应服务 `internal/cmd/cmd.go` 的 `publicMethods`，以及 `app/gateway-h5/internal/cmd/cmd.go` 的 `publicRoutes`，并补测试。
- 只有 `app/user`（商城用户）、`app/admin`（管理员）和 `app/order`（服务身份）可以持有 JWT 私钥，三者必须是**不同的**密钥对；其他进程只能配置对应的公钥。支付回调的签名私钥是第四对，只放在 `app/paysim`。
- 支付模拟器代表外部系统：订单服务只能通过它的 gRPC 接口打交道，不能读写 `paysim` 库。支付回调只当提示，必须验签后再查询模拟器，以查询结果为准。
- 一个服务接受多种身份时，用 `utility.AuthPolicy` 为每个接口列出允许的角色；未列出的接口默认拒绝。
- 订单状态只能用带条件的 UPDATE（`WHERE status IN (...)`）修改，或者在先 `SELECT ... FOR UPDATE` 锁住订单行的事务里修改（支付记账 `ConfirmPaid`）；只有改到行的一方才能继续下一步（比如释放库存）。调用外部服务（goods、支付模拟器）必须在事务之外。
- 金额、价格、地址等决定交易的数据，必须由服务端查询，不得取自客户端请求。
- 会被前端放进链接的 URL 必须限定为 http/https 绝对地址（参考 `app/banner/internal/logic/check`）。
- 不得用"事务提交后再起 goroutine 发消息"的方式发布事件：用 `eventbus.Append` 在业务事务里写入 outbox，由 Relay 发出（详细设计 §8）。
- 消费事件必须经过 `eventbus.Handle`，业务修改写在它给的事务里；事件是事实不是命令，处理前按自己的当前状态核对。
- 新的路由键和队列只能加在 `utility/rabbitmq/topology.go`；发布是 mandatory 的，没有队列接收的事件会一直重试。
- 跨服务的事件内容写在 `utility/eventbus/contracts.go`，只能加字段，不能改或删。
- 每个进程的 `main` 第一行调用 `telemetry.Start`。后台循环使用它返回的 Quiet 上下文，每一件真正的工作用 `telemetry.Root` 开新链路；不要让轮询记录 span。
- span 里不得放令牌、密码、手机号、地址、支付凭证。新增依赖或功能会自动记录新的敏感字段时，先在 `utility/telemetry/redact.go` 加规则和测试，并同步 `deploy/otel/collector.yaml` 的 `transform/scrub`。
- 不得新增公开的管理员注册接口。
- 上传的文件必须按内容判断类型，对象 key 由服务端生成，不能使用客户端提供的文件名。
- 不得提交或复用任何第三方凭据（包括教程中出现的七牛 AccessKey/SecretKey）。
- `gf gen ctrl` 生成的 `NewV1()` 必须保持零参数，否则重新生成会追加重复函数。
- 缓存只能做优化：Redis 出错时必须回退到 MySQL；每个写操作成功后都要删除受影响的缓存键（包括"不存在"标记），并补测试证明这一点。
- 判断下游 gRPC 错误码用 `utility.RPCCode`，不要用 `status.Code`（grpcx 客户端会改写错误）。
- 日志中不得出现缓存值、令牌、密码或请求正文。
- 库存只能由库存账本的事务修改，每个事务先按 goods_id 顺序锁库存行；改动库存逻辑必须跑 `GOFRAMESHOP_IT=1` 的集成测试。
- 修改 goods 表结构时，同步修改 `app/search/internal/source/mysql.go` 的 `docQuery`，并检查 search 的 `idColumns`。
- 迁移脚本必须可以重复执行，并且能安全作用于已有数据（参考 `006_inventory_and_shelf.sql` 的条件 DDL 写法）。
- 列表、更新、删除必须按调用者限定范围，写入语句本身也要带上 `user_id` 条件，不能只依赖事先的检查。
- 调用者身份只能从 `utility.UserIDFromContext` 取得，不得从请求字段或客户端可控的 metadata 读取。
- 在设计明确的审批边界前，Agent 工具保持只读。
- 不把生成的 DAO Entity 直接作为公共 API 协议。
- 带有 `DO NOT EDIT` 标记的生成文件视为不可手工修改。

## 配置约束

- 只提交 `*.example` 配置文件。
- 本地 `config.yaml` 和 `.env` 文件由 Git 忽略。
- Compose 默认值只面向本地开发，不能作为生产默认值。
- 不通过本仓库修改 Docker 的全局公共镜像源配置。

## 完成前验证

在宣布任务完成前：

1. 先运行范围最小且与修改直接相关的测试。
2. Go 代码发生变化时运行 `go test ./...` 和 `go mod verify`。
3. Compose 发生变化时运行 `docker compose config`。
4. 无法执行的检查必须说明具体外部阻塞原因。
5. 在 `docs/changelog.md` 最上面写一节：新增了什么、需要手动执行什么。
