# 项目从零搭建与文件来源

本文说明 `goframeshop` 从空目录开始执行过哪些操作，以及当前每类文件由谁产生、是否允许手工修改。

本文主要记录协议任务之前的搭建历史；用户 gRPC 协议完成后的现状见[文档索引](../index.md)和[任务 0001 报告](../reports/0001-user-grpc-contract.md)。

## 1. 判断依据

本仓库目前没有 Git 提交记录，因此无法通过 commit 精确还原每一次修改。本文依据以下证据整理：

- GoFrame CLI 的标准 MonoRepo 和 MonoRepo App 结构。
- 文件头中的 `Code generated`、`DO NOT EDIT` 等标记。
- 已有文档记录的命令。
- 文件修改时间：MonoRepo 模板、`app/user` 脚手架和数据库代码分别成批产生。
- 当前 SQL、CLI 配置和生成结果之间的对应关系。

来源分为五类：

| 来源 | 含义 |
| --- | --- |
| **GoFrame 生成** | 由 `gf init` 或 `gf gen` 创建，不是逐行手写 |
| **手写** | 为本项目新增的配置、脚本、SQL 或文档 |
| **混合** | 先由模板生成，随后为本项目修改 |
| **Go 工具维护** | 由 `go mod tidy`、`go mod download` 等命令计算和更新 |
| **本机生成** | 操作系统或本地运行过程产生，不属于项目源代码 |

## 2. 从零开始做了什么

### 第一步：整理原始教程

先阅读用户提供的 GoFrame 商城操作资料，没有直接照抄其中的 `latest` 依赖、root 数据库连接、匿名 etcd 和公共镜像源配置。修订原因记录在 [`source-notes.md`](source-notes.md)。

### 第二步：检查开发工具

检查 Go、GoFrame CLI、`protoc`、`protoc-gen-go`、`protoc-gen-go-grpc` 和 Docker。项目采用以下基线：

```text
Go >= 1.24.5
GoFrame CLI / Framework = 2.10.2
protoc-gen-go = 1.36.10
protoc-gen-go-grpc = 1.5.1
```

对应的安装命令和代理说明在 [`local-environment.md`](local-environment.md)。代理变量只用于解决下载网络问题，不会生成项目文件。

### 第三步：生成 GoFrame MonoRepo

在用户已经创建的 `goframeshop` 目录中执行：

```bash
gf init . -m -g github.com/hughtaiyaki/goframeshop
```

这一步生成三个示例模板 `app/api-template`、`app/job-template`、`app/svc-template`，以及根目录的 `hack/`、`utility/` 和 `.gitattributes` 等基础文件。它们展示 GoFrame MonoRepo 结构，不代表商城功能已经实现。

### 第四步：生成用户服务脚手架

执行：

```bash
gf init app/user -a
```

这一步生成 `app/user`，包括入口、Hello 示例、Controller、Makefile、部署模板、Dockerfile、资源占位目录和 CLI 配置。此时它只是可以启动的 HTTP 示例，没有用户注册、登录或地址管理逻辑。

### 第五步：固定 Go 模块

模块名设置为：

```text
github.com/hughtaiyaki/goframeshop
```

在 `go.mod` 中固定 Go 1.24.5 和 GoFrame v2.10.2，然后执行依赖整理或下载，生成 `go.sum`。`go.mod` 是人工决策与 Go 工具共同维护的文件；`go.sum` 由 Go 工具计算，不能手工编造。

### 第六步：手写本地基础设施

新增或改写以下内容：

- `compose.yaml`：定义 MySQL 8.4.11、etcd 3.6.1、健康检查、端口和命名数据卷。
- `.env.example`：列出本地环境变量和需要自己设置的密码（占位符 `change-me`）。
- `deploy/mysql/init/001_user_schema.sql`：创建 `user_info` 和 `consignee_info` 表。
- `scripts/check-env.sh`：检查工具版本和 Docker 守护进程。
- 根 `Makefile`：封装环境检查、基础设施启停和验证命令。
- `.gitignore`：忽略本地配置、构建产物和系统文件。

这些是针对本项目手写或在生成模板上改写的工程文件，不是 GoFrame 根据业务自动生成的。

### 第七步：启动并检查本地依赖

执行过以下流程：

```bash
docker compose config
make infra-up
make infra-ps
```

MySQL 首次创建数据卷时自动执行 SQL 初始化脚本，随后检查 MySQL 与 etcd 的状态。容器现在是否仍在运行，应以 `make infra-ps` 的实时结果为准。

### 第八步：准备用户服务配置

在 GoFrame 生成的 `app/user/hack/config.yaml` 基础上，改成使用 `shop` 本地账号连接 Compose MySQL，并配置 DAO 与 pbentity 生成。

同时新增 `app/user/manifest/config/config.yaml.example`。本地运行所需的 `config.yaml` 由示例复制产生并被 Git 忽略：

```bash
cp app/user/manifest/config/config.yaml.example \
  app/user/manifest/config/config.yaml
```

### 第九步：从数据库生成持久化代码

MySQL 表创建完成后执行：

```bash
cd app/user
gf gen dao
gf gen pbentity
```

`gf gen dao` 读取数据库表结构并生成 DAO、DO 和 Entity；`gf gen pbentity` 生成与数据库表对应的 `.proto` 消息。这里生成的是数据库映射，不是对外 gRPC API。

### 第十步：建立 AI Coding 文档闭环

手写 `README.MD`、`AGENTS.md`、`docs/` 和 `tasks/`：

- `README.MD` 负责让人快速启动项目。
- `AGENTS.md` 约束 AI 如何按需加载上下文和验证修改。
- `docs/index.md` 是渐进式披露入口。
- `docs/setup/` 说明环境、教程修订和文件来源。
- `docs/architecture/` 说明系统边界和实现顺序。
- `docs/development/` 说明领域开发约束。
- `tasks/` 把下一次实现限制在可验收的小范围内。

项目在初始搭建阶段停在这里。当时 `tasks/0001-user-grpc-contract.md` 只是任务说明，正式 Protobuf 协议和业务代码都还没有创建；协议现已完成，业务代码仍待实现。

## 3. 根目录逐项说明

| 路径 | 来源 | 作用与修改规则 |
| --- | --- | --- |
| `.gitattributes` | GoFrame 生成 | Git 属性模板，一般不需要修改 |
| `.gitignore` | 混合 | 基于项目需要补充了配置、构建产物和 `.DS_Store` 忽略规则 |
| `.env.example` | 手写 | Compose 本地变量示例，只有占位符；真实 `.env` 不提交 |
| `go.mod` | 混合 | 模块初始化后人工固定版本，再由 Go 工具维护依赖关系 |
| `go.sum` | Go 工具维护 | 依赖校验和，随依赖命令更新，禁止手工编辑 |
| `Makefile` | 混合 | 保留 GoFrame 的 `hack/hack-cli.mk` 入口，并手写环境与基础设施命令 |
| `compose.yaml` | 手写 | 本地 MySQL/etcd 编排 |
| `README.MD` | 手写 | 项目入口文档 |
| `AGENTS.md` | 手写 | AI Coding 规则与渐进式文档加载入口 |
| `app/` | GoFrame 生成后扩展 | 包含三个 MonoRepo 模板和实际的 `user` 服务脚手架 |
| `hack/` | GoFrame 生成 | 根级 Makefile 公共规则，由 `gf init -m` 创建 |
| `utility/` | GoFrame 生成 | MonoRepo 共享工具包占位目录，目前只有 `.gitkeep` |
| `deploy/` | 手写 | 本地基础设施初始化资源 |
| `scripts/` | 手写 | 项目辅助脚本 |
| `docs/` | 手写 | 架构、环境和开发文档 |
| `tasks/` | 手写 | 可验收的 AI Coding 任务单 |
| `.DS_Store` | 本机生成 | macOS Finder 元数据，不属于源代码，应由 Git 忽略 |

## 4. `app/` 下每个目录的来源

### `app/api-template/`

整个目录由 `gf init . -m` 生成，是 HTTP API 服务模板。

| 路径 | 来源 | 说明 |
| --- | --- | --- |
| `main.go` | GoFrame 生成 | 进程入口 |
| `Makefile` | GoFrame 生成 | 单服务构建命令 |
| `README.MD` | GoFrame 生成后改写 | 原模板只有英文标题，现改为中文来源说明 |
| `api/hello/` | GoFrame 生成 | Hello API 定义和 v1 请求/响应示例 |
| `internal/cmd/` | GoFrame 生成 | 启动命令和路由注册 |
| `internal/consts/` | GoFrame 生成 | 常量占位 |
| `internal/controller/hello/` | GoFrame 生成 | Hello Controller 及装配文件 |
| `internal/dao/`、`internal/model/`、`internal/service/` | GoFrame 生成 | 仅含占位文件，等待实际业务代码 |
| `manifest/config/` | GoFrame 生成 | 模板运行配置 |
| `manifest/deploy/` | GoFrame 生成 | Kustomize/Kubernetes 部署模板 |
| `manifest/docker/` | GoFrame 生成 | Dockerfile 和构建脚本 |
| `manifest/i18n/` | GoFrame 生成 | 国际化资源占位目录 |
| `resource/` | GoFrame 生成 | 静态文件和模板占位目录 |

### `app/job-template/`

整个目录由 `gf init . -m` 生成，是后台任务模板。

| 路径 | 来源 | 说明 |
| --- | --- | --- |
| `main.go`、`internal/cmd/`、`internal/consts/` | GoFrame 生成 | 任务进程入口与基础结构 |
| `Makefile` | GoFrame 生成 | 单服务构建命令 |
| `README.MD` | GoFrame 生成后改写 | 原英文标题已替换为中文来源说明 |
| `hack/config.yaml` | GoFrame 生成 | CLI 模板配置 |
| `internal/dao/`、`internal/model/`、`internal/service/` | GoFrame 生成 | 业务层占位目录 |
| `manifest/config/` | GoFrame 生成 | 模板运行配置 |
| `manifest/deploy/` | GoFrame 生成 | Kustomize/Kubernetes 部署模板 |
| `manifest/docker/` | GoFrame 生成 | Dockerfile 和构建脚本 |

### `app/svc-template/`

整个目录由 `gf init . -m` 生成，是后端服务模板。

| 路径 | 来源 | 说明 |
| --- | --- | --- |
| `main.go`、`Makefile` | GoFrame 生成 | 服务入口和构建命令 |
| `README.MD` | GoFrame 生成后改写 | 原英文标题已替换为中文来源说明 |
| `api/echo/` | GoFrame 生成 | Echo API 定义与 v1 示例 |
| `internal/cmd/`、`internal/consts/` | GoFrame 生成 | 启动命令和常量占位 |
| `internal/controller/echo/` | GoFrame 生成 | Echo Controller 示例 |
| `internal/dao/`、`internal/model/`、`internal/service/` | GoFrame 生成 | 业务层占位目录 |
| `hack/config.yaml` | GoFrame 生成 | CLI 配置模板 |
| `manifest/config/` | GoFrame 生成 | 模板运行配置 |
| `manifest/deploy/` | GoFrame 生成 | Kustomize/Kubernetes 部署模板 |
| `manifest/docker/` | GoFrame 生成 | Dockerfile 和构建脚本 |
| `manifest/i18n/`、`resource/` | GoFrame 生成 | 国际化和资源占位目录 |
| `manifest/protobuf/` | GoFrame 生成 | Protobuf 占位目录 |
| `manifest/sql/user.sql` | GoFrame 生成 | 示例 SQL，不是当前 `user` 数据库的正式初始化脚本 |

### `app/user/`

该目录最初由 `gf init app/user -a` 生成，之后修改了配置并生成数据库映射代码。

> 后续变更：下表记录的是初始搭建时的状态。Hello 示例已被正式的 `user`、`consignee` 协议和控制器取代并删除；误执行 `gf init` 留下的嵌套目录 `app/user/app/user/` 也已删除。当前结构以代码和 [`development/user-service.md`](../development/user-service.md) 为准。

| 路径 | 来源 | 说明与修改规则 |
| --- | --- | --- |
| `main.go` | GoFrame 生成 | 用户服务进程入口，目前仍为模板入口 |
| `Makefile` | GoFrame 生成 | 用户服务构建命令 |
| `README.MD` | GoFrame 生成后改写 | 现已改为中文状态说明 |
| `api/hello/hello.go` | GoFrame 生成，`DO NOT EDIT` | Hello API 聚合接口，不手工修改 |
| `api/hello/v1/hello.go` | GoFrame 生成 | Hello 请求/响应示例，后续会被正式协议替换 |
| `internal/cmd/cmd.go` | GoFrame 生成 | 启动和路由注册，目前是模板代码 |
| `internal/consts/consts.go` | GoFrame 生成 | 常量占位 |
| `internal/controller/hello/hello.go` | GoFrame 生成一次、允许续写 | Controller 实现占位；尚未写商城逻辑 |
| `internal/controller/hello/hello_new.go` | GoFrame 生成，`DO NOT EDIT` | Controller 构造代码 |
| `internal/controller/hello/hello_v1_hello.go` | GoFrame 生成示例 | Hello 处理函数示例 |
| `hack/hack.mk`、`hack/hack-cli.mk` | GoFrame 生成 | Makefile 公共规则 |
| `hack/config.yaml.example` | 混合 | GoFrame 模板生成后，手工改为本地 MySQL 连接和代码生成配置；密码是占位符，`make dev-config` 生成本地的 `hack/config.yaml`（不提交） |
| `manifest/config/config.yaml.example` | 手写 | 可提交的本地运行配置模板 |
| `manifest/config/config.yaml` | 本地复制 | 从 example 复制，供本机运行；被 Git 忽略 |
| `manifest/deploy/` | GoFrame 生成 | Kustomize/Kubernetes 模板，尚未按商城部署完善 |
| `manifest/docker/` | GoFrame 生成 | Dockerfile 和构建脚本 |
| `manifest/i18n/`、`resource/`、`utility/` | GoFrame 生成 | 当前为空的占位目录 |
| `internal/dao/internal/*.go` | `gf gen dao` 生成，`DO NOT EDIT` | 数据表 DAO 核心实现，重新生成时会覆盖 |
| `internal/dao/*.go` | `gf gen dao` 生成、允许扩展 | DAO 对外对象；当前没有手写扩展方法 |
| `internal/model/do/*.go` | `gf gen dao` 生成，`DO NOT EDIT` | DAO 写入/查询使用的动态字段模型 |
| `internal/model/entity/*.go` | `gf gen dao` 生成，`DO NOT EDIT` | 与数据库行对应的强类型实体 |
| `manifest/protobuf/pbentity/*.proto` | `gf gen pbentity` 生成，`DO NOT EDIT` | 数据库表的 Protobuf 映射，不是公共 API |
| 各目录中的 `.gitkeep` 或 `.keep-if-necessary` | GoFrame 生成 | 让 Git 能保留空目录的占位文件 |

## 5. 手写基础设施、文档和任务文件

| 文件 | 来源 | 说明 |
| --- | --- | --- |
| `deploy/mysql/init/001_user_schema.sql` | 手写 | 定义用户和收货地址表，是 DAO 生成的上游事实来源 |
| `scripts/check-env.sh` | 手写 | 检查命令存在性、最低版本和 Docker 状态 |
| `docs/index.md` | 手写 | 渐进式文档入口 |
| `docs/setup/local-environment.md` | 手写 | 环境安装、启动、代理和验证步骤 |
| `docs/setup/source-notes.md` | 手写 | 原教程与本项目工程决策的差异 |
| `docs/setup/project-origin.md` | 手写 | 本文，说明完整搭建过程和来源 |
| `docs/architecture/overview.md` | 手写 | 服务边界、开发顺序和 Agent 边界 |
| `docs/development/user-service.md` | 手写 | 用户服务代码生成和后续实现规则 |
| `tasks/0001-user-grpc-contract.md` | 手写 | 下一项开发任务的范围、决策和验收标准 |

## 6. 判断一个文件能否手改

1. 文件头包含 `DO NOT EDIT`：禁止手改，应修改上游 SQL、Protobuf 或生成配置后重新生成。
2. 文件头写明只生成一次、可以填充：允许在指定文件中实现业务，但不要修改配套的生成装配文件。
3. `go.sum`：由 Go 工具维护，禁止手改。
4. `.example`、Compose、SQL、脚本、文档和任务文件：属于项目手写资产，可以正常评审和修改。
5. Kustomize、Dockerfile 等初始化模板：可以修改，但必须先明确实际部署需求，不能把模板默认值当成生产方案。

## 7. 初始搭建结束时尚未做的内容

- 未创建正式的用户和收货地址 gRPC 协议。
- 未执行正式协议对应的 `gf gen pb`。
- 未实现注册、登录、JWT、地址 CRUD 或事务逻辑。
- 未实现 gateway、order、inventory 和 payment-simulator。
- 未接入 OpenTelemetry、日志采集和确定性故障注入。
- 未实现日志诊断 Agent 或导购 Agent。
- 未接入真实支付渠道。

因此，当前项目应准确描述为“可运行的开发环境和代码生成基线”，而不是“已完成的分布式商城”。
