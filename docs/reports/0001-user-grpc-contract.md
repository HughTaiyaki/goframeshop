# 用户服务 gRPC 协议：实施报告与入门指南

## 这次完成了什么

`app/user` 已从活动的 HTTP Hello 示例切换为 gRPC 服务入口，监听 `:31001`。它公开两个 v1 服务：`UserService`（注册、登录、查询个人资料、修改密码）和 `ConsigneeService`（地址列表、创建、更新、删除）。协议、Go 类型、客户端/服务端接口、控制器占位方法和服务注册代码均已生成或接通。

**当前还不能真正注册、登录或管理地址。** 8 个业务方法会返回 gRPC 标准状态 `Unimplemented`。数据库读写、密码哈希、令牌签发、鉴权以及 etcd 服务发现是下一阶段；现阶段的目标是把服务之间如何“说话”定义清楚，并验证生成链路。

## 关键文件

| 路径 | 作用 | 是否手动编辑 |
| --- | --- | --- |
| `app/user/manifest/protobuf/user/v1/user.proto` | 用户 API 的协议源文件 | 是 |
| `app/user/manifest/protobuf/consignee/v1/consignee.proto` | 地址 API 的协议源文件 | 是 |
| `app/user/api/user/v1/`、`app/user/api/consignee/v1/` | `gf gen pb` 生成的消息和 gRPC 接口 | 否，改 `.proto` 后重新生成 |
| `app/user/api/pbentity/` | 数据库表镜像，供内部代码生成使用 | 否，不能直接作为公共 API |
| `app/user/internal/controller/user/`、`consignee/` | 可续写的业务处理入口；当前为占位实现 | 是 |
| `app/user/internal/cmd/cmd.go` | 启动 gRPC 服务、启用请求校验并注册两个服务 | 是 |
| `app/user/manifest/config/config.yaml.example` | 可提交的本地配置样例 | 是 |
| `app/user/api/contract_test.go` | 检查 RPC、公共字段和校验标签 | 是 |

本机使用的 `app/user/manifest/config/config.yaml` 已改成 gRPC 配置。它被 Git 忽略，不会当作团队共享配置提交。原活动用户服务中的 HTTP Hello 示例已移除；`app/user/app/user` 内仍有历史重复脚手架，此次没有把它当成实际服务入口。

GoFrame 默认的 gRPC 错误日志会打印完整请求，可能泄露密码。服务入口关闭了这项默认请求日志，改为只记录 RPC 方法、标准状态码和耗时，不记录请求体、令牌或错误详情。

## 用一个例子理解 gRPC、Protobuf 和 Controller

假设未来网关要查询某个已登录用户的资料：

```text
网关调用 UserService.GetProfile
        ↓
Protobuf 定义请求、响应及字段编号
        ↓
gRPC 根据生成的代码序列化请求并调用服务端
        ↓
Controller 接收请求，后续会调用应用层和 DAO
        ↓
服务端返回 GetProfileResponse 或标准 gRPC 状态码
```

`.proto` 是双方共用的**协议合同**。`gf gen pb` 将它生成 `.pb.go`（消息类型）、`_grpc.pb.go`（客户端和服务端接口）与 Controller 模板。字段后的 `= 1`、`= 2` 是 Protobuf 在线上传输时使用的编号；协议发布后不能随意给旧编号换含义。路径中的 `v1` 是 API 版本，未来不兼容的变更可放到 `v2`。

`pbentity.UserInfo` 是数据库表的镜像，包含密码哈希、盐和密保答案哈希，因此公共响应使用独立的 `UserProfile`。地址响应同样使用独立的 `Consignee`，不暴露内部删除时间。协议 ID 使用 `uint64`，与数据库的 `BIGINT UNSIGNED` 对齐；时间字段使用 `google.protobuf.Timestamp`。

## 这次确定的接口规则

- 登录标识先只支持用户名。请求中有密码；响应中的 `access_token` 是将来登录成功后主动签发给客户端的凭据，当前不会签发。
- 修改密码需要当前密码和新密码；密保答案找回流程暂不设计。
- 地址请求不接受客户端指定 `user_id`。未来的鉴权层应从调用者身份确定用户，并把所有地址查询和写入限制在该用户范围内。
- 地址列表要求 `page >= 1`、`1 <= size <= 100`；合同规定按 `consignee_id` 降序返回。实现时须按同一用户、排除软删除记录后计数，并使用相同排序分页。
- 姓名、电话、地址和 ID 的基础规则写在 `.proto` 注释中的 `v:` 标签。`gf gen pb` 把这些注释注入生成的 Go 字段标签；服务入口启用 `UnaryValidate` 后，请求会在进入 Controller 前校验。手机号格式、身份归属和业务冲突仍需在后续应用层校验。
- 每个 RPC 都有独立请求与响应消息，包括目前没有字段的消息，这为将来添加字段留出兼容空间。

`RegisterUser` 的 RPC 名称刻意不用 `Register`。GoFrame v2.10.2 的控制器生成器也会写一个名为 `Register` 的服务注册函数；同名时，它误以为 RPC 方法已存在，跳过该占位方法。改名并重新生成后，8 个方法均已检查齐全。

## 自己动手验证

在仓库根目录运行：

```bash
cd app/user
gf gen pb
cd ../..
go test ./...
go vet ./...
go mod verify
```

想观察服务启动时，在 `app/user` 目录运行 `go run .`。本次实测日志显示 `grpc server started listening on [:31001]`；检查完可按 `Ctrl+C` 停止。因为现在没有业务实现，调用有效的 RPC 会得到 `Unimplemented`；输入违反校验规则时会得到 `InvalidArgument`。**看到服务启动，不等于用户业务完成。**

本次验收结果：`gf gen pb`、用户服务范围测试、`go test ./...`、`go vet ./...`、`go mod verify` 和 `docker compose config --quiet` 均通过。内存 gRPC 测试覆盖了请求序列化、服务注册、校验与状态码；同时确认测试日志不含明文密码。实际启动后确认了 31001 端口监听，并正常停止。MySQL 与 etcd 容器健康，但此服务目前使用 GoFrame 默认的本地文件注册表，**尚未注册到 etcd**。

## 下一步学习与实现顺序

1. 先实现用户应用层与 Repository 接口，把 Controller 保持为传输层；注册与登录使用专用密码哈希算法和明确的 gRPC 状态码。
2. 加入认证拦截器，让个人资料和地址操作只能访问调用者自己的数据。
3. 实现地址 CRUD。切换默认地址时使用数据库事务，并补充跨用户访问与分页测试。
4. 再接入 etcd 注册、带超时的 gRPC 客户端和集成测试。etcd 负责发现服务位置，不存储用户或地址业务数据。

参考：[GoFrame 微服务工程与代码生成](https://goframe.org/docs/micro-service/structure)、[GoFrame 用户服务开发约定](../development/user-service.md)。
