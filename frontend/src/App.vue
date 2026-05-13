<script setup>
import { computed, onMounted, ref } from 'vue'
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

const visiblePlans = computed(() => plans.value)

onMounted(async () => {
  await auth.loadMe()
  await loadPlans()
})

async function loadPlans() {
  try {
    const res = await api.get('/plans')
    plans.value = res.data
  } catch (err) {
    error.value = err.message
  }
}

function openAuth(mode) {
  authMode.value = mode
  authOpen.value = true
}

function price(plan) {
  return `¥${(plan.PriceCents / 100).toFixed(0)}`
}
</script>

<template>
  <div class="min-h-screen bg-ink text-slate-100">
    <header class="sticky top-0 z-30 border-b border-line bg-ink/90 backdrop-blur">
      <div class="mx-auto flex max-w-7xl items-center justify-between px-4 py-4 sm:px-6">
        <div class="flex items-center gap-3">
          <div class="grid h-9 w-9 place-items-center rounded bg-brand text-sm font-black text-ink">AI</div>
          <div>
            <div class="text-base font-semibold">AI Gateway</div>
            <div class="text-xs text-slate-400">OpenAI-compatible account routing</div>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button
            v-if="!auth.loggedIn"
            class="focus-ring rounded border border-line px-4 py-2 text-sm text-slate-200"
            @click="openAuth('login')"
          >
            登录
          </button>
          <button
            v-if="!auth.loggedIn"
            class="focus-ring rounded bg-brand px-4 py-2 text-sm font-semibold text-ink"
            @click="openAuth('register')"
          >
            注册
          </button>
          <button v-else class="focus-ring rounded border border-line px-4 py-2 text-sm" @click="auth.logout">
            退出
          </button>
        </div>
      </div>
    </header>

    <main>
      <section class="border-b border-line">
        <div class="mx-auto grid max-w-7xl gap-8 px-4 py-16 sm:px-6 lg:grid-cols-[1.1fr_0.9fr] lg:py-20">
          <div class="max-w-3xl">
            <p class="mb-4 inline-flex rounded border border-brand/40 px-3 py-1 text-sm text-brand">
              多账号路由 / 多租户隔离 / 订阅制中转
            </p>
            <h1 class="text-4xl font-bold tracking-normal text-white sm:text-6xl">
              面向团队和客户的 AI API 中转系统
            </h1>
            <p class="mt-6 max-w-2xl text-lg leading-8 text-slate-300">
              用户使用平台 API Key 接入，系统自动路由到管理员绑定的上游账号，兼容 OpenAI API、SSE Stream 与实时调用日志。
            </p>
            <div class="mt-8 flex flex-wrap gap-3">
              <button class="focus-ring rounded bg-brand px-5 py-3 font-semibold text-ink" @click="openAuth('register')">
                开始接入
              </button>
              <a class="focus-ring rounded border border-line px-5 py-3 text-slate-200" href="#pricing">查看套餐</a>
            </div>
          </div>
          <div class="grid content-start gap-3">
            <div class="rounded border border-line bg-panel p-5">
              <div class="mb-3 text-sm text-slate-400">请求链路</div>
              <div class="space-y-3 text-sm">
                <div class="rounded bg-ink p-3">Client API Key</div>
                <div class="rounded bg-ink p-3">Gateway Auth + Redis Rate Limit</div>
                <div class="rounded bg-ink p-3">Bound Upstream Account</div>
                <div class="rounded bg-ink p-3">OpenAI-compatible API</div>
              </div>
            </div>
            <div class="grid grid-cols-3 gap-3">
              <div class="rounded border border-line bg-panel p-4">
                <div class="text-2xl font-bold text-brand">1:1</div>
                <div class="text-xs text-slate-400">用户绑定</div>
              </div>
              <div class="rounded border border-line bg-panel p-4">
                <div class="text-2xl font-bold text-accent">SSE</div>
                <div class="text-xs text-slate-400">流式响应</div>
              </div>
              <div class="rounded border border-line bg-panel p-4">
                <div class="text-2xl font-bold text-white">WS</div>
                <div class="text-xs text-slate-400">实时日志</div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="mx-auto max-w-7xl px-4 py-12 sm:px-6">
        <div class="grid gap-4 md:grid-cols-3">
          <div class="rounded border border-line bg-panel p-5">
            <h2 class="text-lg font-semibold">租户隔离</h2>
            <p class="mt-2 text-sm leading-6 text-slate-400">每个用户绑定独立上游账号，平台 API Key 只访问自己的上游资源。</p>
          </div>
          <div class="rounded border border-line bg-panel p-5">
            <h2 class="text-lg font-semibold">限流与缓存</h2>
            <p class="mt-2 text-sm leading-6 text-slate-400">Redis 承担接口限流、会话与账号状态缓存，数据库保留核心记录。</p>
          </div>
          <div class="rounded border border-line bg-panel p-5">
            <h2 class="text-lg font-semibold">管理审核</h2>
            <p class="mt-2 text-sm leading-6 text-slate-400">管理员审核订单后配置渠道和上游 API Key，用户即可创建平台 Key。</p>
          </div>
        </div>
      </section>

      <section id="pricing" class="border-y border-line bg-[#10141b]">
        <div class="mx-auto max-w-7xl px-4 py-12 sm:px-6">
          <div class="mb-6 flex items-end justify-between gap-4">
            <div>
              <h2 class="text-2xl font-bold">套餐</h2>
              <p class="mt-2 text-sm text-slate-400">选择套餐后生成待审核订单。</p>
            </div>
            <p v-if="error" class="text-sm text-red-300">{{ error }}</p>
          </div>
          <div class="grid gap-4 md:grid-cols-3">
            <div v-for="plan in visiblePlans" :key="plan.ID" class="rounded border border-line bg-panel p-5">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <h3 class="text-xl font-semibold">{{ plan.Name }}</h3>
                  <p class="mt-2 text-sm text-slate-400">{{ plan.Description }}</p>
                </div>
                <div class="text-right">
                  <div class="text-2xl font-bold text-brand">{{ price(plan) }}</div>
                  <div class="text-xs text-slate-500">{{ plan.DurationDays }} 天</div>
                </div>
              </div>
              <div class="mt-5 text-sm text-slate-300">{{ plan.QuotaTokens.toLocaleString() }} tokens</div>
            </div>
          </div>
        </div>
      </section>

      <Dashboard v-if="auth.loggedIn && !auth.isAdmin" :plans="plans" />
      <AdminPanel v-if="auth.loggedIn && auth.isAdmin" />
    </main>

    <footer class="mx-auto max-w-7xl px-4 py-8 text-sm text-slate-500 sm:px-6">
      联系方式：support@example.com
    </footer>

    <AuthModal v-model:open="authOpen" v-model:mode="authMode" />
  </div>
</template>
