import axios from 'axios'

const messageMap = {
  'active subscription in effect': '当前套餐仍在有效期内，请待到期后再购买其他套餐',
  'api key already exists': '每个账号仅允许一条 API Key，请使用「更新密钥」替换',
  'no api key to rotate': '当前没有可更新的 API Key，请先创建',
  'api key not found': '未找到 API Key',
  'api key secret unavailable': '该密钥无法解密，请使用「更新密钥」重新生成',
  'email already exists': '该邮箱已存在，请更换邮箱或直接登录',
  'email already registered': '该邮箱已注册，请直接登录',
  'email not verified': '邮箱尚未完成验证，请先通过邮箱验证后再登录',
  'invalid credentials': '邮箱或密码不正确，请检查后重试',
  'invalid email verification code': '邮箱验证码不正确或已过期',
  'invalid old password': '旧密码不正确，请重新输入',
  'invalid slide captcha': '安全验证未通过，请重新拖动滑块',
  'no active upstream account bound': '当前账号尚未绑定可用上游通道，请联系管理员开通',
  'order already waiting review': '该套餐已有待审核订单，请勿重复提交',
  'order not pending payment': '订单当前状态不允许继续支付，请刷新后查看',
  'payment config missing': '支付配置未完成，请联系管理员',
  'payment not completed': '支付结果尚未确认，请完成支付后再试',
  'subscription expired': '订阅已到期，请续费后继续使用',
  'subscription quota exceeded': '本周美元额度已用完，请升级或续费后继续使用',
  'user disabled': '账号已被禁用，请联系管理员',
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
  (error) => {
    const rawMessage = error.response?.data?.message || error.message || '请求失败'
    const message = messageMap[rawMessage] || normalizeMessage(rawMessage)
    return Promise.reject(new Error(message))
  }
)

function normalizeMessage(message) {
  if (message.includes('Field validation')) return '请检查表单必填项和格式是否正确'
  if (message.startsWith('failed to send email:')) return '邮件发送失败，请检查 SMTP 配置后重试'
  return message
}
