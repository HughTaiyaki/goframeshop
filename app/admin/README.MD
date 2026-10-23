# admin 服务

gRPC 服务，监听 `:31003`，使用 `admin` 库：管理员登录和创建管理员。它持有**管理员**密钥对的私钥（和 user 服务的那对不是同一对）。

开发前请先阅读 [`../../docs/development/admin-and-file.md`](../../docs/development/admin-and-file.md)。

```bash
cp manifest/config/config.yaml.example manifest/config/config.yaml
# 把管理员密钥对的 JWT_PRIVATE_KEY 填进 jwt.privateKey
ADMIN_PASSWORD='至少 12 位的密码' go run . create-admin -name root   # 首次：创建超级管理员
go run .
```

链路追踪：配置里的 `otel.endpoint` 指向 OpenTelemetry Collector，在 Jaeger（<http://127.0.0.1:16686>）里的服务名是 `admin-service`；留空则不追踪。见 [`../../docs/development/tracing.md`](../../docs/development/tracing.md)。
