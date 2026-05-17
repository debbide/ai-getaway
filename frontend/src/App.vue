<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessageBox } from 'element-plus'
import { api } from './api/client'
import { useAuthStore } from './stores/auth'
import AuthModal from './components/AuthModal.vue'
import Dashboard from './components/Dashboard.vue'
import AdminPanel from './components/AdminPanel.vue'
import UsageRecords from './components/UsageRecords.vue'
import DocsPage from './components/DocsPage.vue'
import ModelsPage from './components/ModelsPage.vue'
import FaqPage from './components/FaqPage.vue'

const defaultNavigation = [
  { label: '首页', path: '/' },
  { label: '教程 ↗', path: '/docs' },
  { label: '定价', path: '/plans' },
  { label: '模型', path: '/models' },
  { label: '常见问题', path: '/faq' }
]

const defaultSettings = {
  site_title: '星空AI',
  contact_email: 'support@example.com',
  api_endpoints: JSON.stringify([{ label: '默认', description: '主线路', url: 'https://ai.itzkb.cn' }]),
  navigation_items: JSON.stringify(defaultNavigation),
  pricing_title: '简单透明的定价',
  pricing_subtitle: '保质保量无降智不掺假',
  pricing_notice: '本站仅支持 GPT 模型使用，具体型号请查看 /models 页面；如需使用 Claude 模型，请前往顶部菜单更多中转 → Claude Code 中转',
  allow_registration: true,
  online_payment_enabled: true,
  manual_payment_enabled: true,
  mock_api_online_enabled: false,
  mock_api_online_base: 0
}
const manualPaymentConfirmMessage = '请确认已成功支付，恶意创建人工支付订单且未支付的用户将会遭到封禁处理，请知悉。'

const auth = useAuthStore()
const plans = ref([])
const authOpen = ref(false)
const authMode = ref('login')
const error = ref('')
const currentPath = ref(window.location.pathname)
const publicSettings = ref({ ...defaultSettings })
const themeMode = ref(localStorage.getItem('themeMode') || 'dark')
const themeMenuOpen = ref(false)
const accountMenuOpen = ref(false)
const passwordModalOpen = ref(false)
const passwordSaving = ref(false)
const passwordError = ref('')
const passwordNotice = ref('')
const pricingTab = ref('daily')
const passwordForm = reactive({ oldPassword: '', newPassword: '', confirmPassword: '' })
const apiOnlineWidgetCollapsed = ref(localStorage.getItem('apiOnlineWidgetCollapsed') === '1')
const apiOnlineCount = ref(0)
let apiOnlineTimer = null
const orderModal = reactive({
  open: false,
  loading: false,
  error: '',
  plan: null,
  paymentMethod: 'online',
  order: null,
  paymentUrl: '',
  paymentOpened: false,
  manualQRCode: '',
  manualNote: ''
})

const isConsolePage = computed(() => currentPath.value === '/console')
const isAdminPage = computed(() => currentPath.value === '/admin')
const isUsageRecordsPage = computed(() => currentPath.value === '/usage-records')
const isPlansPage = computed(() => currentPath.value === '/plans')
const isModelsPage = computed(() => currentPath.value === '/models')
const isDocsPage = computed(() => currentPath.value === '/docs' || currentPath.value.startsWith('/docs/'))
const isFaqPage = computed(() => currentPath.value === '/faq')
const navItems = computed(() => parseNavigation(publicSettings.value.navigation_items))
const activeThemeLabel = computed(() => ({ light: '浅色', dark: '深色', system: '系统' })[themeMode.value] || '深色')
const accountEmail = computed(() => auth.user?.email || '')
const accountName = computed(() => auth.user?.username || accountEmail.value.split('@')[0] || '用户')
const dailyPlans = computed(() => plans.value.filter((plan) => !isLotteryPlan(plan) && !isFreePlan(plan) && plan.QuotaPeriod === 'daily' && plan.PlanType !== 'public'))
const weeklyPlans = computed(() => plans.value.filter((plan) => !isLotteryPlan(plan) && !isFreePlan(plan) && plan.QuotaPeriod !== 'daily' && plan.QuotaPeriod !== 'public' && plan.PlanType !== 'public'))
const publicPlans = computed(() => plans.value.filter((plan) => !isLotteryPlan(plan) && !isFreePlan(plan) && (plan.QuotaPeriod === 'public' || plan.PlanType === 'public')))
const freePlans = computed(() => plans.value.filter((plan) => !isLotteryPlan(plan) && Number(plan.PriceCents || 0) === 0))
const lotteryPlans = computed(() => plans.value.filter((plan) => isLotteryPlan(plan)))
const onlinePaymentEnabled = computed(() => publicSettings.value.online_payment_enabled !== false)
const manualPaymentEnabled = computed(() => publicSettings.value.manual_payment_enabled !== false)
const mockAPIOnlineEnabled = computed(() => publicSettings.value.mock_api_online_enabled === true)
const mockAPIOnlineBase = computed(() => Math.max(0, Number(publicSettings.value.mock_api_online_base || 0)))
const hasEnabledPaymentMethod = computed(() => onlinePaymentEnabled.value || manualPaymentEnabled.value)
const visiblePricingPlans = computed(() => {
  if (pricingTab.value === 'daily') return dailyPlans.value
  if (pricingTab.value === 'weekly') return weeklyPlans.value
  if (pricingTab.value === 'public') return publicPlans.value
  if (pricingTab.value === 'free') return freePlans.value
  if (pricingTab.value === 'lottery') return lotteryPlans.value
  return dailyPlans.value
})
const avatarText = computed(() => {
  const source = accountEmail.value || accountName.value || 'U'
  return source.slice(0, 2).toUpperCase()
})

onMounted(async () => {
  window.addEventListener('popstate', syncPath)
  window.addEventListener('app-data-updated', refreshAppData)
  window.addEventListener('auth-expired', handleAuthExpired)
  window.matchMedia?.('(prefers-color-scheme: dark)').addEventListener?.('change', applyTheme)
  applyTheme()
  await auth.loadMe()
  await loadPublicSettings()
  await loadPlans()
  syncMockAPIOnlineWidget()
})

watch(currentPath, () => {
  refreshAppData()
})

watch(
  () => [publicSettings.value.mock_api_online_enabled, publicSettings.value.mock_api_online_base],
  () => {
    syncMockAPIOnlineWidget()
  }
)

onBeforeUnmount(() => {
  window.removeEventListener('popstate', syncPath)
  window.removeEventListener('app-data-updated', refreshAppData)
  window.removeEventListener('auth-expired', handleAuthExpired)
  window.matchMedia?.('(prefers-color-scheme: dark)').removeEventListener?.('change', applyTheme)
  stopMockAPIOnlineTicker()
})

async function loadPlans() {
  try {
    const res = await api.get('/plans')
    plans.value = res.data || []
  } catch (err) {
    error.value = err.message
  }
}

async function loadPublicSettings() {
  try {
    const res = await api.get('/settings/public')
    publicSettings.value = { ...defaultSettings, ...res.data }
    if (publicSettings.value.site_title) document.title = publicSettings.value.site_title
  } catch {
    publicSettings.value = { ...defaultSettings }
  }
}

async function refreshAppData() {
  await Promise.allSettled([loadPublicSettings(), loadPlans(), auth.loadMe()])
}

function toggleAPIOnlineWidget() {
  apiOnlineWidgetCollapsed.value = !apiOnlineWidgetCollapsed.value
  localStorage.setItem('apiOnlineWidgetCollapsed', apiOnlineWidgetCollapsed.value ? '1' : '0')
}

function syncMockAPIOnlineWidget() {
  if (!mockAPIOnlineEnabled.value) {
    stopMockAPIOnlineTicker()
    apiOnlineCount.value = 0
    return
  }
  if (apiOnlineCount.value < currentMockAPIOnlineFloor()) {
    apiOnlineCount.value = computeMockAPIOnlineBaseline()
  }
  startMockAPIOnlineTicker()
}

function startMockAPIOnlineTicker() {
  stopMockAPIOnlineTicker()
  tickMockAPIOnlineCount()
}

function stopMockAPIOnlineTicker() {
  if (!apiOnlineTimer) return
  clearTimeout(apiOnlineTimer)
  apiOnlineTimer = null
}

function scheduleMockAPIOnlineTick() {
  const delay = 3200 + Math.floor(Math.random() * 5200)
  apiOnlineTimer = setTimeout(tickMockAPIOnlineCount, delay)
}

function tickMockAPIOnlineCount() {
  if (!mockAPIOnlineEnabled.value) {
    apiOnlineCount.value = 0
    stopMockAPIOnlineTicker()
    return
  }

  const floor = currentMockAPIOnlineFloor()
  const baseline = computeMockAPIOnlineBaseline()
  const steps = [-2, -1, 1, 3]
  const drift = baseline - apiOnlineCount.value
  let delta = steps[Math.floor(Math.random() * steps.length)]

  if (drift >= 18) {
    delta = Math.random() < 0.7 ? 3 : 1
  } else if (drift >= 8) {
    delta = Math.random() < 0.65 ? 1 : 3
  } else if (drift <= -18) {
    delta = Math.random() < 0.7 ? -2 : -1
  } else if (drift <= -8) {
    delta = Math.random() < 0.65 ? -1 : -2
  } else if (Math.random() < 0.18) {
    delta = drift >= 0 ? 1 : -1
  }

  const next = Math.max(floor, apiOnlineCount.value + delta)
  const upperCap = Math.max(floor + 6, baseline + 18)
  apiOnlineCount.value = Math.min(next, upperCap)
  scheduleMockAPIOnlineTick()
}

function computeMockAPIOnlineBaseline() {
  const base = mockAPIOnlineBase.value
  const phase = apiOnlinePhase()
  if (phase === 'night') {
    return 1 + Math.floor(Math.random() * 5)
  }
  if (base <= 0) return 0

  const now = new Date()
  const day = now.getDay()
  const hour = now.getHours()
  const minute = now.getMinutes()
  const totalMinutes = hour * 60 + minute
  const weekday = day >= 1 && day <= 5
  const peak = weekday && totalMinutes >= 13 * 60 && totalMinutes <= 17 * 60 + 30

  let offset = 0
  if (peak) {
    offset = Math.max(6, Math.round(base * 0.18)) + Math.floor(Math.random() * 8)
  } else if (weekday && totalMinutes >= 9 * 60 && totalMinutes <= 12 * 60) {
    offset = Math.max(3, Math.round(base * 0.1)) + Math.floor(Math.random() * 5)
  } else if (weekday && totalMinutes >= 19 * 60 && totalMinutes < 22 * 60) {
    offset = Math.max(2, Math.round(base * 0.08)) + Math.floor(Math.random() * 4)
  } else {
    offset = Math.floor(Math.random() * 4)
  }

  return base + offset
}

function currentMockAPIOnlineFloor() {
  const phase = apiOnlinePhase()
  const base = mockAPIOnlineBase.value
  if (phase === 'night') return 1
  if (base <= 0) return 0
  if (phase === 'peak') return Math.max(1, base - Math.max(3, Math.round(base * 0.06)))
  if (phase === 'busy') return Math.max(1, base - Math.max(4, Math.round(base * 0.08)))
  return Math.max(1, base - Math.max(5, Math.round(base * 0.12)))
}

function apiOnlineDisplayCount() {
  return Math.max(currentMockAPIOnlineFloor(), apiOnlineCount.value).toLocaleString('zh-CN')
}

function apiOnlinePhase() {
  const now = new Date()
  const day = now.getDay()
  const hour = now.getHours()
  const minute = now.getMinutes()
  const totalMinutes = hour * 60 + minute
  const weekday = day >= 1 && day <= 5
  if (weekday && totalMinutes >= 13 * 60 && totalMinutes <= 17 * 60 + 30) return 'peak'
  if (weekday && totalMinutes >= 9 * 60 && totalMinutes <= 12 * 60) return 'busy'
  if (totalMinutes >= 22 * 60 || totalMinutes <= 6 * 60) return 'night'
  if (weekday && totalMinutes >= 19 * 60 && totalMinutes < 22 * 60) return 'steady'
  return 'steady'
}

function apiOnlineStatusLabel() {
  return {
    peak: '高峰期',
    busy: '繁忙中',
    steady: '平稳中',
    night: '夜间'
  }[apiOnlinePhase()] || '平稳中'
}

function apiOnlineIndicatorClass() {
  const count = Math.max(currentMockAPIOnlineFloor(), apiOnlineCount.value)
  if (count >= 1000) return 'critical'
  const phase = apiOnlinePhase()
  if (phase === 'peak' || phase === 'busy') return 'busy'
  return 'normal'
}

function handleAuthExpired() {
  auth.logout()
  if (authOpen.value) return
  openAuth('login')
}

function parseNavigation(value) {
  try {
    const parsed = JSON.parse(value || '[]')
    return Array.isArray(parsed) && parsed.length ? parsed : defaultNavigation
  } catch {
    return defaultNavigation
  }
}

function syncPath() {
  currentPath.value = window.location.pathname
}

function navigate(path) {
  if (!path || path === '#') return
  if (/^https?:\/\//i.test(path)) {
    window.open(path, '_blank', 'noopener,noreferrer')
    return
  }
  if (path.startsWith('#')) {
    navigateSection(path.slice(1))
    return
  }
  if (window.location.pathname !== path) {
    window.history.pushState({}, '', path)
    syncPath()
  }
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function navigateSection(id) {
  const scrollToSection = () => document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' })
  if (window.location.pathname !== '/') {
    window.history.pushState({}, '', '/')
    syncPath()
    requestAnimationFrame(scrollToSection)
    return
  }
  scrollToSection()
}

function navigateItem(item) {
  if (!item?.path || item.path === '#') return
  if (item.external && !item.path.startsWith('#')) {
    window.open(item.path, '_blank', 'noopener,noreferrer')
    return
  }
  navigate(item.path)
}

function openAuth(mode) {
  if (mode === 'register' && !publicSettings.value.allow_registration) {
    error.value = '当前站点暂未开放新用户注册'
    mode = 'login'
  }
  authMode.value = mode
  authOpen.value = true
}

function enterConsole() {
  if (!auth.loggedIn) {
    openAuth('login')
    return
  }
  navigate('/console')
}

function enterAdmin() {
  if (!auth.loggedIn) {
    openAuth('login')
    return
  }
  navigate('/admin')
}

function afterPrimaryAction() {
  if (auth.loggedIn) {
    navigate('/console')
    return
  }
  openAuth(publicSettings.value.allow_registration ? 'register' : 'login')
}

function setTheme(mode) {
  themeMode.value = mode
  localStorage.setItem('themeMode', mode)
  themeMenuOpen.value = false
  applyTheme()
}

function openPasswordModal() {
  accountMenuOpen.value = false
  passwordError.value = ''
  passwordNotice.value = ''
  Object.assign(passwordForm, { oldPassword: '', newPassword: '', confirmPassword: '' })
  passwordModalOpen.value = true
}

function closePasswordModal() {
  passwordModalOpen.value = false
  passwordSaving.value = false
}

async function submitPassword() {
  passwordError.value = ''
  passwordNotice.value = ''
  if (passwordForm.newPassword.length <= 6) {
    passwordError.value = '新密码长度需要超过 6 位'
    return
  }
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    passwordError.value = '两次输入的新密码不一致'
    return
  }
  passwordSaving.value = true
  try {
    await api.patch('/auth/password', {
      old_password: passwordForm.oldPassword,
      new_password: passwordForm.newPassword,
      confirm_password: passwordForm.confirmPassword
    })
    passwordNotice.value = '密码已修改，请使用新密码登录'
    Object.assign(passwordForm, { oldPassword: '', newPassword: '', confirmPassword: '' })
    setTimeout(() => {
      passwordModalOpen.value = false
    }, 620)
  } catch (err) {
    passwordError.value = err.message
  } finally {
    passwordSaving.value = false
  }
}

function logoutAccount() {
  accountMenuOpen.value = false
  auth.logout()
  navigate('/')
}

function applyTheme() {
  const systemDark = window.matchMedia?.('(prefers-color-scheme: dark)').matches
  const resolved = themeMode.value === 'system' ? (systemDark ? 'dark' : 'light') : themeMode.value
  document.documentElement.dataset.theme = resolved
}

function priceRmb(plan) {
  if (isLotteryPlan(plan)) return '抽奖'
  if (isFreePlan(plan)) return '免费'
  return ((plan.PriceCents || 0) / 100).toFixed((plan.PriceCents || 0) % 100 === 0 ? 0 : 1)
}

async function confirmManualPaymentSubmission() {
  try {
    await ElMessageBox.confirm(manualPaymentConfirmMessage, '确认人工支付', {
      confirmButtonText: '确认已支付',
      cancelButtonText: '取消',
      type: 'warning'
    })
    return true
  } catch {
    return false
  }
}

function periodUsd(plan) {
  return ((plan.SettlementUSDCents || 0) / 100).toFixed((plan.SettlementUSDCents || 0) % 100 === 0 ? 0 : 2)
}

function quotaPeriodLabel(plan) {
  if (isLotteryPlan(plan)) return '抽奖套餐'
  if (plan.QuotaPeriod === 'public' || plan.PlanType === 'public') return '公共套餐'
  return plan.QuotaPeriod === 'daily' ? '日限额度' : '周限额度'
}

function totalUsd(plan) {
  if (plan.QuotaPeriod === 'public' || plan.PlanType === 'public') return ((plan.SettlementUSDCents || 0) / 100).toFixed(0)
  const units = plan.QuotaPeriod === 'daily' ? (plan.DurationDays || 1) : Math.max(1, Math.round((plan.DurationDays || 30) / 7))
  return (((plan.SettlementUSDCents || 0) / 100) * units).toFixed(0)
}

function planPeriod(plan) {
  if (isLotteryPlan(plan)) return '活动'
  if (plan.QuotaPeriod === 'public' || plan.PlanType === 'public') return '次'
  if ((plan.DurationDays || 0) <= 1) return '天'
  if ((plan.DurationDays || 0) >= 28) return '月'
  return `${plan.DurationDays} 天`
}

function planSoldOut(plan) {
  if (isLotteryPlan(plan)) return false
  return (plan.QuotaPeriod === 'public' || plan.PlanType === 'public') && Number(plan.PublicChannel?.RemainingUSDCents || 0) < Number(plan.SettlementUSDCents || 0)
}

function isLotteryPlan(plan) {
  return Boolean(plan?.IsLottery || plan?.is_lottery)
}

function isFreePlan(plan) {
  return !isLotteryPlan(plan) && Number(plan?.PriceCents || plan?.price_cents || 0) === 0
}

function openPlanAction(plan) {
  if (isLotteryPlan(plan)) {
    const url = String(plan?.LotteryURL || '').trim()
    if (url) window.location.href = url
    return
  }
  openPricingOrder(plan)
}

function openPricingOrder(plan) {
  if (!auth.loggedIn) {
    openAuth(publicSettings.value.allow_registration ? 'register' : 'login')
    return
  }
  if (planSoldOut(plan)) return
  const paymentMethod = onlinePaymentEnabled.value ? 'online' : (manualPaymentEnabled.value ? 'manual' : '')
  Object.assign(orderModal, {
    open: true,
    loading: false,
    error: paymentMethod ? '' : '当前没有可用的支付方式，请联系管理员',
    plan,
    paymentMethod: isFreePlan(plan) ? 'free' : paymentMethod,
    order: null,
    paymentUrl: '',
    manualQRCode: '',
    paymentOpened: false,
    manualNote: accountPaymentNote()
  })
}

function closeOrderModal({ force = false } = {}) {
  if (orderModal.loading && !force) return
  Object.assign(orderModal, {
    open: false,
    loading: false,
    error: '',
    plan: null,
    paymentMethod: 'online',
    order: null,
    paymentUrl: '',
    paymentOpened: false,
    manualQRCode: '',
    manualNote: ''
  })
}

async function submitPlanOrder() {
  if (!orderModal.plan?.ID) return
  if (!isFreePlan(orderModal.plan) && (!hasEnabledPaymentMethod.value || !orderModal.paymentMethod)) {
    orderModal.error = '当前没有可用的支付方式，请联系管理员'
    return
  }
  orderModal.loading = true
  orderModal.error = ''
  try {
    const res = await api.post('/orders', {
      plan_id: Number(orderModal.plan.ID),
      payment_method: isFreePlan(orderModal.plan) ? 'free' : orderModal.paymentMethod
    })
    orderModal.order = res.data?.order
    if (isFreePlan(orderModal.plan)) {
      await auth.loadMe()
      navigate('/console')
      await nextTick()
      closeOrderModal({ force: true })
      return
    }
    if (orderModal.paymentMethod === 'manual') {
      await loadManualPaymentInfo()
      return
    }
    await openOnlinePaymentWindow()
  } catch (err) {
    orderModal.error = err.message
  } finally {
    orderModal.loading = false
  }
}

async function openOnlinePaymentWindow() {
  if (!orderModal.order?.ID) return
  const payRes = await api.post(`/orders/${orderModal.order.ID}/pay`)
  orderModal.paymentUrl = payRes.data?.payment_url || ''
  if (orderModal.paymentUrl) {
    orderModal.paymentOpened = true
    window.open(orderModal.paymentUrl, '_blank', 'noopener,noreferrer')
  }
}

async function confirmOnlinePayment() {
  if (!orderModal.order?.ID) return
  orderModal.loading = true
  orderModal.error = ''
  try {
    await api.patch(`/orders/${orderModal.order.ID}/paid`)
    navigate('/console')
    await nextTick()
    closeOrderModal({ force: true })
  } catch (err) {
    orderModal.error = err.message
  } finally {
    orderModal.loading = false
  }
}

async function loadManualPaymentInfo() {
  const res = await api.get('/payment/manual')
  orderModal.manualQRCode = res.data?.manual_payment_qr_code || ''
}

async function submitManualPayment() {
  if (!orderModal.order?.ID) return
  if (!String(orderModal.manualNote || '').trim()) {
    orderModal.error = '请填写当前账号或转账留言，方便管理员核对'
    return
  }
  if (!(await confirmManualPaymentSubmission())) return
  orderModal.loading = true
  orderModal.error = ''
  try {
    await api.post(`/orders/${orderModal.order.ID}/manual-payment`, {
      user_payment_note: orderModal.manualNote
    })
    navigate('/console')
    await nextTick()
    closeOrderModal({ force: true })
  } catch (err) {
    orderModal.error = err.message
  } finally {
    orderModal.loading = false
  }
}

function accountPaymentNote() {
  return auth.user?.email || auth.user?.username || ''
}

function publicRemainingUsd(plan) {
  return ((plan.PublicChannel?.RemainingUSDCents || 0) / 100).toFixed(0)
}

function planBadge(index) {
  return ['日用特惠', '热卖推荐', '高频进阶'][index % 3]
}

function planSubtitle(index) {
  return ['灵活应对突发需求', '覆盖常规研发工作量', '为高频团队保驾护航'][index % 3]
}
</script>

<template>
  <div class="app-frame min-h-screen">
    <header class="site-header">
      <div class="site-nav mx-auto flex max-w-7xl items-center justify-between px-4 sm:px-6">
        <button class="brand-lockup focus-ring" @click="navigate('/')">
          <span class="brand-mark">XK</span>
          <strong>{{ publicSettings.site_title || '星空AI' }}</strong>
        </button>

        <nav class="hidden items-center gap-7 text-sm font-bold md:flex">
          <div v-for="item in navItems" :key="item.label" class="nav-menu-item">
            <button class="nav-link" @click="navigateItem(item)">{{ item.label }}</button>
            <div v-if="item.children?.length" class="nav-submenu">
              <button v-for="child in item.children" :key="child.label" @click="navigateItem(child)">
                {{ child.label }}
              </button>
            </div>
          </div>
        </nav>

        <div class="header-actions">
          <button class="icon-pill" title="语言">◎</button>
          <div class="theme-switcher">
            <button class="icon-pill" title="主题" @click="themeMenuOpen = !themeMenuOpen">☾</button>
            <div v-if="themeMenuOpen" class="theme-menu">
              <button :class="{ active: themeMode === 'light' }" @click="setTheme('light')">☼ 浅色</button>
              <button :class="{ active: themeMode === 'dark' }" @click="setTheme('dark')">☾ 深色</button>
              <button :class="{ active: themeMode === 'system' }" @click="setTheme('system')">▣ 系统</button>
            </div>
          </div>
          <template v-if="auth.loggedIn">
            <button class="console-link" @click="enterConsole">控制台</button>
            <button v-if="auth.isAdmin" class="console-link" @click="enterAdmin">管理后台</button>
            <div class="account-menu-wrap">
              <button class="score-badge" @click="accountMenuOpen = !accountMenuOpen">{{ avatarText }}</button>
              <Transition name="account-menu">
                <div v-if="accountMenuOpen" class="account-menu">
                  <div class="account-card-head">
                    <span class="account-avatar">{{ avatarText }}</span>
                    <div>
                      <strong>{{ accountName }}</strong>
                      <small>{{ accountEmail }}</small>
                    </div>
                  </div>
                  <button class="account-menu-item" @click="openPasswordModal">
                    <span class="account-icon">⚿</span>
                    修改密码
                  </button>
                  <button class="account-menu-item" @click="logoutAccount">
                    <span class="account-icon">↪</span>
                    退出登录
                  </button>
                </div>
              </Transition>
            </div>
          </template>
          <button v-else class="login-pill" @click="openAuth('login')">登录</button>
        </div>
      </div>
    </header>

    <DocsPage v-if="isDocsPage" :key="currentPath" />

    <ModelsPage v-else-if="isModelsPage" :key="currentPath" @navigate="navigate" @start="afterPrimaryAction" />

    <FaqPage v-else-if="isFaqPage" :key="currentPath" @navigate="navigate" @start="afterPrimaryAction" />

    <main v-else-if="!isConsolePage && !isAdminPage && !isPlansPage && !isUsageRecordsPage">
      <section class="home-hero">
        <div class="home-hero-inner mx-auto max-w-7xl px-4 sm:px-6">
          <div class="hero-badge">✣ 为中国开发者量身打造</div>
          <h1>
            <span>{{ publicSettings.site_title || '星空AI' }}</span>
            AI驱动的编程助手
          </h1>
          <p>
            {{ publicSettings.site_title || '星空AI' }} 是专为中国开发者设计的智能编程助手，通过先进的AI技术提供代码生成、调试优化和实时协作功能，让编程更高效、更智能。
          </p>
          <div class="hero-tags">
            <span>✓ 强大功能，助力开发</span>
            <span>✓ 体验AI编程的革新力量</span>
          </div>
          <div class="hero-actions">
            <button class="hero-primary" @click="afterPrimaryAction">立即使用 <span>→</span></button>
            <button class="hero-secondary" @click="navigate('/models')">查看模型</button>
          </div>
        </div>
      </section>

      <section class="home-value-stage">
        <div class="mx-auto grid max-w-7xl gap-6 px-4 py-14 sm:px-6">
          <article class="home-value-panel reveal-copy">
            <p class="section-kicker">Low Cost API</p>
            <h2>超低价爽用的AI大模型API服务</h2>
            <p>
              全站调用0.06RMB=1USD，坦白说：我们至少比友商便宜 60%，亚洲服务器中转，快速稳定，价低量大。
            </p>
            <div class="value-metrics">
              <span><strong>0.06RMB</strong><small>= 1USD</small></span>
              <span><strong>60%+</strong><small>成本优势</small></span>
              <span><strong>亚洲节点</strong><small>稳定中转</small></span>
            </div>
          </article>

          <article class="home-value-panel home-value-panel-alt reveal-copy">
            <p class="section-kicker">Fast Stable Service</p>
            <h2>更低廉的价格 · 更快速的响应 · 更稳定的服务</h2>
            <p>
              0.06RMB=1USD，价格低得令人难以置信。亚洲服务器中转，快速稳定，价低量大。
            </p>
            <div class="value-strip">
              <span>更低价格</span>
              <span>更快响应</span>
              <span>更稳服务</span>
            </div>
          </article>

          <div class="home-pricing-cta reveal-copy">
            <span>想看具体套餐和模型价格？</span>
            <button type="button" @click="navigate('/plans')">查看模型价格 <b>→</b></button>
          </div>
        </div>
      </section>
    </main>

    <main v-else-if="isPlansPage" class="pricing-stage">
      <section class="mx-auto max-w-7xl px-4 py-14 sm:px-6">
        <div class="pricing-title">
          <h1>{{ publicSettings.pricing_title || '简单透明的定价' }}</h1>
          <span>{{ publicSettings.pricing_subtitle || '保质保量无降智不掺假' }}</span>
          <div v-if="publicSettings.pricing_notice" class="pricing-notice">
            <span class="notice-icon">i</span>
            <p>{{ publicSettings.pricing_notice }}</p>
          </div>
          <div class="pricing-dots">
            <span></span><span></span><span></span>
          </div>
        </div>

        <p v-if="error" class="alert alert-danger mt-5">{{ error }}</p>
        <div class="pricing-tabs">
          <button :class="{ active: pricingTab === 'daily' }" @click="pricingTab = 'daily'">日套餐</button>
          <button :class="{ active: pricingTab === 'weekly' }" @click="pricingTab = 'weekly'">周套餐</button>
          <button :class="{ active: pricingTab === 'public' }" @click="pricingTab = 'public'">活动套餐</button>
          <button :class="{ active: pricingTab === 'free' }" @click="pricingTab = 'free'">免费套餐</button>
          <button :class="{ active: pricingTab === 'lottery' }" @click="pricingTab = 'lottery'">抽奖套餐</button>
        </div>
        <div class="subscription-grid">
          <article
            v-for="(plan, index) in visiblePricingPlans"
            :key="plan.ID"
            class="subscription-card"
            :class="{ featured: index === 1, soldout: planSoldOut(plan) }"
          >
            <div class="plan-ribbon">{{ plan.BadgeText || planBadge(index) }}</div>
            <h2>{{ plan.Name }}</h2>
            <p>{{ plan.Description || planSubtitle(index) }}</p>
            <div class="subscription-price">
              <strong>{{ isLotteryPlan(plan) ? '抽奖' : (isFreePlan(plan) ? '免费' : `￥${priceRmb(plan)}`) }}</strong>
              <span>/{{ planPeriod(plan) }}</span>
            </div>
            <div class="subscription-facts">
              <div><span class="fact-icon">▣</span><span>{{ quotaPeriodLabel(plan) }}：${{ plan.QuotaPeriod === 'public' ? totalUsd(plan) : periodUsd(plan) }}</span></div>
              <div v-if="plan.QuotaPeriod === 'public' && !isLotteryPlan(plan)"><span class="fact-icon">□</span><span>公共渠道剩余：${{ publicRemainingUsd(plan) }}</span></div>
              <div v-else><span class="fact-icon">□</span><span>套餐时长：{{ plan.DurationDays }} 天</span></div>
              <div><span class="fact-icon">↗</span><span>总额度：约${{ totalUsd(plan) }}</span></div>
            </div>
            <button class="subscription-action" :disabled="planSoldOut(plan) || (isLotteryPlan(plan) && !plan.LotteryURL)" @click="openPlanAction(plan)">
              {{ isLotteryPlan(plan) ? '参与抽奖' : (planSoldOut(plan) ? '售罄' : (isFreePlan(plan) ? '免费领取' : (auth.loggedIn ? '立即续费' : '立即订阅'))) }}
            </button>
            <small>安全支付 · 透明价格</small>
          </article>
        </div>
      </section>
    </main>

    <main v-else-if="isUsageRecordsPage" class="console-page">
      <section class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
        <div class="console-title">
          <div>
            <p class="section-kicker">Console</p>
            <h1>{{ auth.loggedIn ? '使用记录' : '登录后查看使用记录' }}</h1>
            <p>查看 API 调用日志、Token、费用和响应耗时。</p>
          </div>
          <div v-if="auth.loggedIn" class="console-title-actions">
            <button class="ghost-button" type="button" @click="navigate('/console')">返回控制台</button>
            <div class="user-chip">{{ auth.user?.email || auth.user?.username }}</div>
          </div>
        </div>

        <div v-if="!auth.loggedIn" class="panel-surface grid gap-4 p-5 sm:grid-cols-[1fr_auto] sm:items-center">
          <div>
            <h2 class="text-xl font-black">需要先登录</h2>
            <p class="mt-2 text-sm leading-6 text-muted">登录后可查看当前账号的 API 调用使用记录。</p>
          </div>
          <div class="flex gap-3">
            <button class="ghost-button" @click="openAuth('login')">登录</button>
            <button v-if="publicSettings.allow_registration" class="primary-button" @click="openAuth('register')">注册</button>
          </div>
        </div>
      </section>
      <UsageRecords v-if="auth.loggedIn" :key="currentPath" @navigate="navigate" />
    </main>

    <main v-else-if="isAdminPage" class="console-page">
      <section class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
        <div class="console-title">
          <div>
            <p class="section-kicker">Admin</p>
            <h1>{{ auth.loggedIn ? '管理后台' : '登录后进入管理后台' }}</h1>
            <p>管理用户、套餐、模型价格、渠道和系统配置。</p>
          </div>
          <div v-if="auth.loggedIn" class="user-chip">{{ auth.user?.email || auth.user?.username }}</div>
        </div>

        <div v-if="!auth.loggedIn" class="panel-surface grid gap-4 p-5 sm:grid-cols-[1fr_auto] sm:items-center">
          <div>
            <h2 class="text-xl font-black">需要先登录</h2>
            <p class="mt-2 text-sm leading-6 text-muted">管理员登录后可以进入独立的管理后台。</p>
          </div>
          <div class="flex gap-3">
            <button class="ghost-button" @click="openAuth('login')">登录</button>
          </div>
        </div>

        <div v-else-if="!auth.isAdmin" class="panel-surface grid gap-4 p-5 sm:grid-cols-[1fr_auto] sm:items-center">
          <div>
            <h2 class="text-xl font-black">无管理权限</h2>
            <p class="mt-2 text-sm leading-6 text-muted">当前账号可以使用用户控制台，但不能访问管理后台。</p>
          </div>
          <button class="primary-button" @click="navigate('/console')">返回控制台</button>
        </div>
      </section>
      <AdminPanel v-if="auth.loggedIn && auth.isAdmin" :key="currentPath" />
    </main>

    <main v-else class="console-page">
      <section class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
        <div class="console-title">
          <div>
            <p class="section-kicker">Console</p>
            <h1>{{ auth.loggedIn ? '用户控制台' : '登录后进入控制台' }}</h1>
            <p>负责下单、API Key 管理和使用记录。</p>
          </div>
          <div v-if="auth.loggedIn" class="user-chip">{{ auth.user?.email || auth.user?.username }}</div>
        </div>

        <div v-if="!auth.loggedIn" class="panel-surface grid gap-4 p-5 sm:grid-cols-[1fr_auto] sm:items-center">
          <div>
            <h2 class="text-xl font-black">需要先登录</h2>
            <p class="mt-2 text-sm leading-6 text-muted">登录后可以创建订单、管理 API Key 和查看使用记录。</p>
          </div>
          <div class="flex gap-3">
            <button class="ghost-button" @click="openAuth('login')">登录</button>
            <button v-if="publicSettings.allow_registration" class="primary-button" @click="openAuth('register')">注册</button>
          </div>
        </div>
      </section>
      <Dashboard v-if="auth.loggedIn" :key="currentPath" :plans="plans" :api-endpoints="publicSettings.api_endpoints" @navigate="navigate" />
    </main>

    <footer class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-4 py-8 text-sm text-muted sm:px-6">
      <span>{{ publicSettings.site_title || '星空AI' }}</span>
      <span>联系邮箱：{{ publicSettings.contact_email || 'support@example.com' }}</span>
    </footer>

    <Transition name="account-menu">
      <div v-if="mockAPIOnlineEnabled" class="api-online-widget" :class="{ collapsed: apiOnlineWidgetCollapsed }">
        <button type="button" class="api-online-toggle" @click="toggleAPIOnlineWidget">
          <span class="api-online-dot" :class="apiOnlineIndicatorClass()"></span>
          <strong v-if="!apiOnlineWidgetCollapsed">在线 API 人数</strong>
          <strong v-else>API</strong>
          <span class="api-online-chevron">{{ apiOnlineWidgetCollapsed ? '＋' : '－' }}</span>
        </button>
        <div v-if="!apiOnlineWidgetCollapsed" class="api-online-body">
          <div class="api-online-summary">
            <span class="api-online-badge" :class="`phase-${apiOnlineIndicatorClass()}`">{{ apiOnlineStatusLabel() }}</span>
            <strong>{{ apiOnlineDisplayCount() }}</strong>
          </div>
        </div>
      </div>
    </Transition>

    <AuthModal v-model:open="authOpen" v-model:mode="authMode" :allow-registration="publicSettings.allow_registration" />

    <Transition name="modal-fade">
      <div v-if="orderModal.open" class="modal-backdrop" @click.self="closeOrderModal">
        <form class="modal-card order-modal-card" @submit.prevent="orderModal.order && orderModal.paymentMethod === 'manual' ? submitManualPayment() : (orderModal.order && orderModal.paymentMethod === 'online' ? confirmOnlinePayment() : submitPlanOrder())">
          <div class="modal-head order-modal-head">
            <div>
              <p class="section-kicker">Order</p>
              <h2>{{ orderModal.order ? '完成支付' : '确认购买套餐' }}</h2>
            </div>
            <button type="button" class="icon-button" :disabled="orderModal.loading" @click="closeOrderModal">×</button>
          </div>
          <div class="modal-body order-modal-body">
            <section v-if="orderModal.plan" class="order-summary-card">
              <div>
                <strong>{{ orderModal.plan.Name }}</strong>
                <span>{{ quotaPeriodLabel(orderModal.plan) }}</span>
              </div>
              <dl>
                <div><dt>价格</dt><dd>{{ isFreePlan(orderModal.plan) ? '免费' : `￥${priceRmb(orderModal.plan)}` }}</dd></div>
                <div><dt>额度</dt><dd>${{ orderModal.plan.QuotaPeriod === 'public' ? totalUsd(orderModal.plan) : periodUsd(orderModal.plan) }}</dd></div>
                <div v-if="orderModal.plan.QuotaPeriod !== 'public'"><dt>有效期</dt><dd>{{ orderModal.plan.DurationDays }} 天</dd></div>
              </dl>
            </section>

            <section v-if="!orderModal.order && !isFreePlan(orderModal.plan)" class="payment-method-field order-section-card">
              <strong>选择支付方式</strong>
              <div class="payment-method-options" role="radiogroup" aria-label="选择支付方式">
                <button v-if="onlinePaymentEnabled" type="button" class="payment-method-option" :class="{ active: orderModal.paymentMethod === 'online' }" role="radio" :aria-checked="orderModal.paymentMethod === 'online'" @click="orderModal.paymentMethod = 'online'">
                  <span>在线支付</span>
                  <small>新窗口打开易支付页面，支付后回到这里确认。</small>
                </button>
                <button v-if="manualPaymentEnabled" type="button" class="payment-method-option" :class="{ active: orderModal.paymentMethod === 'manual' }" role="radio" :aria-checked="orderModal.paymentMethod === 'manual'" @click="orderModal.paymentMethod = 'manual'">
                  <span>人工支付</span>
                  <small>扫码转账并填写账号或备注，提交后等待管理员审核。</small>
                </button>
              </div>
            </section>

            <section v-if="orderModal.order && orderModal.paymentMethod === 'online'" class="order-section-card online-payment-panel">
              <div class="online-payment-status">
                <span class="online-payment-dot"></span>
                <div>
                  <strong>等待支付结果</strong>
                  <p>支付页已在新窗口打开。完成支付后点击下方按钮查询结果。</p>
                </div>
              </div>
              <div class="online-payment-actions">
                <button type="button" class="ghost-button" :disabled="orderModal.loading" @click="openOnlinePaymentWindow">重新打开支付页</button>
                <button class="primary-button" :disabled="orderModal.loading">{{ orderModal.loading ? '查询中...' : '我已完成支付' }}</button>
              </div>
            </section>

            <section v-if="orderModal.order && orderModal.paymentMethod === 'manual'" class="order-section-card manual-payment-panel">
              <strong>人工支付订单 #{{ orderModal.order.ID }}</strong>
              <div v-if="orderModal.manualQRCode" class="manual-payment-qr">
                <img :src="orderModal.manualQRCode" alt="人工支付付款二维码" />
              </div>
              <div v-else class="manual-payment-empty">管理员尚未配置人工支付二维码，请联系站点支持。</div>
              <label class="password-field">
                <span>当前账号或转账留言</span>
                <textarea v-model="orderModal.manualNote" rows="3" placeholder="请填写当前账号邮箱、转账备注或其他便于核对的信息"></textarea>
              </label>
            </section>

            <p v-if="orderModal.error" class="modal-inline-error">{{ orderModal.error }}</p>
          </div>
          <div class="modal-actions order-modal-actions">
            <button type="button" class="ghost-button" :disabled="orderModal.loading" @click="closeOrderModal">{{ orderModal.order && orderModal.paymentMethod === 'online' ? '稍后处理' : '取消' }}</button>
            <button v-if="!orderModal.order || orderModal.paymentMethod === 'manual'" class="primary-button" :disabled="orderModal.loading || (!isFreePlan(orderModal.plan) && !orderModal.paymentMethod) || (orderModal.order && orderModal.paymentMethod === 'manual' && !orderModal.manualQRCode)">
              {{ orderModal.loading ? '提交中...' : (orderModal.order && orderModal.paymentMethod === 'manual' ? '提交审核' : (isFreePlan(orderModal.plan) ? '免费领取' : '确认下单')) }}
            </button>
          </div>
        </form>
      </div>
    </Transition>

    <Transition name="modal-fade">
      <div v-if="passwordModalOpen" class="modal-backdrop password-backdrop" @click.self="closePasswordModal">
        <form class="password-card" @submit.prevent="submitPassword">
          <button type="button" class="password-close" @click="closePasswordModal">×</button>
          <h2>修改密码</h2>
          <label class="password-field">
            <span>旧密码</span>
            <input v-model="passwordForm.oldPassword" type="password" required autocomplete="current-password" />
          </label>
          <label class="password-field">
            <span>新密码</span>
            <input v-model="passwordForm.newPassword" type="password" required autocomplete="new-password" />
          </label>
          <label class="password-field">
            <span>确认新密码</span>
            <input v-model="passwordForm.confirmPassword" type="password" required autocomplete="new-password" />
          </label>
          <p v-if="passwordError" class="password-message error">{{ passwordError }}</p>
          <p v-else-if="passwordNotice" class="password-message success">{{ passwordNotice }}</p>
          <p v-else class="password-hint">长度需超过 6 位</p>
          <div class="password-actions">
            <button type="button" class="password-cancel" @click="closePasswordModal">取消</button>
            <button class="password-submit" :disabled="passwordSaving">{{ passwordSaving ? '修改中' : '确认修改' }}</button>
          </div>
        </form>
      </div>
    </Transition>
  </div>
</template>
