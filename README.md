# 星空 AI

轻量级 AI API 中转系统，兼容 OpenAI API 格式，按用户绑定独立上游账号，实现订阅制多租户网关。

## 已实现

- Go 1.21 + Gin + GORM + MariaDB 后端
- Redis API Key 分钟级限流
- 用户注册、登录、JWT 会话
- 套餐列表、订单创建、管理员审核
- 管理员绑定上游渠道、Base URL、上游 API Key
- 用户 API Key 创建、查看、禁用
- `/v1/*` OpenAI-compatible 反向代理，支持普通响应和 SSE Stream 透传
- WebSocket 实时调用日志广播：`/api/admin/logs/ws`
- Vue 3 + Vite + TailwindCSS + Pinia 前端 MVP

## 目录

```text
.
├── main.go
├── config/
├── router/
├── middleware/
├── controller/
├── service/
├── model/
├── database/
├── upstream/
├── utils/
├── response/
├── scripts/
├── logs/
└── frontend/
```

## 后端启动

1. 创建 MariaDB 数据库：

```sql
CREATE DATABASE ai_gateway CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

2. 配置环境变量，可参考 `.env.example`。如果手写 MariaDB DSN，`loc=Asia/Shanghai` 需要写成 `loc=Asia%2FShanghai`。

3. 启动后端：

```bash
go mod tidy
go run .
```

首次启动会自动迁移数据表，创建默认套餐和默认管理员账号。

## 前端启动

```bash
cd frontend
npm install
npm run dev
```

Vite 默认运行在 `http://127.0.0.1:5173`，并将 `/api` 代理到 `http://127.0.0.1:8080`。

## 基本流程

1. 用户注册后状态为 `pending`。
2. 用户登录并选择套餐，创建订单，订单状态为 `pending_review`。
3. 管理员登录后台，审核订单并填写上游渠道、Base URL、上游 API Key。
4. 系统将用户置为 `approved`，并保存一用户一上游账号绑定关系。
5. 用户创建平台 API Key，用该 Key 请求 `/v1/chat/completions` 等 OpenAI 兼容接口。

## 调用示例

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer ag_xxx" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'
```

网关会将平台 Key 替换为该用户绑定的上游 API Key，再转发到用户绑定的上游 Base URL。
