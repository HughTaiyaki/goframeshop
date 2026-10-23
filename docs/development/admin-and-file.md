# 后台：admin、file 与 gateway-admin

后台由三个进程组成，任务与决定见 [`tasks/0006-admin-file-upload.md`](../../tasks/0006-admin-file-upload.md)。

```text
浏览器（后台）
  │  POST /backend/upload/image   Authorization: Bearer <管理员令牌>
  ▼
gateway-admin (:8299)   管理员公钥验签 → 校验图片 → 写入对象存储
  │  gRPC CreateFile（令牌原样转发）
  ▼
file (:31004)           管理员公钥再验一次 → uploader_id 取自令牌 → 写 file 库
```

## 1. 两套身份

| | 商城用户 | 管理员 |
| --- | --- | --- |
| 签发方 | `user` | `admin` |
| 签发者 `iss` | `goframeshop-user` | `goframeshop-admin` |
| 密钥对 | 第一对 | **另一对**，用 `make jwt-keys` 再生成一次 |
| 验签方 | gateway-h5、interaction | gateway-admin、file |

两套令牌签名用的密钥不同、签发者也不同，所以商城用户的令牌拿到后台会被拒绝，反过来也一样。千万不要把同一对密钥配给两边。

## 2. 管理员

- 没有公开注册。创建第一个超级管理员：

  ```bash
  cd app/admin
  ADMIN_PASSWORD='至少 12 位的密码' go run . create-admin -name root
  ```

  不设 `ADMIN_PASSWORD` 时会从标准输入读取密码。密码不要放在命令行参数里，否则会留在 shell 历史和进程列表中。
- 之后，超级管理员可以调用 `POST /backend/admin` 创建管理员。普通管理员调用会得到 `code:7`（PermissionDenied）。
- 每次调用都会从数据库读取"是否超级管理员"，降级或删除管理员会立即生效。

## 3. 上传

- 表单字段名为 `file`（`multipart/form-data`），默认上限 5 MiB（`upload.maxImageBytes`）。
- 只接受 JPEG、PNG、GIF、WebP。类型由文件内容判断，扩展名也由内容决定；SVG 可能携带脚本，所以被拒绝。
- 对象 key：`images/YYYY/MM/DD/<32 位十六进制随机数>.<ext>`。原始文件名去掉路径和控制字符后存进 `file_info.name`，只用于展示。
- 返回 `fileId` 和 `url`。**其他表应保存 `fileId`**，不要复制 URL（教程"图片关联表结构优化"一节的思路）。
- 先写存储、再记录元数据。记录失败时删除刚写入的对象；删除也失败时，日志里会有 `orphaned object <key>`。

## 4. 存储后端

`storage.driver` 选择后端，代码在 `app/gateway-admin/internal/storage`：

| driver | 说明 |
| --- | --- |
| `local`（默认） | 写到 `storage.local.dir`（默认 `app/gateway-admin/uploads/`，已被 Git 忽略），由网关在 `/files/` 下提供访问。只适合单机开发 |
| `qiniu` | 上传到七牛 Kodo，需要 `qiniu.accessKey/secretKey/bucket/domain/region` |

启用七牛的步骤：

1. 用**你自己的**七牛账号在控制台创建存储空间，记下空间名、所在区域和下载域名。
2. 在"密钥管理"中获取 AccessKey/SecretKey。**不要使用教程里贴出的密钥**，那是别人的账号。
3. 填进 `app/gateway-admin/manifest/config/config.yaml`（已被 Git 忽略），或者导出 `QINIU_ACCESS_KEY` / `QINIU_SECRET_KEY`。
4. 把 `storage.driver` 改成 `qiniu`，`qiniu.region` 填空间所在区域（`z0` 华东、`z1` 华北、`z2` 华南）。
5. 上传一张图片，确认返回的 URL 可以访问。

## 5. 本地启动

```bash
make db-init        # 已有数据卷时补建 admin、file 库
make jwt-keys       # 再生成一对，专门给管理员用
for svc in admin file gateway-admin; do
  cp app/$svc/manifest/config/config.yaml.example app/$svc/manifest/config/config.yaml
done
# 私钥 → app/admin 的 jwt.privateKey；公钥 → app/file 和 app/gateway-admin 的 jwt.publicKey
```

然后创建超级管理员（见第 2 节），再分别启动 `app/admin`、`app/file`、`app/gateway-admin`。演示命令见 [`tutorial/03-自己跑一遍.md`](../tutorial/03-自己跑一遍.md#后台管理员与图片上传)。

## 6. 注意

- `gf gen ctrl` 只认零参数的 `NewV1()`。改成带参数后，生成器会在 `*_new.go` 末尾追加一个空的 `NewV1`，导致编译失败。依赖应在 `NewV1()` 内部从配置构建；配置是否有效，由 `internal/cmd` 在绑定路由前先检查。
- 本地存储目录是静态资源目录，绕过鉴权中间件（图片要给顾客看）。只有经过校验的图片才会写进这里。
