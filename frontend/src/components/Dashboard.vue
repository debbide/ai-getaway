<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'

const props = defineProps({ plans: { type: Array, default: () => [] } })

const auth = useAuthStore()
const orders = ref([])
const keys = ref([])
const selectedPlan = ref('')
const pendingPlainKey = ref('')
const lastKeyMasked = ref('')
const error = ref('')
const notice = ref('')
const copyToast = ref('')
const copySuccessModalOpen = ref(false)
const modalError = ref('')
const loading = ref(false)
const orderPage = ref(1)
const orderPageSize = 5
const modal = reactive({ open: false, type: '', title: '', actionLabel: '', payload: null, danger: false })
const orderForm = reactive({ planId: '', order: null, paymentUrl: '', paymentOpened: false })
const keyForm = reactive({ name: 'Default' })

const totalOrderPages = computed(() => Math.max(1, Math.ceil(orders.value.length / orderPageSize)))
const pagedOrders = computed(() => {
  const page = Math.min(orderPage.value, totalOrderPages.value)
  const start = (page - 1) * orderPageSize
  return orders.value.slice(start, start + orderPageSize)
})

const hasActiveSubscription = computed(() => {
  const u = auth.user
  if (!u || u.status !== 'approved') return false
  if (!u.expires_at) return false
  return new Date(u.expires_at) > new Date()
})

const planPeriodStartIso = computed(() => {
  const u = auth.user
  if (!u || !hasActiveSubscription.value) return null
  if (u.subscription_started_at) return u.subscription_started_at
  if (!u.expires_at || !u.plan?.duration_days) return null
  const end = new Date(u.expires_at)
  const s = new Date(end.getTime())
  s.setDate(s.getDate() - Number(u.plan.duration_days))
  return s.toISOString()
})

const soloKey = computed(() => (keys.value.length ? keys.value[0] : null))
const hasApiKey = computed(() => Boolean(soloKey.value))

onMounted(loadAll)

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [orderRes, keyRes] = await Promise.all([api.get('/orders'), api.get('/keys')])
    orders.value = orderRes.data || []
    keys.value = keyRes.data || []
    if (orderPage.value > totalOrderPages.value) orderPage.value = totalOrderPages.value
    await auth.loadMe()
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

async function refreshDashboard() {
  notice.value = ''
  await loadAll()
}

function setOrderPage(page) {
  orderPage.value = Math.min(Math.max(1, page), totalOrderPages.value)
}

function openOrderModal(planId = selectedPlan.value) {
  if (hasActiveSubscription.value) {
    notice.value = '当前套餐仍在有效期内，请待到期后再购买'
    return
  }
  orderForm.planId = planId || ''
  orderForm.order = null
  orderForm.paymentUrl = ''
  orderForm.paymentOpened = false
  showModal('create-order', '创建订单', '确认下单')
}

function openPayModal(order) {
  orderForm.planId = String(order.PlanID || order.Plan?.ID || '')
  orderForm.order = order
  orderForm.paymentUrl = ''
  orderForm.paymentOpened = false
  showModal('pay-order', `支付订单 #${order.ID}`, '已完成支付')
}

function openKeyModal() {
  keyForm.name = 'Default'
  showModal('create-key', '创建 API Key', '创建密钥')
}

function openRotateModal() {
  keyForm.name = soloKey.value?.name || 'Default'
  showModal('rotate-key', '更新密钥', '确认替换', null, true)
}

function confirmDisableKey(key) {
  showModal('disable-key', '禁用 API Key', '确认禁用', { key }, true)
}

async function enableKey(k) {
  error.value = ''
  notice.value = ''
  try {
    await api.patch(`/keys/${k.id}/enable`)
    notice.value = 'API Key 已启用'
    await loadAll()
    window.dispatchEvent(new Event('app-data-updated'))
  } catch (err) {
    error.value = err.message
  }
}

async function createOrder() {
  if (!orderForm.planId) {
    modalError.value = '请选择套餐'
    return
  }
  modalError.value = ''
  try {
    const res = await api.post('/orders', { plan_id: Number(orderForm.planId) })
    orderForm.order = res.data.order
    orderForm.paymentUrl = ''
    orderForm.paymentOpened = false
    modal.type = 'pay-order'
    modal.title = `支付订单 #${orderForm.order.ID}`
    modal.actionLabel = '已完成支付'
    notice.value = res.data.reused ? '已为你找到未支付订单，请继续支付' : '订单已创建，请完成支付'
    await loadAll()
    window.dispatchEvent(new Event('app-data-updated'))
  } catch (err) {
    modalError.value = err.message
  }
}

async function startPayment() {
  if (!orderForm.order?.ID) return
  modalError.value = ''
  try {
    const res = await api.post(`/orders/${orderForm.order.ID}/pay`)
    orderForm.paymentUrl = res.data.payment_url
    orderForm.paymentOpened = true
    window.open(orderForm.paymentUrl, '_blank', 'noopener,noreferrer')
  } catch (err) {
    modalError.value = err.message
  }
}

async function markPaid() {
  if (!orderForm.order?.ID) return
  modalError.value = ''
  try {
    await api.patch(`/orders/${orderForm.order.ID}/paid`)
    notice.value = '支付已确认，订单已进入待审核'
    closeModal()
    await loadAll()
    window.dispatchEvent(new Event('app-data-updated'))
  } catch (err) {
    modalError.value = err.message
  }
}

async function createKey() {
  pendingPlainKey.value = ''
  lastKeyMasked.value = ''
  await runAction(async () => {
    const res = await api.post('/keys', { name: keyForm.name })
    pendingPlainKey.value = res.data.key
    lastKeyMasked.value = res.data.key_masked || ''
    notice.value = 'API Key 已创建，请尽快复制完整密钥保存（界面仅显示掩码）'
  })
}

async function rotateKey() {
  pendingPlainKey.value = ''
  lastKeyMasked.value = ''
  await runAction(async () => {
    const res = await api.post('/keys/rotate', { name: keyForm.name })
    pendingPlainKey.value = res.data.key
    lastKeyMasked.value = res.data.key_masked || ''
    notice.value = '密钥已更新，旧 Key 立即失效，请复制新密钥保存'
  })
}

async function disableKey() {
  await runAction(async () => {
    await api.patch(`/keys/${modal.payload.key.id}/disable`)
    notice.value = 'API Key 已禁用'
  })
}

async function runAction(action) {
  error.value = ''
  notice.value = ''
  modalError.value = ''
  try {
    await action()
    closeModal()
    await loadAll()
    window.dispatchEvent(new Event('app-data-updated'))
  } catch (err) {
    if (modal.open) {
      modalError.value = err.message
    } else {
      error.value = err.message
    }
  }
}

function showModal(type, title, actionLabel, payload = null, danger = false) {
  modalError.value = ''
  Object.assign(modal, { open: true, type, title, actionLabel, payload, danger })
}

function closeModal() {
  modalError.value = ''
  Object.assign(modal, { open: false, type: '', title: '', actionLabel: '', payload: null, danger: false })
}

function submitModal() {
  const actions = {
    'create-order': createOrder,
    'pay-order': markPaid,
    'create-key': createKey,
    'rotate-key': rotateKey,
    'disable-key': disableKey
  }
  actions[modal.type]?.()
}

function money(cents, currency = '￥') {
  return `${currency}${((cents || 0) / 100).toFixed(2)}`
}

function usd(cents) {
  return `$${((cents || 0) / 100).toFixed(2)}`
}

function pad2(n) {
  return String(n).padStart(2, '0')
}

function formatDateTime(value) {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '—'
  return `${d.getFullYear()}/${pad2(d.getMonth() + 1)}/${pad2(d.getDate())} ${pad2(d.getHours())}:${pad2(d.getMinutes())}`
}

async function copyKey(text, showSuccessModal = false) {
  copyToast.value = ''
  try {
    await navigator.clipboard.writeText(text)
    if (showSuccessModal) {
      copySuccessModalOpen.value = true
    } else {
      copyToast.value = '已复制'
      window.setTimeout(() => {
        copyToast.value = ''
      }, 2000)
    }
    if (pendingPlainKey.value && text === pendingPlainKey.value) {
      pendingPlainKey.value = ''
    }
  } catch {
    copyToast.value = '复制失败，请手动选择文本复制'
    window.setTimeout(() => {
      copyToast.value = ''
    }, 3000)
  }
}

async function copySecretFromServer() {
  copyToast.value = ''
  error.value = ''
  try {
    const res = await api.get('/keys/secret')
    await copyKey(res.data.key, true)
  } catch (err) {
    error.value = err.message
  }
}

function closeCopySuccessModal() {
  copySuccessModalOpen.value = false
}

function statusLabel(value) {
  return {
    pending_review: '待审核',
    pending_payment: '待支付',
    approved: '已通过',
    rejected: '已拒绝',
    active: '启用中',
    disabled: '已禁用',
    pending: '待审核'
  }[value] || value
}
</script>

<template>
  <section class="console-shell mx-auto max-w-7xl px-4 pb-12 sm:px-6">
    <div class="dashboard-hero">
      <div>
        <p class="section-kicker">User Console</p>
        <h2>控制台</h2>
        <p>账号状态：{{ statusLabel(auth.user?.status) }}</p>
      </div>
      <div class="usage-card">
        <span>账号</span>
        <strong>{{ auth.user?.email || '—' }}</strong>
        <small class="text-muted">登录后即可管理套餐与 API Key</small>
      </div>
    </div>

    <div v-if="error" class="alert alert-danger">{{ error }}</div>
    <div v-if="notice" class="alert alert-success">{{ notice }}</div>
    <div v-if="copyToast" class="alert alert-success">{{ copyToast }}</div>
    <div v-if="pendingPlainKey || lastKeyMasked" class="key-reveal">
      <span>密钥已就绪（下方仅掩码，完整内容请用按钮复制）</span>
      <code v-if="lastKeyMasked" class="api-key-code api-key-code--mask">{{ lastKeyMasked }}</code>
      <button v-if="pendingPlainKey" type="button" class="primary-button small" @click="copyKey(pendingPlainKey, true)">复制完整密钥</button>
    </div>

    <div class="console-stack">
      <div class="console-dashboard-grid">
        <div class="console-dashboard-main">
          <!-- 套餐购买 -->
          <section class="panel-surface dashboard-card p-5">
            <div class="section-head">
              <div>
                <p class="section-kicker">Pricing</p>
                <h3>选择套餐</h3>
              </div>
              <button class="primary-button" :disabled="hasActiveSubscription" @click="openOrderModal()">
                {{ hasActiveSubscription ? '等待当前套餐过期后购买' : '新建订单' }}
              </button>
            </div>

            <div class="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
              <article
                v-for="plan in props.plans"
                :key="plan.ID"
                class="select-plan-card"
                :class="{ active: selectedPlan === String(plan.ID), disabled: hasActiveSubscription }"
                @click="!hasActiveSubscription && (selectedPlan = String(plan.ID))"
              >
                <h4>{{ plan.Name }}</h4>
                <p>{{ plan.Description || '暂无说明' }}</p>
                <div>
                  <strong>{{ money(plan.PriceCents) }}</strong>
                  <span>{{ plan.DurationDays }} 天 · 周限额度 {{ usd(plan.SettlementUSDCents) }}</span>
                </div>
                <button
                  class="ghost-button small"
                  :disabled="hasActiveSubscription"
                  @click.stop="openOrderModal(String(plan.ID))"
                >
                  {{ hasActiveSubscription ? '等待当前套餐过期后购买' : '选择并下单' }}
                </button>
              </article>
            </div>
          </section>

          <!-- 订单 -->
          <section class="panel-surface dashboard-card p-5">
            <div class="section-head">
              <div>
                <p class="section-kicker">Orders</p>
                <h3>订单记录</h3>
              </div>
              <button class="icon-button refresh-button" type="button" :disabled="loading" aria-label="刷新" title="刷新" @click="refreshDashboard">↻</button>
            </div>

            <div class="mt-6 table-wrap">
              <table class="data-table">
                <thead>
                  <tr>
                    <th>订单</th>
                    <th>套餐</th>
                    <th>金额</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="order in pagedOrders" :key="order.ID">
                    <td>#{{ order.ID }}</td>
                    <td>{{ order.Plan?.Name || '-' }}</td>
                    <td>{{ money(order.AmountCents) }}</td>
                    <td><span class="status-badge">{{ statusLabel(order.Status) }}</span></td>
                    <td>
                      <button v-if="order.Status === 'pending_payment'" class="primary-button small" @click="openPayModal(order)">继续支付</button>
                      <span v-else class="text-muted">-</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div class="pagination-bar">
              <span>共 {{ orders.length }} 个订单，第 {{ Math.min(orderPage, totalOrderPages) }} / {{ totalOrderPages }} 页</span>
              <div class="table-actions">
                <button class="ghost-button small" :disabled="orderPage <= 1" @click="setOrderPage(orderPage - 1)">上一页</button>
                <button class="ghost-button small" :disabled="orderPage >= totalOrderPages" @click="setOrderPage(orderPage + 1)">下一页</button>
              </div>
            </div>
          </section>
        </div>

        <aside class="console-dashboard-aside">
          <!-- 套餐管理：侧栏紧凑区 -->
          <section class="panel-surface dashboard-card p-4">
            <div class="section-head">
              <div>
                <p class="section-kicker">Plan</p>
                <h3>套餐管理</h3>
                <p class="section-subtitle text-muted">订阅周期与额度</p>
              </div>
              <button class="icon-button refresh-button" type="button" :disabled="loading" aria-label="刷新" title="刷新" @click="refreshDashboard">↻</button>
            </div>

            <div v-if="hasActiveSubscription" class="plan-snapshot-card">
              <div class="plan-snapshot-row">
                <div class="plan-snapshot-icon" aria-hidden="true">💳</div>
                <div class="plan-snapshot-primary">
                  <div class="plan-snapshot-title-row">
                    <strong>{{ auth.user?.plan?.name || '当前套餐' }}</strong>
                    <span class="badge-active">活跃</span>
                  </div>
                  <p class="plan-snapshot-quota text-muted">
                    每周额度：{{ usd(auth.user?.plan?.settlement_usd_cents || 0) }}/周
                  </p>
                </div>
                <div class="plan-snapshot-times">
                  <div class="plan-snapshot-timecell">
                    <span class="detail-label text-muted">套餐开始</span>
                    <span class="detail-value">{{ formatDateTime(planPeriodStartIso) }}</span>
                  </div>
                  <div class="plan-snapshot-timecell">
                    <span class="detail-label text-muted">套餐结束</span>
                    <span class="detail-value">{{ formatDateTime(auth.user?.expires_at) }}</span>
                  </div>
                </div>
              </div>
            </div>

            <div v-else class="plan-snapshot-card plan-snapshot-card--empty">
              <div class="plan-snapshot-row">
                <div class="plan-snapshot-icon plan-snapshot-icon--dim" aria-hidden="true">📋</div>
                <div class="plan-snapshot-primary">
                  <div class="plan-snapshot-title-row">
                    <strong>暂无生效套餐</strong>
                  </div>
                  <p class="text-muted plan-snapshot-empty-desc">支付并审核通过后，此处显示套餐信息与周期。</p>
                </div>
              </div>
            </div>
          </section>

          <!-- API Key -->
          <section class="panel-surface dashboard-card p-4">
            <div class="section-head">
              <div>
                <p class="section-kicker">Keys</p>
                <h3>API 密钥管理</h3>
              </div>
              <div class="toolbar-actions">
                <button class="icon-button refresh-button" type="button" :disabled="loading" aria-label="刷新" title="刷新" @click="refreshDashboard">↻</button>
                <button v-if="!hasApiKey" class="primary-button" @click="openKeyModal">创建 Key</button>
              </div>
            </div>

            <div class="notice-card notice-warn mt-3">
              <strong>安全提示</strong>
              <span>每个账号仅保留一条 API Key。列表中只显示掩码，复制时会从服务端安全取出完整密钥。更新密钥将删除旧密钥并立即生效。</span>
            </div>

            <div v-if="!hasApiKey" class="notice-card api-key-empty-panel mt-4">
              <strong>尚未创建 API Key</strong>
              <span class="text-muted">通过审核并绑定上游后，点击右上角「创建 Key」生成密钥。</span>
            </div>

            <div v-else class="mt-4">
              <article class="api-key-block">
                <div class="api-key-block-head">
                  <div>
                    <strong>{{ soloKey.name }}</strong>
                    <span class="text-muted">{{ statusLabel(soloKey.status) }}</span>
                  </div>
                  <div class="api-key-head-actions">
                    <button
                      v-if="soloKey.status === 'disabled'"
                      type="button"
                      class="ghost-button small"
                      @click="enableKey(soloKey)"
                    >
                      启用
                    </button>
                    <button
                      v-else
                      type="button"
                      class="danger-button small"
                      @click="confirmDisableKey(soloKey)"
                    >
                      禁用
                    </button>
                  </div>
                </div>
                <div class="api-key-strip">
                  <code class="api-key-code api-key-code--mask">{{ soloKey.key_masked || soloKey.key_prefix + '···' }}</code>
                  <div class="api-key-strip-actions">
                    <button
                      type="button"
                      class="ghost-button small"
                      :disabled="!soloKey.can_copy"
                      @click="copySecretFromServer"
                    >
                      复制完整密钥
                    </button>
                    <button type="button" class="ghost-button small" @click="openRotateModal">更新密钥</button>
                  </div>
                </div>
                <p v-if="!soloKey.can_copy" class="api-key-legacy-hint text-muted">该密钥无法在线解密，请点击「更新密钥」重新生成后即可复制。</p>
              </article>
            </div>
          </section>
        </aside>
      </div>
    </div>

    <div v-if="modal.open" class="modal-backdrop" @click.self="closeModal">
      <form class="modal-card" @submit.prevent="submitModal">
        <div class="modal-head">
          <h3>{{ modal.title }}</h3>
          <button type="button" class="icon-button" @click="closeModal">×</button>
        </div>

        <div v-if="modal.type === 'create-order'" class="modal-body form-grid">
          <label class="field md:col-span-2">
            <span>选择套餐</span>
            <select v-model="orderForm.planId" required>
              <option value="">请选择套餐</option>
              <option v-for="plan in props.plans" :key="plan.ID" :value="plan.ID">
                {{ plan.Name }} / {{ money(plan.PriceCents) }} / {{ plan.DurationDays }} 天
              </option>
            </select>
          </label>
          <div class="order-flow-note md:col-span-2">
            <strong>下单后会先创建待支付订单</strong>
            <span>下一步打开支付窗口。支付完成后回到本页面点击“已完成支付”，系统会核验支付结果。</span>
          </div>
        </div>

        <div v-if="modal.type === 'pay-order'" class="modal-body">
          <div class="payment-panel">
            <strong>{{ orderForm.order?.Plan?.Name || '套餐订单' }}</strong>
            <span>订单金额：{{ money(orderForm.order?.AmountCents) }}</span>
            <p>请先点击“去支付”打开支付页面。完成支付后回到这里点击“已完成支付”，系统确认支付成功后才会进入待审核。</p>
            <button type="button" class="primary-button" @click="startPayment">
              {{ orderForm.paymentOpened ? '重新打开支付页面' : '去支付' }}
            </button>
          </div>
        </div>

        <div v-if="modal.type === 'create-key' || modal.type === 'rotate-key'" class="modal-body">
          <div v-if="modal.type === 'rotate-key'" class="order-flow-note md:col-span-2">
            <strong>将替换当前唯一密钥</strong>
            <span>确认后旧密钥立即失效，所有使用旧 Key 的客户端需同步更新。</span>
          </div>
          <label class="field">
            <span>Key 名称</span>
            <input v-model="keyForm.name" required minlength="2" placeholder="生产环境 Key" />
          </label>
        </div>

        <div v-if="modal.type === 'disable-key'" class="modal-body confirm-copy">
          <strong>确定禁用「{{ modal.payload?.key?.name }}」吗？</strong>
          <p>禁用后该 Key 将不能继续调用网关接口。</p>
        </div>

        <div v-if="modalError" class="modal-inline-error">
          <strong>操作未完成</strong>
          <span>{{ modalError }}</span>
        </div>

        <div class="modal-actions">
          <button type="button" class="ghost-button" @click="closeModal">取消</button>
          <button :class="modal.danger ? 'danger-solid-button' : 'primary-button'">{{ modal.actionLabel }}</button>
        </div>
      </form>
    </div>

    <div v-if="copySuccessModalOpen" class="modal-backdrop" @click.self="closeCopySuccessModal">
      <div class="modal-card" role="dialog" aria-labelledby="copy-success-title">
        <div class="modal-head">
          <h3 id="copy-success-title">复制成功</h3>
          <button type="button" class="icon-button" aria-label="关闭" @click="closeCopySuccessModal">×</button>
        </div>
        <div class="modal-body confirm-copy">
          <p>完整密钥已复制到剪贴板。请粘贴到安全环境保存，勿发送给他人或提交到公开仓库。</p>
        </div>
        <div class="modal-actions">
          <button type="button" class="primary-button" @click="closeCopySuccessModal">知道了</button>
        </div>
      </div>
    </div>
  </section>
</template>
