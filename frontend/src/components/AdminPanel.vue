<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api/client'

const stats = ref({})
const orders = ref([])
const users = ref([])
const error = ref('')
const approve = reactive({ orderId: '', channel: 'openai', baseUrl: 'https://api.openai.com', apiKey: '' })

onMounted(loadAll)

async function loadAll() {
  try {
    const [statsRes, ordersRes, usersRes] = await Promise.all([
      api.get('/admin/stats'),
      api.get('/admin/orders'),
      api.get('/admin/users')
    ])
    stats.value = statsRes.data
    orders.value = ordersRes.data
    users.value = usersRes.data
  } catch (err) {
    error.value = err.message
  }
}

async function approveOrder() {
  error.value = ''
  try {
    await api.post(`/admin/orders/${approve.orderId}/approve`, {
      channel: approve.channel,
      base_url: approve.baseUrl,
      api_key: approve.apiKey
    })
    approve.orderId = ''
    approve.apiKey = ''
    await loadAll()
  } catch (err) {
    error.value = err.message
  }
}
</script>

<template>
  <section class="mx-auto max-w-7xl px-4 py-12 sm:px-6">
    <div class="mb-6">
      <h2 class="text-2xl font-bold">管理后台</h2>
      <p class="mt-1 text-sm text-slate-400">审核订单并为用户绑定上游账号。</p>
    </div>

    <p v-if="error" class="mb-4 rounded border border-red-800 bg-red-950/40 p-3 text-sm text-red-200">{{ error }}</p>

    <div class="mb-4 grid gap-3 md:grid-cols-4">
      <div class="rounded border border-line bg-panel p-4">
        <div class="text-2xl font-bold text-brand">{{ stats.users || 0 }}</div>
        <div class="text-xs text-slate-400">用户</div>
      </div>
      <div class="rounded border border-line bg-panel p-4">
        <div class="text-2xl font-bold text-brand">{{ stats.orders || 0 }}</div>
        <div class="text-xs text-slate-400">订单</div>
      </div>
      <div class="rounded border border-line bg-panel p-4">
        <div class="text-2xl font-bold text-brand">{{ stats.api_keys || 0 }}</div>
        <div class="text-xs text-slate-400">API Keys</div>
      </div>
      <div class="rounded border border-line bg-panel p-4">
        <div class="text-2xl font-bold text-brand">{{ stats.calls || 0 }}</div>
        <div class="text-xs text-slate-400">调用</div>
      </div>
    </div>

    <div class="grid gap-4 lg:grid-cols-[0.9fr_1.1fr]">
      <form class="rounded border border-line bg-panel p-5" @submit.prevent="approveOrder">
        <h3 class="mb-4 text-lg font-semibold">审核通过</h3>
        <div class="space-y-3">
          <input v-model="approve.orderId" class="focus-ring w-full rounded border border-line bg-ink px-3 py-2" placeholder="订单 ID" required />
          <input v-model="approve.channel" class="focus-ring w-full rounded border border-line bg-ink px-3 py-2" placeholder="渠道" required />
          <input v-model="approve.baseUrl" class="focus-ring w-full rounded border border-line bg-ink px-3 py-2" placeholder="上游 Base URL" required />
          <input v-model="approve.apiKey" class="focus-ring w-full rounded border border-line bg-ink px-3 py-2" placeholder="上游 API Key" required />
          <button class="focus-ring w-full rounded bg-brand px-4 py-2 font-semibold text-ink">审核通过</button>
        </div>
      </form>

      <div class="rounded border border-line bg-panel p-5">
        <h3 class="mb-4 text-lg font-semibold">订单</h3>
        <div class="max-h-96 overflow-auto">
          <table class="w-full text-left text-sm">
            <thead class="text-slate-400">
              <tr>
                <th class="py-2">ID</th>
                <th class="py-2">用户</th>
                <th class="py-2">套餐</th>
                <th class="py-2">状态</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="order in orders" :key="order.ID" class="border-t border-line">
                <td class="py-2">#{{ order.ID }}</td>
                <td class="py-2">{{ order.User?.Email }}</td>
                <td class="py-2">{{ order.Plan?.Name }}</td>
                <td class="py-2">{{ order.Status }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <div class="mt-4 rounded border border-line bg-panel p-5">
      <h3 class="mb-4 text-lg font-semibold">用户</h3>
      <div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
        <div v-for="user in users" :key="user.ID" class="rounded border border-line bg-ink p-3">
          <div class="truncate text-sm font-medium">{{ user.Email }}</div>
          <div class="mt-1 text-xs text-slate-500">{{ user.Role }} · {{ user.Status }}</div>
        </div>
      </div>
    </div>
  </section>
</template>
