<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { api } from '../api/client'

const emit = defineEmits(['navigate'])

const loading = ref(false)
const error = ref('')
const keys = ref([])
const records = ref([])
const summary = ref(null)
const total = ref(0)
const pages = ref(1)
const autoRefresh = ref(false)
const autoRefreshing = ref(false)
let autoRefreshTimer = null

const filters = reactive({
  apiKeyId: '',
  range: '7d',
  page: 1,
  pageSize: 20
})

const selectedKeyLabel = computed(() => {
  if (!filters.apiKeyId) return '全部密钥'
  const key = keys.value.find((item) => String(item.id) === String(filters.apiKeyId))
  return key?.name || '指定密钥'
})

const totalPages = computed(() => Math.max(1, pages.value || Math.ceil(total.value / filters.pageSize) || 1))
const displayStart = computed(() => (total.value > 0 ? (filters.page - 1) * filters.pageSize + 1 : 0))
const displayEnd = computed(() => (total.value > 0 ? Math.min(filters.page * filters.pageSize, total.value) : 0))

onMounted(loadAll)
onBeforeUnmount(stopAutoRefresh)

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [keyRes] = await Promise.all([api.get('/keys'), loadRecords()])
    keys.value = keyRes.data || []
  } catch (err) {
    error.value = err.message || '使用记录暂时不可用，请稍后重试'
  } finally {
    loading.value = false
  }
}

async function loadRecords() {
  const params = {
    page: filters.page,
    page_size: filters.pageSize,
    range: filters.range
  }
  if (filters.apiKeyId) params.api_key_id = filters.apiKeyId
  const res = await api.get('/usage/logs', { params })
  records.value = res.data?.items || []
  summary.value = res.data?.summary || null
  total.value = res.data?.total || 0
  pages.value = res.data?.pages || 1
}

async function refreshRecords() {
  loading.value = true
  error.value = ''
  try {
    await loadRecords()
  } catch (err) {
    error.value = err.message || '使用记录暂时不可用，请稍后重试'
  } finally {
    loading.value = false
  }
}

async function refreshRecordsSilently() {
  if (loading.value || autoRefreshing.value) return
  autoRefreshing.value = true
  try {
    await loadRecords()
    error.value = ''
  } catch (err) {
    error.value = err.message || '使用记录暂时不可用，请稍后重试'
  } finally {
    autoRefreshing.value = false
  }
}

function toggleAutoRefresh() {
  if (autoRefresh.value) {
    stopAutoRefresh()
    return
  }
  autoRefresh.value = true
  refreshRecordsSilently()
  autoRefreshTimer = window.setInterval(refreshRecordsSilently, 5000)
}

function stopAutoRefresh() {
  autoRefresh.value = false
  if (autoRefreshTimer) {
    window.clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
}

async function applyFilters() {
  filters.page = 1
  await refreshRecords()
}

async function resetFilters() {
  filters.apiKeyId = ''
  filters.range = '7d'
  filters.page = 1
  await refreshRecords()
}

async function setPage(page) {
  filters.page = Math.min(Math.max(1, page), totalPages.value)
  await refreshRecords()
}

function exportCsv() {
  const head = ['API密钥', '模型', '推理强度', '端点', '类型', '计费模式', '输入Token', '输出Token', '总Token', '费用', '首Token', '耗时', '状态', '时间']
  const rows = records.value.map((item) => [
    item.api_key_name || maskKey(item),
    item.model || '-',
    '-',
    item.endpoint || item.path || '-',
    requestTypeLabel(item.request_type),
    billingModeLabel(item.billing_mode),
    item.prompt_tokens || 0,
    item.completion_tokens || 0,
    item.total_tokens || 0,
    usd(item.estimated_usd_cents || 0),
    '-',
    latency(item.latency_ms),
    item.status_code,
    formatDateTime(item.created_at)
  ])
  const csv = [head, ...rows].map((row) => row.map(csvCell).join(',')).join('\n')
  const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `usage-records-${new Date().toISOString().slice(0, 10)}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

function csvCell(value) {
  return `"${String(value ?? '').replace(/"/g, '""')}"`
}

function statToken(value) {
  const n = Number(value || 0)
  if (n >= 1000000) return `${(n / 1000000).toFixed(2)}M`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`
  return n.toLocaleString()
}

function usd(cents) {
  return `$${((cents || 0) / 100).toFixed(4)}`
}

function latency(ms) {
  if (!ms) return '-'
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${ms}ms`
}

function pad2(n) {
  return String(n).padStart(2, '0')
}

function formatDateTime(value) {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '-'
  return `${d.getFullYear()}/${pad2(d.getMonth() + 1)}/${pad2(d.getDate())} ${pad2(d.getHours())}:${pad2(d.getMinutes())}`
}

function maskKey(item) {
  return item.api_key_prefix ? `${item.api_key_prefix}...` : `Key #${item.api_key_id || '-'}`
}

function requestTypeLabel(value) {
  return { chat: '对话', stream: '流式' }[value] || '调用'
}

function billingModeLabel(value) {
  return { usage: '按量', subscription: '订阅' }[value] || '按量'
}

function statusClass(code) {
  if (code >= 500) return 'danger'
  if (code >= 400) return 'warn'
  return 'ok'
}
</script>

<template>
  <section class="console-shell usage-records-page mx-auto max-w-7xl px-4 pb-12 sm:px-6">
    <div class="usage-records-head">
      <button class="ghost-button small" type="button" @click="emit('navigate', '/console')">返回控制台</button>
      <div>
        <p class="section-kicker">Usage Logs</p>
        <h2>使用记录</h2>
        <p>按 API 密钥和时间范围查看接口调用、Token、费用、耗时和状态。</p>
      </div>
    </div>

    <div v-if="error" class="alert alert-danger">{{ error }}</div>

    <div class="usage-stat-grid">
      <article class="usage-stat-card">
        <span>总请求数</span>
        <strong>{{ summary?.total_requests || 0 }}</strong>
        <small>{{ selectedKeyLabel }}</small>
      </article>
      <article class="usage-stat-card">
        <span>总 Token</span>
        <strong>{{ statToken(summary?.total_tokens) }}</strong>
        <small>输入 {{ statToken(summary?.prompt_tokens) }} / 输出 {{ statToken(summary?.completion_tokens) }}</small>
      </article>
      <article class="usage-stat-card">
        <span>总费用</span>
        <strong>{{ usd(summary?.total_usd_cents || 0) }}</strong>
        <small>按实际日志估算</small>
      </article>
      <article class="usage-stat-card">
        <span>平均耗时</span>
        <strong>{{ latency(summary?.average_latency_ms) }}</strong>
        <small>每次请求</small>
      </article>
    </div>

    <section class="panel-surface usage-filter-card">
      <label class="field">
        <span>API 密钥</span>
        <select v-model="filters.apiKeyId" @change="applyFilters">
          <option value="">全部密钥</option>
          <option v-for="key in keys" :key="key.id" :value="key.id">
            {{ key.name }} / {{ key.key_masked || key.key_prefix }}
          </option>
        </select>
      </label>
      <label class="field">
        <span>时间范围</span>
        <select v-model="filters.range" @change="applyFilters">
          <option value="24h">近 24 小时</option>
          <option value="7d">近 7 天</option>
          <option value="30d">近 30 天</option>
          <option value="all">全部时间</option>
        </select>
      </label>
      <div class="usage-filter-actions">
        <button
          class="ghost-button usage-auto-refresh-button"
          :class="{ active: autoRefresh }"
          type="button"
          @click="toggleAutoRefresh"
        >
          <span class="usage-auto-refresh-icon" aria-hidden="true">↻</span>
          {{ autoRefresh ? '自动刷新中' : '自动刷新' }}
        </button>
        <button class="ghost-button" type="button" :disabled="loading" @click="refreshRecords">刷新</button>
        <button class="ghost-button" type="button" :disabled="loading" @click="resetFilters">重置</button>
        <button class="primary-button" type="button" :disabled="!records.length" @click="exportCsv">导出 CSV</button>
      </div>
    </section>

    <section class="panel-surface usage-table-card">
      <div class="table-wrap usage-table-wrap">
        <table class="data-table usage-table">
          <thead>
            <tr>
              <th>API 密钥</th>
              <th>模型</th>
              <th>推理强度</th>
              <th>端点</th>
              <th>类型</th>
              <th>计费模式</th>
              <th>Token</th>
              <th>费用</th>
              <th>首 Token</th>
              <th>耗时</th>
              <th>状态</th>
              <th>时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!loading && !records.length">
              <td colspan="12" class="usage-empty">暂无调用记录</td>
            </tr>
            <tr v-for="item in records" :key="item.id">
              <td>
                <strong>{{ item.api_key_name || maskKey(item) }}</strong>
                <small>{{ maskKey(item) }}</small>
              </td>
              <td><strong>{{ item.model || '-' }}</strong></td>
              <td>Medium</td>
              <td><code>{{ item.endpoint || item.path || '-' }}</code></td>
              <td><span class="usage-chip">{{ requestTypeLabel(item.request_type) }}</span></td>
              <td><span class="usage-chip muted">{{ billingModeLabel(item.billing_mode) }}</span></td>
              <td>
                <strong>{{ statToken(item.total_tokens) }}</strong>
                <small>↓ {{ statToken(item.prompt_tokens) }} / ↑ {{ statToken(item.completion_tokens) }}</small>
              </td>
              <td><strong class="usage-cost">{{ usd(item.estimated_usd_cents || 0) }}</strong></td>
              <td>-</td>
              <td>{{ latency(item.latency_ms) }}</td>
              <td><span class="usage-status" :class="statusClass(item.status_code)">{{ item.status_code }}</span></td>
              <td>{{ formatDateTime(item.created_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="pagination-bar usage-pagination">
        <span>显示 {{ displayStart }} 至 {{ displayEnd }} 共 {{ total }} 条结果</span>
        <div class="table-actions">
          <button class="ghost-button small" :disabled="filters.page <= 1 || loading" @click="setPage(filters.page - 1)">上一页</button>
          <span class="usage-page-number">{{ filters.page }} / {{ totalPages }}</span>
          <button class="ghost-button small" :disabled="filters.page >= totalPages || loading" @click="setPage(filters.page + 1)">下一页</button>
        </div>
      </div>
    </section>
  </section>
</template>
