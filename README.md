# 韩国签证状态监控（Go）

本应用定时查询韩国签证门户，在首次运行或申请状态变化时发送通知；签证签发后会停止访问网页。业务逻辑已全部使用 Go 实现，单个静态二进制即可运行。

## 功能

- 查询驻外使领馆护照申请，解析申请编号、入境目的和当前状态
- UTC+8 查询时间窗，支持普通时间窗和跨午夜时间窗
- PushDeer 或 Server酱通知
- 本地文件、S3/S3 兼容服务或 Upstash Redis 状态存储
- 原子本地写入、HTTP 超时、优雅停止和配置校验
- 兼容旧版 Python 应用写入的状态 JSON

## 本地运行

需要 Go 1.24 或更高版本。复制配置模板并填写签证信息与通知密钥：

```bash
cp .env.example .env
```

程序读取系统环境变量，不会自动加载 `.env`。在 shell 或部署平台中导入变量后运行：

```bash
go run ./cmd/krvisa
```

只查询并打印当前状态、不读取状态存储也不发送通知：

```bash
go run ./cmd/krvisa query
```

构建：

```bash
go build -o bin/krvisa ./cmd/krvisa
```

## Docker

```bash
docker build -t krvisa .
docker run --rm --env-file .env -v krvisa-state:/state krvisa
```

若挂载 `/state`，请把 `VISA_STATE_FILE` 设为 `/state/visa_state.json`。运行镜像基于 Alpine，其中只有 Go 二进制及 HTTPS 所需的 CA 证书。

## Modal 部署

`modal_app.py` 只是 Modal 平台所需的部署适配层；部署时会通过 Alpine 的 `apk` 在 Modal 派生层安装原生 musl Python，并清除容器默认入口以便 Modal 启动 Python Runner。查询、通知和存储仍由 Go 二进制执行，GHCR 只需维护一个 `go` 镜像。首次使用先配置 Modal：

```bash
uvx modal setup
```

填写 `.env` 后部署：

```bash
uvx --with python-dotenv modal deploy modal_app.py
```

任务默认使用 GHCR 的 `go` 镜像标签，每 10 分钟在北京时间 08:00–20:00 运行，并自动挂载名为 `krvisa-state` 的持久化 Volume。Modal 使用本地存储时会把状态文件固定到 `/state/visa_state.json`；选择 S3 或 Upstash 时不使用该文件。

## 配置

### 必填申请信息

| 环境变量 | 格式 | 说明 |
| --- | --- | --- |
| `VISA_PASSPORT_NUMBER` | 字符串 | 护照号码 |
| `VISA_ENGLISH_NAME` | 如 `ZHANG SAN` | 护照英文姓名，程序自动转为大写 |
| `VISA_BIRTHDAY` | `YYYY-MM-DD` | 有效公历出生日期 |

### 查询与时间窗

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `VISA_WINDOW_START` | `08:00` | 每日开始时间（UTC+8，包含） |
| `VISA_WINDOW_END` | `20:00` | 每日结束时间（UTC+8，不包含）；与开始时间相同表示全天 |
| `VISA_QUERY_URL` | 韩国签证门户地址 | 通常无需修改，可用于代理或集成测试 |

### 通知

默认使用 PushDeer：

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `VISA_PUSH_CHANNEL` | `pushdeer` | `pushdeer` 或 `serverchan` |
| `VISA_PUSHDEER_KEY` | 无 | PushDeer PushKey |
| `VISA_PUSHDEER_ENDPOINT` | `https://api2.pushdeer.com/message/push` | PushDeer API 地址 |
| `VISA_SERVERCHAN_KEY` | 无 | Server酱 SendKey（选择 `serverchan` 时必填） |

### 状态存储

本地文件是默认方案：

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `VISA_STATE_STORAGE` | `local` | `local`、`s3` 或 `upstash` |
| `VISA_STATE_FILE` | 当前目录下 `visa_state.json` | 本地状态文件路径 |

S3 使用官方 AWS SDK 的标准凭据链，支持环境变量、共享配置、工作负载角色和实例角色：

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `VISA_S3_BUCKET` | 无 | Bucket，选择 S3 时必填 |
| `VISA_S3_KEY` | `visa_state.json` | 对象 key |
| `VISA_S3_REGION` | `AWS_DEFAULT_REGION` 或 `ap-northeast-2` | AWS 区域 |
| `VISA_S3_ENDPOINT_URL` | 无 | S3 兼容服务端点；省略协议时使用 HTTPS |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | 无 | 无角色凭据时设置 |
| `AWS_SESSION_TOKEN` | 无 | 临时凭据可选项 |

Upstash Redis 使用 REST API：

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `UPSTASH_REDIS_REST_URL` | 无 | REST API URL |
| `UPSTASH_REDIS_REST_TOKEN` | 无 | REST API Token |
| `VISA_UPSTASH_KEY` | `krvisa:visa_state` | 状态 key；多个监控任务应使用不同值 |

完整示例见 `.env.example`。不要提交 `.env` 或任何密钥。
