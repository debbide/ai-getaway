<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'

const emit = defineEmits(['navigate'])

const auth = useAuthStore()
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
const quotaUsage = computed(() => auth.user?.quota_usage || null)
const totalQuotaUsage = computed(() => auth.user?.total_quota_usage || null)
const quotaUsagePercent = computed(() => normalizePercent(quotaUsage.value?.percent))
const totalQuotaUsagePercent = computed(() => normalizePercent(totalQuotaUsage.value?.percent))
const quotaProgressStyle = computed(() => ({ '--quota-progress': `${quotaUsagePercent.value}%` }))
const totalQuotaProgressStyle = computed(() => ({ '--quota-progress': `${totalQuotaUsagePercent.value}%` }))
const quotaResetText = computed(() => {
  if (!quotaUsage.value?.window_end) return '暂无重置时间'
  return `${quotaPeriodUnit(auth.user?.plan)}额度重置：${formatDateTime(quotaUsage.value.window_end)}`
})

onMounted(loadAll)
onBeforeUnmount(stopAutoRefresh)

watch(error, (message) => {
  if (message) ElMessage.error(message)
})

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [keyRes] = await Promise.all([api.get('/keys'), loadRecords(), auth.loadMe()])
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
    await auth.loadMe()
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
    await auth.loadMe()
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
  const head = ['API密钥', '模型', '推理强度', '端点', '类型', '计费模式', '计费输入Token', '缓存Token', '输出Token', '总Token', '费用', '首Token', '耗时', '状态', '时间']
  const rows = records.value.map((item) => [
    item.api_key_name || maskKey(item),
    item.model || '-',
    '-',
    item.endpoint || item.path || '-',
    requestTypeLabel(item.request_type),
    billingModeLabel(item.billing_mode),
    billableInputTokens(item),
    item.cached_input_tokens || 0,
    item.completion_tokens || 0,
    item.total_tokens || 0,
    item.estimated_usd_micros ? usdMicros(item.estimated_usd_micros) : usd(item.estimated_usd_cents || 0),
    latency(item.first_token_ms),
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

function billableInputTokens(item) {
  return Math.max(0, Number(item?.prompt_tokens || 0) - Number(item?.cached_input_tokens || 0))
}

function usd(cents) {
  return `$${((cents || 0) / 100).toFixed(4)}`
}

function usdCompact(cents) {
  return `$${((cents || 0) / 100).toFixed(2)}`
}

function normalizePercent(value) {
  const percent = Number(value || 0)
  if (!Number.isFinite(percent)) return 0
  return Math.min(100, Math.max(0, percent))
}

function quotaPeriodUnit(plan) {
  const period = plan?.QuotaPeriod || plan?.quota_period
  return period === 'daily' ? '日' : '周'
}

function usdMicros(value) {
  return `$${(Number(value || 0) / 1000000).toFixed(6)}`
}

function modelUnit(value) {
  return `$${Number(value || 0).toFixed(4)} / 1M Token`
}

function billingSourceLabel(value) {
  return { model_management: '后台模型', official_fallback: '官方兜底', fallback: '系统兜底' }[value] || value || '-'
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
      <div>
        <p class="section-kicker">Usage Logs</p>
        <h2>使用记录</h2>
        <p>按 API 密钥和时间范围查看接口调用、Token、费用、耗时和状态。</p>
      </div>
    </div>

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

    <div v-if="quotaUsage || totalQuotaUsage" class="usage-quota-grid">
      <article v-if="quotaUsage" class="usage-quota-card">
        <div class="quota-meter-head">
          <span>周期额度</span>
          <strong>{{ quotaUsagePercent.toFixed(1) }}%</strong>
        </div>
        <div class="quota-meter-values">
          <span>已用 {{ usdCompact(quotaUsage.used_usd_cents || 0) }}</span>
          <span>剩余 {{ usdCompact(quotaUsage.remaining_usd_cents || 0) }}</span>
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
      </article>

      <article v-if="totalQuotaUsage" class="usage-quota-card usage-quota-card-total">
        <div class="quota-meter-head">
          <span>总额度</span>
          <strong>{{ totalQuotaUsagePercent.toFixed(1) }}%</strong>
        </div>
        <div class="quota-meter-values">
          <span>已用 {{ usdCompact(totalQuotaUsage.used_usd_cents || 0) }}</span>
          <span>总额 {{ usdCompact(totalQuotaUsage.limit_usd_cents || 0) }}</span>
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
      </article>
    </div>

    <section class="panel-surface usage-filter-card">
      <el-form-item label="API 密钥">
        <el-select v-model="filters.apiKeyId" class="w-full" @change="applyFilters">
          <el-option label="全部密钥" value="" />
          <el-option
            v-for="key in keys"
            :key="key.id"
            :label="`${key.name} / ${key.key_masked || key.key_prefix}`"
            :value="key.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="时间范围">
        <el-select v-model="filters.range" class="w-full" @change="applyFilters">
          <el-option label="近 24 小时" value="24h" />
          <el-option label="近 7 天" value="7d" />
          <el-option label="近 30 天" value="30d" />
          <el-option label="全部时间" value="all" />
        </el-select>
      </el-form-item>
      <div class="usage-filter-actions">
        <el-button :type="autoRefresh ? 'success' : 'default'" plain @click="toggleAutoRefresh">
          <span class="usage-auto-refresh-icon" aria-hidden="true">↻</span>
          {{ autoRefresh ? '自动刷新中' : '自动刷新' }}
        </el-button>
        <el-button :loading="loading" @click="refreshRecords">刷新</el-button>
        <el-button :disabled="loading" @click="resetFilters">重置</el-button>
        <el-button type="primary" :disabled="!records.length" @click="exportCsv">导出 CSV</el-button>
      </div>
    </section>

    <section class="panel-surface usage-table-card">
      <div class="usage-table-wrap">
        <el-table v-loading="loading" :data="records" class="usage-table" empty-text="暂无调用记录" border>
          <el-table-column label="API 密钥" min-width="170">
            <template #default="{ row: item }">
                <div class="usage-main-value">
                  <strong>{{ item.api_key_name || maskKey(item) }}</strong>
                  <el-tooltip placement="top" :content="`密钥标识：${maskKey(item)}`">
                    <span class="usage-info-dot" tabindex="0">!</span>
                  </el-tooltip>
                </div>
            </template>
          </el-table-column>
          <el-table-column label="模型" min-width="160">
            <template #default="{ row: item }">
                <strong>{{ item.model || '-' }}</strong>
                <span class="usage-cell-sub">Medium</span>
            </template>
          </el-table-column>
          <el-table-column label="端点" min-width="220">
            <template #default="{ row: item }">
              <div class="usage-endpoint-cell">
                <code>{{ item.endpoint || item.path || '-' }}</code>
                <span class="usage-cell-chips">
                  <span class="usage-chip">{{ requestTypeLabel(item.request_type) }}</span>
                  <span class="usage-chip muted">{{ billingModeLabel(item.billing_mode) }}</span>
                </span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="Token" min-width="150">
            <template #default="{ row: item }">
                <div class="usage-main-value">
                  <strong>{{ statToken(item.total_tokens) }}</strong>
                  <el-tooltip
                    placement="top"
                    :content="`输入 ${statToken(billableInputTokens(item))} / 输出 ${statToken(item.completion_tokens)} / 缓存 ${statToken(item.cached_input_tokens)}`"
                  >
                    <span class="usage-info-dot" tabindex="0">!</span>
                  </el-tooltip>
                </div>
            </template>
          </el-table-column>
          <el-table-column label="费用" min-width="150">
            <template #default="{ row: item }">
                <div class="usage-main-value">
                  <strong class="usage-cost">{{ item.estimated_usd_micros ? usdMicros(item.estimated_usd_micros) : usd(item.estimated_usd_cents || 0) }}</strong>
                  <el-tooltip placement="top" popper-class="usage-cost-popper">
                    <template #content>
                      <span class="usage-tooltip-content usage-tooltip-wide">
                      <span class="usage-tip-title">费用明细</span>
                      <span class="usage-tip-grid">
                        <span>输入</span><b>{{ usdMicros(item.input_usd_micros) }}</b>
                        <span>输出</span><b>{{ usdMicros(item.output_usd_micros) }}</b>
                        <span v-if="item.cached_input_usd_micros">缓存</span><b v-if="item.cached_input_usd_micros">{{ usdMicros(item.cached_input_usd_micros) }}</b>
                        <span>倍率</span><b>{{ Number(item.billing_multiplier || 1).toFixed(2) }}x</b>
                        <span>分组倍率</span><b>{{ Number(item.group_multiplier || 1).toFixed(2) }}x</b>
                        <span>来源</span><b>{{ billingSourceLabel(item.billing_source) }}</b>
                      </span>
                      <span class="usage-tip-rate">{{ modelUnit(item.input_usd_per_million) }} 输入 / {{ modelUnit(item.output_usd_per_million) }} 输出</span>
                      </span>
                    </template>
                    <span class="usage-info-dot" tabindex="0">!</span>
                  </el-tooltip>
                </div>
            </template>
          </el-table-column>
          <el-table-column label="首 Token" min-width="110">
            <template #default="{ row: item }">{{ latency(item.first_token_ms) }}</template>
          </el-table-column>
          <el-table-column label="耗时 / 状态" min-width="130">
            <template #default="{ row: item }">
                <strong>{{ latency(item.latency_ms) }}</strong>
                <span class="usage-status mt-1" :class="statusClass(item.status_code)">{{ item.status_code }}</span>
            </template>
          </el-table-column>
          <el-table-column label="时间" min-width="160">
            <template #default="{ row: item }">{{ formatDateTime(item.created_at) }}</template>
          </el-table-column>
        </el-table>
      </div>
      <div class="pagination-bar usage-pagination">
        <span>显示 {{ displayStart }} 至 {{ displayEnd }} 共 {{ total }} 条结果</span>
        <el-pagination
          layout="prev, pager, next"
          :current-page="filters.page"
          :page-size="filters.pageSize"
          :total="total"
          :disabled="loading"
          @current-change="setPage"
        />
      </div>
    </section>
  </section>
</template>
