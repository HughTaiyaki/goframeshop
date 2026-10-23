# 任务 0003：JWT 签发与 gRPC 鉴权拦截器

状态：已完成。`Login` 签发访问令牌，`user` 服务用拦截器验签，临时的 `X-User-Id` / `x-user-id` 身份传递已删除。

> 后续变更：签名算法已在[任务 0004](0004-eddsa-and-gateway-auth.md) 改为 Ed25519，配置项 `jwt.secret` / `JWT_SECRET` 已由 `jwt.privateKey` / `jwt.publicKey` 取代；网关也开始用公钥验签。下文"网关不验签"等描述保留为历史记录。

## 目标

把"网关说你是谁你就是谁"改成"拿得出 user 服务签发的令牌才算数"。

## 范围内

- `utility/auth.go`：HS256 签发与校验、`Authorization: Bearer` 解析、gRPC 服务端鉴权拦截器、网关侧令牌透传。
- `app/user`：`Login` 返回 `token_type`、`access_token`、`expires_in_seconds`（字段在任务 0001 的协议里已经预留，**本任务没有改 .proto**）；拦截器链加入鉴权；配置新增 `jwt` 段。
- `app/gateway-h5`：登录响应带令牌；需要身份的接口把 `Authorization` 头原样转成 gRPC metadata；CORS 不再放行 `X-User-Id`。

## 范围外

- 刷新令牌、登出与令牌吊销。令牌在过期前一直有效。
- 修改密码或封禁用户后让旧令牌立即失效（需要 `token_version` 一类的列，属于后续任务）。
- 非对称签名（EdDSA/RS256）与密钥轮换（`kid`）。
- 网关把 gRPC 状态码映射成 HTTP 状态码。目前所有业务错误仍是 HTTP 200，靠响应体 `code` 区分。

## 已采用的决定

| 决定 | 原因 |
| --- | --- |
| 只有 `user` 服务持有密钥，签发和验签都在这里 | 网关不需要密钥，少一个能伪造令牌的进程 |
| 网关只检查"有没有 Bearer 头"，不验签，原样转发 | 早拒绝能省一次 RPC；是否可信由持有密钥的服务决定 |
| 鉴权拦截器默认拒绝，只白名单 `RegisterUser`、`Login` | 以后新加的 RPC 忘了配置时是"拒绝"而不是"放行" |
| 验证后的用户 ID 放进私有 context key，不放 metadata | 客户端发一个头就能伪造 metadata，但伪造不了 context key |
| 拦截器顺序：日志 → 鉴权 → 参数校验 | 未登录调用方拿不到参数校验规则这类信息 |
| 算法固定 HS256，同时要求 `iss` 和 `exp` | 防止 `alg: none` 和换算法攻击；没有过期时间的令牌直接拒绝 |
| 密钥至少 32 字节，缺失时服务拒绝启动 | 不允许悄悄用一个众所周知的默认密钥签名 |
| 密钥可以来自 `jwt.secret`，也可以来自环境变量 `JWT_SECRET` | 部署时密钥可以不落文件 |
| 所有令牌错误统一返回 `invalid access token` | 不告诉调用方是过期、伪造还是格式错误 |
| 网关早拒绝和服务端拒绝都用响应码 `16` | 前端只需要认一个"请重新登录"信号 |
| 访问令牌默认有效期 2 小时 | 没有吊销机制时，有效期就是泄露后的最长暴露时间 |

## 验收标准

- `alg: none`、HS512、错误密钥、错误签发者、缺 `exp`、过期、篡改载荷、非数字或为 0 的 `sub` 全部被拒绝（`utility/auth_test.go`）。
- 只带旧的 `x-user-id` metadata 的调用被拒绝。
- 每个受保护 RPC 在没有令牌或令牌伪造时返回 `Unauthenticated`（`app/user/internal/cmd/cmd_test.go`，走真实 gRPC 拦截器链）。
- `Login` 返回的令牌可以被验签，并解析出正确的用户 ID。
- 密钥不足 32 字节时服务启动失败，退出码非 0。
- 日志中不出现令牌和密码。
- `go test ./...`、`go vet ./...`、`go mod verify` 全部通过，GoFrame 仍为 v2.10.2。

## 验证命令

```bash
go test ./...
go vet ./...
go mod verify
go list -m github.com/gogf/gf/v2 github.com/golang-jwt/jwt/v5   # v2.10.2 / v5.3.0
```

端到端 curl 示例见 [`docs/tutorial/03-自己跑一遍.md`](../docs/tutorial/03-自己跑一遍.md)。
