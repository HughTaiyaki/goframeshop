# paysim 支付模拟器

代替真实支付渠道（微信、支付宝）的本地模拟器，使用自己的 `paysim` 库，订单服务只能通过它的接口打交道。开发说明见 [`../../docs/development/payment.md`](../../docs/development/payment.md)。

| 端口 | 给谁用 | 内容 |
| --- | --- | --- |
| gRPC `:31009` | 订单服务（服务令牌） | 创建、关闭、查询、退款 |
| HTTP `:8399` | 付款人（浏览器或 curl） | 收银台页面 `GET /checkout/{支付单号}`，付款 `POST /checkout/{支付单号}/pay` |

付款成功后，模拟器带签名 POST 到 `notify.url`（经 gateway-h5 转给订单服务），没有确认就按 2s、4s、8s…… 重试。

```bash
cp manifest/config/config.yaml.example manifest/config/config.yaml
# jwtOrder.publicKey：订单服务那对密钥的公钥（和 goods 的 jwtOrder 一样）
# notify.privateKey：再生成一对（make jwt-keys），私钥填这里，公钥填进订单服务的 paysim.publicKey
go run .
```

演练"回调丢失"：`curl -X POST 'http://127.0.0.1:8399/checkout/<支付单号>/pay?notify=0'`。

链路追踪：配置里的 `otel.endpoint` 指向 OpenTelemetry Collector，在 Jaeger（<http://127.0.0.1:16686>）里的服务名是 `payment-simulator`；留空则不追踪。见 [`../../docs/development/tracing.md`](../../docs/development/tracing.md)。
