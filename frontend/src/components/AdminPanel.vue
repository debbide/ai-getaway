<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api/client'

const menu = [
  { key: 'overview', label: '概览' },
  { key: 'plans', label: '套餐管理' },
  { key: 'orders', label: '审核管理' },
  { key: 'users', label: '用户管理' },
  { key: 'settings', label: '系统设置' }
]

const active = ref('overview')
const stats = ref({})
const orders = ref([])
const users = ref([])
const plans = ref([])
const error = ref('')
const notice = ref('')
const editingPlanId = ref(null)
const approve = reactive({ orderId: '', channel: 'openai', baseUrl: 'https://api.openai.com', apiKey: '', adminNote: '' })
const planForm = reactive(emptyPlan())
const settings = reactive({
  site_title: '',
  tutorial_video_url: '',
  smtp_host: '',
  smtp_port: 587,
  smtp_username: '',
  smtp_password: '',
  smtp_from_email: '',
  smtp_from_name: '',
  smtp_use_tls: true,
  epay_pid: '',
  epay_key: '',
  epay_notify_url: '',
  epay_return_url: '',
  epay_submit_url: ''
})

onMounted(loadAll)

function emptyPlan() {
  return {
    name: '',
    code: '',
    plan_type: 'subscription',
    price_cents: 0,
    settlement_usd_cents: 0,
    quota_tokens: 0,
    daily_quota_tokens: 0,
    weekly_quota_tokens: 0,
    duration_days: 30,
    description: '',
    enabled: true
  }
}

async function loadAll() {
  try {
    const [statsRes, ordersRes, usersRes, plansRes, settingsRes] = await Promise.all([
      api.get('/admin/stats'),
      api.get('/admin/orders'),
      api.get('/admin/users'),
      api.get('/admin/plans'),
      api.get('/admin/settings')
    ])
    stats.value = statsRes.data
    orders.value = ordersRes.data
    users.value = usersRes.data
    plans.value = plansRes.data
    Object.assign(settings, settingsRes.data, { smtp_password: '', epay_key: '' })
  } catch (err) {
    error.value = err.message
  }
}

async function savePlan() {
  error.value = ''
  const payload = normalizePlan(planForm)
  try {
    if (editingPlanId.value) {
      await api.put(`/admin/plans/${editingPlanId.value}`, payload)
      notice.value = '套餐已更新'
    } else {
      await api.post('/admin/plans', payload)
      notice.value = '套餐已创建'
    }
    resetPlan()
    await loadAll()
  } catch (err) {
    error.value = err.message
  }
}

function editPlan(plan) {
  editingPlanId.value = plan.ID
  Object.assign(planForm, {
    name: plan.Name,
    code: plan.Code,
    plan_type: plan.PlanType || 'subscription',
    price_cents: plan.PriceCents,
    settlement_usd_cents: plan.SettlementUSDCents,
    quota_tokens: plan.QuotaTokens,
    daily_quota_tokens: plan.DailyQuotaTokens,
    weekly_quota_tokens: plan.WeeklyQuotaTokens,
    duration_days: plan.DurationDays,
    description: plan.Description,
    enabled: plan.Enabled
  })
}

function resetPlan() {
  editingPlanId.value = null
  Object.assign(planForm, emptyPlan())
}

async function deletePlan(id) {
  if (!window.confirm('确认删除这个套餐？')) return
  await api.delete(`/admin/plans/${id}`)
  await loadAll()
}

async function approveOrder() {
  error.value = ''
  try {
    await api.post(`/admin/orders/${approve.orderId}/approve`, {
      channel: approve.channel,
      base_url: approve.baseUrl,
      api_key: approve.apiKey,
      admin_note: approve.adminNote
    })
    Object.assign(approve, { orderId: '', channel: 'openai', baseUrl: 'https://api.openai.com', apiKey: '', adminNote: '' })
    notice.value = '订单已审核通过'
    await loadAll()
  } catch (err) {
    error.value = err.message
  }
}

async function rejectOrder(id) {
  const adminNote = window.prompt('拒绝原因') || ''
  await api.post(`/admin/orders/${id}/reject`, { admin_note: adminNote })
  await loadAll()
}

async function updateUser(user, updates) {
  await api.patch(`/admin/users/${user.ID}`, updates)
  await loadAll()
}

async function deleteUser(id) {
  if (!window.confirm('确认删除这个用户？')) return
  await api.delete(`/admin/users/${id}`)
  await loadAll()
}

async function saveSettings() {
  error.value = ''
  try {
    await api.put('/admin/settings', {
      ...settings,
      smtp_port: Number(settings.smtp_port || 587)
    })
    settings.smtp_password = ''
    settings.epay_key = ''
    notice.value = '系统设置已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message
  }
}

function normalizePlan(plan) {
  return {
    ...plan,
    price_cents: Number(plan.price_cents || 0),
    settlement_usd_cents: Number(plan.settlement_usd_cents || 0),
    quota_tokens: Number(plan.quota_tokens || 0),
    daily_quota_tokens: Number(plan.daily_quota_tokens || 0),
    weekly_quota_tokens: Number(plan.weekly_quota_tokens || 0),
    duration_days: Number(plan.duration_days || 1)
  }
}

function money(cents, currency = '¥') {
  return `${currency}${((cents || 0) / 100).toFixed(2)}`
}
</script>

<template>
  <section class="mx-auto max-w-6xl px-4 pb-12 sm:px-6">
    <div class="grid gap-5 lg:grid-cols-[220px_1fr]">
      <aside class="rounded border border-line bg-white p-3 shadow-sm">
        <button
          v-for="item in menu"
          :key="item.key"
          class="focus-ring mb-1 w-full rounded px-4 py-3 text-left text-sm font-bold"
          :class="active === item.key ? 'bg-brand text-white' : 'text-muted hover:bg-mint hover:text-forest'"
          @click="active = item.key"
        >
          {{ item.label }}
        </button>
      </aside>

      <div class="min-w-0">
        <p v-if="error" class="mb-4 rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ error }}</p>
        <p v-if="notice" class="mb-4 rounded border border-brand/30 bg-brand/10 p-3 text-sm font-bold text-brand">{{ notice }}</p>

        <div v-if="active === 'overview'" class="grid gap-3 md:grid-cols-4">
          <div class="rounded border border-line bg-panel p-4 shadow-sm">
            <div class="text-2xl font-black text-brand">{{ stats.users || 0 }}</div>
            <div class="text-xs font-semibold text-muted">用户</div>
          </div>
          <div class="rounded border border-line bg-panel p-4 shadow-sm">
            <div class="text-2xl font-black text-brand">{{ stats.orders || 0 }}</div>
            <div class="text-xs font-semibold text-muted">订单</div>
          </div>
          <div class="rounded border border-line bg-panel p-4 shadow-sm">
            <div class="text-2xl font-black text-brand">{{ stats.api_keys || 0 }}</div>
            <div class="text-xs font-semibold text-muted">API Keys</div>
          </div>
          <div class="rounded border border-line bg-panel p-4 shadow-sm">
            <div class="text-2xl font-black text-brand">{{ stats.calls || 0 }}</div>
            <div class="text-xs font-semibold text-muted">调用</div>
          </div>
        </div>

        <div v-if="active === 'plans'" class="grid gap-4 xl:grid-cols-[360px_1fr]">
          <form class="rounded border border-line bg-panel p-5 shadow-sm" @submit.prevent="savePlan">
            <h3 class="mb-4 text-lg font-black text-forest">{{ editingPlanId ? '编辑套餐' : '创建套餐' }}</h3>
            <div class="grid gap-3">
              <input v-model="planForm.name" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="套餐名称，例如日卡套餐" required />
              <input v-model="planForm.code" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="套餐编码" />
              <div class="grid grid-cols-2 gap-3">
                <input v-model.number="planForm.price_cents" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="售价分 RMB" type="number" min="1" required />
                <input v-model.number="planForm.settlement_usd_cents" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="到账美分" type="number" min="0" />
              </div>
              <div class="grid grid-cols-2 gap-3">
                <input v-model.number="planForm.duration_days" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="有效期天数" type="number" min="1" required />
                <input v-model.number="planForm.quota_tokens" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="总额度 tokens" type="number" min="0" />
              </div>
              <div class="grid grid-cols-2 gap-3">
                <input v-model.number="planForm.daily_quota_tokens" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="每日额度" type="number" min="0" />
                <input v-model.number="planForm.weekly_quota_tokens" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="每周额度" type="number" min="0" />
              </div>
              <textarea v-model="planForm.description" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="套餐说明"></textarea>
              <label class="flex items-center gap-2 text-sm font-bold text-muted">
                <input v-model="planForm.enabled" type="checkbox" />
                启用套餐
              </label>
              <div class="flex gap-2">
                <button class="focus-ring rounded bg-accent px-4 py-2 font-bold text-white">{{ editingPlanId ? '保存' : '创建' }}</button>
                <button class="focus-ring rounded border border-line bg-white px-4 py-2 font-bold" type="button" @click="resetPlan">重置</button>
              </div>
            </div>
          </form>

          <div class="space-y-3">
            <div v-for="plan in plans" :key="plan.ID" class="rounded border border-line bg-panel p-4 shadow-sm">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h4 class="text-lg font-black text-forest">{{ plan.Name }}</h4>
                  <p class="mt-1 text-sm text-muted">{{ plan.Description }}</p>
                  <p class="mt-2 text-xs font-semibold text-muted">
                    {{ money(plan.PriceCents) }} / 到账 {{ money(plan.SettlementUSDCents, '$') }} / {{ plan.DurationDays }} 天
                  </p>
                  <p class="mt-1 text-xs text-muted">
                    总 {{ plan.QuotaTokens || 0 }}，日 {{ plan.DailyQuotaTokens || 0 }}，周 {{ plan.WeeklyQuotaTokens || 0 }} tokens
                  </p>
                </div>
                <div class="flex gap-2">
                  <button class="focus-ring rounded border border-line bg-white px-3 py-2 text-sm font-bold" @click="editPlan(plan)">编辑</button>
                  <button class="focus-ring rounded border border-red-200 bg-red-50 px-3 py-2 text-sm font-bold text-red-700" @click="deletePlan(plan.ID)">删除</button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div v-if="active === 'orders'" class="grid gap-4 xl:grid-cols-[360px_1fr]">
          <form class="rounded border border-line bg-panel p-5 shadow-sm" @submit.prevent="approveOrder">
            <h3 class="mb-4 text-lg font-black text-forest">审核通过</h3>
            <div class="space-y-3">
              <input v-model="approve.orderId" class="focus-ring w-full rounded border border-line bg-white px-3 py-2" placeholder="订单 ID" required />
              <input v-model="approve.channel" class="focus-ring w-full rounded border border-line bg-white px-3 py-2" placeholder="渠道" required />
              <input v-model="approve.baseUrl" class="focus-ring w-full rounded border border-line bg-white px-3 py-2" placeholder="上游 Base URL" required />
              <input v-model="approve.apiKey" class="focus-ring w-full rounded border border-line bg-white px-3 py-2" placeholder="上游 API Key" required />
              <input v-model="approve.adminNote" class="focus-ring w-full rounded border border-line bg-white px-3 py-2" placeholder="审核备注" />
              <button class="focus-ring w-full rounded bg-accent px-4 py-2 font-bold text-white">审核通过</button>
            </div>
          </form>

          <div class="rounded border border-line bg-panel p-5 shadow-sm">
            <h3 class="mb-4 text-lg font-black text-forest">订单</h3>
            <div class="overflow-auto">
              <table class="w-full text-left text-sm">
                <thead class="text-muted">
                  <tr>
                    <th class="py-2">ID</th>
                    <th class="py-2">用户</th>
                    <th class="py-2">套餐</th>
                    <th class="py-2">金额</th>
                    <th class="py-2">状态</th>
                    <th class="py-2">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="order in orders" :key="order.ID" class="border-t border-line">
                    <td class="py-2">#{{ order.ID }}</td>
                    <td class="py-2">{{ order.User?.Email }}</td>
                    <td class="py-2">{{ order.Plan?.Name }}</td>
                    <td class="py-2">{{ money(order.AmountCents) }}</td>
                    <td class="py-2">{{ order.Status }}</td>
                    <td class="py-2">
                      <button class="focus-ring rounded border border-line bg-white px-2 py-1 text-xs font-bold" @click="approve.orderId = String(order.ID)">审核</button>
                      <button class="focus-ring ml-2 rounded border border-red-200 bg-red-50 px-2 py-1 text-xs font-bold text-red-700" @click="rejectOrder(order.ID)">拒绝</button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <div v-if="active === 'users'" class="grid gap-3 md:grid-cols-2">
          <div v-for="user in users" :key="user.ID" class="rounded border border-line bg-panel p-4 shadow-sm">
            <div class="truncate text-sm font-black text-forest">{{ user.Email }}</div>
            <div class="mt-1 text-xs text-muted">{{ user.Username }} / {{ user.Role }} / {{ user.Status }}</div>
            <div class="mt-3 grid grid-cols-2 gap-2">
              <select class="focus-ring rounded border border-line bg-white px-2 py-2 text-sm" :value="user.Status" @change="updateUser(user, { status: $event.target.value })">
                <option value="pending">pending</option>
                <option value="approved">approved</option>
                <option value="disabled">disabled</option>
              </select>
              <select class="focus-ring rounded border border-line bg-white px-2 py-2 text-sm" :value="user.Role" @change="updateUser(user, { role: $event.target.value })">
                <option value="user">user</option>
                <option value="admin">admin</option>
              </select>
            </div>
            <div class="mt-3 flex flex-wrap gap-2">
              <button class="focus-ring rounded border border-line bg-white px-3 py-1 text-xs font-bold" @click="updateUser(user, { email_verified: true })">标记邮箱已验证</button>
              <button class="focus-ring rounded border border-red-200 bg-red-50 px-3 py-1 text-xs font-bold text-red-700" @click="deleteUser(user.ID)">删除</button>
            </div>
          </div>
        </div>

        <form v-if="active === 'settings'" class="rounded border border-line bg-panel p-5 shadow-sm" @submit.prevent="saveSettings">
          <h3 class="mb-4 text-lg font-black text-forest">系统设置</h3>
          <div class="grid gap-3 md:grid-cols-2">
            <input v-model="settings.site_title" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="网站标题" />
            <input v-model="settings.tutorial_video_url" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="视频教程播放地址" />
            <input v-model="settings.smtp_host" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="SMTP Host" />
            <input v-model.number="settings.smtp_port" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="SMTP Port" type="number" />
            <input v-model="settings.smtp_username" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="SMTP 用户名" />
            <input v-model="settings.smtp_password" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="SMTP 密码，留空不修改" type="password" />
            <input v-model="settings.smtp_from_email" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="发件邮箱" />
            <input v-model="settings.smtp_from_name" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="发件名称" />
            <input v-model="settings.epay_pid" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="易支付 PID" />
            <input v-model="settings.epay_key" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="易支付 Key，留空不修改" type="password" />
            <input v-model="settings.epay_submit_url" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="易支付提交地址" />
            <input v-model="settings.epay_notify_url" class="focus-ring rounded border border-line bg-white px-3 py-2" placeholder="异步通知地址" />
            <input v-model="settings.epay_return_url" class="focus-ring rounded border border-line bg-white px-3 py-2 md:col-span-2" placeholder="同步返回地址" />
            <label class="flex items-center gap-2 text-sm font-bold text-muted">
              <input v-model="settings.smtp_use_tls" type="checkbox" />
              SMTP 使用 TLS
            </label>
          </div>
          <button class="focus-ring mt-4 rounded bg-accent px-4 py-2 font-bold text-white">保存设置</button>
        </form>
      </div>
    </div>
  </section>
</template>
