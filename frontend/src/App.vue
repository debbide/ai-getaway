<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from './api/client'
import { useAuthStore } from './stores/auth'
import AuthModal from './components/AuthModal.vue'
import Dashboard from './components/Dashboard.vue'
import AdminPanel from './components/AdminPanel.vue'

const auth = useAuthStore()
const plans = ref([])
const authOpen = ref(false)
const authMode = ref('login')
const error = ref('')
const currentPath = ref(window.location.pathname)
const publicSettings = ref({ site_title: 'AI Gateway', tutorial_video_url: '' })

const isConsolePage = computed(() => currentPath.value === '/console')
const visiblePlans = computed(() => plans.value)
const consoleTitle = computed(() => (auth.isAdmin ? '管理后台' : '用户控制台'))

const featureCards = [
  { title: '兼容 OpenAI API', desc: '保留熟悉的调用方式，将用户请求转发到管理员配置的上游账号。' },
  { title: '独立租户通道', desc: '用户只使用自己的平台 Key，管理员在后台完成账号和额度配置。' },
  { title: '订单审核流程', desc: '套餐购买、人工审核、上游绑定和 API Key 创建拆成清晰步骤。' }
]

onMounted(async () => {
  window.addEventListener('popstate', syncPath)
  await auth.loadMe()
  await loadPublicSettings()
  await loadPlans()
})

onBeforeUnmount(() => {
  window.removeEventListener('popstate', syncPath)
})

async function loadPlans() {
  try {
    const res = await api.get('/plans')
    plans.value = res.data
  } catch (err) {
    error.value = err.message
  }
}

async function loadPublicSettings() {
  try {
    const res = await api.get('/settings/public')
    publicSettings.value = res.data
    if (res.data.site_title) document.title = res.data.site_title
  } catch {
    publicSettings.value = { site_title: 'AI Gateway', tutorial_video_url: '' }
  }
}

function syncPath() {
  currentPath.value = window.location.pathname
}

function navigate(path) {
  if (window.location.pathname !== path) {
    window.history.pushState({}, '', path)
    syncPath()
  }
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function navigateSection(id) {
  const scrollToSection = () => {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' })
  }

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

function price(plan) {
  return `¥${(plan.PriceCents / 100).toFixed(0)}`
}

function usd(plan) {
  return `$${((plan.SettlementUSDCents || 0) / 100).toFixed(2)}`
}
</script>

<template>
  <div class="min-h-screen bg-canvas text-forest">
    <header class="sticky top-0 z-30 border-b border-line/80 bg-canvas/95 backdrop-blur-xl">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-4 py-4 sm:px-6">
        <button class="focus-ring flex items-center gap-3 rounded px-1 py-1 text-left" @click="navigate('/')">
          <span class="grid h-10 w-10 place-items-center rounded bg-brand text-sm font-black text-white shadow-soft">
            AI
          </span>
          <span>
            <span class="block text-base font-bold leading-5">{{ publicSettings.site_title || 'AI Gateway' }}</span>
            <span class="block text-xs text-muted">OpenAI API 中转平台</span>
          </span>
        </button>

        <nav class="hidden items-center gap-7 text-sm font-medium text-muted md:flex">
          <button class="focus-ring rounded px-1 py-1 hover:text-forest" @click="navigate('/')">首页</button>
          <button class="focus-ring rounded px-1 py-1 hover:text-forest" @click="navigateSection('tutorial')">视频教程</button>
          <button class="focus-ring rounded px-1 py-1 hover:text-forest" @click="navigateSection('pricing')">套餐</button>
          <button class="focus-ring rounded px-1 py-1 hover:text-forest" @click="enterConsole">控制台</button>
        </nav>

        <div class="flex items-center gap-2">
          <button
            v-if="!auth.loggedIn"
            class="focus-ring hidden rounded border border-line bg-white px-4 py-2 text-sm font-semibold text-forest shadow-sm sm:inline-flex"
            @click="openAuth('login')"
          >
            登录
          </button>
          <button
            v-if="!auth.loggedIn"
            class="focus-ring rounded bg-accent px-4 py-2 text-sm font-bold text-white shadow-soft"
            @click="openAuth('register')"
          >
            注册
          </button>
          <template v-else>
            <button
              class="focus-ring rounded bg-brand px-4 py-2 text-sm font-bold text-white shadow-soft"
              @click="navigate('/console')"
            >
              控制台
            </button>
            <button class="focus-ring rounded border border-line bg-white px-4 py-2 text-sm font-semibold" @click="auth.logout">
              退出
            </button>
          </template>
        </div>
      </div>
    </header>

    <main v-if="!isConsolePage">
      <section class="relative overflow-hidden border-b border-line">
        <div class="mx-auto grid max-w-6xl gap-12 px-4 py-16 sm:px-6 lg:grid-cols-[1fr_0.92fr] lg:items-center lg:py-24">
          <div>
            <p class="mb-5 inline-flex rounded-full border border-brand/25 bg-brand/10 px-4 py-2 text-sm font-semibold text-brand">
              多账号路由 / 订阅审核 / API Key 托管
            </p>
            <h1 class="max-w-3xl text-5xl font-black leading-[1.06] tracking-normal text-forest sm:text-6xl">
              {{ publicSettings.site_title || 'AI Gateway' }}
            </h1>
            <p class="mt-6 max-w-2xl text-lg leading-8 text-muted">
              首页只承担介绍、教程和套餐展示，真实的下单、Key 管理与管理员审核统一进入独立控制台页面完成。
            </p>
            <div class="mt-8 flex flex-wrap gap-3">
              <button class="focus-ring rounded bg-accent px-6 py-3 font-bold text-white shadow-soft" @click="afterPrimaryAction">
                开始使用
              </button>
              <a class="focus-ring rounded border border-line bg-white px-6 py-3 font-bold text-forest shadow-sm" href="#tutorial">
                查看教程
              </a>
            </div>
          </div>

          <div class="relative">
            <div class="absolute -right-6 -top-6 h-28 w-28 rounded-full bg-accent/16 blur-2xl"></div>
            <div class="absolute -bottom-8 -left-8 h-32 w-32 rounded-full bg-brand/16 blur-2xl"></div>
            <div class="relative overflow-hidden rounded-lg border border-line bg-white shadow-panel">
              <div class="flex items-center gap-2 border-b border-line bg-[#f7faf5] px-4 py-3">
                <span class="h-3 w-3 rounded-full bg-[#ff6b4a]"></span>
                <span class="h-3 w-3 rounded-full bg-[#f4c542]"></span>
                <span class="h-3 w-3 rounded-full bg-[#2fbf71]"></span>
                <span class="ml-3 text-xs font-semibold text-muted">gateway.example.com</span>
              </div>
              <div class="grid gap-4 p-5">
                <div class="rounded bg-forest p-5 text-white">
                  <div class="mb-4 flex items-center justify-between text-xs text-white/60">
                    <span>Request Pipeline</span>
                    <span>Live</span>
                  </div>
                  <div class="space-y-3 text-sm">
                    <div class="rounded border border-white/10 bg-white/[0.08] p-3">Client API Key</div>
                    <div class="rounded border border-white/10 bg-white/[0.08] p-3">Gateway Auth + Rate Limit</div>
                    <div class="rounded border border-white/10 bg-white/[0.08] p-3">Bound Upstream Account</div>
                  </div>
                </div>
                <div class="grid grid-cols-3 gap-3">
                  <div class="rounded border border-line bg-mint p-4">
                    <div class="text-2xl font-black text-brand">1:1</div>
                    <div class="text-xs font-semibold text-muted">账号绑定</div>
                  </div>
                  <div class="rounded border border-line bg-mint p-4">
                    <div class="text-2xl font-black text-accent">SSE</div>
                    <div class="text-xs font-semibold text-muted">流式响应</div>
                  </div>
                  <div class="rounded border border-line bg-mint p-4">
                    <div class="text-2xl font-black text-forest">API</div>
                    <div class="text-xs font-semibold text-muted">兼容调用</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="mx-auto max-w-6xl px-4 py-14 sm:px-6">
        <div class="grid gap-4 md:grid-cols-3">
          <article v-for="card in featureCards" :key="card.title" class="rounded border border-line bg-white p-6 shadow-sm">
            <h2 class="text-lg font-black">{{ card.title }}</h2>
            <p class="mt-3 text-sm leading-6 text-muted">{{ card.desc }}</p>
          </article>
        </div>
      </section>

      <section id="tutorial" class="border-y border-line bg-mint">
        <div class="mx-auto grid max-w-6xl gap-8 px-4 py-14 sm:px-6 lg:grid-cols-[0.85fr_1.15fr] lg:items-center">
          <div>
            <p class="text-sm font-bold uppercase tracking-[0.2em] text-brand">Tutorial</p>
            <h2 class="mt-3 text-3xl font-black text-forest sm:text-4xl">视频教程</h2>
            <p class="mt-4 leading-7 text-muted">
              参考首页保留独立教程区，用户先了解接入流程，再进入控制台完成下单、审核和 Key 创建。
            </p>
          </div>
          <div class="overflow-hidden rounded-lg border border-line bg-forest shadow-panel">
            <iframe
              v-if="publicSettings.tutorial_video_url"
              class="aspect-video w-full"
              :src="publicSettings.tutorial_video_url"
              title="视频教程"
              allowfullscreen
            ></iframe>
            <div v-else class="aspect-video p-5">
              <div class="grid h-full place-items-center rounded border border-white/10 bg-white/[0.08]">
                <div class="grid h-16 w-16 place-items-center rounded-full bg-accent text-2xl font-black text-white shadow-soft">
                  ▶
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section id="pricing" class="mx-auto max-w-6xl px-4 py-14 sm:px-6">
        <div class="mb-7 flex flex-wrap items-end justify-between gap-4">
          <div>
            <p class="text-sm font-bold uppercase tracking-[0.2em] text-brand">Pricing</p>
            <h2 class="mt-3 text-3xl font-black text-forest">套餐</h2>
            <p class="mt-2 text-sm text-muted">首页只展示套餐，购买动作进入控制台完成。</p>
          </div>
          <p v-if="error" class="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>
        </div>

        <div class="grid gap-4 md:grid-cols-3">
          <article v-for="plan in visiblePlans" :key="plan.ID" class="rounded border border-line bg-white p-6 shadow-sm">
            <div class="flex items-start justify-between gap-3">
              <div>
                <h3 class="text-xl font-black">{{ plan.Name }}</h3>
                <p class="mt-2 text-sm leading-6 text-muted">{{ plan.Description }}</p>
              </div>
              <div class="text-right">
                <div class="text-2xl font-black text-brand">{{ price(plan) }}</div>
                <div class="text-xs font-semibold text-muted">{{ plan.DurationDays }} 天</div>
              </div>
            </div>
            <div class="mt-5 rounded bg-mint px-3 py-2 text-sm font-bold text-forest">
              到账 {{ usd(plan) }} / 日 {{ (plan.DailyQuotaTokens || 0).toLocaleString() }} / 周 {{ (plan.WeeklyQuotaTokens || 0).toLocaleString() }}
            </div>
          </article>
        </div>

        <div class="mt-8 rounded-lg border border-line bg-forest px-6 py-5 text-white shadow-panel">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <div>
              <h3 class="text-xl font-black">准备开始操作？</h3>
              <p class="mt-1 text-sm text-white/70">下单、API Key 和管理员审核都在独立控制台中处理。</p>
            </div>
            <button class="focus-ring rounded bg-accent px-5 py-3 font-bold text-white" @click="afterPrimaryAction">进入控制台</button>
          </div>
        </div>
      </section>
    </main>

    <main v-else class="border-b border-line bg-[#f7faf5]">
      <section class="mx-auto max-w-6xl px-4 py-10 sm:px-6">
        <button class="focus-ring mb-6 rounded border border-line bg-white px-4 py-2 text-sm font-bold text-muted" @click="navigate('/')">
          返回首页
        </button>
        <div class="mb-8 flex flex-wrap items-start justify-between gap-4">
          <div>
            <p class="text-sm font-bold uppercase tracking-[0.2em] text-brand">Console</p>
            <h1 class="mt-2 text-3xl font-black text-forest">{{ auth.loggedIn ? consoleTitle : '登录后进入控制台' }}</h1>
            <p class="mt-2 text-sm text-muted">这里是独立页面，不再把控制台功能塞在首页底部。</p>
          </div>
          <div v-if="auth.loggedIn" class="rounded border border-line bg-white px-4 py-3 text-sm font-bold text-forest shadow-sm">
            {{ auth.user?.email || auth.user?.username }}
          </div>
        </div>

        <div v-if="!auth.loggedIn" class="grid gap-4 rounded border border-line bg-white p-5 shadow-panel sm:grid-cols-[1fr_auto] sm:items-center">
          <div>
            <h2 class="text-xl font-black">需要先登录</h2>
            <p class="mt-2 text-sm leading-6 text-muted">登录后可以创建订单、管理 API Key；管理员账号会显示审核后台。</p>
          </div>
          <div class="flex gap-3">
            <button class="focus-ring rounded border border-line bg-white px-4 py-2 font-bold" @click="openAuth('login')">登录</button>
            <button class="focus-ring rounded bg-accent px-4 py-2 font-bold text-white" @click="openAuth('register')">注册</button>
          </div>
        </div>
      </section>
      <Dashboard v-if="auth.loggedIn && !auth.isAdmin" :plans="plans" />
      <AdminPanel v-if="auth.loggedIn && auth.isAdmin" />
    </main>

    <footer class="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-4 py-8 text-sm text-muted sm:px-6">
      <span>{{ publicSettings.site_title || 'AI Gateway' }}</span>
      <span>联系方式：support@example.com</span>
    </footer>

    <AuthModal v-model:open="authOpen" v-model:mode="authMode" />
  </div>
</template>
