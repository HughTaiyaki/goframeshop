# order 订单服务

gRPC 服务，监听 `:31007`，使用 `orders` 库：下单、支付、取消、超时关单、订单查询、售后申请。下单时向 goods 预留库存；付款后提交库存；关单时先请支付模拟器关闭支付单，再释放库存；付款晚于关单时自动退款。开发说明见 [`../../docs/development/order-service.md`](../../docs/development/order-service.md) 和 [`../../docs/development/payment.md`](../../docs/development/payment.md)。

需要四把钥匙：

| 配置 | 内容 |
| --- | --- |
| `jwtUser.publicKey` | 用户公钥 |
| `jwtAdmin.publicKey` | 管理员公钥 |
| `jwtService.privateKey` | 订单服务**自己的**私钥（公钥配给 goods 和 paysim 的 `jwtOrder`） |
| `paysim.publicKey` | 支付模拟器回调签名的公钥 |

另外需要 `rabbitmq.url`：订单进入待支付时写一条延迟事件，到期那一刻经 RabbitMQ 送回来关单（数据库扫描兜底），见 [`../../docs/development/events.md`](../../docs/development/events.md)。

```bash
cp manifest/config/config.yaml.example manifest/config/config.yaml
go run .
```

链路追踪：配置里的 `otel.endpoint` 指向 OpenTelemetry Collector，在 Jaeger（<http://127.0.0.1:16686>）里的服务名是 `order-service`；留空则不追踪。见 [`../../docs/development/tracing.md`](../../docs/development/tracing.md)。
