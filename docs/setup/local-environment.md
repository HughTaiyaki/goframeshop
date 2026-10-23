# 本地开发环境

本文将用户提供的操作资料整理为可复现的本地环境。命令默认在 macOS、zsh 和仓库根目录下执行。

## 1. 版本要求

| 工具 | 最低或锁定版本 | 检查命令 |
| --- | --- | --- |
| Go | 1.24.5 | `go version` |
| GoFrame CLI | 2.10.2 | `gf version` |
| protoc | 当前兼容 Protobuf v3 的版本 | `protoc --version` |
| protoc-gen-go | 1.36.x 或更高版本 | `protoc-gen-go --version` |
| protoc-gen-go-grpc | 1.5.x 或更高版本 | `protoc-gen-go-grpc --version` |
| Docker | 支持 Compose v2 | `docker compose version` |

运行仓库内置检查：

```bash
make env-check
```

如果 Docker Desktop 已安装但守护进程没有启动，该检查会按预期失败。

## 2. 安装缺失的 Go 工具

复现本项目时使用固定版本：

```bash
go install github.com/gogf/gf/cmd/gf/v2@v2.10.2
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
```

确保 Go 的二进制目录在 `PATH` 中：

```bash
go env GOPATH
export PATH="$(go env GOPATH)/bin:$PATH"
```

只有该目录尚未加入环境变量时，才需要把 `export` 写入 `~/.zshrc`。

## 3. 初始化仓库

以下命令已经执行过。对于当前已有内容的仓库，不要重复执行：

```bash
gf init . -m -g github.com/hughtaiyaki/goframeshop
gf init app/user -a
```

仓库已经在 `go.mod` 中锁定 GoFrame 版本。与升级无关的功能开发中不要执行 `gf up -a`。

具体生成了哪些文件，参见[从零搭建与文件来源](project-origin.md)。

## 4. 本地配置

创建被 Git 忽略的运行时配置：

```bash
for svc in user interaction gateway-h5 admin file goods search order paysim banner gateway-admin; do
  cp app/$svc/manifest/config/config.yaml.example app/$svc/manifest/config/config.yaml
done
make jwt-keys   # 第一对：商城用户
make jwt-keys   # 第二对：管理员
make jwt-keys   # 第三对：订单服务的服务身份
make jwt-keys   # 第四对：支付模拟器给回调签名
```

`make jwt-keys` 每次打印一对新的 Ed25519 密钥，需要执行四次：

| 密钥对 | 私钥填进 | 公钥填进 |
| --- | --- | --- |
| 商城用户 | `app/user` 的 `jwt.privateKey` | `app/interaction`、`app/gateway-h5` 的 `jwt.publicKey` |
| 管理员 | `app/admin` 的 `jwt.privateKey` | `app/file`、`app/goods`、`app/banner`、`app/gateway-admin` 的 `jwt.publicKey`，以及 `app/order` 的 `jwtAdmin.publicKey` |
| 订单服务 | `app/order` 的 `jwtService.privateKey` | `app/goods`、`app/paysim` 的 `jwtOrder.publicKey` |
| 支付回调 | `app/paysim` 的 `notify.privateKey` | `app/order` 的 `paysim.publicKey` |

另外，`app/order` 的 `jwtUser.publicKey` 填商城用户那一对的公钥。

也可以不写进文件，启动时导出 `JWT_PRIVATE_KEY` / `JWT_PUBLIC_KEY`。

提交的示例文件使用 Compose 的本地账号。生产凭据必须由部署平台的 Secret 管理能力提供。

## 5. 启动 MySQL 和 etcd

先启动 Docker Desktop，再检查并启动容器：

```bash
docker compose config
make infra-up
make infra-ps
```

预期服务：

- MySQL 8.4.11：`127.0.0.1:3306`
- etcd 3.6.1 客户端端点：`127.0.0.1:2379`
- Elasticsearch 8.11.0（带 IK）：`127.0.0.1:9200`。镜像在第一次 `make infra-up` 时由 `deploy/elasticsearch/Dockerfile` 构建，需要联网下载 IK 插件，并校验 sha256。没有开启安全认证，只用于本地开发。本机设置了 HTTP 代理时，curl 访问它要加 `--noproxy '*'`。Kibana 是可选的：`docker compose --profile tools up -d kibana`，访问 `127.0.0.1:5601`。
- Redis 7.4.2：`127.0.0.1:6379`，密码默认 `change-me`（可用 `REDIS_PASSWORD` 覆盖）。只做缓存：不持久化，`/data` 是 tmpfs，容器重启后一定是空的。
- OpenTelemetry Collector 0.130.0：OTLP gRPC `127.0.0.1:4317`、HTTP `127.0.0.1:4318`，配置在 `deploy/otel/collector.yaml`。Jaeger 2.11.0：界面 <http://127.0.0.1:16686>，数据只在内存里，重启清空。两者都没有登录，只用于本地。每个服务的配置需要 `otel.endpoint`，见 [`development/tracing.md`](../development/tracing.md#3-数据怎么走)；留空则不追踪。
- RabbitMQ 4.2.0：`127.0.0.1:5672`，管理界面 <http://127.0.0.1:15672>，账号默认 `shop` / `change-me`（可用 `RABBITMQ_USER`、`RABBITMQ_PASSWORD` 覆盖，只用于本地）。数据在 `rabbitmq-data` 卷里；就算丢了也不会丢事件，事件的记录在各服务的 `outbox_event` 表里。order、goods、banner 的配置需要 `rabbitmq.url`，见 [`development/events.md`](../development/events.md#6-本地运行)。

MySQL 第一次启动时会按文件名顺序执行 `deploy/mysql/init/` 下的脚本（`001_user_schema.sql` 到 `013_event_trace_schema.sql`；其中 006、011、012、013 是对已有数据也安全的迁移；010 建的是支付模拟器自己的库 `paysim`；012 在 orders、goods、banner 里各建一套事件表）。命名数据卷会在容器停止后继续保留数据。

**已有数据卷时，新加的脚本不会自动执行。** 所有脚本都可以重复执行，用下面的命令补建，不会丢数据：

```bash
make db-init
```

健康检查：

```bash
docker compose exec mysql \
  mysqladmin ping -h 127.0.0.1 -uroot -p"$MYSQL_ROOT_PASSWORD"
docker compose exec etcd etcdctl endpoint health
```

停止容器但保留数据：

```bash
make infra-down
```

`make infra-reset` 会删除两个命名数据卷，只能在明确需要重建全部本地数据时使用。

## 6. 网络与代理排错

镜像仓库或 Go 代理请求失败时，先区分 Docker 守护进程、DNS、本地代理和远端仓库故障：

```bash
docker info
docker pull mysql:8.4.11
go env GOPROXY
go mod download
```

对于读取 Shell 代理变量的工具，可以临时显式启用本机代理：

```bash
export https_proxy=http://127.0.0.1:6789
export http_proxy=http://127.0.0.1:6789
export all_proxy=socks5://127.0.0.1:6789
```

这些环境变量只对当前终端及其子进程生效。新开终端后需要重新设置，除非写入 Shell 配置。

Docker Desktop 拉取镜像时使用的是 Desktop 自己的 Containers Proxy，而不是 `docker compose` 进程本身。应让 Docker Desktop 使用 macOS 系统代理，或在 **Settings > Resources > Proxies** 中配置。仅在一条 `docker compose` 命令前设置代理变量，并不能配置 Desktop 引擎。

不要把大量未经验证的公共镜像源写进 Docker 守护进程配置。优先使用官方仓库、组织维护的可信镜像，或临时切换网络。本仓库的配置不得修改机器级 Docker 设置。

## 7. 验证

```bash
go mod tidy
go mod verify
go test ./...
```

只有 Compose 中的两个服务均健康，并且 Go 检查通过，才能认为本地环境准备完成。

## 官方参考

- [GoFrame 项目初始化](https://goframe.org/docs/cli/init)
- [GoFrame 代码生成](https://goframe.org/docs/cli/gen)
- [etcd 容器安装](https://etcd.io/docs/v3.6/install/)
- [MySQL 官方镜像](https://hub.docker.com/_/mysql)
- [Docker Desktop 代理设置](https://docs.docker.com/desktop/settings-and-maintenance/settings/)
