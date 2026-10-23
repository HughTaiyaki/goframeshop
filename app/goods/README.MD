# goods 商品服务

gRPC 服务，监听 `:31005`，使用 `goods` 库和 Redis：商品、商品图片、三级分类。读接口公开，并且走 Redis 缓存；写接口只接受管理员令牌（只持有管理员公钥）。

开发前请先阅读 [`../../docs/development/goods-service.md`](../../docs/development/goods-service.md)。

```bash
cp manifest/config/config.yaml.example manifest/config/config.yaml
# 把管理员密钥对的 JWT_PUBLIC_KEY 填进 jwt.publicKey
go run .
```

改名、上下架、删除商品时会在同一个事务里写 `goods.changed` 事件，由投递器发给 RabbitMQ，所以配置里还要有 `rabbitmq.url`。见 [`../../docs/development/events.md`](../../docs/development/events.md)。

链路追踪：配置里的 `otel.endpoint` 指向 OpenTelemetry Collector，在 Jaeger（<http://127.0.0.1:16686>）里的服务名是 `goods-service`；留空则不追踪。见 [`../../docs/development/tracing.md`](../../docs/development/tracing.md)。
