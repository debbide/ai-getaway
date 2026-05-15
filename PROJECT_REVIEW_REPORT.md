# 项目前后端审查与优化建议

审查时间：2026-05-15  
审查范围：Go 后端、Vue/Vite 前端、支付/订单/鉴权/API Key/代理计费/系统配置。

## 验证结果

- 后端编译与现有测试：通过，命令为 `go test ./...`。
- 前端生产构建：通过，命令为 `node node_modules\vite\bin\vite.js build`。
- 直接执行 `npm run build` 失败，原因是当前机器全局 npm 指向了缺失的 `C:\Users\63138\AppData\Roaming\npm\node_modules\npm\bin\npm-cli.js`，不是项目代码编译错误。

## 总体结论

项目主流程已经具备注册、登录、套餐、订单、易支付、人工审核、API Key、上游代理和用量计费能力。当前最需要优先处理的是支付完成链路的幂等性、支付结果校验完整性、订单状态机约束和密钥暴露面。  

支付签名校验已经存在，但业务校验不完整；公共套餐扣减使用了条件更新防超卖，但订单完成不是原子幂等操作，重复通知或并发点击有重复扣减风险。后台还存在可以把任意订单拒绝、修改已生成支付链接订单金额/套餐、手动完成支付但缺少审计约束等状态一致性问题。

## 必须优先修复

### P0：支付完成缺少原子幂等保护

位置：`controller/order.go:155`、`controller/order.go:197`、`controller/order.go:266`

`MarkPaid`、`EpayNotify` 和后台 `CompleteOrderPayment` 都会进入 `completePaidOrder`。该函数在更新订单时没有用 `WHERE status = pending_payment` 做原子状态迁移。公共套餐分支会先扣减 `public_channels.remaining_usd_cents`，再更新订单状态。如果同一订单被回调和用户手动确认同时触发，或支付平台重复通知，两条并发请求可能都扣减公共渠道额度。

建议：

- 在事务内先执行 `UPDATE orders SET status = ... WHERE id = ? AND status = 'pending_payment'`，检查 `RowsAffected`。
- `RowsAffected == 0` 时重新读取订单，若已是目标状态则返回幂等成功，不再扣减、不再发邮件。
- 公共套餐扣减必须发生在订单状态抢占成功之后，或将订单状态更新与扣减放在同一事务内并以订单状态更新作为幂等锁。
- 给 `payment_ref` 加唯一索引。

### P0：支付回调和主动查单没有校验金额、商户号、订单号一致性

位置：`controller/order.go:197`、`controller/order.go:396`、`controller/order.go:433`

`EpayNotify` 只校验签名和 `trade_status`，随后按 `out_trade_no` 找订单；`queryEpayPaid` 只判断返回是否“已支付”。目前没有校验：

- `pid` 是否等于系统配置的商户 ID。
- `out_trade_no` 是否等于当前订单 `PaymentRef`。
- 支付金额 `money` 是否等于订单 `AmountCents`。
- 第三方交易号是否已处理过。

这会导致金额不一致、配置串商户、订单被后台改价后仍按旧支付链接完成等风险。

建议：

- 回调和查单都解析并校验 `pid/out_trade_no/money/trade_status`。
- 新增字段保存 `trade_no`、支付渠道、实付金额、支付完成时间、原始回调摘要。
- 对 `trade_no` 建唯一索引，防止同一第三方流水重复入账。
- 金额不一致时进入 `payment_mismatch` 或 `pending_manual_review`，不要自动开通。

### P0：支付超时后成功回调被直接忽略

位置：`controller/order.go:231`

订单 5 分钟超时后，支付成功回调会返回 `success` 但不入账。这会出现用户已付款但系统未开通的情况，后续也缺少自动补偿或退款入口。

建议：

- 超时订单收到成功回调时，记录支付流水并进入 `paid_late` 或 `pending_manual_review`。
- 后台提供“补开通/退款标记/关闭”的处理入口。
- 前端支付倒计时到期后应提示“若已付款请联系客服”，不要只要求重新下单。

### P0：后台可修改待支付订单金额/套餐，且支付完成不校验金额

位置：`controller/admin.go:627`、`controller/order.go:340`、`controller/order.go:217`

用户获取支付链接时金额已经参与签名，但后台 `UpdateOrder` 仍可修改待支付订单的 `plan_id` 和 `amount_cents`。由于支付完成时没有校验回调金额，用户可能按旧金额支付后获得新套餐，或支付记录与订单金额不一致。

建议：

- 支付链接生成后冻结订单核心字段，至少包括 `plan_id`、`amount_cents`、`settlement_usd_cents`。
- 如必须改价，应作废旧订单并生成新订单、新支付流水号。
- 支付成功时强制校验第三方返回金额与订单金额。

### P1：后台拒绝订单没有状态约束，会造成订单和用户权益不一致

位置：`controller/admin.go:611`

`RejectOrder` 对任意订单 ID 直接更新为 `rejected`，没有限制只能拒绝 `pending_review`。如果误操作已通过订单，订单状态会变成拒绝，但用户套餐、API Key、上游绑定不会同步回滚。

建议：

- 只允许拒绝 `pending_review`。
- 如果要撤销已通过订单，应设计单独的“撤销权益”流程，同时禁用 API Key、上游绑定并记录审计。

### P1：后台审核订单不是原子状态迁移

位置：`controller/admin.go:513`

`ApproveOrder` 先读订单状态，再事务内更新，没有用 `WHERE status = pending_review` 防止两个管理员并发审核同一订单。并发下可能重复发开通通知或覆盖上游绑定信息。

建议：

- 在事务内使用条件更新抢占状态。
- 审核操作写入审计日志，记录管理员、旧状态、新状态、上游通道、时间。

### P1：易支付通知/返回地址配置未完整保存

位置：`controller/settings.go:112`

`updateSettingsRequest` 包含 `epay_notify_url` 和 `epay_return_url`，模型也有字段，但 `Update` 只保存了 `epay_pid` 和 `epay_submit_url`，没有保存通知地址和返回地址。后台页面配置这两项后可能不会生效。

建议：

- 在 `updates` 中补充 `epay_notify_url`、`epay_return_url`。
- 增加设置接口测试，覆盖保存后读取。

### P1：默认密钥和默认管理员密码不适合生产

位置：`config/config.go:35`、`config/config.go:37`

如果生产环境未配置环境变量，会使用 `JWT_SECRET=change-this-secret` 和 `DEFAULT_ADMIN_PASSWORD=admin123456`。这属于高风险默认配置。

建议：

- `APP_ENV=production` 时检测默认值并直接启动失败。
- 首次启动强制生成随机管理员密码或要求环境变量。
- README 和 `.env.example` 明确生产必填项。

## 安全与合规建议

### 密钥明文暴露面偏大

位置：`controller/api_key.go:260`、`controller/admin.go:565`、`controller/admin.go:742`、`controller/admin.go:777`

用户可通过 `/keys/secret` 反复取回完整 API Key；管理员列表和详情会返回上游密码、上游 API Key、公共渠道 API Key。虽然便于运营，但风险是任意一次管理员 Token 泄漏或前端 XSS 都会扩大到所有上游密钥。

建议：

- 用户 API Key 改为创建/轮换时一次性展示，不提供长期明文取回；如保留取回，要求二次验证。
- 管理端列表默认脱敏，仅详情按需展示，展示前要求二次确认或重新输入管理员密码。
- 上游密钥使用独立加密密钥，不与 JWTSecret 共用。

### CORS 在生产环境过宽

位置：`router/router.go:127`

当前固定 `Access-Control-Allow-Origin: *`。虽然没有开启 Cookie 凭证，但生产环境建议限制为前端域名，减少跨站点滥用接口的面。

建议：

- 增加 `ALLOWED_ORIGINS` 配置。
- 生产环境只允许正式前端域名。

### 支付地址生成信任客户端转发头

位置：`controller/order.go:448`

默认通知地址和返回地址来自 `X-Forwarded-Proto`、`X-Forwarded-Host` 或 `Host`。如果应用没有可信代理校验，客户端伪造头可能生成错误的支付回调地址。

建议：

- 生产环境强制配置站点公网 Base URL。
- 只在可信反向代理注入的头上使用 `X-Forwarded-*`。

### 登录和邮件验证码缺少频率限制

位置：`controller/auth.go`

登录、发送邮箱验证码主要依赖滑块验证码，但没有按 IP、邮箱、账号维度做频率限制。验证码发送还可能被用于邮件轰炸。

建议：

- Redis 增加邮箱验证码发送频率限制，例如同邮箱 60 秒一次、每天上限。
- 登录失败按 IP 和邮箱组合限速。
- 对管理员登录失败增加告警。

## 业务逻辑与计费建议

### API 额度检查不是预扣，允许并发超额

位置：`middleware/api_key.go:92`、`service/quota.go:129`、`upstream/proxy.go:265`

调用前只检查历史用量，实际用量在上游响应完成后写入日志。多个并发请求都可能在额度足够时放行，最终超出套餐额度。

建议：

- 对高价值套餐或公共池增加“预估额度冻结”机制。
- 至少对并发请求数、最大输出 token、单次最大花费做限制。
- 响应完成后按实际费用结算，多退少补。

### Redis 限流故障时默认放行

位置：`middleware/api_key.go:106`

`allowAPIKey` 在 Redis 不可用或 `INCR` 失败时返回 `true`。这保证可用性，但会让限流失效。

建议：

- 生产环境可配置 fail-open/fail-closed。
- Redis 异常时记录告警，并降低单 Key 并发或启用本地兜底限流。

### 公共套餐的公共渠道余额只在售卖时扣减，不按实际消耗回补

位置：`controller/order.go:291`、`middleware/api_key.go:68`

公共套餐购买时扣减公共渠道剩余额度，后续调用时只检查渠道仍有余额和用户套餐额度。这个模型等价于“售卖时锁定公共池份额”，不是“按真实消耗扣公共池余额”。如果这是设计目标可以保留；如果公共池余额代表上游真实余额，则需要按实际使用扣减。

建议：

- 明确公共渠道余额语义：库存额度还是真实余额。
- 如果是真实余额，应在 `APILog` 写入后同步扣减公共渠道余额，并保证幂等。

## 前端与体验建议

### 支付流程依赖用户点击“已完成支付”，但后端有校验

位置：`frontend/src/components/Dashboard.vue:221`、`frontend/src/components/Dashboard.vue:884`

前端会打开第三方支付页，然后用户点击“已完成支付”。后端会主动查单，不是纯前端信任，因此基本合规。但建议补充回调自动刷新和超时后的人工处理提示。

建议：

- 支付弹窗定时轮询订单状态。
- 收到 `pending_review/approved/payment_timeout/paid_late` 后自动更新 UI。
- 超时且用户已付款时提供联系入口和订单号。

### Token 存储在 localStorage

位置：`frontend/src/stores/auth.js`

localStorage 方案实现简单，但任何前端 XSS 都能读取 Token。当前项目有公告、文档、邮件模板等富文本/Markdown 管理功能，后续如果渲染 HTML，需要特别注意 XSS。

建议：

- 增加 CSP。
- Markdown 渲染必须做白名单净化。
- 中长期可改为 HttpOnly Cookie + CSRF 防护。

## 可维护性建议

### 状态机建议集中封装

当前订单状态流转分散在 `order.go` 和 `admin.go`：创建、支付中、待审核、通过、拒绝、超时。建议抽出订单状态机服务，所有状态变更都通过统一函数完成，并记录审计日志。

建议状态迁移：

- `pending_payment -> pending_review`：订阅套餐支付成功。
- `pending_payment -> approved`：公共套餐支付成功并扣减成功。
- `pending_payment -> payment_timeout`：未支付超时。
- `payment_timeout -> paid_late/pending_manual_review`：超时后收到成功支付。
- `pending_review -> approved/rejected`：管理员审核。
- `approved -> revoked`：单独撤销流程，不复用 `rejected`。

### 数据库约束需要补强

建议新增：

- `orders.payment_ref` 唯一索引。
- 支付流水表：`provider_trade_no` 唯一索引。
- 每个用户活跃 API Key 数量用数据库约束或事务保证。
- `orders.status`、`users.status`、`plans.plan_type` 等字段用枚举约束或应用层集中校验。

### 设置和迁移逻辑不应散落在控制器

位置：`controller/settings.go:197`

`ensureSystemSettingColumns` 在请求路径中执行 DDL。运行时接口触发 ALTER TABLE 会增加锁表和不可预期延迟。

建议：

- 将 DDL 迁移移动到启动迁移或独立 migration 脚本。
- 请求路径只读写业务字段。

## 建议修复顺序

1. 修复支付幂等：订单状态原子迁移、公共套餐扣减顺序、支付流水唯一约束。
2. 补齐支付校验：金额、商户号、订单号、第三方流水号、超时后支付处理。
3. 收紧后台订单状态操作：拒绝、审核、改价、手动完成支付都加状态约束和审计。
4. 修复易支付通知/返回地址保存 bug。
5. 生产安全基线：禁止默认 JWTSecret/管理员密码、限制 CORS、配置可信 Base URL。
6. 收敛密钥明文展示，补二次验证和脱敏。
7. 增加额度预扣/并发控制和 Redis 异常告警。

## 建议补充的测试

- 同一支付回调重复通知 2 次，只开通一次，只扣减一次。
- 用户点击“已完成支付”和异步回调并发，只处理一次。
- 回调金额少于订单金额时不批准订单。
- `pid` 不匹配时拒绝回调。
- 超时后收到成功回调进入人工处理状态。
- 已批准订单不能被 `RejectOrder` 改为拒绝。
- 待支付订单生成支付链接后不能改金额或套餐。
- 设置接口保存 `epay_notify_url`、`epay_return_url` 后读取一致。
