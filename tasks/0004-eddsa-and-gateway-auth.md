# 任务 0004：JWT 改为 Ed25519，网关鉴权中间件

状态：已完成。触发原因：原教程要求网关加 JWT 中间件，同时 interaction 服务也要验签。用 HS256 的话，每个验签方都得持有共享密钥；按[用户服务开发 · 鉴权](../docs/development/user-service.md#7-鉴权)的既定规则，此时改用非对称签名。

## 目标

只有 `user` 能签发令牌；网关和其他服务只能验证令牌，不能伪造。

## 范围内

- `utility/auth.go`：`TokenSigner`（私钥，只在 `user`）与 `TokenVerifier`（公钥，网关和其他服务），算法固定为 EdDSA。
- `utility/http.go`：网关鉴权中间件 `MiddlewareAuth`，默认拒绝，公开路由白名单。
- `utility/auth.go`：gRPC 客户端拦截器 `GrpcClientForwardAuth`，把 HTTP 的 `Authorization` 自动带到每次 gRPC 调用上。网关的 controller 因此只做字段映射，`withCaller` 已删除。
- `utility/grpc.go`：`NewGrpcServerConfig` 统一服务端拦截器链（安全日志 → 鉴权 → 校验），`user` 和 `interaction` 共用。
- `scripts/jwtkeys` 与 `make jwt-keys`：生成本地开发密钥对。

## 范围外

- 密钥轮换（`kid`、多公钥并存）。现在换密钥会让所有已签发的令牌立即失效。
- 刷新令牌、登出、吊销，见任务 0003 的范围外。
- 原教程的 admin 服务与 gateway-admin（教程标注"可以跳过"；其双重 MD5 密码方案不能照搬）。

## 与原教程的差异

| 原教程 | 本仓库 | 原因 |
| --- | --- | --- |
| 密钥写死在源码 `JWTSecretKey = "..."` | 私钥只在 `user` 的本地配置或环境变量里，缺失就拒绝启动 | 写进源码等于公开密钥，任何人都能签发任意用户的令牌 |
| HS256，网关和服务共用一个密钥 | Ed25519，只有 `user` 持有私钥 | 能验签的一方用 HS256 也能签发；公钥泄露不会导致令牌被伪造 |
| `ParseWithClaims` 不限定算法 | 只接受 EdDSA | 防 `alg: none`，以及用公钥当 HMAC 密钥的算法混淆攻击 |
| 只去掉 `"Bearer "` 前缀，其他格式也照样解析 | 必须是 `Bearer <token>` | 头格式不对直接拒绝 |
| 中间件按路由分组，忘记加组的新接口默认公开 | 整个 `/frontend` 默认拒绝，只放行白名单 | 新接口忘了配置时，结果是"拒绝"而不是"裸奔" |
| 只有网关验签，后端信任网关 | 网关验一次，后端服务再验一次 | 后端可能被网关以外的调用方直接访问 |
| `CustomClaims.UserId int` | 标准 `sub` 声明，`uint64` | 和数据库 `BIGINT UNSIGNED` 一致，不发明自定义字段 |

## 验收标准

- `utility/auth_test.go`：签发/验签往返；过期、其他密钥签名、`alg: none`、用公钥做 HS256 密钥、错误签发者、缺 `exp`、非法 `sub`、篡改载荷全部拒绝；配置缺失或格式错误时加载失败。
- `utility/http_test.go`：用真实 GoFrame HTTP 服务验证中间件。受保护路由在无令牌、伪造令牌、只带 `X-User-Id` 时返回 `code:16`；公开路由无令牌也可访问；同一路径换成非公开方法仍受保护；令牌被原样转发到 gRPC metadata。
- 三个进程缺少各自的密钥时都以非 0 退出码拒绝启动。
- `go test ./...`、`go vet ./...`、`go mod verify` 通过，GoFrame 仍为 v2.10.2。
