# file 服务

gRPC 服务，监听 `:31004`，使用 `file` 库：记录已上传文件的元数据。它从不接触文件内容；内容由 gateway-admin 写进对象存储。所有接口都需要管理员令牌，只持有管理员公钥。

开发前请先阅读 [`../../docs/development/admin-and-file.md`](../../docs/development/admin-and-file.md)。

```bash
cp manifest/config/config.yaml.example manifest/config/config.yaml
# 把管理员密钥对的 JWT_PUBLIC_KEY 填进 jwt.publicKey
go run .
```

链路追踪：配置里的 `otel.endpoint` 指向 OpenTelemetry Collector，在 Jaeger（<http://127.0.0.1:16686>）里的服务名是 `file-service`；留空则不追踪。见 [`../../docs/development/tracing.md`](../../docs/development/tracing.md)。
