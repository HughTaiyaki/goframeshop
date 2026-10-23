# 任务 0005：interaction 服务（点赞、收藏、评论）

状态：已完成。来源是原教程的 interaction 章节，按本仓库的分层、鉴权和协议规则重写。开发说明见 [`docs/development/interaction-service.md`](../docs/development/interaction-service.md)。

## 目标

新增 `app/interaction`（gRPC `:31002`，库 `interaction`），并通过 gateway-h5 暴露 HTTP 接口：

| HTTP | 鉴权 | gRPC |
| --- | --- | --- |
| `GET/POST/DELETE /frontend/praise` | 需要 | `PraiseService` |
| `GET/POST/DELETE /frontend/collection` | 需要 | `CollectionService` |
| `GET /frontend/comment` | **公开** | `CommentService.ListComments` |
| `POST/DELETE /frontend/comment` | 需要 | `CommentService` |

## 范围外

- 校验被点赞、收藏、评论的对象是否真实存在：商品服务还没有，`object_id` 目前只做"大于 0"校验。
- 点赞数、收藏数等聚合统计。
- 评论里展示作者昵称：需要调用 `user` 服务批量查询，现在只返回 `userId`。
- 防止重复提交评论（需要幂等键）。
- 评论审核、敏感词过滤。

## 修正原教程的问题

| 原教程 | 本仓库 | 后果 |
| --- | --- | --- |
| `GetList` 不加任何条件，`Count()` 统计全表 | 点赞、收藏按"当前用户 + 对象类型"过滤；评论按对象过滤 | 原写法会把所有用户的数据返回给任何人 |
| `Delete` 只按 `id` 删除 | 先确认归属，写入语句也带上 `user_id` 条件；不是自己的就返回 `NotFound` | 原写法下任何登录用户都能删除别人的点赞、收藏和评论 |
| `InsertAndGetId(req)` 直接插入请求对象 | 只写入白名单字段，`user_id` 来自令牌 | 原写法里 `user_id` 根本没有赋值，也让请求字段能直接控制数据库写入 |
| 转换出错时 `continue`，静默丢数据 | 出错就返回错误 | 静默丢行会让 total 和 items 对不上，也会给日志诊断留下假证据 |
| 公共响应使用 `pbentity` | 独立的 proto 消息和映射 | 见 [`docs/setup/source-notes.md`](../docs/setup/source-notes.md) |
| `gerror.WrapCode(CodeDbOperationError, err, ...)` 把数据库错误传给调用方 | 统一映射为 gRPC 状态码，未知错误只返回 `internal error` | 不泄露表名、约束名和驱动错误 |
| 重复点赞依赖唯一索引报错，报错内容原样返回 | 识别 MySQL 1062，返回 `AlreadyExists` | 客户端能区分"已经点过"和"系统故障" |
| 评论的唯一索引包含 `content` | 去掉这个唯一索引 | 软删除后无法再次发表相同内容；大字段放进唯一索引也不合适 |
| 回复的 `parent_id` 不做校验 | 父评论必须存在、未删除、属于同一个对象 | 原写法可以把回复挂到另一个商品的评论下面 |
| 字段名 `type`，`int` 主键 | `object_type`，`BIGINT UNSIGNED` | 与 user 服务的表结构约定保持一致 |
| 示例数据引用不存在的用户 | 不提供种子数据 | 种子数据中的 `user_id` 在本仓库里不存在 |
| proto 服务名小写（`service praise_info`），消息叫 `XxxReq` | `PraiseService`，`ListPraisesRequest` | 遵循[用户服务开发](../docs/development/user-service.md#3-定义-protobuf-协议)的协议规则 |

## 验收标准

- `logic/mark`：重复创建返回 `AlreadyExists`；列表只包含调用者自己、指定类型的数据，且按 ID 倒序；删除别人的记录返回 `NotFound`，且记录仍然存在；删除后可以重新创建。
- `logic/comment`：空内容、纯空白、超过 255 个字符被拒绝，正好 255 个中文字符可以通过；回复不存在、已删除或属于其他对象的评论被拒绝；只有作者能删除；列表不包含已删除评论。
- `internal/cmd`：服务只持有公钥；8 个受保护 RPC 在无令牌、伪造令牌、只带 `x-user-id` 时全部返回 `Unauthenticated`；`ListComments` 不需要令牌。
- 本地端到端（Compose MySQL + etcd + 三个进程）跑通，日志中没有令牌、密码和评论正文。
- `go test ./...`、`go vet ./...`、`go mod verify` 通过。

## 验证命令

```bash
make db-init      # 已有数据卷时补建 interaction 库
go test ./...
go vet ./...
go mod verify
```
