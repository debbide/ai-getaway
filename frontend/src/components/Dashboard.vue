<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Present, Refresh, Stopwatch } from '@element-plus/icons-vue'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { plainTextFromMarkdown, renderMarkdown } from '../utils/markdown'

const props = defineProps({
  plans: { type: Array, default: () => [] },
  apiEndpoints: { type: String, default: '[]' }
})
const emit = defineEmits(['navigate'])

const auth = useAuthStore()
const orders = ref([])
const keys = ref([])
const announcements = ref([])
const announcementExpanded = ref(localStorage.getItem('announcementExpanded') !== 'false')
const historyModalOpen = ref(false)
const pendingPlainKey = ref('')
const lastKeyMasked = ref('')
const error = ref('')
const notice = ref('')
const copySuccessModalOpen = ref(false)
const useKeyModalOpen = ref(false)
const keyUsageTab = ref('claude')
const claudeOSTab = ref('powershell')
const usagePlainKey = ref('')
const modalError = ref('')
const loading = reactive({ announcements: false, keys: false, orders: false, plan: false })
const endpointSpeedStates = reactive({})
const redeemForm = reactive({ code: '' })
const redeeming = ref(false)
const orderPage = ref(1)
const nowMs = ref(Date.now())
const orderPageSize = 3
let orderTimer = null
let paymentPollTimer = null
const modal = reactive({ open: false, type: '', title: '', actionLabel: '', payload: null, danger: false })
const orderForm = reactive({ planId: '', paymentMethod: 'online', order: null, paymentUrl: '', paymentOpened: false, manualQRCode: '', manualNote: '' })
const keyForm = reactive({ name: 'Default' })
const manualPaymentConfirmMessage = '请确认已成功支付，恶意创建人工支付订单且未支付的用户将会遭到封禁处理，请知悉。'

const totalOrderPages = computed(() => Math.max(1, Math.ceil(orders.value.length / orderPageSize)))
const pagedOrders = computed(() => {
  const page = Math.min(orderPage.value, totalOrderPages.value)
  const start = (page - 1) * orderPageSize
  return orders.value.slice(start, start + orderPageSize)
})

const hasActiveSubscription = computed(() => {
  const u = auth.user
  if (!u || u.status !== 'approved') return false
  if (!u.plan) return false
  if (isPublicPlan(u.plan) && !u.expires_at) return true
  if (!u.expires_at) return false
  return new Date(u.expires_at) > new Date()
})

const planPeriodStartIso = computed(() => {
  const u = auth.user
  if (!u || !hasActiveSubscription.value) return null
  if (u.subscription_started_at) return u.subscription_started_at
  if (!u.expires_at || !u.plan?.duration_days) return null
  const end = new Date(u.expires_at)
  const s = new Date(end.getTime())
  s.setDate(s.getDate() - Number(u.plan.duration_days))
  return s.toISOString()
})

const quotaUsage = computed(() => auth.user?.quota_usage || null)
const totalQuotaUsage = computed(() => auth.user?.total_quota_usage || (isPublicPlan(auth.user?.plan) ? auth.user?.quota_usage : null) || null)
const publicQuotaUsage = computed(() => totalQuotaUsage.value || quotaUsage.value)
const quotaUsagePercent = computed(() => {
  const percent = Number(quotaUsage.value?.percent || 0)
  if (!Number.isFinite(percent)) return 0
  return Math.min(100, Math.max(0, percent))
})
const totalQuotaUsagePercent = computed(() => {
  const percent = Number(totalQuotaUsage.value?.percent || 0)
  if (!Number.isFinite(percent)) return 0
  return Math.min(100, Math.max(0, percent))
})
const quotaProgressStyle = computed(() => ({ '--quota-progress': `${quotaUsagePercent.value}%` }))
const totalQuotaProgressStyle = computed(() => ({ '--quota-progress': `${totalQuotaUsagePercent.value}%` }))
const currentPlanIsPublic = computed(() => isPublicPlan(auth.user?.plan))
const currentPublicPlanExhausted = computed(() => currentPlanIsPublic.value && publicQuotaUsage.value && publicQuotaUsage.value.limit_usd_cents > 0 && publicQuotaUsage.value.used_usd_cents >= publicQuotaUsage.value.limit_usd_cents)
const purchaseBlockedByActivePlan = computed(() => hasActiveSubscription.value && !currentPublicPlanExhausted.value)
const quotaResetText = computed(() => {
  if (!quotaUsage.value?.window_end) return ''
  return `${quotaPeriodUnit(auth.user?.plan)}额度重置：${formatDateTime(quotaUsage.value.window_end)}`
})
const displayApiEndpoints = computed(() => parseApiEndpoints(props.apiEndpoints))
const apiBaseURL = computed(() => window.location.origin)
const apiV1BaseURL = computed(() => `${apiBaseURL.value}/v1`)

const soloKey = computed(() => (keys.value.length ? keys.value[0] : null))
const hasApiKey = computed(() => Boolean(soloKey.value))
const currentAnnouncement = computed(() => announcements.value[0] || null)
const historyAnnouncements = computed(() => announcements.value.slice(1))
const announcementSummary = computed(() => {
  const item = currentAnnouncement.value
  if (!item) return ''
  return item.Summary || plainTextFromMarkdown(item.Content).split('\n').find(Boolean) || ''
})

onMounted(() => {
  loadAll()
  orderTimer = window.setInterval(() => {
    nowMs.value = Date.now()
    if (orders.value.some((order) => order.Status === 'pending_payment' && orderRemainingSeconds(order) <= 0)) {
      loadOrders({ showLoading: false })
    }
  }, 1000)
})

onBeforeUnmount(() => {
  if (orderTimer) window.clearInterval(orderTimer)
  stopPaymentPolling()
})

watch(modalError, (message) => {
  if (message) showNotice(message, 'error')
})

async function loadAll() {
  loading.announcements = true
  loading.keys = true
  loading.orders = true
  loading.plan = true
  error.value = ''
  try {
    const [orderRes, keyRes, announcementRes] = await Promise.all([api.get('/orders'), api.get('/keys'), api.get('/announcements')])
    orders.value = orderRes.data || []
    keys.value = keyRes.data || []
    announcements.value = announcementRes.data || []
    if (orderPage.value > totalOrderPages.value) orderPage.value = totalOrderPages.value
    await auth.loadMe()
    if (auth.meError) showNotice(auth.meError, 'warning')
  } catch (err) {
    if (err.authExpired) {
      showNotice(err.message, 'error')
    } else {
      showNotice(err.message || '账号信息暂时不可用，请稍后刷新重试', 'warning')
    }
  } finally {
    loading.announcements = false
    loading.keys = false
    loading.orders = false
    loading.plan = false
  }
}

async function refreshDashboard() {
  notice.value = ''
  await loadAll()
}

async function loadOrders({ showLoading = true } = {}) {
  if (showLoading) loading.orders = true
  error.value = ''
  try {
    const orderRes = await api.get('/orders')
    orders.value = orderRes.data || []
    if (orderPage.value > totalOrderPages.value) orderPage.value = totalOrderPages.value
  } catch (err) {
    showNotice(err.message, err.authExpired ? 'error' : 'warning')
  } finally {
    if (showLoading) loading.orders = false
  }
}

async function refreshKeys() {
  loading.keys = true
  notice.value = ''
  error.value = ''
  try {
    const keyRes = await api.get('/keys')
    keys.value = keyRes.data || []
  } catch (err) {
    showNotice(err.message, err.authExpired ? 'error' : 'warning')
  } finally {
    loading.keys = false
  }
}

async function refreshOrders() {
  notice.value = ''
  await loadOrders()
}

async function refreshPlan() {
  loading.plan = true
  notice.value = ''
  error.value = ''
  try {
    await auth.loadMe()
    if (auth.meError) showNotice(auth.meError, 'warning')
  } catch (err) {
    showNotice(err.message, err.authExpired ? 'error' : 'warning')
  } finally {
    loading.plan = false
  }
}

async function redeemCode() {
  const code = normalizeRedeemInput(redeemForm.code)
  if (code.length !== 12) {
    showNotice('请输入 12 位激活码', 'warning')
    return
  }
  redeeming.value = true
  notice.value = ''
  error.value = ''
  try {
    const res = await api.post('/redeem-codes/redeem', { code })
    redeemForm.code = ''
    const status = res.data?.order?.Status
    showNotice(status === 'approved' ? '兑换成功，套餐已开通' : '兑换成功，订单已提交审核', 'success')
    await Promise.all([auth.loadMe(), loadOrders({ showLoading: false })])
  } catch (err) {
    showNotice(err.message || '兑换失败，请检查激活码', err.authExpired ? 'error' : 'warning')
  } finally {
    redeeming.value = false
  }
}

function setOrderPage(page) {
  orderPage.value = Math.min(Math.max(1, page), totalOrderPages.value)
}

function openPayModal(order) {
  orderForm.planId = String(order.PlanID || order.Plan?.ID || '')
  orderForm.paymentMethod = order.PaymentMethod || 'online'
  orderForm.order = order
  orderForm.paymentUrl = ''
  orderForm.paymentOpened = false
  orderForm.manualNote = order.UserPaymentNote || accountPaymentNote()
  if (isManualPaymentOrder(order)) {
    orderForm.manualQRCode = ''
    showModal('manual-pay-order', `人工支付订单 #${order.ID}`, '已扫码，提交审核')
    loadManualPaymentInfo()
    return
  }
  showModal('pay-order', `支付订单 #${order.ID}`, '已完成支付')
  startPaymentPolling()
}

function openKeyModal() {
  keyForm.name = 'Default'
  showModal('create-key', '创建 API Key', '创建密钥')
}

function openRotateModal() {
  keyForm.name = soloKey.value?.name || 'Default'
  showModal('rotate-key', '更新密钥', '确认替换', null, true)
}

function confirmDisableKey(key) {
  showModal('disable-key', '禁用 API Key', '确认禁用', { key }, true)
}

async function enableKey(k) {
  error.value = ''
  notice.value = ''
  try {
    await api.patch(`/keys/${k.id}/enable`)
    showNotice('API Key 已启用', 'success')
    await loadAll()
    window.dispatchEvent(new Event('app-data-updated'))
  } catch (err) {
    showNotice(err.message, 'error')
  }
}

async function startPayment() {
  if (!orderForm.order?.ID) return
  modalError.value = ''
  try {
    const res = await api.post(`/orders/${orderForm.order.ID}/pay`)
    orderForm.paymentUrl = res.data.payment_url
    orderForm.paymentOpened = true
    window.open(orderForm.paymentUrl, '_blank', 'noopener,noreferrer')
    startPaymentPolling()
  } catch (err) {
    modalError.value = err.message
  }
}

async function loadManualPaymentInfo() {
  try {
    const res = await api.get('/payment/manual')
    orderForm.manualQRCode = res.data?.manual_payment_qr_code || ''
  } catch (err) {
    modalError.value = err.message
  }
}

async function submitManualPayment() {
  if (!orderForm.order?.ID) return
  modalError.value = ''
  if (!String(orderForm.manualNote || '').trim()) {
    modalError.value = '请填写当前账号或留言，方便管理员核对付款'
    return
  }
  try {
    await ElMessageBox.confirm(manualPaymentConfirmMessage, '确认人工支付', {
      confirmButtonText: '确认已支付',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await api.post(`/orders/${orderForm.order.ID}/manual-payment`, {
      user_payment_note: orderForm.manualNote
    })
    showNotice('人工支付信息已提交，订单已进入待审核', 'success')
    closeModal()
    await loadAll()
    window.dispatchEvent(new Event('app-data-updated'))
  } catch (err) {
    modalError.value = err.message
  }
}

async function markPaid() {
  if (!orderForm.order?.ID) return
  modalError.value = ''
  try {
    await api.patch(`/orders/${orderForm.order.ID}/paid`)
    showNotice('支付已确认，订单已进入待审核', 'success')
    closeModal()
    await loadAll()
    window.dispatchEvent(new Event('app-data-updated'))
  } catch (err) {
    modalError.value = err.message
  }
}

async function createKey() {
  pendingPlainKey.value = ''
  lastKeyMasked.value = ''
  await runAction(async () => {
    const res = await api.post('/keys', { name: keyForm.name })
    pendingPlainKey.value = res.data.key
    lastKeyMasked.value = res.data.key_masked || ''
    showNotice('API Key 已创建，请尽快复制完整密钥保存（界面仅显示掩码）', 'success')
  })
}

async function rotateKey() {
  pendingPlainKey.value = ''
  lastKeyMasked.value = ''
  await runAction(async () => {
    const res = await api.post('/keys/rotate', { name: keyForm.name })
    pendingPlainKey.value = res.data.key
    lastKeyMasked.value = res.data.key_masked || ''
    showNotice('密钥已更新，旧 Key 立即失效，请复制新密钥保存', 'success')
  })
}

async function disableKey() {
  await runAction(async () => {
    await api.patch(`/keys/${modal.payload.key.id}/disable`)
    showNotice('API Key 已禁用', 'success')
  })
}

async function runAction(action) {
  error.value = ''
  notice.value = ''
  modalError.value = ''
  try {
    await action()
    closeModal()
    await loadAll()
    window.dispatchEvent(new Event('app-data-updated'))
  } catch (err) {
    if (modal.open) {
      modalError.value = err.message
    } else {
      showNotice(err.message, 'error')
    }
  }
}

function showNotice(message, type = 'success') {
  if (!message) return
  ElMessage({
    message,
    type,
    grouping: true,
    showClose: true,
    duration: type === 'error' ? 3000 : 2200
  })
}

function showModal(type, title, actionLabel, payload = null, danger = false) {
  modalError.value = ''
  Object.assign(modal, { open: true, type, title, actionLabel, payload, danger })
}

function closeModal() {
  modalError.value = ''
  if (modal.type === 'pay-order') stopPaymentPolling()
  Object.assign(modal, { open: false, type: '', title: '', actionLabel: '', payload: null, danger: false })
}

function startPaymentPolling() {
  if (paymentPollTimer || !orderForm.order?.ID) return
  paymentPollTimer = window.setInterval(refreshPayingOrder, 3000)
}

function stopPaymentPolling() {
  if (!paymentPollTimer) return
  window.clearInterval(paymentPollTimer)
  paymentPollTimer = null
}

async function refreshPayingOrder() {
  if (!modal.open || modal.type !== 'pay-order' || !orderForm.order?.ID) {
    stopPaymentPolling()
    return
  }
  try {
    const res = await api.get('/orders')
    orders.value = res.data || []
    const fresh = orders.value.find((item) => item.ID === orderForm.order.ID)
    if (!fresh) return
    orderForm.order = fresh
    if (['pending_review', 'approved', 'payment_timeout', 'paid_late', 'pending_manual_review'].includes(fresh.Status)) {
      stopPaymentPolling()
    }
  } catch {}
}

function submitModal() {
  const actions = {
    'pay-order': markPaid,
    'manual-pay-order': submitManualPayment,
    'create-key': createKey,
    'rotate-key': rotateKey,
    'disable-key': disableKey
  }
  actions[modal.type]?.()
}

function isManualPaymentOrder(order) {
  return order?.PaymentMethod === 'manual' || order?.PaymentChannel === 'manual'
}

function accountPaymentNote() {
  return auth.user?.email || auth.user?.username || ''
}

function openUsageRecords() {
  emit('navigate', '/usage-records')
}

function openDocs() {
  emit('navigate', '/docs')
}

function toggleAnnouncement() {
  announcementExpanded.value = !announcementExpanded.value
  localStorage.setItem('announcementExpanded', String(announcementExpanded.value))
}

function openAnnouncementHistory() {
  historyModalOpen.value = true
}

function closeAnnouncementHistory() {
  historyModalOpen.value = false
}

function announcementDate(item) {
  const value = item?.PublishedAt || item?.CreatedAt
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
}

function announcementHtml(item) {
  return renderMarkdown(item?.Content || '')
}

function orderExpiresAt(order) {
  const created = new Date(order?.CreatedAt || order?.created_at || '')
  if (Number.isNaN(created.getTime())) return null
  const ttlMs = isManualPaymentOrder(order) ? 2 * 60 * 60 * 1000 : 5 * 60 * 1000
  return new Date(created.getTime() + ttlMs)
}

function orderRemainingSeconds(order) {
  const expiresAt = orderExpiresAt(order)
  if (!expiresAt) return 0
  return Math.max(0, Math.ceil((expiresAt.getTime() - nowMs.value) / 1000))
}

function orderCountdown(order) {
  const seconds = orderRemainingSeconds(order)
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

function money(cents, currency = '￥') {
  return `${currency}${((cents || 0) / 100).toFixed(2)}`
}

function usd(cents) {
  return `$${((cents || 0) / 100).toFixed(2)}`
}

function quotaPeriodText(plan) {
  const period = plan?.QuotaPeriod || plan?.quota_period
  if (period === 'public' || plan?.PlanType === 'public' || plan?.plan_type === 'public') return '公共套餐'
  return period === 'daily' ? '日限额度' : '周限额度'
}

function quotaPeriodUnit(plan) {
  const period = plan?.QuotaPeriod || plan?.quota_period
  if (period === 'public' || plan?.PlanType === 'public' || plan?.plan_type === 'public') return '总'
  return period === 'daily' ? '日' : '周'
}

function isPublicPlan(plan) {
  return plan?.QuotaPeriod === 'public' || plan?.PlanType === 'public' || plan?.quota_period === 'public' || plan?.plan_type === 'public'
}

function planSoldOut(plan) {
  return isPublicPlan(plan) && Number(plan.PublicChannel?.RemainingUSDCents || 0) < Number(plan.SettlementUSDCents || 0)
}

function totalPlanUsd(plan) {
  if (isPublicPlan(plan)) return usd(plan.SettlementUSDCents)
  const units = plan?.QuotaPeriod === 'daily' ? Number(plan.DurationDays || 1) : Math.max(1, Math.round(Number(plan.DurationDays || 30) / 7))
  return usd(Number(plan?.SettlementUSDCents || 0) * units)
}

function publicRemainingUsd(plan) {
  return usd(plan.PublicChannel?.RemainingUSDCents || 0)
}

function pad2(n) {
  return String(n).padStart(2, '0')
}

function formatDateTime(value) {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '—'
  return `${d.getFullYear()}/${pad2(d.getMonth() + 1)}/${pad2(d.getDate())} ${pad2(d.getHours())}:${pad2(d.getMinutes())}`
}

function normalizeRedeemInput(value) {
  return String(value || '').trim().toUpperCase().replace(/[\s-]/g, '')
}

function parseApiEndpoints(value) {
  try {
    const parsed = JSON.parse(value || '[]')
    if (!Array.isArray(parsed)) return defaultApiEndpoints()
    const endpoints = parsed
      .map((item) => ({
        label: String(item.label || 'API').trim() || 'API',
        description: String(item.description || '').trim(),
        url: String(item.url || '').trim()
      }))
      .filter((item) => item.url)
    return endpoints.length ? endpoints : defaultApiEndpoints()
  } catch {
    return defaultApiEndpoints()
  }
}

function defaultApiEndpoints() {
  return [{ label: '默认', description: '主线路', url: 'https://ai.itzkb.cn' }]
}

function endpointSpeedState(url) {
  return endpointSpeedStates[url] || null
}

async function testEndpointSpeed(endpoint) {
  const url = String(endpoint?.url || '').trim()
  if (!url) return
  endpointSpeedStates[url] = { loading: true, value: '', error: '' }
  try {
    const res = await fetch(`https://v2.xxapi.cn/api/speed?url=${encodeURIComponent(url)}`)
    const data = await res.json()
    if (!res.ok || Number(data?.code) !== 200 || !data?.data) {
      throw new Error(data?.msg || '测速失败')
    }
    endpointSpeedStates[url] = { loading: false, value: String(data.data), error: '' }
  } catch (err) {
    endpointSpeedStates[url] = { loading: false, value: '', error: err.message || '测速失败' }
    showNotice('测速失败，请稍后重试', 'error')
  }
}

async function copyKey(text, showSuccessModal = false) {
  try {
    await navigator.clipboard.writeText(text)
    if (showSuccessModal) {
      copySuccessModalOpen.value = true
    } else {
      showNotice('已复制', 'success')
    }
    if (pendingPlainKey.value && text === pendingPlainKey.value) {
      pendingPlainKey.value = ''
    }
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    try { document.execCommand('copy') } catch {}
    document.body.removeChild(ta)
    if (showSuccessModal) {
      copySuccessModalOpen.value = true
    } else {
      showNotice('已复制', 'success')
    }
    if (pendingPlainKey.value && text === pendingPlainKey.value) {
      pendingPlainKey.value = ''
    }
  }
}

async function copySecretFromServer() {
  error.value = ''
  try {
    const res = await api.get('/keys/secret')
    await copyKey(res.data.key, true)
  } catch (err) {
    showNotice(err.message, 'error')
  }
}

async function openUseKeyModal() {
  error.value = ''
  try {
    const res = await api.get('/keys/secret')
    usagePlainKey.value = res.data.key || ''
    useKeyModalOpen.value = true
  } catch (err) {
    showNotice(err.message, 'error')
  }
}

function closeUseKeyModal() {
  useKeyModalOpen.value = false
}

function claudeSnippet(kind) {
  const base = apiBaseURL.value
  const key = usagePlainKey.value
  if (kind === 'cmd') {
    return `set ANTHROPIC_BASE_URL=${base}\nset ANTHROPIC_AUTH_TOKEN=${key}\nset CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
  }
  if (kind === 'powershell') {
    return `$env:ANTHROPIC_BASE_URL="${base}"\n$env:ANTHROPIC_AUTH_TOKEN="${key}"\n$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1"`
  }
  return `export ANTHROPIC_BASE_URL="${base}"\nexport ANTHROPIC_AUTH_TOKEN="${key}"\nexport CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
}

const claudeVSCodeSnippet = computed(() => JSON.stringify({
  'claude-code.env': {
    ANTHROPIC_BASE_URL: apiBaseURL.value,
    ANTHROPIC_AUTH_TOKEN: usagePlainKey.value,
    CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: '1',
    CLAUDE_CODE_ATTRIBUTION_HEADER: '0'
  }
}, null, 2))

const openCodeSnippet = computed(() => JSON.stringify({
  provider: {
    anthropic: {
      options: {
        baseURL: apiV1BaseURL.value,
        apiKey: usagePlainKey.value
      },
      npm: '@ai-sdk/anthropic'
    }
  },
  $schema: 'https://opencode.ai/config.json'
}, null, 2))

const codexConfigSnippet = computed(() => `model_provider = "codexxkai"\nmodel = "gpt-5.2"\nmodel_reasoning_effort = "high"\ndisable_response_storage = false\n\n[model_providers.codexxkai]\nname = "codexxkai"\nbase_url = "${apiV1BaseURL.value}"\nwire_api = "responses"\nrequires_openai_auth = true\nweb_search = "live"`)

const codexAuthSnippet = computed(() => JSON.stringify({
  OPENAI_API_KEY: usagePlainKey.value
}, null, 2))

function closeCopySuccessModal() {
  copySuccessModalOpen.value = false
}

function statusLabel(value) {
  return {
    pending_review: '待审核',
    pending_payment: '待支付',
    payment_timeout: '支付超时',
    paid_late: '超时已支付',
    pending_manual_review: '待人工处理',
    approved: '已通过',
    rejected: '已拒绝',
    active: '启用中',
    disabled: '已禁用',
    pending: '待审核'
  }[value] || value
}
</script>

<template>
  <section class="console-shell mx-auto max-w-7xl px-4 pb-12 sm:px-6">
    <div v-if="pendingPlainKey || lastKeyMasked" class="key-reveal">
      <span>密钥已就绪（下方仅掩码，完整内容请用按钮复制）</span>
      <code v-if="lastKeyMasked" class="api-key-code api-key-code--mask">{{ lastKeyMasked }}</code>
      <button v-if="pendingPlainKey" type="button" class="primary-button small" @click="copyKey(pendingPlainKey, true)">复制完整密钥</button>
    </div>

    <div class="console-stack">
      <div class="console-dashboard-grid">
        <div class="console-dashboard-main">
          <section v-if="currentAnnouncement" class="announcement-card console-mobile-contained" :class="{ 'announcement-card--collapsed': !announcementExpanded }">
            <div class="announcement-icon" aria-hidden="true">i</div>
            <div class="announcement-main">
              <div class="announcement-head">
                <h3>{{ currentAnnouncement.Title }}</h3>
                <div class="announcement-actions">
                  <button type="button" class="announcement-link-button" @click="openAnnouncementHistory">历史公告</button>
                  <button type="button" class="announcement-link-button" @click="toggleAnnouncement">
                    {{ announcementExpanded ? '收起' : '展开' }}
                  </button>
                </div>
              </div>
              <div v-if="announcementExpanded" class="announcement-content">
                <div class="markdown-body" v-html="announcementHtml(currentAnnouncement)"></div>
                <a v-if="currentAnnouncement.LinkURL" :href="currentAnnouncement.LinkURL" target="_blank" rel="noopener noreferrer">
                  {{ currentAnnouncement.LinkText || '查看详情' }}
                </a>
              </div>
              <p v-else class="announcement-summary">{{ announcementSummary }}</p>
            </div>
          </section>

          <!-- API Key -->
          <section class="panel-surface dashboard-card console-mobile-contained p-4">
            <div class="section-head">
              <div>
                <p class="section-kicker">Keys</p>
                <h3>API 密钥管理</h3>
              </div>
              <div class="toolbar-actions">
                <el-button class="refresh-button" circle :icon="Refresh" :loading="loading.keys" aria-label="刷新" title="刷新" @click="refreshKeys" />
                <el-button v-if="!hasApiKey" type="primary" @click="openKeyModal">创建 Key</el-button>
              </div>
            </div>

            <div class="notice-card notice-warn mt-3">
              <strong>安全提示</strong>
              <span>每个账号仅保留一条 API Key。列表中只显示掩码，复制时会从服务端安全取出完整密钥。更新密钥将删除旧密钥并立即生效。</span>
            </div>

            <div v-if="!hasApiKey" class="notice-card api-key-empty-panel mt-4">
              <strong>尚未创建 API Key</strong>
              <span class="text-muted">通过审核并绑定上游后，点击右上角「创建 Key」生成密钥。</span>
            </div>

            <div v-else class="mt-4">
              <article class="api-key-block">
                <div class="api-key-block-head">
                  <div>
                    <strong>{{ soloKey.name }}</strong>
                    <span class="text-muted">{{ statusLabel(soloKey.status) }}</span>
                  </div>
                  <div class="api-key-head-actions">
                    <button
                      v-if="soloKey.status === 'disabled'"
                      type="button"
                      class="ghost-button small"
                      @click="enableKey(soloKey)"
                    >
                      启用
                    </button>
                    <button
                      v-else
                      type="button"
                      class="danger-button small"
                      @click="confirmDisableKey(soloKey)"
                    >
                      禁用
                    </button>
                  </div>
                </div>
                <div class="api-key-strip">
                  <code class="api-key-code api-key-code--mask">{{ soloKey.key_masked || soloKey.key_prefix + '···' }}</code>
                  <div class="api-key-strip-actions">
                    <button
                      type="button"
                      class="primary-button small"
                      :disabled="!soloKey.can_copy"
                      @click="openUseKeyModal"
                    >
                      使用密钥
                    </button>
                    <button
                      type="button"
                      class="ghost-button small"
                      :disabled="!soloKey.can_copy"
                      @click="copySecretFromServer"
                    >
                      复制完整密钥
                    </button>
                    <button type="button" class="ghost-button small" @click="openRotateModal">更新密钥</button>
                  </div>
                </div>
                <p v-if="!soloKey.can_copy" class="api-key-legacy-hint text-muted">该密钥无法在线解密，请点击「更新密钥」重新生成后即可复制。</p>
              </article>
            </div>
          </section>

          <!-- 订单 -->
          <section class="panel-surface dashboard-card dashboard-card--orders console-mobile-contained p-5">
            <div class="section-head">
              <div>
                <p class="section-kicker">Orders</p>
                <h3>订单记录</h3>
              </div>
              <el-button class="refresh-button" circle :icon="Refresh" :loading="loading.orders" aria-label="刷新" title="刷新" @click="refreshOrders" />
            </div>

            <div class="mt-6 order-table-shell">
              <el-table :data="pagedOrders" border empty-text="暂无订单">
                <el-table-column label="订单" width="90">
                  <template #default="{ row: order }">#{{ order.ID }}</template>
                </el-table-column>
                <el-table-column label="套餐" min-width="140">
                  <template #default="{ row: order }">{{ order.Plan?.Name || '-' }}</template>
                </el-table-column>
                <el-table-column label="金额" width="110">
                  <template #default="{ row: order }">{{ money(order.AmountCents) }}</template>
                </el-table-column>
                <el-table-column label="状态" min-width="150">
                  <template #default="{ row: order }">
                    <div class="order-status-cell">
                      <el-tag>{{ statusLabel(order.Status) }}</el-tag>
                      <el-popover
                        v-if="order.Status === 'pending_review'"
                        trigger="click"
                        placement="top"
                        width="240"
                        content="后台审核需要 5-30 分钟，正在一对一开号中。"
                      >
                        <template #reference>
                          <button type="button" class="order-review-tip" aria-label="查看审核说明">!</button>
                        </template>
                      </el-popover>
                    </div>
                      <small v-if="order.Status === 'pending_payment'" class="order-countdown">剩余 {{ orderCountdown(order) }}</small>
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="140">
                  <template #default="{ row: order }">
                      <el-button v-if="order.Status === 'pending_payment'" type="primary" size="small" @click="openPayModal(order)">
                        {{ isManualPaymentOrder(order) ? '继续人工支付' : '继续支付' }}
                      </el-button>
                      <span v-else class="text-muted">-</span>
                  </template>
                </el-table-column>
              </el-table>
            </div>
            <div class="pagination-bar">
              <span>共 {{ orders.length }} 个订单，第 {{ Math.min(orderPage, totalOrderPages) }} / {{ totalOrderPages }} 页</span>
              <el-pagination
                layout="prev, pager, next"
                :current-page="orderPage"
                :page-size="orderPageSize"
                :total="orders.length"
                @current-change="setOrderPage"
              />
            </div>
          </section>
        </div>

        <aside class="console-dashboard-aside">
          <section class="redeem-card">
            <div class="redeem-card-head">
              <div class="redeem-icon" aria-hidden="true"><Present /></div>
              <div>
                <p class="section-kicker">Redeem</p>
                <h3>激活码兑换</h3>
              </div>
            </div>
            <p>使用激活码兑换包月订阅，享受更多功能</p>
            <form class="redeem-form" @submit.prevent="redeemCode">
              <input
                v-model="redeemForm.code"
                maxlength="14"
                autocomplete="off"
                placeholder="请输入12位激活码"
                @input="redeemForm.code = normalizeRedeemInput(redeemForm.code)"
              />
              <button type="submit" :disabled="redeeming || normalizeRedeemInput(redeemForm.code).length !== 12">
                {{ redeeming ? '兑换中' : '兑换' }}
              </button>
            </form>
          </section>

          <section class="endpoint-card">
            <div>
              <p class="section-kicker">Endpoint</p>
              <h3>API 端点</h3>
            </div>
            <div class="endpoint-list">
              <article v-for="endpoint in displayApiEndpoints" :key="endpoint.url" class="endpoint-item">
                <div class="endpoint-icon" aria-hidden="true">▤</div>
                <div class="endpoint-main">
                  <div class="endpoint-meta">
                    <strong>{{ endpoint.label }}</strong>
                    <span v-if="endpoint.description">{{ endpoint.description }}</span>
                  </div>
                  <code>{{ endpoint.url }}</code>
                  <span
                    v-if="endpointSpeedState(endpoint.url)?.value || endpointSpeedState(endpoint.url)?.error"
                    class="endpoint-speed-result"
                    :class="{ 'endpoint-speed-result--error': endpointSpeedState(endpoint.url)?.error }"
                  >
                    {{ endpointSpeedState(endpoint.url)?.value || '\u6d4b\u901f\u5931\u8d25' }}
                  </span>
                </div>
                <div class="endpoint-actions">
                  <button
                    type="button"
                    class="endpoint-tool-button endpoint-speed-button"
                    :class="{ 'is-loading': endpointSpeedState(endpoint.url)?.loading }"
                    :disabled="endpointSpeedState(endpoint.url)?.loading"
                    aria-label="Test API endpoint speed"
                    title="Test API endpoint speed"
                    @click="testEndpointSpeed(endpoint)"
                  >
                    <Stopwatch />
                    <span>{{ endpointSpeedState(endpoint.url)?.loading ? '\u6d4b\u901f\u4e2d' : '\u6d4b\u901f' }}</span>
                  </button>
                  <button type="button" class="endpoint-tool-button endpoint-copy-button" aria-label="复制 API 端点" title="复制 API 端点" @click="copyKey(endpoint.url)">
                    ⧉
                  </button>
                </div>
              </article>
            </div>
          </section>

          <!-- 套餐管理：侧栏紧凑区 -->
          <section class="panel-surface dashboard-card dashboard-card--plan p-4">
            <div class="section-head">
              <div>
                <p class="section-kicker">Plan</p>
                <h3>套餐管理</h3>
                <p class="section-subtitle text-muted">订阅周期与额度</p>
              </div>
              <el-button class="refresh-button" circle :icon="Refresh" :loading="loading.plan" aria-label="刷新" title="刷新" @click="refreshPlan" />
            </div>

            <div v-if="hasActiveSubscription" class="plan-snapshot-card">
              <div class="plan-snapshot-header">
                <div class="plan-snapshot-icon" aria-hidden="true">▣</div>
                <div class="plan-snapshot-primary">
                  <div>
                    <strong>{{ auth.user?.plan?.name || '当前套餐' }}</strong>
                    <p>{{ quotaPeriodText(auth.user?.plan) }}：{{ usd(auth.user?.plan?.settlement_usd_cents || 0) }}/{{ quotaPeriodUnit(auth.user?.plan) }}</p>
                  </div>
                </div>
                <span class="badge-active">活跃</span>
              </div>

              <div v-if="!currentPlanIsPublic" class="plan-snapshot-meters">
                <div v-if="quotaUsage" class="quota-meter">
                  <div class="quota-meter-head">
                    <span>周期额度</span>
                    <strong>{{ quotaUsagePercent.toFixed(1) }}%</strong>
                  </div>
                  <div class="quota-meter-values">
                    <span>已用 {{ usd(quotaUsage.used_usd_cents || 0) }}</span>
                    <span>剩余 {{ usd(quotaUsage.remaining_usd_cents || 0) }}</span>
                  </div>
                  <div
                    class="quota-progress-track"
                    role="progressbar"
                    :aria-valuenow="Math.round(quotaUsagePercent)"
                    aria-valuemin="0"
                    aria-valuemax="100"
                    :style="quotaProgressStyle"
                  >
                    <span class="quota-progress-fill"></span>
                  </div>
                  <div class="quota-meter-foot text-muted">{{ quotaResetText }}</div>
                </div>

                <div v-if="totalQuotaUsage" class="quota-meter quota-meter--total">
                  <div class="quota-meter-head">
                    <span>总额度</span>
                    <strong>{{ totalQuotaUsagePercent.toFixed(1) }}%</strong>
                  </div>
                  <div class="quota-meter-values">
                    <span>已用 {{ usd(totalQuotaUsage.used_usd_cents || 0) }}</span>
                    <span>总额 {{ usd(totalQuotaUsage.limit_usd_cents || 0) }}</span>
                  </div>
                  <div
                    class="quota-progress-track quota-progress-track--total"
                    role="progressbar"
                    :aria-valuenow="Math.round(totalQuotaUsagePercent)"
                    aria-valuemin="0"
                    aria-valuemax="100"
                    :style="totalQuotaProgressStyle"
                  >
                    <span class="quota-progress-fill"></span>
                  </div>
                  <div class="quota-meter-foot quota-meter-foot--range text-muted">
                    <span>套餐总周期</span>
                    <strong>{{ formatDateTime(totalQuotaUsage.window_start) }} - {{ formatDateTime(totalQuotaUsage.window_end) }}</strong>
                  </div>
                </div>
              </div>

              <div v-else-if="totalQuotaUsage" class="plan-snapshot-meters">
                <div class="quota-meter quota-meter--total">
                  <div class="quota-meter-head">
                    <span>总额度</span>
                    <strong>{{ totalQuotaUsagePercent.toFixed(1) }}%</strong>
                  </div>
                  <div class="quota-meter-values">
                    <span>已用 {{ usd(totalQuotaUsage.used_usd_cents || 0) }}</span>
                    <span>总额 {{ usd(totalQuotaUsage.limit_usd_cents || 0) }}</span>
                  </div>
                  <div
                    class="quota-progress-track quota-progress-track--total"
                    role="progressbar"
                    :aria-valuenow="Math.round(totalQuotaUsagePercent)"
                    aria-valuemin="0"
                    aria-valuemax="100"
                    :style="totalQuotaProgressStyle"
                  >
                    <span class="quota-progress-fill"></span>
                  </div>
                </div>
              </div>

              <div v-if="!currentPlanIsPublic" class="plan-snapshot-times">
                <div class="plan-snapshot-timecell">
                  <span class="detail-label text-muted">套餐开始</span>
                  <span class="detail-value">{{ formatDateTime(planPeriodStartIso) }}</span>
                </div>
                <div class="plan-snapshot-timecell">
                  <span class="detail-label text-muted">套餐结束</span>
                  <span class="detail-value">{{ formatDateTime(auth.user?.expires_at) }}</span>
                </div>
              </div>
            </div>

            <div v-else class="plan-snapshot-card plan-snapshot-card--empty">
              <div class="plan-snapshot-empty-hero">
                <div class="plan-snapshot-icon plan-snapshot-icon--dim" aria-hidden="true">▣</div>
                <div class="plan-snapshot-primary">
                  <div class="plan-snapshot-title-row">
                    <strong>暂无生效套餐</strong>
                  </div>
                  <p class="text-muted plan-snapshot-empty-desc">选择套餐并完成支付审核后，这里会显示额度、周期和到期时间。</p>
                </div>
              </div>
              <div class="plan-snapshot-empty-steps">
                <span>选择套餐</span>
                <span>完成支付</span>
                <span>审核开通</span>
              </div>
              <el-button type="primary" class="plan-snapshot-empty-action" @click="$emit('navigate', '/plans')">去订购套餐</el-button>
            </div>

            <div class="plan-card-actions">
              <button type="button" class="primary-button" @click="openUsageRecords">使用记录</button>
              <button type="button" class="ghost-button" @click="openDocs">使用教程</button>
            </div>
          </section>

        </aside>
      </div>
    </div>

    <el-dialog v-model="modal.open" :title="modal.title" width="560px" align-center @close="closeModal">
      <el-form class="modal-card" label-position="top" @submit.prevent="submitModal">

        <div v-if="modal.type === 'pay-order'" class="modal-body">
          <div class="payment-panel">
            <strong>{{ orderForm.order?.Plan?.Name || '套餐订单' }}</strong>
            <span>订单金额：{{ money(orderForm.order?.AmountCents) }}</span>
            <span v-if="orderForm.order?.Status === 'pending_payment'" class="payment-countdown">支付剩余时间：{{ orderCountdown(orderForm.order) }}</span>
            <span v-else-if="orderForm.order?.Status" class="payment-countdown">当前状态：{{ statusLabel(orderForm.order.Status) }}</span>
            <p>请在 5 分钟内点击“去支付”打开支付页面。完成支付后回到这里点击“已完成支付”，系统确认支付成功后才会进入待审核。</p>
            <el-button type="primary" @click="startPayment">
              {{ orderForm.paymentOpened ? '重新打开支付页面' : '去支付' }}
            </el-button>
          </div>
        </div>

        <div v-if="modal.type === 'manual-pay-order'" class="modal-body">
          <div class="payment-panel manual-payment-panel">
            <strong>{{ orderForm.order?.Plan?.Name || '套餐订单' }}</strong>
            <span>订单金额：{{ money(orderForm.order?.AmountCents) }}</span>
            <p>请使用下方付款二维码扫码支付，支付时备注或留言你的当前账号，方便管理员核对。人工支付可节省在线支付手续费。</p>
            <div v-if="orderForm.manualQRCode" class="manual-payment-qr">
              <img :src="orderForm.manualQRCode" alt="人工支付付款二维码" />
            </div>
            <div v-else class="manual-payment-empty">
              管理员尚未上传付款二维码，请联系站点支持。
            </div>
            <el-form-item label="付款备注 / 当前账号" required>
              <el-input v-model="orderForm.manualNote" type="textarea" :rows="3" placeholder="请填写当前账号邮箱、转账备注或其他便于核对的信息" />
            </el-form-item>
          </div>
        </div>

        <div v-if="modal.type === 'create-key' || modal.type === 'rotate-key'" class="modal-body">
          <div v-if="modal.type === 'rotate-key'" class="order-flow-note md:col-span-2">
            <strong>将替换当前唯一密钥</strong>
            <span>确认后旧密钥立即失效，所有使用旧 Key 的客户端需同步更新。</span>
          </div>
          <el-form-item label="Key 名称" required>
            <el-input v-model="keyForm.name" minlength="2" placeholder="生产环境 Key" />
          </el-form-item>
        </div>

        <div v-if="modal.type === 'disable-key'" class="modal-body confirm-copy">
          <strong>确定禁用「{{ modal.payload?.key?.name }}」吗？</strong>
          <p>禁用后该 Key 将不能继续调用网关接口。</p>
        </div>

      </el-form>
      <template #footer>
        <div class="modal-actions">
          <el-button @click="closeModal">取消</el-button>
          <el-button :type="modal.danger ? 'danger' : 'primary'" @click="submitModal">{{ modal.actionLabel }}</el-button>
        </div>
      </template>
    </el-dialog>

    <div v-if="copySuccessModalOpen" class="modal-backdrop" @click.self="closeCopySuccessModal">
      <div class="modal-card" role="dialog" aria-labelledby="copy-success-title">
        <div class="modal-head">
          <h3 id="copy-success-title">复制成功</h3>
          <button type="button" class="icon-button" aria-label="关闭" @click="closeCopySuccessModal">×</button>
        </div>
        <div class="modal-body confirm-copy">
          <p>完整密钥已复制到剪贴板。请粘贴到安全环境保存，勿发送给他人或提交到公开仓库。</p>
        </div>
        <div class="modal-actions">
          <button type="button" class="primary-button" @click="closeCopySuccessModal">知道了</button>
        </div>
      </div>
    </div>

    <div v-if="useKeyModalOpen" class="modal-backdrop key-usage-backdrop" @click.self="closeUseKeyModal">
      <div class="modal-card key-usage-modal" role="dialog" aria-labelledby="key-usage-title">
        <div class="key-usage-head">
          <div>
            <p class="section-kicker">API Key Setup</p>
            <h3 id="key-usage-title">使用密钥</h3>
          </div>
          <button type="button" class="icon-button key-usage-close" aria-label="关闭" @click="closeUseKeyModal">×</button>
        </div>
        <div class="key-usage-summary">
          <div>
            <span>API 地址</span>
            <code>{{ apiBaseURL }}</code>
            <button type="button" class="ghost-button small" @click="copyKey(apiBaseURL)">复制</button>
          </div>
          <div>
            <span>API Key</span>
            <code>{{ usagePlainKey }}</code>
            <button type="button" class="ghost-button small" @click="copyKey(usagePlainKey)">复制</button>
          </div>
        </div>
        <div class="key-usage-tabs">
          <button type="button" :class="{ active: keyUsageTab === 'claude' }" @click="keyUsageTab = 'claude'">Claude Code</button>
          <button type="button" :class="{ active: keyUsageTab === 'opencode' }" @click="keyUsageTab = 'opencode'">OpenCode</button>
          <button type="button" :class="{ active: keyUsageTab === 'codex' }" @click="keyUsageTab = 'codex'">Codex</button>
        </div>
        <div v-if="keyUsageTab === 'claude'" class="key-usage-body">
          <div class="key-usage-tabs key-usage-tabs--small">
            <button type="button" :class="{ active: claudeOSTab === 'shell' }" @click="claudeOSTab = 'shell'">macOS/Linux</button>
            <button type="button" :class="{ active: claudeOSTab === 'cmd' }" @click="claudeOSTab = 'cmd'">Windows CMD</button>
            <button type="button" :class="{ active: claudeOSTab === 'powershell' }" @click="claudeOSTab = 'powershell'">PowerShell</button>
          </div>
          <div class="snippet-card">
            <div><strong>终端环境变量</strong><button type="button" class="ghost-button small" @click="copyKey(claudeSnippet(claudeOSTab))">复制</button></div>
            <pre><code>{{ claudeSnippet(claudeOSTab) }}</code></pre>
          </div>
          <div class="snippet-card">
            <div><strong>VSCode settings.json</strong><button type="button" class="ghost-button small" @click="copyKey(claudeVSCodeSnippet)">复制</button></div>
            <pre><code>{{ claudeVSCodeSnippet }}</code></pre>
          </div>
        </div>
        <div v-else-if="keyUsageTab === 'opencode'" class="key-usage-body">
          <div class="snippet-card">
            <div><strong>opencode.json</strong><button type="button" class="ghost-button small" @click="copyKey(openCodeSnippet)">复制</button></div>
            <pre><code>{{ openCodeSnippet }}</code></pre>
          </div>
        </div>
        <div v-else class="key-usage-body">
          <div class="snippet-card">
            <div><strong>config.toml</strong><button type="button" class="ghost-button small" @click="copyKey(codexConfigSnippet)">复制</button></div>
            <pre><code>{{ codexConfigSnippet }}</code></pre>
          </div>
          <div class="snippet-card">
            <div><strong>auth.json</strong><button type="button" class="ghost-button small" @click="copyKey(codexAuthSnippet)">复制</button></div>
            <pre><code>{{ codexAuthSnippet }}</code></pre>
          </div>
        </div>
      </div>
    </div>

    <Transition name="history-pop">
      <div v-if="historyModalOpen" class="modal-backdrop announcement-history-backdrop" @click.self="closeAnnouncementHistory">
        <div class="modal-card announcement-history-modal" role="dialog" aria-labelledby="announcement-history-title">
          <div class="announcement-history-head">
            <div>
              <p class="section-kicker">Announcements</p>
              <h3 id="announcement-history-title">历史公告</h3>
            </div>
            <button type="button" class="icon-button" aria-label="关闭" @click="closeAnnouncementHistory">×</button>
          </div>
          <div class="announcement-timeline">
            <div v-if="!historyAnnouncements.length" class="announcement-history-empty">
              <strong>暂无历史公告</strong>
              <span>当前只有这一条最新公告，后续发布的新公告会把旧公告归入这里。</span>
            </div>
            <article v-for="(item, index) in historyAnnouncements" :key="item.ID" class="announcement-history-item">
              <div class="announcement-history-marker">
                <span>{{ index + 1 }}</span>
              </div>
              <div class="announcement-history-panel">
                <div class="announcement-history-meta">
                  <time>{{ announcementDate(item) }}</time>
                  <span v-if="item.Pinned">置顶</span>
                </div>
                <h4>{{ item.Title }}</h4>
                <div class="markdown-body" v-html="announcementHtml(item)"></div>
                <a v-if="item.LinkURL" :href="item.LinkURL" target="_blank" rel="noopener noreferrer">
                  {{ item.LinkText || '查看详情' }}
                </a>
              </div>
            </article>
          </div>
        </div>
      </div>
    </Transition>
  </section>
</template>
