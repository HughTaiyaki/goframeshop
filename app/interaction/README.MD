# interaction 服务

gRPC 服务，监听 `:31002`，使用 `interaction` 库：点赞、收藏、评论。只持有 JWT 公钥；除评论列表外，所有接口都需要登录。

开发前请先阅读 [`../../docs/development/interaction-service.md`](../../docs/development/interaction-service.md)。

本地启动：

```bash
cp manifest/config/config.yaml.example manifest/config/config.yaml
# 把 `make jwt-keys` 输出的 JWT_PUBLIC_KEY 填进 jwt.publicKey
go run .
```

`manifest/config/config.yaml` 只在本机使用，已被 Git 忽略；仓库提交的是 `config.yaml.example`。

链路追踪：配置里的 `otel.endpoint` 指向 OpenTelemetry Collector，在 Jaeger（<http://127.0.0.1:16686>）里的服务名是 `interaction-service`；留空则不追踪。见 [`../../docs/development/tracing.md`](../../docs/development/tracing.md)。
