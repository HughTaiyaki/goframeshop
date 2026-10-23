# search 搜索服务

gRPC 服务，监听 `:31006`：在 Elasticsearch（带 IK 中文分词）里搜索商品。只读、公开、不持有任何密钥；经 gateway-h5 的 `GET /frontend/search/goods` 对外。

索引是 goods 库的只读投影：每次启动先全量重建，再通过 binlog 跟随变化。开发说明见 [`../../docs/development/search-service.md`](../../docs/development/search-service.md)。

```bash
make infra-up && make db-init       # 需要 elasticsearch 和 search_sync 账号
cp manifest/config/config.yaml.example manifest/config/config.yaml
go run .
```

链路追踪：配置里的 `otel.endpoint` 指向 OpenTelemetry Collector，在 Jaeger（<http://127.0.0.1:16686>）里的服务名是 `search-service`；留空则不追踪。见 [`../../docs/development/tracing.md`](../../docs/development/tracing.md)。
