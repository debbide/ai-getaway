import axios from 'axios'

const AUTH_EXPIRED_MESSAGE = '登录状态已失效，请重新登录'

const messageMap = {
  'active subscription in effect': '当前套餐仍在有效期内，请待到期后再购买其他套餐',
  'api key already exists': '每个账号仅允许一个 API Key，请使用“更新密钥”替换',
  'api key not found': '未找到 API Key',
  'api key secret unavailable': '该密钥当前无法解密展示，请使用“更新密钥”重新生成',
  'email already exists': '该邮箱已存在，请更换邮箱或直接登录',
  'email already registered': '该邮箱已注册，请直接登录',
  'registration disabled': '当前站点暂未开放新用户注册',
  'email not verified': '邮箱尚未完成验证，请先完成邮箱验证',
  'invalid credentials': '邮箱或密码不正确，请检查后重试',
  'invalid email verification code': '邮箱验证码不正确或已过期',
  'invalid old password': '旧密码不正确，请重新输入',
  'invalid slide captcha': '安全验证未通过，请重新拖动滑块',
  'no active subscription assigned': '当前账号未分配有效套餐，已禁止调用，请联系管理员处理',
  'no active upstream account bound': '当前账号尚未绑定可用上游渠道，请联系管理员开通',
  'no api key to rotate': '当前没有可更新的 API Key，请先创建',
  'order already waiting review': '该套餐已有待审核订单，请勿重复提交',
  'order payment timeout': '订单支付已超时，请重新创建订单',
  'order not pending payment': '订单当前状态不允许继续支付，请刷新后查看',
  'manual payment selected': '该订单已选择人工支付，请扫码并提交审核',
  'manual payment not selected': '该订单未选择人工支付',
  'online payment disabled': '在线支付已关闭，请选择其他支付方式',
  'manual payment disabled': '人工支付已关闭，请选择其他支付方式',
  'manual payment qr code missing': '管理员尚未上传人工支付二维码，请联系站点支持',
  'payment config missing': '支付配置未完成，请联系管理员',
  'payment not completed': '暂未查询到支付成功结果，请确认已完成支付后再刷新',
  'plan price required': '套餐价格不能小于 0',
  'failed to verify payment': '暂未查询到支付成功结果，请确认已完成支付后再刷新',
  'payment amount mismatch': '支付金额不一致，订单已转人工处理',
  'payment pid mismatch': '支付商户不一致，订单已转人工处理',
  'payment order mismatch': '支付订单号不一致，订单已转人工处理',
  'order not rejectable': '订单当前状态不允许拒绝',
  'paid or payment-started order plan cannot be changed': '订单已开始支付，不能修改套餐',
  'paid or payment-started order amount cannot be changed': '订单已开始支付，不能修改金额',
  'public channel sold out': '公共渠道额度已售罄，请选择其他套餐',
  'public plan sold out': '该活动套餐已售罄，请选择其他套餐',
  'free plan sold out': '免费套餐已领完',
  'free plan user limit reached': '你已达到该免费套餐的领取上限',
  'subscription expired': '订阅已到期，请续费后继续使用',
  'subscription quota exceeded': '本周美元额度已用完，请升级或续费后继续使用',
  'user disabled': '账号已被禁用，请联系管理员',
  'user not found': '账号信息暂时不可用，请刷新后重试',
  'password confirmation mismatch': '两次输入的新密码不一致',
  'failed to update password': '密码修改失败，请稍后重试',
  'failed to update order': '订单状态更新失败，请稍后重试'
}

export const api = axios.create({
  baseURL: '/api',
  timeout: 20000
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

api.interceptors.response.use(
  (response) => response.data,
  async (error) => {
    const status = error.response?.status
    const rawMessage = error.response?.data?.message || error.message || '请求失败'
    if (status === 401) {
      const token = localStorage.getItem('token')
      if (shouldRetryAuthRequest(error, rawMessage, token)) {
        error.config.__authRetry = true
        error.config.headers = { ...(error.config.headers || {}), Authorization: `Bearer ${token}` }
        return api.request(error.config)
      }
      if (token && isAuthExpiredMessage(rawMessage)) {
        return expireAuth(AUTH_EXPIRED_MESSAGE)
      }
    }
    if (status === 403 && rawMessage.includes('user disabled')) {
      return expireAuth(messageMap[rawMessage] || normalizeMessage(rawMessage))
    }
    const message = messageMap[rawMessage] || normalizeMessage(rawMessage)
    return Promise.reject(apiError(message, { status, rawMessage }))
  }
)

function shouldRetryAuthRequest(error, rawMessage, token) {
  if (!token || !error.config || error.config.__authRetry) return false
  if (isAuthExpiredMessage(rawMessage)) return false
  return rawMessage.includes('missing authorization token') || rawMessage.includes('user not found')
}

function isAuthExpiredMessage(message) {
  return [
    'invalid authorization token',
    '登录状态已失效',
    '缺少登录凭证',
    '账号不存在'
  ].some((text) => String(message || '').includes(text))
}

function expireAuth(message) {
  localStorage.removeItem('token')
  window.dispatchEvent(new CustomEvent('auth-expired', { detail: { message } }))
  return Promise.reject(apiError(message, { authExpired: true }))
}

function apiError(message, props = {}) {
  const err = new Error(message)
  Object.assign(err, props)
  return err
}

function normalizeMessage(message) {
  if (message.includes('Field validation')) return '请检查表单必填项和格式是否正确'
  if (message.startsWith('failed to send email:')) return '邮件发送失败，请检查 SMTP 配置后重试'
  return message
}
