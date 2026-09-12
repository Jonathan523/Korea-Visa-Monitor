# 韩国签证状态监控

本项目用于按照给定的时间间隔自动查询并监控韩国签证申请状态。在首次运行或检测到状态变化时，程序会通过配置的推送渠道发送通知；查询时间窗口、申请人信息和推送方式均可通过环境变量配置。

## 状态存储配置

### 本地文件（默认，需要持久化存储）

| 环境变量 | 是否必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `VISA_STATE_STORAGE` | 否 | `local` | 状态存储方式。可选值：`local`、`s3`、`upstash`；本地文件使用 `local`。 |
| `VISA_STATE_FILE` | 否 | 程序目录下的 `visa_state.json` | 状态文件路径；容器部署时可设为 `/<持久化路径>/visa_state.json`。 |

### S3

| 环境变量 | 是否必填 | 默认值/示例 | 说明 |
| --- | --- | --- | --- |
| `VISA_STATE_STORAGE` | 是 | `s3` | 状态存储方式。可选值：`local`、`s3`、`upstash`；使用 S3 时必须设为 `s3`。 |
| `VISA_S3_BUCKET` | 是 | `my-bucket` | 保存状态的 bucket。 |
| `VISA_S3_KEY` | 否 | `visa_state.json` | 状态对象的 key。 |
| `VISA_S3_REGION` | 否 | `ap-northeast-2` | 有效的 AWS 区域代码；也可使用 `AWS_DEFAULT_REGION`。 |
| `VISA_S3_ENDPOINT_URL` | 否 | 无 | S3 兼容服务端点；未填写协议时自动使用 `https://`。例如 `s3.us-west-004.backblazeb2.com`。 |
| `AWS_ACCESS_KEY_ID` | 视情况 | 无 | 没有实例角色、任务角色或其他 AWS 凭据来源时必填。 |
| `AWS_SECRET_ACCESS_KEY` | 视情况 | 无 | 与 `AWS_ACCESS_KEY_ID` 配套使用。 |
| `AWS_SESSION_TOKEN` | 否 | 无 | 使用临时凭据时设置。 |

凭据由 AWS SDK 的标准凭据链读取，因此部署在 AWS 上时也可以不传静态密钥，改用 IAM Role。所用身份至少需要目标对象的 `s3:GetObject` 和 `s3:PutObject` 权限。

### Upstash Redis（推荐用于无状态容器）

| 环境变量 | 是否必填 | 默认值/示例 | 说明 |
| --- | --- | --- | --- |
| `VISA_STATE_STORAGE` | 是 | `upstash` | 状态存储方式。可选值：`local`、`s3`、`upstash`；使用 Upstash Redis 时必须设为 `upstash`。 |
| `UPSTASH_REDIS_REST_URL` | 是 | `https://your-database.upstash.io` | 有效的 HTTP(S) URL，即 Upstash 数据库的 REST API 地址。 |
| `UPSTASH_REDIS_REST_TOKEN` | 是 | 无 | Upstash REST API Token，应保存到部署平台的 Secret 中。 |
| `VISA_UPSTASH_KEY` | 否 | `krvisa:visa_state` | 状态记录的 key；多个监控任务应使用不同的 key。 |

此方式直接复用程序已有的 HTTP 客户端，不需要安装额外依赖。Token 不要写入镜像或提交到代码仓库。

## 推送设置

程序支持 `pushdeer` 和 `serverchan`（Server酱） 两种推送渠道，默认使用 PushDeer。只需配置所选渠道对应的密钥。

### PushDeer（默认）

| 环境变量 | 是否必填 | 默认值/示例 | 说明 |
| --- | --- | --- | --- |
| `VISA_PUSH_CHANNEL` | 否 | `pushdeer` | 推送渠道。可选值：`pushdeer`、`serverchan`；使用 PushDeer 时设为 `pushdeer`。 |
| `VISA_PUSHDEER_KEY` | 是 | `你的 PushKey` | PushKey，可在 [PushDeer](https://www.pushdeer.com/) 获取。 |
| `VISA_PUSHDEER_ENDPOINT` | 否 | `https://api2.pushdeer.com/message/push` | 有效的 HTTP(S) URL；使用自建服务时修改。 |

### Server酱

| 环境变量 | 是否必填 | 默认值/示例 | 说明 |
| --- | --- | --- | --- |
| `VISA_PUSH_CHANNEL` | 是 | `serverchan` | 推送渠道。可选值：`pushdeer`、`serverchan`；使用 Server酱时必须设为 `serverchan`。 |
| `VISA_SERVERCHAN_KEY` | 是 | `你的 SendKey` | SendKey，可在 [Server酱](https://sct.ftqq.com/) 获取。 |

## 其他环境变量

| 环境变量 | 是否必填 | 默认值/格式 | 说明 |
| --- | --- | --- | --- |
| `VISA_PASSPORT_NUMBER` | 是 | 护照号码 | 用于查询签证状态的护照号码。 |
| `VISA_ENGLISH_NAME` | 是 | 如 `ZHANG SAN` | 护照上的英文姓名，程序会自动转为大写。 |
| `VISA_BIRTHDAY` | 是 | `YYYY-MM-DD` | 有效的公历出生日期，例如 `1990-01-31`。 |
| `VISA_WINDOW_START` | 否 | `08:00` | `HH:MM` 格式，范围为 `00:00`–`23:59`；每日查询开始时间。 |
| `VISA_WINDOW_END` | 否 | `20:00` | `HH:MM` 格式，范围为 `00:00`–`23:59`；每日查询结束时间，不含该时刻。 |

查询时间窗口使用 UTC+8 时区，默认从 `08:00` 开始，到 `20:00` 结束（不含结束时间）。
