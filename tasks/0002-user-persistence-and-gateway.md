# 任务 0002：用户服务持久化与 H5 网关

状态：已完成用户与地址业务的持久化、etcd 服务注册、gateway-h5 端到端打通。

> 后续变更：下文的 `x-user-id` / `X-User-Id` 临时身份传递已被[任务 0003](0003-jwt-auth.md) 的 JWT 鉴权替换并删除。本文保留为历史记录。

## 目标

让任务 0001 里定义好的 8 个 gRPC 方法真正落到 MySQL，并新建一个 HTTP 网关把浏览器请求翻译成 gRPC 调用。

## 范围内

- `utility/`：共享的时间转换、gRPC 客户端超时拦截器、身份 metadata 传递、CORS 中间件、etcd resolver 注册。
- `app/user`：`internal/service`（接口 + 领域类型 + 错误）、`internal/repository`（DAO 实现）、`internal/logic`（Argon2id 密码哈希、默认地址事务互斥、越权判断）。
- `app/user`：etcd 服务注册，`main.go` 与配置新增 `etcd.address`。
- `app/gateway-h5`：`gf init` 建服务，定义 HTTP api，`gf gen ctrl` 生成控制器，实现 gRPC 客户端调用。

## 范围外

- JWT 签发与 gRPC 鉴权拦截器。地址与资料类 RPC 通过 metadata `x-user-id` 取身份，**由上游填写且不校验**。
- 订单、库存、支付、商品目录。
- OpenTelemetry、日志诊断 Agent。
- `gateway-admin`。

## 已采用的决定

| 决定 | 原因 |
| --- | --- |
| 密码用 Argon2id，盐写入 `password_salt` | MD5 太快，可被批量撞库；schema 本来就设计了独立盐列 |
| 登录失败与用户不存在返回同一个错误 | 分开会让接口变成用户名枚举工具 |
| 跨用户访问地址返回 `NotFound` 而不是 `Forbidden` | 避免泄露"这个 ID 存在" |
| 默认地址切换放进 repository 的事务里 | 调用方无法忘记开事务 |
| 列表固定按 `id` 降序 | 否则分页结果不稳定，同一页可能重复或漏行 |
| 软删除而不是物理删除 | 已下单引用的地址仍需可解析 |
| 公共响应不使用 `pbentity` | 数据库结构与 API 契约的演进节奏不同 |
| etcd 缺失时警告而非 panic | 单服务应能脱离 etcd 开发 |

## 验收标准

- `gf gen pb`、`gf gen ctrl` 产物未被手工修改。
- `go test ./...`、`go vet ./...`、`go mod verify` 全部通过。
- 业务逻辑测试不依赖 MySQL（用内存 fake repository）。
- 地址列表、跨用户隔离、默认地址互斥、密码哈希不可逆均有测试覆盖。
- GoFrame 及相关 contrib 模块锁定 v2.10.2。

## 验证命令

```bash
cd app/gateway-h5 && gf gen ctrl
cd ../..
go test ./...
go vet ./...
go mod verify
go list -m github.com/gogf/gf/v2   # 必须是 v2.10.2
```

端到端 curl 见 [`docs/tutorial/03-自己跑一遍.md`](../docs/tutorial/03-自己跑一遍.md)。
