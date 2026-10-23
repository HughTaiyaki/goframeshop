# search 搜索服务开发

`app/search` 在 Elasticsearch 里搜索商品。它是一个 gRPC 服务，监听 `:31006`，只读、公开，不持有任何密钥，经 gateway-h5 的 `GET /frontend/search/goods` 对外。任务和决定见 [`tasks/0009-search-service.md`](../../tasks/0009-search-service.md)。

## 1. 数据怎么进索引

```text
goods 库 ──binlog──▶ search 服务（canal）──"商品 N 变了"──▶ 用 search_sync 账号查 N 的当前状态
                                                              │
                                     上架且未删除 → 写入 ES ◀──┤
                                     否则        → 从 ES 删除 ◀┘
```

- **监听的表**：`goods_info`（取 `id`）、`inventory`（取 `goods_id`）、`goods_images`（取 `goods_id`）。商品本身、库存销量或封面图有变化，都会触发对应商品的更新。
- **启动顺序**：记下 binlog 位置 → 从 MySQL 全量重建索引 → 从记下的位置开始跟随。这样重建期间的变更会被补上。
- **binlog 只用来通知**：同步器只从事件里按列名取出商品 ID，然后回 MySQL 读取最新状态（`internal/source/mysql.go` 里的 `docQuery`）。所以不依赖列的位置，事件重放、乱序、重复都没有副作用。
- **goods 表结构变化时**：要同步修改 `docQuery`。它是按列名查询的，列被删除或改名时会直接报错，不会悄悄错位。
- **失败时**：同步器对同一个商品按退避间隔一直重试（100ms 起，最长 30s），期间暂停消费，不丢弃事件。

## 2. 索引

- **别名和物理索引**：读写都通过别名 `goods`。每次启动新建一个物理索引 `goods_<时间戳>`，批量写入，刷新后在一个请求里把别名原子地切到新索引，再删掉旧索引。
- **映射**：`dynamic: strict`，定义在 `internal/index/index.go` 的 `Mapping` 里。`name` 和 `tags` 用 IK 分词：写入时用 `ik_max_word`，查询时用 `ik_smart`。
- **只放可售商品**：索引里只有上架且未删除的商品，所以查询时不需要再过滤软删除。

## 3. 查询

| 参数 | 说明 |
| --- | --- |
| `keyword` | 匹配名称（权重 3）和标签；两个词以内必须全部命中，更多词时命中 75% 即可 |
| `categoryId` | 一、二、三级任意一级匹配都算 |
| `brand` | 精确匹配 |
| `minPriceCents` / `maxPriceCents` | 闭区间，0 表示不限 |
| `sort` | `relevance`（默认；没有关键词时按最新）、`price_asc`、`price_desc`、`sales`、`newest`；都以商品 ID 倒序兜底，保证分页稳定 |
| `page` × `size` | 不能超过 10000 |

筛选条件放在 `filter` 里，不参与相关度打分。`highlightedName` 里只有 `<em>` 是原样的 HTML，其他内容都已转义，前端可以直接插入；`name` 是纯文本。

## 4. 本地运行

```bash
make infra-up        # 第一次会构建带 IK 的 ES 镜像
make db-init         # 006（库存）和 007（search_sync 账号）
cd app/search
cp manifest/config/config.yaml.example manifest/config/config.yaml
go run .
```

看看索引和分词：

```bash
curl --noproxy '*' '127.0.0.1:9200/_cat/aliases?v'
curl --noproxy '*' -H 'Content-Type: application/json' 127.0.0.1:9200/_analyze \
  -d '{"analyzer":"ik_smart","text":"全面屏智能电视机"}'
docker compose --profile tools up -d kibana    # 可选：中文界面的 Kibana，http://127.0.0.1:5601
```

本机设置了 HTTP 代理时，curl 访问 `127.0.0.1` 要加 `--noproxy '*'`。

## 5. 测试

```bash
go test ./app/search/...
GOFRAMESHOP_IT=1 go test ./app/search/internal/index/ -run Integration -v   # 需要 Compose 里的 ES
```

集成测试使用自己的别名，结束时按名字删掉自己的索引。ES 8 默认不允许用通配符删除索引，所以不能写成 `DELETE it_goods_*`。

## 6. 已知局限

- 每次启动都全量重建。商品多了以后，应该持久化 binlog 位置。
- 搜索结果比 MySQL 晚约 1 秒，库存和销量以商品详情为准。
- binlog 同步意外停止时会记 ERROR 日志，之后索引不再更新，直到服务重启。
