# banner 服务开发

`app/banner` 管理首页的**轮播图**（rotation）和**手工位**（position，运营挑选的商品位）。gRPC 服务，监听 `:31008`，使用 `banner` 库。任务和决定见 [`tasks/0011-banner-service.md`](../../tasks/0011-banner-service.md)。

## 1. 谁能做什么

| 接口 | 前台 gateway-h5 | 后台 gateway-admin |
| --- | --- | --- |
| 轮播图列表、手工位列表 | 公开：`GET /frontend/rotation`、`GET /frontend/position` | 需要管理员令牌 |
| 新增、修改、删除 | 不存在 | 需要管理员令牌：`/backend/rotation`、`/backend/position` |

## 2. 规则

- **图片**：先用 `POST /backend/upload/image` 上传拿到 `fileId`，新增或修改时传 `fileId`。banner 服务带着同一个管理员令牌到 file 服务确认文件存在，并把 URL 存一份；所以前台读列表时不需要再调用其他服务。
- **跳转链接**：可以为空。不为空时必须是 `http://` 或 `https://` 开头的完整地址。`javascript:`、`data:` 这类地址会被拒绝：它们放进首页的链接里，顾客一点就会执行脚本。
- **手工位商品**：传 `goodsId`。banner 服务到商品服务读取商品名并保存；还没上架的商品也可以提前配置，因为调用时用的是管理员令牌。
- **排序**：`sort` 越小越靠前，相同时按 ID。前台不能改变顺序。
- **修改是整体替换**：图片、链接、排序（手工位还有商品）都要传。
- **删除是软删除**。

## 3. 本地运行

```bash
make db-init           # 009：建 banner 库
cd app/banner
cp manifest/config/config.yaml.example manifest/config/config.yaml
# jwt.publicKey 填管理员那对密钥的公钥（和 file、goods、gateway-admin 一样）
go run .
```

它依赖 file 服务和 goods 服务（新增、修改时会调用），前台读列表不需要它们。

```bash
A=http://127.0.0.1:8299/backend
curl -X POST $A/rotation -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"fileId":1,"link":"https://shop.example.com/sale","sort":1}'
curl -X POST $A/position -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"fileId":1,"goodsId":2,"sort":1}'
curl http://127.0.0.1:8199/frontend/rotation      # 不需要登录
curl http://127.0.0.1:8199/frontend/position
```

## 4. 代码在哪

```text
app/banner/
  manifest/protobuf/{rotation,position}/v1/   协议
  internal/logic/check/                       链接、图片、分页校验（两类共用）
  internal/logic/{rotation,position}/         业务规则
  internal/repository/                        MySQL
  internal/clients/                           调用 file、goods（转发管理员令牌）
deploy/mysql/init/009_banner_schema.sql
```

## 5. 已知局限

- ~~商品名在保存时复制，之后不会同步~~：任务 0013 起由 `goods.changed` 事件同步。商品改名，手工位跟着改；商品下架或删除，前台不再显示这个手工位，后台列表显示 `goodsOnShelf: false`。见 [`events.md`](events.md#52-商品变更goodschanged)。
- 列表没有缓存。
