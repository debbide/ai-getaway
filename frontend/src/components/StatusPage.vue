<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { api } from '../api/client'

const AUTO_REFRESH_SECONDS = 45
const rangeDays = ref(7)
const items = ref([])
const loading = ref(false)
const error = ref('')
const lastUpdatedAt = ref(null)
const autoRefreshEnabled = ref(true)
const autoRefreshRemaining = ref(AUTO_REFRESH_SECONDS)
let autoRefreshTimer = null

const operational = computed(() => items.value.length > 0 && items.value.every((item) => item.status !== 'unavailable'))
const availableCount = computed(() => items.value.filter((item) => item.status !== 'unavailable').length)
const autoRefreshLabel = computed(() => {
  if (!autoRefreshEnabled.value) return '自动刷新: 已关闭'
  return `自动刷新: ${autoRefreshRemaining.value}s`
})

onMounted(() => {
  loadStatus()
  startAutoRefresh()
})

onBeforeUnmount(() => {
  stopAutoRefresh()
})

async function loadStatus() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/status/monitors', { params: { range_days: rangeDays.value } })
    items.value = Array.isArray(res.data?.items) ? res.data.items : []
    lastUpdatedAt.value = new Date()
  } catch (err) {
    error.value = err.message || '状态加载失败'
  } finally {
    loading.value = false
    resetAutoRefreshCountdown()
  }
}

function setRange(days) {
  if (rangeDays.value === days) return
  rangeDays.value = days
  loadStatus()
}

function startAutoRefresh() {
  stopAutoRefresh()
  autoRefreshEnabled.value = true
  autoRefreshRemaining.value = AUTO_REFRESH_SECONDS
  autoRefreshTimer = setInterval(() => {
    autoRefreshRemaining.value = Math.max(0, autoRefreshRemaining.value - 1)
    if (autoRefreshRemaining.value <= 0) loadStatus()
  }, 1000)
}

function stopAutoRefresh() {
  if (autoRefreshTimer) clearInterval(autoRefreshTimer)
  autoRefreshTimer = null
}

function toggleAutoRefresh() {
  if (autoRefreshEnabled.value) {
    autoRefreshEnabled.value = false
    stopAutoRefresh()
    return
  }
  startAutoRefresh()
}

function resetAutoRefreshCountdown() {
  if (!autoRefreshEnabled.value) return
  autoRefreshRemaining.value = AUTO_REFRESH_SECONDS
}

function statusText(status) {
  return {
    available: '正常',
    degraded: '波动',
    unavailable: '不可用'
  }[status] || '待检测'
}

function statusClass(status) {
  return {
    available: 'is-good',
    degraded: 'is-warn',
    unavailable: 'is-down'
  }[status] || 'is-empty'
}

function formatTime(value) {
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '--:--'
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function formatDateTime(value) {
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '-'
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}/${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function availability(item) {
  return Number(item.availability || 0).toFixed(2)
}

function tooltipUnavailableReason(item) {
  if (!item.summary?.unavailable) return '暂无不可用记录'
  return `不可用 ${item.summary.unavailable} 次`
}

function displayStatusRecords(item) {
  const records = Array.isArray(item.records) ? item.records.slice(-60) : []
  if (records.length >= 60) return records
  return [
    ...Array.from({ length: 60 - records.length }, () => ({ status: '', latency_ms: 0, checked_at: null })),
    ...records
  ]
}
</script>

<template>
  <main class="status-page">
    <section class="status-toolbar">
      <div>
        <p class="status-kicker">Status</p>
        <h1>渠道监控</h1>
        <span>模型可用性与近期延迟</span>
      </div>
      <div class="status-actions">
        <div class="range-switch" role="tablist" aria-label="选择时间范围">
          <button :class="{ active: rangeDays === 7 }" type="button" @click="setRange(7)">7 天</button>
          <button :class="{ active: rangeDays === 15 }" type="button" @click="setRange(15)">15 天</button>
          <button :class="{ active: rangeDays === 30 }" type="button" @click="setRange(30)">30 天</button>
        </div>
        <span class="global-status" :class="{ down: !operational }"><i></i>{{ operational ? 'OPERATIONAL' : 'DEGRADED' }}</span>
        <button class="manual-refresh-button" type="button" :disabled="loading" aria-label="刷新" title="刷新" @click="loadStatus">
          <Refresh class="refresh-icon" />
        </button>
        <button class="auto-refresh-button" :class="{ active: autoRefreshEnabled }" type="button" @click="toggleAutoRefresh">
          <Refresh class="refresh-icon" />
          <span>{{ autoRefreshLabel }}</span>
        </button>
      </div>
    </section>

    <section v-if="error" class="status-empty">{{ error }}</section>
    <section v-else-if="!items.length && !loading" class="status-empty">暂无渠道监控</section>
    <section v-else class="status-grid" :aria-busy="loading">
      <article v-for="item in items" :key="item.id" class="status-card">
        <div class="status-card-head">
          <span class="provider-mark">AI</span>
          <div>
            <h2>{{ item.model_name }}</h2>
            <p>{{ formatTime(item.last_checked_at) }} · {{ item.latency_ms || 0 }}ms</p>
          </div>
          <strong :class="statusClass(item.status)">{{ statusText(item.status) }}</strong>
        </div>

        <div class="status-metrics">
          <div>
            <span>端点 PING</span>
            <strong>{{ item.latency_ms || 0 }}<small>ms</small></strong>
          </div>
          <div>
            <span>可用性 · {{ rangeDays }} 天</span>
            <strong>{{ availability(item) }}<small>%</small></strong>
          </div>
        </div>

        <div class="status-bars">
          <div class="status-bars-head">
            <span>近 60 次记录</span>
            <span>{{ formatTime(item.last_checked_at) }} 后刷新</span>
          </div>
          <div class="bar-row">
            <span
              v-for="(record, index) in displayStatusRecords(item)"
              :key="`${item.id}-${record.checked_at || 'empty'}-${index}`"
              class="status-bar"
              :class="statusClass(record.status)"
            >
              <span class="status-tooltip">
                <strong>{{ formatDateTime(record.checked_at) }}</strong>
                <b>可用率: {{ availability(item) }}%</b>
                <em>延迟: {{ record.latency_ms || 0 }}ms</em>
                <small>可用 {{ item.summary?.available || 0 }} 次 · 波动 {{ item.summary?.degraded || 0 }} 次 · {{ tooltipUnavailableReason(item) }}</small>
              </span>
            </span>
          </div>
          <div class="bar-axis"><span>PAST</span><span>NOW</span></div>
        </div>
      </article>
    </section>

    <section class="status-summary">
      <span>{{ availableCount }}/{{ items.length }} 模型可用</span>
      <span>最后更新 {{ lastUpdatedAt ? formatDateTime(lastUpdatedAt) : '-' }}</span>
    </section>
  </main>
</template>

<style scoped>
.status-page {
  min-height: calc(100vh - 160px);
  padding: 34px clamp(16px, 4vw, 64px) 56px;
  background:
    linear-gradient(180deg, rgba(12, 20, 36, 0.96), rgba(9, 14, 25, 1)),
    #0b1220;
  color: #e5edf8;
}

.status-toolbar {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 24px;
  max-width: 1480px;
  margin: 0 auto 24px;
}

.status-kicker {
  margin: 0 0 8px;
  color: #62d59f;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.status-toolbar h1 {
  margin: 0;
  font-size: clamp(30px, 4vw, 44px);
  font-weight: 950;
  letter-spacing: 0;
}

.status-toolbar span {
  color: #8d99ad;
  font-weight: 800;
}

.status-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
}

.range-switch {
  display: inline-flex;
  padding: 4px;
  border: 1px solid rgba(99, 116, 145, 0.45);
  border-radius: 8px;
  background: rgba(32, 43, 63, 0.88);
}

.range-switch button,
.manual-refresh-button,
.auto-refresh-button {
  border: 0;
  border-radius: 6px;
  color: #aeb8ca;
  font-weight: 900;
  background: transparent;
  cursor: pointer;
}

.range-switch button {
  min-width: 70px;
  padding: 9px 13px;
}

.range-switch button.active {
  color: #f6f8fb;
  background: #3a465f;
}

.global-status {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 11px 17px;
  border-radius: 8px;
  background: rgba(21, 76, 63, 0.68);
  color: #8defbd;
}

.global-status.down {
  background: rgba(92, 45, 44, 0.78);
  color: #ff9f8f;
}

.global-status i {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: currentColor;
}

.manual-refresh-button {
  display: inline-grid;
  width: 42px;
  height: 42px;
  place-items: center;
  border: 1px solid rgba(99, 116, 145, 0.55);
  background: rgba(32, 43, 63, 0.78);
}

.auto-refresh-button {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  height: 42px;
  padding: 0 14px;
  border: 1px solid rgba(99, 116, 145, 0.6);
  border-radius: 8px;
  background: rgba(32, 43, 63, 0.78);
  font-size: 15px;
}

.refresh-icon {
  width: 18px;
  height: 18px;
}

.auto-refresh-button.active .refresh-icon {
  animation: status-refresh-spin 1.2s linear infinite;
}

@keyframes status-refresh-spin {
  to {
    transform: rotate(360deg);
  }
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(330px, 430px));
  justify-content: center;
  gap: 22px;
  max-width: 1880px;
  margin: 0 auto;
}

.status-card {
  position: relative;
  padding: 24px;
  border: 1px solid rgba(82, 97, 124, 0.68);
  border-radius: 8px;
  background: #172033;
  box-shadow: 0 18px 50px rgba(0, 0, 0, 0.22);
}

.status-card-head {
  display: grid;
  grid-template-columns: 58px minmax(0, 1fr) auto;
  gap: 16px;
  align-items: center;
}

.provider-mark {
  display: grid;
  width: 58px;
  height: 58px;
  place-items: center;
  border-radius: 8px;
  color: #7af0b6;
  font-size: 18px;
  font-weight: 950;
  background: rgba(36, 89, 76, 0.72);
}

.status-card h2 {
  margin: 0;
  overflow-wrap: anywhere;
  color: #f4f7fb;
  font-size: 25px;
  font-weight: 950;
  letter-spacing: 0;
}

.status-card p {
  margin: 7px 0 0;
  color: #8995a8;
  font-size: 14px;
  font-weight: 850;
}

.status-card-head > strong {
  padding: 8px 15px;
  border-radius: 999px;
  font-size: 16px;
}

.is-good {
  color: #6ee7a8;
  background: rgba(38, 92, 72, 0.58);
}

.is-warn {
  color: #f1c65c;
  background: rgba(100, 80, 35, 0.55);
}

.is-down {
  color: #f36b77;
  background: rgba(106, 42, 54, 0.62);
}

.is-empty {
  color: #aeb8ca;
  background: rgba(82, 97, 124, 0.42);
}

.status-metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin: 26px 0;
}

.status-metrics div {
  min-width: 0;
  min-height: 118px;
  padding: 18px;
  border: 1px solid rgba(82, 97, 124, 0.45);
  border-radius: 8px;
  background: #121a2b;
}

.status-metrics span {
  display: block;
  color: #a3adbe;
  font-size: 13px;
  font-weight: 900;
}

.status-metrics strong {
  display: block;
  margin-top: 16px;
  color: #f6f8fb;
  font-size: 28px;
  font-weight: 950;
}

.status-metrics small {
  margin-left: 3px;
  color: #a3adbe;
  font-size: 14px;
}

.status-bars {
  padding-top: 24px;
  border-top: 1px solid rgba(82, 97, 124, 0.55);
}

.status-bars-head,
.bar-axis,
.status-summary {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  color: #9ca7ba;
  font-size: 13px;
  font-weight: 900;
}

.bar-row {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  min-height: 40px;
  margin-top: 16px;
}

.status-bar {
  position: relative;
  flex: 1 1 4px;
  min-width: 3px;
  max-width: 5px;
  height: 32px;
  border-radius: 999px;
  background: #51b879;
}

.status-bar.is-warn {
  background: #c59b3e;
}

.status-bar.is-down {
  background: #d94f5e;
}

.status-bar.is-empty {
  background: #52617c;
}

.status-tooltip {
  position: absolute;
  left: 50%;
  bottom: calc(100% + 16px);
  z-index: 3;
  display: none;
  width: min(330px, 80vw);
  padding: 20px;
  border: 1px solid rgba(99, 116, 145, 0.65);
  border-radius: 8px;
  background: #222c3d;
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.34);
  transform: translateX(-50%);
}

.status-tooltip::after {
  content: "";
  position: absolute;
  left: 50%;
  bottom: -8px;
  width: 16px;
  height: 16px;
  border-right: 1px solid rgba(99, 116, 145, 0.65);
  border-bottom: 1px solid rgba(99, 116, 145, 0.65);
  background: #222c3d;
  transform: translateX(-50%) rotate(45deg);
}

.status-bar:hover .status-tooltip {
  display: grid;
  gap: 10px;
}

.status-tooltip strong,
.status-tooltip b,
.status-tooltip em,
.status-tooltip small {
  position: relative;
  z-index: 1;
  font-style: normal;
}

.status-tooltip strong {
  color: #cbd5e1;
  font-size: 18px;
}

.status-tooltip b {
  color: #96d95a;
  font-size: 24px;
}

.status-tooltip em,
.status-tooltip small {
  color: #9ca7ba;
  font-size: 15px;
  font-weight: 800;
}

.bar-axis {
  margin-top: 8px;
  margin-bottom: 0;
}

.status-empty,
.status-summary {
  max-width: 1480px;
  margin: 28px auto 0;
  padding: 20px 24px;
  border: 1px solid rgba(82, 97, 124, 0.55);
  border-radius: 8px;
  background: #172033;
  color: #aeb8ca;
  font-weight: 900;
}

@media (max-width: 820px) {
  .status-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .status-actions {
    justify-content: flex-start;
  }

  .range-switch {
    width: 100%;
  }

  .range-switch button {
    flex: 1;
    min-width: 0;
  }

  .status-card {
    padding: 20px;
  }

  .status-card-head {
    grid-template-columns: 48px minmax(0, 1fr);
  }

  .status-card-head > strong {
    grid-column: 2;
    justify-self: start;
  }

  .status-metrics {
    grid-template-columns: 1fr;
  }
}
</style>
