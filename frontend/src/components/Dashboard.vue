<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'

const props = defineProps({ plans: { type: Array, default: () => [] } })

const auth = useAuthStore()
const orders = ref([])
const keys = ref([])
const selectedPlan = ref('')
const newKey = ref('')
const error = ref('')
const notice = ref('')
const modalError = ref('')
const modal = reactive({ open: false, type: '', title: '', actionLabel: '', payload: null, danger: false })
const orderForm = reactive({ planId: '', order: null, paymentUrl: '', paymentOpened: false })
const keyForm = reactive({ name: 'Default' })

onMounted(loadAll)

async function loadAll() {
  try {
    const [orderRes, keyRes] = await Promise.all([api.get('/orders'), api.get('/keys')])
    orders.value = orderRes.data || []
    keys.value = keyRes.data || []
  } catch (err) {
    error.value = err.message
  }
}

function openOrderModal(planId = selectedPlan.value) {
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

function confirmDisableKey(key) {
  showModal('disable-key', '禁用 API Key', '确认禁用', { key }, true)
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
    notice.value = '已提交支付完成确认，请等待管理员审核'
    closeModal()
    await loadAll()
    window.dispatchEvent(new Event('app-data-updated'))
  } catch (err) {
    modalError.value = err.message
  }
}

async function createKey() {
  newKey.value = ''
  await runAction(async () => {
    const res = await api.post('/keys', { name: keyForm.name })
    newKey.value = res.data.key
    notice.value = 'API Key 已创建，请立即保存'
  })
}

async function disableKey() {
  await runAction(async () => {
    await api.patch(`/keys/${modal.payload.key.ID}/disable`)
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

function formatDate(value) {
  if (!value) return '待开通'
  return new Date(value).toLocaleDateString('zh-CN')
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
        <span>订阅状态</span>
        <strong>{{ statusLabel(auth.user?.status) }}</strong>
        <small>到期时间：{{ formatDate(auth.user?.expires_at) }}</small>
      </div>
    </div>

    <div v-if="error" class="alert alert-danger">{{ error }}</div>
    <div v-if="notice" class="alert alert-success">{{ notice }}</div>
    <div v-if="newKey" class="key-reveal">
      <span>新 API Key</span>
      <code>{{ newKey }}</code>
    </div>

    <div class="grid gap-5 lg:grid-cols-[1.05fr_0.95fr]">
      <section class="panel-surface p-5">
        <div class="section-head">
          <div>
            <p class="section-kicker">Orders</p>
            <h3>套餐与订单</h3>
          </div>
          <button class="primary-button" @click="openOrderModal()">新建订单</button>
        </div>

        <div class="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          <article v-for="plan in props.plans" :key="plan.ID" class="select-plan-card" :class="{ active: selectedPlan === String(plan.ID) }" @click="selectedPlan = String(plan.ID)">
            <h4>{{ plan.Name }}</h4>
            <p>{{ plan.Description || '暂无说明' }}</p>
            <div>
              <strong>{{ money(plan.PriceCents) }}</strong>
              <span>{{ plan.DurationDays }} 天 · 周限额度 {{ usd(plan.SettlementUSDCents) }}</span>
            </div>
            <button class="ghost-button small" @click.stop="openOrderModal(String(plan.ID))">选择并下单</button>
          </article>
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
              <tr v-for="order in orders" :key="order.ID">
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
      </section>

      <section class="panel-surface p-5">
        <div class="section-head">
          <div>
            <p class="section-kicker">Keys</p>
            <h3>API Key 管理</h3>
          </div>
          <button class="primary-button" @click="openKeyModal">创建 Key</button>
        </div>

        <div class="mt-5 grid gap-3">
          <article v-for="key in keys" :key="key.ID" class="key-card">
            <div>
              <strong>{{ key.Name }}</strong>
              <span>{{ key.KeyPrefix }} · {{ statusLabel(key.Status) }}</span>
            </div>
            <button class="danger-button small" :disabled="key.Status === 'disabled'" @click="confirmDisableKey(key)">禁用</button>
          </article>
        </div>
      </section>
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
            <span>下一步打开支付窗口。支付完成后回到本页面点击“已完成支付”，订单才会进入待审核。</span>
          </div>
        </div>

        <div v-if="modal.type === 'pay-order'" class="modal-body">
          <div class="payment-panel">
            <strong>{{ orderForm.order?.Plan?.Name || '套餐订单' }}</strong>
            <span>订单金额：{{ money(orderForm.order?.AmountCents) }}</span>
            <p>请先点击“去支付”打开支付页面。完成支付后回到这里点击“已完成支付”，请勿重复支付同一订单。</p>
            <button type="button" class="primary-button" @click="startPayment">
              {{ orderForm.paymentOpened ? '重新打开支付页面' : '去支付' }}
            </button>
          </div>
        </div>

        <div v-if="modal.type === 'create-key'" class="modal-body">
          <label class="field">
            <span>Key 名称</span>
            <input v-model="keyForm.name" required minlength="2" placeholder="生产环境 Key" />
          </label>
        </div>

        <div v-if="modal.type === 'disable-key'" class="modal-body confirm-copy">
          <strong>确定禁用「{{ modal.payload?.key?.Name }}」吗？</strong>
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
  </section>
</template>
