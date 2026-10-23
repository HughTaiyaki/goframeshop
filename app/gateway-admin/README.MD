# 后台网关

HTTP 服务，监听 `:8299`，把后台的 HTTP 请求翻译成对 `admin`、`file`、`goods`、`order` 和 `banner` 服务的 gRPC 调用，并负责把上传的图片写入对象存储（七牛或本地目录）。它没有数据库，只持有**管理员**令牌的公钥。商城用户的令牌由另一对密钥签发，在这里会被拒绝。

- 开发说明：[`../../docs/development/admin-and-file.md`](../../docs/development/admin-and-file.md)
- 公开路由白名单：`internal/cmd/cmd.go` 的 `publicRoutes`（只有登录）

本地启动：

```bash
cp manifest/config/config.yaml.example manifest/config/config.yaml
# 把管理员密钥对的 JWT_PUBLIC_KEY 填进 jwt.publicKey
go run .
```

默认把图片存到 `./uploads` 并通过 `/files/` 提供访问。改用七牛时，把 `storage.driver` 改成 `qiniu`，并填入**你自己的**七牛凭据。

链路追踪：配置里的 `otel.endpoint` 指向 OpenTelemetry Collector，在 Jaeger（<http://127.0.0.1:16686>）里的服务名是 `gateway-admin`；留空则不追踪。见 [`../../docs/development/tracing.md`](../../docs/development/tracing.md)。
