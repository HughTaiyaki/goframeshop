# 任务 0011：banner 服务（轮播图、手工位）

状态：已完成。来源是原教程的"banner 服务-轮播图-手工位图"一节；同一章里 RabbitMQ 延迟消息的部分没有照做，原因见下文。开发说明见 [`docs/development/banner-service.md`](../docs/development/banner-service.md)。

## 新增了什么

| 类别 | 内容 |
| --- | --- |
| 进程 | `app/banner`（gRPC `:31008`，库 `banner`）。教程用的是 `:31006`，和 search 服务冲突 |
| 建表 | `009_banner_schema.sql`：`rotation_info`、`position_info` |
| 后台 HTTP | `GET/POST/PUT/DELETE /backend/rotation`、`GET/POST/PUT/DELETE /backend/position` |
| 前台 HTTP（公开） | `GET /frontend/rotation`、`GET /frontend/position` |

## 与原教程的差异

| 原教程 | 本仓库 | 原因 |
| --- | --- | --- |
| 后台直接填图片 URL（`pic_url`） | 传 `fileId`，到 file 服务确认文件存在，再保存一份 URL | 和商品图片一致；只能引用真实上传过的文件 |
| 链接只校验 `url` 格式 | 只接受 `http`/`https` 的绝对地址；不能带账号密码，不能有控制字符 | `javascript:`、`data:` 链接放进首页 `<a href>`，顾客一点就会执行脚本 |
| 手工位的商品名由后台填写 | 按 `goodsId` 从商品服务读取并保存 | 否则可以给任意商品配任意名字 |
| 列表接口有 `sort=2` 倒序参数 | 固定按 `sort` 升序、再按 ID 排列 | 展示顺序由运营决定，不应让访客改；以 ID 兜底保证分页稳定 |
| 直接插入或更新请求对象，删除不检查是否存在 | 只写允许的字段；修改、删除不存在的记录返回 NotFound | 同前几章 |
| 返回 `pbentity`、`DeletedAt` | 独立的协议消息，不返回删除时间 | 见原教程修订说明 |
| `root` 账号连数据库 | `shop` 账号，只授予 `banner` 库的增删改查权限 | 最小权限 |

## 为什么没有照做 RabbitMQ 这一节

教程用 RabbitMQ 延迟消息实现订单超时关单：下单事务提交后起一个 goroutine 发消息。我们在[任务 0010](0010-order-service.md) 已经用"数据库 + 后台恢复任务"实现了同一件事，而且不会丢单。教程的写法有这些问题：

- **会丢消息**：事务提交后、消息发出前进程崩溃，这个订单就永远收不到超时消息。这正是详细设计 §2.2 列出的问题 B1。
- **发送可能失败**：用的是请求的 `ctx`，请求返回后 ctx 会被取消，发送可能直接失败。
- **不知道发没发成**：没有发送确认（publisher confirms）。
- **延迟插件不可靠**：延迟消息只存在单个节点上。
- **worker 是空的**：不消费任何消息。

RabbitMQ 已在[任务 0013](0013-rabbitmq-outbox-inbox.md) 按详细设计的 Outbox/Inbox 方式引入：消息先和业务数据写在同一个事务里，再由投递器发出。到那时，延迟消息可以作为关单的加速手段，数据库扫描保留作兜底。

## 已知局限

- ~~手工位在保存时复制了商品名，之后商品改名或下架不会自动更新~~：已在[任务 0013](0013-rabbitmq-outbox-inbox.md) 用 `goods.changed` 事件解决。
- 列表没有缓存。首页访问量大时，可以按 goods 服务的方式加 Redis 缓存。

## 验收标准

- 单元测试：链接校验（`javascript:`、`data:`、`vbscript:`、相对路径、协议相对地址、ftp、带账号密码、无主机、控制字符、超长都被拒绝；合法地址会去掉首尾空白）；图片和商品必须存在；修改会刷新图片 URL；修改、删除不存在的记录返回 NotFound；分页参数校验。
- 鉴权测试：列表不需要令牌；6 个写接口在无令牌和商城用户令牌下返回 `Unauthenticated`，管理员令牌可以通过。
- 端到端：上传图片 → 新增两张轮播图和两个手工位 → 前台不带令牌按 `sort` 升序拿到列表，手工位商品名来自商品服务；非法链接、不存在的文件或商品返回 `code:3`；前台没有写接口；后台写接口不带令牌返回 `code:16`。
- `go test ./...`、`go vet ./...`、`go mod verify`、`docker compose config` 通过；重新生成代码无差异。
