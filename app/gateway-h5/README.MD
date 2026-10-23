# H5 网关

HTTP 服务，监听 `:8199`，把浏览器的 HTTP/JSON 请求翻译成对 `user`、`interaction`、`goods`、`search`、`order` 和 `banner` 的 gRPC 调用。商品浏览和搜索是公开的；支付模拟器的回调 `POST /frontend/payment/notify` 也是公开的，它靠签名证明身份（由订单服务验签），网关原样转发请求体和签名头。它没有数据库，只持有 JWT 公钥：验签不通过的请求直接拒绝，通过的请求把令牌原样转发给后端。

- 请求链路：[`../../docs/tutorial/01-一次请求的完整旅程.md`](../../docs/tutorial/01-一次请求的完整旅程.md)
- 公开路由白名单：`internal/cmd/cmd.go` 的 `publicRoutes`
- HTTP 接口定义在 `api/`；改完执行 `gf gen ctrl`，再填写 `internal/controller` 里的字段映射

本地启动：

```bash
cp manifest/config/config.yaml.example manifest/config/config.yaml
# 把 `make jwt-keys` 输出的 JWT_PUBLIC_KEY 填进 jwt.publicKey
go run .
```

Swagger：<http://127.0.0.1:8199/swagger>

链路追踪：配置里的 `otel.endpoint` 指向 OpenTelemetry Collector，在 Jaeger（<http://127.0.0.1:16686>）里的服务名是 `gateway-h5`；留空则不追踪。见 [`../../docs/development/tracing.md`](../../docs/development/tracing.md)。
