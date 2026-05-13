<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'

defineProps({ plans: { type: Array, default: () => [] } })

const auth = useAuthStore()
const orders = ref([])
const keys = ref([])
const selectedPlan = ref('')
const keyName = ref('Default')
const newKey = ref('')
const error = ref('')

onMounted(loadAll)

async function loadAll() {
  try {
    const [orderRes, keyRes] = await Promise.all([api.get('/orders'), api.get('/keys')])
    orders.value = orderRes.data
    keys.value = keyRes.data
  } catch (err) {
    error.value = err.message
  }
}

async function createOrder() {
  if (!selectedPlan.value) return
  error.value = ''
  try {
    await api.post('/orders', { plan_id: Number(selectedPlan.value), payment_ref: `manual-${Date.now()}` })
    await loadAll()
  } catch (err) {
    error.value = err.message
  }
}

async function createKey() {
  error.value = ''
  newKey.value = ''
  try {
    const res = await api.post('/keys', { name: keyName.value })
    newKey.value = res.data.key
    await loadAll()
  } catch (err) {
    error.value = err.message
  }
}

async function disableKey(id) {
  await api.patch(`/keys/${id}/disable`)
  await loadAll()
}
</script>

<template>
  <section class="mx-auto max-w-7xl px-4 py-12 sm:px-6">
    <div class="mb-6 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-2xl font-bold">控制台</h2>
        <p class="mt-1 text-sm text-slate-400">账号状态：{{ auth.user?.status }}</p>
      </div>
      <div class="text-sm text-slate-300">
        {{ auth.user?.used_tokens || 0 }} / {{ auth.user?.quota_tokens || 0 }} tokens
      </div>
    </div>

    <p v-if="error" class="mb-4 rounded border border-red-800 bg-red-950/40 p-3 text-sm text-red-200">{{ error }}</p>
    <p v-if="newKey" class="mb-4 rounded border border-brand/50 bg-brand/10 p-3 text-sm text-brand">
      新 API Key：{{ newKey }}
    </p>

    <div class="grid gap-4 lg:grid-cols-2">
      <div class="rounded border border-line bg-panel p-5">
        <h3 class="mb-4 text-lg font-semibold">选择套餐并生成订单</h3>
        <div class="flex gap-3">
          <select v-model="selectedPlan" class="focus-ring min-w-0 flex-1 rounded border border-line bg-ink px-3 py-2">
            <option value="">请选择套餐</option>
            <option v-for="plan in plans" :key="plan.ID" :value="plan.ID">{{ plan.Name }}</option>
          </select>
          <button class="focus-ring rounded bg-brand px-4 py-2 font-semibold text-ink" @click="createOrder">下单</button>
        </div>

        <div class="mt-5 overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="text-slate-400">
              <tr>
                <th class="py-2">订单</th>
                <th class="py-2">套餐</th>
                <th class="py-2">状态</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="order in orders" :key="order.ID" class="border-t border-line">
                <td class="py-2">#{{ order.ID }}</td>
                <td class="py-2">{{ order.Plan?.Name }}</td>
                <td class="py-2">{{ order.Status }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="rounded border border-line bg-panel p-5">
        <h3 class="mb-4 text-lg font-semibold">API Key 管理</h3>
        <div class="flex gap-3">
          <input v-model="keyName" class="focus-ring min-w-0 flex-1 rounded border border-line bg-ink px-3 py-2" placeholder="Key 名称" />
          <button class="focus-ring rounded bg-brand px-4 py-2 font-semibold text-ink" @click="createKey">创建</button>
        </div>

        <div class="mt-5 space-y-3">
          <div v-for="key in keys" :key="key.ID" class="flex items-center justify-between gap-3 rounded border border-line bg-ink p-3">
            <div class="min-w-0">
              <div class="truncate text-sm font-medium">{{ key.Name }}</div>
              <div class="text-xs text-slate-500">{{ key.KeyPrefix }} · {{ key.Status }}</div>
            </div>
            <button class="focus-ring rounded border border-line px-3 py-1 text-sm" @click="disableKey(key.ID)">禁用</button>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
