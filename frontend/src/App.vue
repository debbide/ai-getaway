<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from './api/client'
import { useAuthStore } from './stores/auth'
import AuthModal from './components/AuthModal.vue'
import Dashboard from './components/Dashboard.vue'
import AdminPanel from './components/AdminPanel.vue'

const defaultNavigation = [
  { label: '首页', path: '/' },
  { label: '教程 ↗', path: '#tutorial', external: true },
  { label: '定价', path: '/plans' },
  { label: '模型', path: '/models' },
  { label: '常见问题', path: '/faq' },
  { label: '更多中转⌄', path: '#', children: [{ label: 'Claude Code 中转', path: '/claude' }] }
]

const defaultSettings = {
  site_title: 'CodexZH',
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

const isConsolePage = computed(() => currentPath.value === '/console')
const isPlansPage = computed(() => currentPath.value === '/plans')
const consoleTitle = computed(() => (auth.isAdmin ? '管理后台' : '用户控制台'))
const navItems = computed(() => parseNavigation(publicSettings.value.navigation_items))
const activeThemeLabel = computed(() => ({ light: '浅色', dark: '深色', system: '系统' })[themeMode.value] || '深色')

onMounted(async () => {
  window.addEventListener('popstate', syncPath)
  window.matchMedia?.('(prefers-color-scheme: dark)').addEventListener?.('change', applyTheme)
  applyTheme()
  await auth.loadMe()
  await loadPublicSettings()
  await loadPlans()
})

onBeforeUnmount(() => {
  window.removeEventListener('popstate', syncPath)
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

function applyTheme() {
  const systemDark = window.matchMedia?.('(prefers-color-scheme: dark)').matches
  const resolved = themeMode.value === 'system' ? (systemDark ? 'dark' : 'light') : themeMode.value
  document.documentElement.dataset.theme = resolved
}

function priceRmb(plan) {
  return ((plan.PriceCents || 0) / 100).toFixed((plan.PriceCents || 0) % 100 === 0 ? 0 : 1)
}

function weeklyUsd(plan) {
  return ((plan.SettlementUSDCents || 0) / 100).toFixed((plan.SettlementUSDCents || 0) % 100 === 0 ? 0 : 2)
}

function totalUsd(plan) {
  const weeks = Math.max(1, Math.round((plan.DurationDays || 30) / 7))
  return (((plan.SettlementUSDCents || 0) / 100) * weeks).toFixed(0)
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
          <span class="brand-mark">CZ</span>
          <strong>{{ publicSettings.site_title || 'CodexZH' }}</strong>
        </button>

        <nav class="hidden items-center gap-10 text-sm font-bold md:flex">
          <div v-for="item in navItems" :key="item.label" class="nav-menu-item">
            <button class="nav-link" @click="navigate(item.path)">{{ item.label }}</button>
            <div v-if="item.children?.length" class="nav-submenu">
              <button v-for="child in item.children" :key="child.label" @click="navigate(child.path)">
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
          <button class="console-link" @click="enterConsole">控制台</button>
          <button class="score-badge" @click="auth.loggedIn ? navigate('/console') : openAuth('login')">63</button>
        </div>
      </div>
    </header>

    <main v-if="!isConsolePage && !isPlansPage">
      <section class="home-hero">
        <div class="home-hero-inner mx-auto max-w-7xl px-4 sm:px-6">
          <div class="hero-badge">✣ 为中国开发者量身打造</div>
          <h1>
            <span>{{ publicSettings.site_title || 'CodexZH' }}</span>
            AI驱动的编程助手
          </h1>
          <p>
            {{ publicSettings.site_title || 'CodexZH' }} 是专为中国开发者设计的智能编程助手，通过先进的AI技术提供代码生成、调试优化和实时协作功能，让编程更高效、更智能。
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
              <div><span class="fact-icon">▣</span><span>周限额度：${{ weeklyUsd(plan) }}</span></div>
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

    <main v-else class="console-page">
      <section class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
        <button class="ghost-button mb-6" @click="navigate('/')">返回首页</button>
        <div class="console-title">
          <div>
            <p class="section-kicker">Console</p>
            <h1>{{ auth.loggedIn ? consoleTitle : '登录后进入控制台' }}</h1>
            <p>控制台是独立工作区，负责下单、Key 管理和后台审核。</p>
          </div>
          <div v-if="auth.loggedIn" class="user-chip">{{ auth.user?.email || auth.user?.username }}</div>
        </div>

        <div v-if="!auth.loggedIn" class="panel-surface grid gap-4 p-5 sm:grid-cols-[1fr_auto] sm:items-center">
          <div>
            <h2 class="text-xl font-black">需要先登录</h2>
            <p class="mt-2 text-sm leading-6 text-muted">登录后可以创建订单、管理 API Key；管理员账号会显示审核后台。</p>
          </div>
          <div class="flex gap-3">
            <button class="ghost-button" @click="openAuth('login')">登录</button>
            <button class="primary-button" @click="openAuth('register')">注册</button>
          </div>
        </div>
      </section>
      <Dashboard v-if="auth.loggedIn && !auth.isAdmin" :plans="plans" />
      <AdminPanel v-if="auth.loggedIn && auth.isAdmin" />
    </main>

    <footer class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-4 py-8 text-sm text-muted sm:px-6">
      <span>{{ publicSettings.site_title || 'CodexZH' }}</span>
      <span>联系邮箱：support@example.com</span>
    </footer>

    <AuthModal v-model:open="authOpen" v-model:mode="authMode" />
  </div>
</template>
