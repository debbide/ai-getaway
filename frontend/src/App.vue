<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { api } from './api/client'
import { useAuthStore } from './stores/auth'
import AuthModal from './components/AuthModal.vue'
import Dashboard from './components/Dashboard.vue'
import AdminPanel from './components/AdminPanel.vue'
import UsageRecords from './components/UsageRecords.vue'
import DocsPage from './components/DocsPage.vue'

const defaultNavigation = [
  { label: '首页', path: '/' },
  { label: '教程 ↗', path: '/docs' },
  { label: '定价', path: '/plans' },
  { label: '模型', path: '/models' },
  { label: '常见问题', path: '/faq' }
]

const defaultSettings = {
  site_title: '星空AI',
  api_endpoints: JSON.stringify([{ label: '默认', description: '主线路', url: 'https://ai.itzkb.cn' }]),
  tutorial_video_url: '',
  navigation_items: JSON.stringify(defaultNavigation),
  pricing_title: '简单透明的定价',
  pricing_subtitle: '保质保量无降智不掺假',
  pricing_notice: '本站仅支持 GPT 模型使用，具体型号请查看 /models 页面；如需使用 Claude 模型，请前往顶部菜单更多中转 → Claude Code 中转'
}

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
const passwordForm = reactive({ oldPassword: '', newPassword: '', confirmPassword: '' })

const isConsolePage = computed(() => currentPath.value === '/console')
const isAdminPage = computed(() => currentPath.value === '/admin')
const isUsageRecordsPage = computed(() => currentPath.value === '/usage-records')
const isPlansPage = computed(() => currentPath.value === '/plans')
const isDocsPage = computed(() => currentPath.value === '/docs' || currentPath.value.startsWith('/docs/'))
const navItems = computed(() => parseNavigation(publicSettings.value.navigation_items))
const activeThemeLabel = computed(() => ({ light: '浅色', dark: '深色', system: '系统' })[themeMode.value] || '深色')
const accountEmail = computed(() => auth.user?.email || '')
const accountName = computed(() => auth.user?.username || accountEmail.value.split('@')[0] || '用户')
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
})

onBeforeUnmount(() => {
  window.removeEventListener('popstate', syncPath)
  window.removeEventListener('app-data-updated', refreshAppData)
  window.removeEventListener('auth-expired', handleAuthExpired)
  window.matchMedia?.('(prefers-color-scheme: dark)').removeEventListener?.('change', applyTheme)
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
  openAuth('register')
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
  return ((plan.PriceCents || 0) / 100).toFixed((plan.PriceCents || 0) % 100 === 0 ? 0 : 1)
}

function periodUsd(plan) {
  return ((plan.SettlementUSDCents || 0) / 100).toFixed((plan.SettlementUSDCents || 0) % 100 === 0 ? 0 : 2)
}

function quotaPeriodLabel(plan) {
  return plan.QuotaPeriod === 'daily' ? '日限额度' : '周限额度'
}

function totalUsd(plan) {
  const units = plan.QuotaPeriod === 'daily' ? (plan.DurationDays || 1) : Math.max(1, Math.round((plan.DurationDays || 30) / 7))
  return (((plan.SettlementUSDCents || 0) / 100) * units).toFixed(0)
}

function planPeriod(plan) {
  if ((plan.DurationDays || 0) <= 1) return '天'
  if ((plan.DurationDays || 0) >= 28) return '月'
  return `${plan.DurationDays} 天`
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

    <DocsPage v-if="isDocsPage" />

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
            <button class="hero-secondary" @click="navigateSection('tutorial')">▷ 观看演示</button>
          </div>
        </div>
      </section>

      <section id="tutorial" class="tutorial-section">
        <div class="mx-auto grid max-w-7xl gap-8 px-4 py-14 sm:px-6 lg:grid-cols-[0.8fr_1.2fr] lg:items-center">
          <div>
            <p class="section-kicker">Tutorial</p>
            <h2 class="mt-3 text-3xl font-black text-ink sm:text-4xl">视频教程</h2>
            <p class="mt-4 leading-7 text-muted">
              先了解接入流程，再进入定价页选择方案。登录控制台后下单，等待管理员审核并开通上游通道。
            </p>
          </div>
          <div class="video-shell">
            <iframe
              v-if="publicSettings.tutorial_video_url"
              class="aspect-video w-full"
              :src="publicSettings.tutorial_video_url"
              title="视频教程"
              allowfullscreen
            ></iframe>
            <div v-else class="video-empty">
              <div class="play-core">▶</div>
            </div>
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
        <div class="subscription-grid">
          <article
            v-for="(plan, index) in plans"
            :key="plan.ID"
            class="subscription-card"
            :class="{ featured: index === 1 }"
          >
            <div class="plan-ribbon">{{ plan.BadgeText || planBadge(index) }}</div>
            <h2>{{ plan.Name }}</h2>
            <p>{{ plan.Description || planSubtitle(index) }}</p>
            <div class="subscription-price">
              <strong>￥{{ priceRmb(plan) }}</strong>
              <span>/{{ planPeriod(plan) }}</span>
            </div>
            <div class="subscription-facts">
              <div><span class="fact-icon">▣</span><span>{{ quotaPeriodLabel(plan) }}：${{ periodUsd(plan) }}</span></div>
              <div><span class="fact-icon">□</span><span>套餐时长：{{ plan.DurationDays }} 天</span></div>
              <div><span class="fact-icon">↗</span><span>总额度：约${{ totalUsd(plan) }}</span></div>
            </div>
            <button class="subscription-action" @click="afterPrimaryAction">
              {{ auth.loggedIn ? '立即续费' : '立即订阅' }}
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
          <div v-if="auth.loggedIn" class="user-chip">{{ auth.user?.email || auth.user?.username }}</div>
        </div>

        <div v-if="!auth.loggedIn" class="panel-surface grid gap-4 p-5 sm:grid-cols-[1fr_auto] sm:items-center">
          <div>
            <h2 class="text-xl font-black">需要先登录</h2>
            <p class="mt-2 text-sm leading-6 text-muted">登录后可查看当前账号的 API 调用使用记录。</p>
          </div>
          <div class="flex gap-3">
            <button class="ghost-button" @click="openAuth('login')">登录</button>
            <button class="primary-button" @click="openAuth('register')">注册</button>
          </div>
        </div>
      </section>
      <UsageRecords v-if="auth.loggedIn" @navigate="navigate" />
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
      <AdminPanel v-if="auth.loggedIn && auth.isAdmin" />
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
            <button class="primary-button" @click="openAuth('register')">注册</button>
          </div>
        </div>
      </section>
      <Dashboard v-if="auth.loggedIn" :plans="plans" :api-endpoints="publicSettings.api_endpoints" @navigate="navigate" />
    </main>

    <footer class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-4 py-8 text-sm text-muted sm:px-6">
      <span>{{ publicSettings.site_title || '星空AI' }}</span>
      <span>联系邮箱：support@example.com</span>
    </footer>

    <AuthModal v-model:open="authOpen" v-model:mode="authMode" />

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
