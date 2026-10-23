# 用户服务开发

当前 `app/user` 的用户与收货地址业务**已经实现**：8 个 RPC 全部落到 MySQL，Controller 之下是 `internal/service`（接口）、`internal/logic`（实现）、`internal/repository`（SQL）三层。

协议与代码生成的历史见[任务 0001 报告](../reports/0001-user-grpc-contract.md)；分层的心智模型与常见错误见[`tutorial/02-代码分四层.md`](../tutorial/02-代码分四层.md)。

调用者身份由 JWT 证明：`Login` 签发令牌，鉴权拦截器验签后把用户 ID 放进 context，见[第 7 节](#7-鉴权)。**未完成**：刷新令牌与令牌吊销。

## 1. 启动依赖

```bash
make infra-up
docker compose ps
```

等待 MySQL 和 etcd 都进入健康状态。

## 2. 已生成的持久化模型

CLI 配置位于 `app/user/hack/config.yaml`，连接本地 Compose 中的 MySQL。

以下命令已经针对 MySQL 8.4.11 成功执行：

```bash
cd app/user
gf gen dao
gf gen pbentity
```

DAO、DO、Entity 和 pbentity 中带有 `DO NOT EDIT` 标记的文件不得手工修改。代码生成失败时必须让任务失败，不能继续使用过期产物。

生成的 `pbentity.UserInfo` 会镜像数据库中的敏感列，包括密码哈希和盐。它只是内部生成产物，禁止直接通过公共接口返回。

## 3. 定义 Protobuf 协议

按领域和版本组织路径：

```text
app/user/manifest/protobuf/user/v1/user.proto
app/user/manifest/protobuf/consignee/v1/consignee.proto
```

协议规则：

- 服务名使用 `UserService` 和 `ConsigneeService`。
- 消息名使用 `CreateConsigneeRequest` 形式。
- 即使响应内容为空，也保留明确的请求和响应包装消息。
- 对外时间字段使用 `google.protobuf.Timestamp`。
- 使用分页令牌，或使用经过校验的 page/size 参数。
- 禁止暴露密码哈希、盐和密保答案。
- 不把数据库 pbentity 直接作为外部 API。

协议变更后重新生成代码：

```bash
gf gen pb
```

`RegisterUser` 采用此名称，是为了避开 GoFrame v2.10.2 控制器生成器自身的 `Register` 注册函数。不要手工编辑 `api/` 中的生成文件；控制器占位文件可以续写。

## 4. 实现业务逻辑

保持 Protobuf Controller 足够薄：

```text
gRPC Controller -> Application Service -> Repository Interface -> GoFrame DAO
```

必须实现的行为包括：

- 校验姓名、手机号、分页大小和各类标识符。
- 收货地址的读写必须限定在已认证用户范围内。
- 修改默认地址必须在事务中完成。
- 区分不存在、冲突、输入无效和内部错误。
- 使用专用密码哈希算法，禁止使用明文 MD5。
- 返回 gRPC 状态码，而不是原始数据库错误。

## 5. 按实际使用添加 gRPC 和 etcd 组件

`grpcx` 已在协议任务中加入，因为生成的 Controller 需要它。MySQL 驱动和 etcd 组件应在业务代码实际导入时再添加：

```bash
go get github.com/gogf/gf/contrib/drivers/mysql/v2@v2.10.2
go get github.com/gogf/gf/contrib/registry/etcd/v2@v2.10.2
go mod tidy
```

不要为了完成清单一次性添加剩余模块。代码实际导入并测试某个模块时，再在同一次变更中增加依赖。

使用 etcd 进行 gRPC 服务发现时，需要先注册 etcd Resolver，再创建 Server，并显式设置 `grpc.name`。客户端仍然必须设置超时；服务发现不会自动提供超时、重试或幂等语义。

## 6. 验证门禁

在用户服务被视为完成前执行：

```bash
go test ./...
go vet ./...
go mod verify
```

还需要补充：验证序列化和状态码的内存 gRPC 测试、验证事务行为的 Repository 测试，以及一项验证数据库结构兼容性的 Compose 集成测试。

## 7. 鉴权

```text
Login ──私钥签发──▶ access_token（EdDSA/Ed25519，sub=用户 ID，默认 2h）
网关  ──公钥验签──▶ 早拒绝，原样转发 Authorization
各服务 ──公钥验签──▶ UnaryServerAuth → context → RequireUserID
```

- 收货地址新增 `GetConsignee`：订单服务带着顾客自己的令牌调用它，生成订单里的地址快照，只能拿到本人的地址。
- 后台管理员用的是另一对密钥，由 `admin` 服务签发，见 [`admin-and-file.md`](admin-and-file.md#1-两套身份)。
- **只有 `user` 持有商城用户的私钥**（`jwt.privateKey`）。网关和其他服务只配公钥（`jwt.publicKey`），能验证令牌，但不能签发。演进过程见任务 [0003](../../tasks/0003-jwt-auth.md)（HS256）和 [0004](../../tasks/0004-eddsa-and-gateway-auth.md)（改为 Ed25519）。
- 代码在 `utility/auth.go`（`TokenSigner`、`TokenVerifier`、gRPC 拦截器）和 `utility/http.go`（网关中间件）。服务端拦截器链由 `utility.NewGrpcServerConfig` 统一组装：`UnarySafeLog → UnaryServerAuth → UnaryValidate`。
- **默认拒绝**，分两层：
  - 服务内：`app/user/internal/cmd/cmd.go` 的 `publicMethods` 只有 `RegisterUser` 和 `Login`。
  - 网关：`app/gateway-h5/internal/cmd/cmd.go` 的 `publicRoutes`。
  新增接口默认都要求登录；要公开时两处都得改，并补测试。
- Controller 只通过 `utility.UserIDFromContext` 取身份。这个值只有验签成功后才会写入，客户端发什么头都伪造不了。
- 配置：

  | 键 | 用在哪里 | 默认值 | 说明 |
  | --- | --- | --- | --- |
  | `jwt.privateKey` | 只在 `user` | 无，必填 | Base64 编码的 32 字节 Ed25519 种子，也可以用环境变量 `JWT_PRIVATE_KEY` |
  | `jwt.publicKey` | 网关、interaction 等 | 无，必填 | Base64 编码的 32 字节公钥，也可以用环境变量 `JWT_PUBLIC_KEY` |
  | `jwt.issuer` | 所有进程 | `goframeshop-user` | 写入并校验 `iss` |
  | `jwt.ttl` | 只在 `user` | `2h` | 访问令牌有效期 |

- 用 `make jwt-keys` 生成本地密钥对。密钥缺失或格式错误时进程拒绝启动。私钥和令牌都不得写进日志、提交进 Git 或贴进 issue。
- 已知限制：
  - 令牌在过期前一直有效，修改密码、封禁用户都不会让旧令牌失效。
  - 没有 `kid`，换密钥会让所有已签发的令牌立即失效。

## 8. 修改资料与密码

任务见 [`tasks/0015-frontend-api-and-hot-goods.md`](../../tasks/0015-frontend-api-and-hot-goods.md)。

| HTTP（gateway-h5，需要登录） | gRPC | 规则 |
| --- | --- | --- |
| `GET /frontend/user/profile` | `GetProfile` | 只能查自己 |
| `PUT /frontend/user/profile` | `UpdateProfile` | 整体替换头像、性别、签名，空值表示清除；用户名和状态不能改；返回新资料 |
| `PUT /frontend/user/password` | `ChangePassword` | 必须给出当前密码；新密码 8–128 位；当前密码错误返回 `code:16` |

- 头像只接受空值或 `http`/`https` 绝对地址，不能带账号密码或控制字符，最长 512 字节；性别 0、1、2；签名最多 255 个字符，前后空格去掉。注册时用的是同一套检查（`logic/user` 的 `checkProfile`）。
- 请求里没有用户 ID，改的永远是令牌里的那个用户。
- 改密码后旧令牌仍然有效，直到过期（见上一节的已知限制）。

## 官方参考

- [GoFrame DAO 生成](https://goframe.org/docs/cli/gen-dao)
- [GoFrame gRPC 注册与发现](https://pages.goframe.org/2.8.x/docs/micro-service/registry-discovery)
