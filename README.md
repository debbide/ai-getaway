# 星空 AI / ai-getaway

星空 AI 是一个轻量级 AI API 中转与运营系统，后端使用 Go + Gin + GORM，前端使用 Vue 3 + Vite。项目面向 OpenAI-compatible 接入场景，提供用户注册登录、套餐订阅、余额充值、API Key 管理、模型计价、上游渠道管理、调用日志、公告文档、支付审核和渠道监控等能力。

> 当前仓库包含 Go 后端和 Vue 前端。后端入口位于仓库根目录 `main.go`，前端位于 `frontend/`。

## 功能概览

- OpenAI-compatible API 代理：支持 `/v1/*` 路径转发和 `/messages` 兼容入口。
- 多上游接入：支持用户独立上游、公共渠道、轮询号池、余额专用渠道。
- GPT / Claude 协议标记：渠道、公共渠道、号池账号均可声明支持协议。
- 用户体系：注册、登录、JWT 会话、邮箱验证码、滑块验证码、密码修改。
- 第三方登录：支持 GitHub、Google OAuth 的登录、绑定、解绑。
- 套餐体系：日套餐、周/月订阅、公共套餐、余额套餐、免费套餐、抽奖套餐。
- 订单流程：在线支付、人工支付、管理员审核、拒绝、关闭、订单超时清理。
- 余额模式：余额充值后按调用成本扣费，可选择公开可用的分组倍率。
- 模型计价：按 token 或按请求计费，支持缓存输入、输出、倍率、精选模型展示。
- 分组倍率：后台可维护计费分组，并绑定到渠道、用户上游或余额通道。
- API Key 管理：用户创建、查看、轮换、启用、禁用，后台可管理全部 Key。
- 用量统计：记录请求模型、token、估算成本、延迟、首 token 时间和错误信息。
- 公告与文档：后台维护 Markdown 文档和公告，前台展示教程、历史公告和模型列表。
- 渠道监控：定时探测监控地址，前台展示可用、降级、不可用状态和延迟。
- 管理后台：用户、订单、套餐、兑换码、模型、渠道、号池、设置、邮件模板等管理。
- 集群辅助：实例注册、内部节点信息、节点日志、后台任务分布式锁。

## 技术栈

| 模块 | 技术 |
| --- | --- |
| 后端 | Go 1.21、Gin、GORM |
| 数据库 | MariaDB / MySQL |
| 缓存与锁 | Redis |
| 前端 | Vue 3、Vite、Pinia、Element Plus、TailwindCSS |
| 文档编辑 | Markdown / wangEditor |
| 认证 | JWT、邮箱验证码、OAuth |
| 支付 | 易支付接口、人工收款审核 |

## 项目结构

```text
.
├── main.go                 # Gin 服务入口
├── config/                 # 环境变量、配置加载与生产校验
├── database/               # MariaDB、Redis 初始化，自动迁移，初始化数据，后台清理任务
├── router/                 # HTTP 路由、CORS、健康检查、集群内部接口
├── middleware/             # JWT 鉴权、管理员鉴权、API Key 鉴权、限流
├── controller/             # API 控制器
├── service/                # 业务服务、额度、邮件、OAuth、监控、集群等逻辑
├── model/                  # GORM 数据模型
├── upstream/               # OpenAI-compatible / Claude-compatible 上游代理
├── response/               # 统一响应封装
├── utils/                  # 密码、加密、辅助方法
├── scripts/                # 辅助脚本
├── logs/                   # 本地运行日志目录
├── md/                     # 项目说明文档
├── frontend/               # Vue 前端项目
└── 部署流程.md             # 部署流程说明
```

## 运行环境

- Windows / Linux 均可运行，本地开发建议使用 Windows PowerShell 或 Bash。
- Go 1.21+
- Node.js 18+
- MariaDB 10.5+ 或 MySQL 8+
- Redis 6+

创建数据库：

```sql
CREATE DATABASE ai_gateway CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

复制环境变量：

```powershell
Copy-Item .env.example .env
```

如果手写 MariaDB DSN，`loc=Asia/Shanghai` 需要写成 `loc=Asia%2FShanghai`。

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_ENV` | `development` | 运行环境。生产环境填 `production` 后会启用关键配置校验。 |
| `APP_PORT` | `8080` | 后端监听端口。 |
| `DB_DSN` | 示例 DSN | MariaDB / MySQL 连接串。 |
| `REDIS_ADDR` | `127.0.0.1:6379` | Redis 地址。 |
| `REDIS_PASSWORD` | 空 | Redis 密码。 |
| `REDIS_DB` | `0` | Redis DB 编号。 |
| `JWT_SECRET` | 示例值 | JWT 签名密钥，生产环境必须修改。 |
| `DEFAULT_ADMIN_EMAIL` | `admin@example.com` | 首次启动自动创建的管理员邮箱。 |
| `DEFAULT_ADMIN_PASSWORD` | `admin123456` | 首次启动自动创建的管理员密码，生产环境必须修改。 |
| `UPSTREAM_TIMEOUT` | `120s` | 请求上游接口超时时间。 |
| `MAX_PROXY_BODY_BYTES` | `10485760` | 代理请求体最大字节数。 |
| `MAX_SSE_USAGE_BUFFER_BYTES` | `1048576` | SSE 统计用缓冲区大小。 |
| `API_KEY_RATE_LIMIT_PER_MINUTE` | `120` | 单个 API Key 分钟级限流。 |
| `PUBLIC_BASE_URL` | 示例域名 | 对外访问域名，生产环境必填，用于支付回调等场景。 |
| `ALLOWED_ORIGINS` | 示例域名 | CORS 允许来源，生产环境不能使用通配符。 |
| `CLUSTER_MODE` | `false` | 是否启用集群模式。 |
| `INSTANCE_ID` | 自动生成 | 当前实例 ID。 |
| `INSTANCE_ADVERTISE_URL` | `http://127.0.0.1:8080` | 当前实例对内通告地址。 |
| `CLUSTER_INTERNAL_TOKEN` | JWT 密钥 | 集群内部接口访问令牌。 |
| `RUN_BACKGROUND_JOBS` | `true` | 是否运行后台任务。多实例部署时可配合集群锁。 |

生产环境会强制检查：`JWT_SECRET`、`DEFAULT_ADMIN_PASSWORD`、`PUBLIC_BASE_URL` 和 `ALLOWED_ORIGINS`，避免使用默认密钥、默认密码或通配符 CORS。

## 后端启动

```powershell
go mod tidy
go run .
```

首次启动会自动迁移数据表、初始化默认套餐、内置文档、邮件模板和默认管理员账号。

健康检查：

```powershell
Invoke-WebRequest http://127.0.0.1:8080/health
Invoke-WebRequest http://127.0.0.1:8080/ready
```

## 前端启动

```powershell
Set-Location frontend
npm install
npm run dev
```

Vite 开发服务会将 `/api` 转发到后端。

## 构建与测试

后端构建：

```powershell
go build .
```

后端测试：

```powershell
$env:GOCACHE='D:\python\ai-getaway\.gocache'
$env:GOMODCACHE='D:\python\ai-getaway\.gomodcache'
go test ./...
```

前端生产构建：

```powershell
Set-Location frontend
npm run build
```

如果本机 npm 全局入口异常，可以直接使用项目本地 Vite 入口完成等价构建：

```powershell
Set-Location frontend
node node_modules\vite\bin\vite.js build
```

仓库也提供 Windows 发布脚本：

```powershell
.\build-release.bat
```

脚本会构建 Linux amd64 后端二进制 `ai-getaway-linux-amd64`，并生成 `frontend/dist/`。

## 核心业务流程

### 套餐订阅流程

1. 用户注册并登录。
2. 用户在定价页选择套餐。
3. 系统创建订单，订单初始状态为 `pending_payment`。
4. 用户选择在线支付或人工支付。
5. 在线支付回调或用户主动标记已支付后，订单进入审核或自动完成流程。
6. 管理员审核通过后，用户状态变为 `approved`，套餐、额度周期和过期时间生效。
7. 管理员为用户绑定上游账号或用户使用公共/余额通道。

### 余额扣费流程

1. 用户选择余额充值套餐或自定义充值金额。
2. 支付完成并经审核后，余额以 USD cents 计入用户账户。
3. 用户没有可用订阅或配置了余额访问时，请求可走余额通道。
4. 实际扣费按模型基础计价、模型倍率、分组倍率和 token / request 统计计算。

### API 调用流程

1. 用户在控制台创建平台 API Key。
2. 客户端使用 `Authorization: Bearer ag_xxx` 请求 `/v1/*` 或 `/messages`。
3. 中间件校验 API Key、用户状态、套餐/余额、限流和额度预留。
4. 代理层选择用户上游、公共渠道、余额通道或轮询号池账号。
5. 网关将平台 Key 替换为上游 API Key，并转发请求。
6. 系统记录调用日志、token 用量、估算成本、延迟和错误信息。

## API 路由概览

### 公开接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/health` | 基础健康检查。 |
| `GET` | `/ready` | 数据库、Redis、集群就绪检查。 |
| `GET` | `/api/settings/public` | 前台公开配置。 |
| `POST` | `/api/captcha/slide` | 创建滑块验证码。 |
| `POST` | `/api/auth/email-code` | 发送邮箱验证码。 |
| `POST` | `/api/auth/register` | 注册账号。 |
| `POST` | `/api/auth/login` | 登录账号。 |
| `GET` | `/api/auth/oauth/:provider/start` | OAuth 登录跳转。 |
| `GET` | `/api/auth/oauth/:provider/callback` | OAuth 登录回调。 |
| `GET` | `/api/plans` | 前台套餐列表。 |
| `GET` | `/api/models` | 前台模型列表。 |
| `GET` | `/api/docs` | 前台文档列表。 |
| `GET` | `/api/docs/:slug` | 文档详情。 |
| `GET` | `/api/announcements` | 公告列表。 |
| `GET` | `/api/status/monitors` | 渠道监控状态。 |
| `GET` | `/api/payment/manual` | 人工支付配置。 |
| `ANY` | `/api/payment/epay/notify` | 易支付异步通知。 |
| `ANY` | `/v1/*path` | OpenAI-compatible 代理入口。 |
| `ANY` | `/messages` | Messages 兼容代理入口。 |

### 登录用户接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/auth/me` | 当前用户信息。 |
| `PATCH` | `/api/auth/password` | 修改密码。 |
| `GET` | `/api/auth/oauth/accounts` | 已绑定 OAuth 账号。 |
| `GET` | `/api/auth/oauth/:provider/bind` | 发起 OAuth 绑定。 |
| `DELETE` | `/api/auth/oauth/:provider` | 解绑 OAuth 账号。 |
| `GET` | `/api/balance/billing-groups` | 可选余额计费分组。 |
| `PUT` | `/api/balance/billing-group` | 更新余额计费分组。 |
| `POST` | `/api/orders` | 创建套餐订单。 |
| `POST` | `/api/balance/recharge` | 创建余额充值订单。 |
| `GET` | `/api/orders` | 我的订单。 |
| `POST` | `/api/orders/:id/pay` | 获取在线支付链接。 |
| `POST` | `/api/orders/:id/manual-payment` | 提交人工付款信息。 |
| `PATCH` | `/api/orders/:id/paid` | 用户标记在线订单已支付。 |
| `POST` | `/api/redeem-codes/redeem` | 兑换套餐码。 |
| `POST` | `/api/endpoint-speed` | 测试线路速度。 |
| `GET` | `/api/keys/secret` | 查看当前用户密钥辅助信息。 |
| `GET` | `/api/keys` | API Key 列表。 |
| `POST` | `/api/keys` | 创建 API Key。 |
| `POST` | `/api/keys/rotate` | 轮换 API Key。 |
| `PATCH` | `/api/keys/:id/disable` | 禁用 API Key。 |
| `PATCH` | `/api/keys/:id/enable` | 启用 API Key。 |
| `GET` | `/api/usage/logs` | 当前用户调用日志。 |

### 管理员接口

管理员接口均位于 `/api/admin` 下，需要登录且角色为 `admin`。

| 模块 | 主要路径 |
| --- | --- |
| 用户管理 | `/users`、`/users/:id`、`/users/:id/upstream`、`/users/:id/upstreams/:access_type`、`/users/reset-quota-batch` |
| 订单审核 | `/orders`、`/orders/:id/approve`、`/orders/:id/reject`、`/orders/:id/close`、`/orders/:id/complete-payment` |
| 套餐管理 | `/plans`、`/plans/:id`、`/plans/:id/draw-lottery` |
| 兑换码 | `/redeem-codes`、`/redeem-codes/:id/disable` |
| 系统设置 | `/settings`、`/settings/test-smtp` |
| 邮件模板 | `/email-templates`、`/email-templates/:type` |
| 文档管理 | `/docs`、`/docs/:id` |
| 公告管理 | `/announcements`、`/announcements/:id` |
| 模型管理 | `/models`、`/models/:id`、`/models/sync-official` |
| 分组倍率 | `/billing-groups`、`/billing-groups/:id` |
| 上游渠道 | `/upstream-channels`、`/upstream-channels/:id` |
| 公共渠道 | `/public-channels`、`/public-channels/:id` |
| 轮询号池 | `/polling-pools`、`/polling-pools/:id` |
| OpenAI OAuth 导入 | `/openai-oauth/auth-url`、`/openai-oauth/exchange-code`、`/openai-oauth/refresh-token` |
| 渠道监控 | `/channel-monitors`、`/channel-monitors/:id`、`/channel-monitors/:id/ping` |
| API Key 管理 | `/keys`、`/keys/:id` |
| 使用记录 | `/usage/logs` |
| 运营统计 | `/stats` |
| 实时日志 | `/logs/ws` |
| 负载均衡 | `/load-balancer/nodes`、`/load-balancer/nodes/:id/ping`、`/load-balancer/nodes/:id/logs` |

### 集群内部接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/internal/cluster/info` | 当前节点信息，需要 `X-Cluster-Token`。 |
| `GET` | `/internal/cluster/logs` | 当前节点运行日志，需要 `X-Cluster-Token`。 |

## OpenAI-compatible 调用示例

普通请求：

```powershell
curl.exe http://127.0.0.1:8080/v1/chat/completions `
  -H "Authorization: Bearer ag_xxx" `
  -H "Content-Type: application/json" `
  -d "{\"model\":\"gpt-4o-mini\",\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}"
```

流式请求：

```powershell
curl.exe http://127.0.0.1:8080/v1/chat/completions `
  -H "Authorization: Bearer ag_xxx" `
  -H "Content-Type: application/json" `
  -d "{\"model\":\"gpt-4o-mini\",\"stream\":true,\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}"
```

客户端配置通常填写：

```text
Base URL: https://your-domain.example/v1
API Key: ag_xxx
Model: 后台启用的模型 ID
```

## 后台配置建议

首次登录后台后建议按顺序配置：

1. 修改默认管理员密码，并确认 `.env` 中默认密码不再使用弱口令。
2. 在系统设置中配置站点标题、联系邮箱、导航、支付方式、SMTP 和 OAuth。
3. 在模型管理中新增或同步模型计价，确认模型 ID 与上游一致。
4. 在分组倍率中创建默认倍率和不同渠道倍率。
5. 在渠道管理中配置上游渠道、公共渠道或轮询号池账号。
6. 在套餐管理中配置订阅套餐、公共套餐、余额套餐、免费套餐或抽奖套餐。
7. 在文档和公告中维护前台用户可见的接入教程和服务通知。
8. 在渠道监控中添加上游健康检查地址。

## 支付说明

系统支持在线支付和人工支付。在线支付通过易支付参数生成跳转链接，异步通知路径为 `/api/payment/epay/notify`；人工支付由后台上传收款二维码，用户提交付款备注，管理员审核订单。

支付相关配置在后台系统设置中维护，主要字段包括易支付提交地址、商户 ID、商户 KEY、回调地址、返回地址、在线支付开关、人工支付开关和余额充值汇率。`PUBLIC_BASE_URL` 会参与支付回调地址生成，生产环境必须配置为真实外网域名。

## 后台任务

服务启动后会根据配置运行以下后台任务：滑块验证码过期清理、待支付订单超时清理、订阅到期邮件提醒、渠道监控定时探测、集群节点注册与心跳。

如果 `RUN_BACKGROUND_JOBS=false`，相关后台任务会停止运行。多实例部署时，后台任务通过 Redis 集群锁避免重复执行。

## 安全注意事项

- 不要提交真实 `.env`、JWT 密钥、数据库密码、Redis 密码、上游 API Key、OAuth Secret、支付商户 KEY。
- 生产环境必须修改 `JWT_SECRET` 和 `DEFAULT_ADMIN_PASSWORD`。
- 生产环境必须配置 `PUBLIC_BASE_URL` 和明确的 `ALLOWED_ORIGINS`。
- 后台绑定的上游 API Key、支付 KEY、SMTP 密码和 OAuth Secret 不会在接口中明文返回。
- 用户平台 API Key 以哈希和加密字段保存，调用时使用 `ag_xxx` 形式。
- 建议将后端服务放在反向代理后，并启用 HTTPS。

## 常见问题

### Redis 不可用会怎样？

服务会记录 Redis 不可用日志。部分 Redis 支撑的功能会降级，集群模式下 `/ready` 会将 Redis 视为关键依赖。

### 为什么生产环境启动失败？

当 `APP_ENV=production` 时，系统会拒绝默认 JWT 密钥、默认管理员密码、空 `PUBLIC_BASE_URL` 或通配符 CORS。

### 用户调用提示没有可用上游？

通常是用户套餐未生效、余额不足、后台未绑定计划上游、公共渠道无剩余额度，或余额模式没有可用余额通道。

### 模型计费不准确怎么办？

检查后台模型管理中的模型 ID、计费模式、输入/缓存输入/输出价格、请求价格、模型倍率和分组倍率，确认和上游返回的 usage 字段匹配。

### 前端构建 npm 入口缺失怎么办？

可以在 `frontend/` 目录直接运行：

```powershell
node node_modules\vite\bin\vite.js build
```

这与 `npm run build` 中的 Vite 构建等价。

## 开发规范

- Go 代码使用 `gofmt`。
- 请求处理放在 `controller/`。
- 业务逻辑放在 `service/`。
- 数据模型放在 `model/`。
- 路由集中维护在 `router/router.go`。
- 前端组件使用 PascalCase 命名。
- 前端 API 封装放在 `frontend/src/api/client.js`。
- 修改后端共享逻辑后建议运行 `go test ./...`。
- 修改前端后至少运行生产构建。

## 许可证

当前仓库未声明开源许可证。对外分发或商用前，请先补充明确的 LICENSE 文件。
