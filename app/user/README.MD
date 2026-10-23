# 用户服务

gRPC 服务，监听 `:31001`，使用 `user` 库：注册、登录、资料、修改密码和收货地址 CRUD。它是唯一持有 JWT 私钥、能签发访问令牌的服务。

开发前请先阅读 [`../../docs/development/user-service.md`](../../docs/development/user-service.md)。

本地启动：

```bash
cp manifest/config/config.yaml.example manifest/config/config.yaml
# 把 `make jwt-keys` 输出的 JWT_PRIVATE_KEY 填进 jwt.privateKey
go run .
```

`manifest/config/config.yaml` 只在本机使用，已被 Git 忽略；仓库提交的是 `config.yaml.example`。

链路追踪：配置里的 `otel.endpoint` 指向 OpenTelemetry Collector，在 Jaeger（<http://127.0.0.1:16686>）里的服务名是 `user-service`；留空则不追踪。见 [`../../docs/development/tracing.md`](../../docs/development/tracing.md)。
