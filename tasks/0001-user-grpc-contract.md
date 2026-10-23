# 任务 0001：用户服务 gRPC 协议

状态：已完成协议、代码生成、服务入口和编译验收；业务实现属于后续任务。

## 目标

用经过评审、按版本管理的 Protobuf 协议替换生成的 Hello 示例，覆盖用户注册、登录、资料查询、密码修改和收货地址管理。本任务只定义并生成传输层，不实现数据库读写。

## 开始前读取

- `AGENTS.md`
- `docs/index.md`
- `docs/development/user-service.md`
- `docs/architecture/overview.md`
- `app/user/manifest/protobuf/pbentity/*.proto`

## 范围内

- 创建 `user/v1/user.proto` 和 `consignee/v1/consignee.proto`。
- 服务名和消息名遵守 Protobuf API 风格。
- 定义 GoFrame 代码生成能够执行的校验约束。
- 响应消息排除密码哈希、盐和密保答案等持久化认证秘密；登录响应可包含将来签发的短期访问令牌。
- 为地址列表定义分页和确定性排序。
- 使用 `gf gen pb` 生成 Go Protobuf 和 Controller 文件。
- 增加生成结果的编译检查。

## 范围外

- 数据库读写。
- 密码哈希、JWT 签发和认证中间件。
- etcd 注册和 gRPC 客户端。
- HTTP 网关、前端、导购 Agent、支付和日志诊断 Agent。

## 已采用的决定

- 登录标识先只支持用户名。
- 不在当前协议中提供密保答案找回密码；修改密码要求当前密码。
- 地址列表使用经过校验的 `page`/`size`，按 `consignee_id` 降序排序。

## 验收标准

- 公共响应消息不包含密码哈希、盐、密保答案或内部删除元数据。
- ID 使用 `uint64`，与数据库结构一致。
- 每个 RPC 都有明确的请求和响应消息。
- `gf gen pb` 成功，且没有手工修改生成文件。
- `go test ./...`、`go vet ./...` 和 `go mod verify` 全部通过。

## 验证命令

```bash
cd app/user
gf gen pb
cd ../..
go test ./...
go vet ./...
go mod verify
```
