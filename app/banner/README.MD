# banner 服务

gRPC 服务，监听 `:31008`，使用 `banner` 库：首页轮播图和手工位。列表公开，写接口只接受管理员令牌。开发说明见 [`../../docs/development/banner-service.md`](../../docs/development/banner-service.md)。

```bash
cp manifest/config/config.yaml.example manifest/config/config.yaml
# 把管理员密钥对的 JWT_PUBLIC_KEY 填进 jwt.publicKey
go run .
```

手工位里的商品名和"是否在售"由 goods 的 `goods.changed` 事件保持同步（需要 `rabbitmq.url`）。前台列表只显示在售商品的手工位，后台显示全部。见 [`../../docs/development/events.md`](../../docs/development/events.md#52-商品变更goodschanged)。

链路追踪：配置里的 `otel.endpoint` 指向 OpenTelemetry Collector，在 Jaeger（<http://127.0.0.1:16686>）里的服务名是 `banner-service`；留空则不追踪。见 [`../../docs/development/tracing.md`](../../docs/development/tracing.md)。
