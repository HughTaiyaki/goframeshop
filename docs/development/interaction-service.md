# interaction 服务开发

`app/interaction` 负责点赞（praise）、收藏（collection）和评论（comment）。它是一个 gRPC 服务，监听 `:31002`，使用独立的 `interaction` 库，只持有 JWT **公钥**。

任务与验收见 [`tasks/0005-interaction-service.md`](../../tasks/0005-interaction-service.md)，分层的心智模型见 [`tutorial/02-代码分四层.md`](../tutorial/02-代码分四层.md)。

## 1. 结构

```text
manifest/protobuf/{praise,collection,comment}/v1/*.proto   协议（手写）
api/{praise,collection,comment}/v1/                         gf gen pb 生成，不手改
internal/controller/{praise,collection,comment}/            薄 gRPC 层：取身份 → 调 logic → 映射
internal/service/                                           接口、领域类型、错误
internal/logic/mark/                                        点赞和收藏共用的业务规则
internal/logic/comment/                                     评论业务规则
internal/repository/                                        SQL 语义（基于生成的 DAO）
internal/dao、internal/model/{do,entity}                     gf gen dao 生成，不手改
```

点赞和收藏的表结构、规则完全一样，所以共用一个 `service.IMark` 和 `logic/mark`。区别只在 `repository.NewPraise()` 和 `repository.NewCollection()` 绑定了不同的表。以后如果两者的规则开始分化，再拆成两个包。

## 2. 规则

- **身份**只来自令牌。请求消息里没有 `user_id` 字段；controller 用 `controller.RequireUserID` 取身份。
- **默认拒绝**：`internal/cmd/cmd.go` 的 `publicMethods` 里只有 `ListComments`。新增 RPC 默认需要登录。
- **对象类型**：`1` 商品、`2` 文章，在 proto（`v:in:1,2`）和 logic（`service.ValidObjectType`）两处校验。`object_id` 指向别的服务，目前不校验对象是否存在。
- **点赞和收藏**：
  - 每个用户对同一对象只能有一条，由唯一键 `(user_id, object_type, object_id)` 保证。重复创建时 repository 识别 MySQL 1062 错误，返回 `ErrAlreadyExists`。不要改成"先查后插"：两个并发请求可能都通过检查。
  - 物理删除，删除后可以再次创建。
- **评论**：
  - 内容去掉首尾空白后需要 1 到 255 个字符（按字符数，不按字节数）。
  - 回复的父评论必须存在、未删除，并且属于同一个对象。
  - 软删除，只有作者能删。父评论删除后，回复保留。
  - 列表是扁平结构、按 ID 倒序，客户端根据 `parentId` 拼成树。
- **越权**：删除别人的记录与删除不存在的记录都返回 `NotFound`，不暴露这个 ID 是否存在。写入语句本身也带 `user_id` 条件，不只依赖事先的检查。

## 3. 数据库与代码生成

建表脚本是 `deploy/mysql/init/002_interaction_schema.sql`，它同时给 `shop` 账号授予该库的增删改查权限。

```bash
make db-init                # 已有数据卷时手动补建；全新数据卷会自动执行
cd app/interaction
gf gen dao                  # 依据 hack/config.yaml 连接本地库（make dev-config 生成）
gf gen pb                   # 修改 .proto 之后
```

这里不生成 pbentity：数据库行永远不作为公共 API 返回。

## 4. 本地启动

```bash
cd app/interaction
cp manifest/config/config.yaml.example manifest/config/config.yaml
# 把 make jwt-keys 输出的 JWT_PUBLIC_KEY 填进 jwt.publicKey
go run .
```

完整演示见 [`tutorial/03-自己跑一遍.md`](../tutorial/03-自己跑一遍.md)。

## 5. 新增一个接口的检查清单

1. 修改 `.proto`，加上 `v:` 校验注释，执行 `gf gen pb`。
2. 在 `service` 加接口方法，在 `logic` 实现，并用内存 fake 写测试。
3. 如果这个 RPC 需要公开，把它加进 `publicMethods`，在 gateway 的 `publicRoutes` 加上对应路由，并补测试证明它确实公开、其他接口仍受保护。
4. 在 gateway 的 `api/interaction/v1` 加上 HTTP 结构体，执行 `gf gen ctrl`，填写字段映射。
