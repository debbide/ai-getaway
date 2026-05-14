<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api/client'

const menu = [
  { key: 'overview', label: '总览', hint: '运营数据' },
  { key: 'plans', label: '套餐管理', hint: '价格与额度' },
  { key: 'orders', label: '审核管理', hint: '订单开通' },
  { key: 'models', label: '模型管理', hint: '计费倍率' },
  { key: 'channels', label: '渠道管理', hint: '上游接口' },
  { key: 'users', label: '用户管理', hint: '账号与权限' },
  { key: 'docs', label: '配置文档', hint: 'Markdown 内容' },
  { key: 'navigation', label: '导航菜单', hint: '顶部菜单' },
  { key: 'settings', label: '系统设置', hint: '邮件与支付' }
]

const statusOptions = [
  { value: 'pending', label: '待审核' },
  { value: 'approved', label: '已通过' },
  { value: 'disabled', label: '已禁用' }
]

const roleOptions = [
  { value: 'user', label: '普通用户' },
  { value: 'admin', label: '管理员' }
]

const defaultNavigation = [
  { label: '首页', path: '/' },
  { label: '教程 ↗', path: '/docs' },
  { label: '定价', path: '/plans' },
  { label: '模型', path: '/models' },
  { label: '常见问题', path: '/faq' }
]

const orderStatusMap = {
  pending_payment: '待支付',
  pending_review: '待审核',
  approved: '已通过',
  rejected: '已拒绝'
}

const active = ref('overview')
const settingsTab = ref('basic')
const stats = ref({})
const orders = ref([])
const users = ref([])
const plans = ref([])
const models = ref([])
const modelSource = ref('')
const channels = ref([])
const docs = ref([])
const error = ref('')
const notice = ref('')
const navDraft = ref([])
const loading = ref(false)
const modal = reactive({ open: false, type: '', title: '', actionLabel: '', danger: false, payload: null })
const approve = reactive({ orderId: '', channelId: '', channel: '', baseUrl: '', apiKey: '', adminNote: '', planId: '', amountCents: 0, status: '' })
const rejectForm = reactive({ orderId: '', adminNote: '' })
const planForm = reactive(emptyPlan())
const modelForm = reactive(emptyModel())
const channelForm = reactive(emptyChannel())
const userForm = reactive(emptyUser())
const docForm = reactive(emptyDoc())
const settings = reactive({
  site_title: '',
  api_endpoint: '',
  tutorial_video_url: '',
  navigation_items: '',
  pricing_title: '',
  pricing_subtitle: '',
  pricing_notice: '',
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
  epay_submit_url: '',
  smtp_password_configured: false,
  epay_key_configured: false
})

const pendingOrders = computed(() => orders.value.filter((order) => order.Status === 'pending_review').length)
const enabledPlans = computed(() => plans.value.filter((plan) => plan.Enabled).length)
const enabledModels = computed(() => models.value.filter((item) => item.Status === 'active').length)
const approvedUsers = computed(() => users.value.filter((user) => user.Status === 'approved').length)
const enabledChannels = computed(() => channels.value.filter((channel) => channel.Enabled).length)
const enabledDocs = computed(() => docs.value.filter((doc) => doc.Enabled).length)

onMounted(loadAll)

function emptyPlan() {
  return {
    id: null,
    name: '',
    code: '',
    badge_text: '',
    plan_type: 'subscription',
    quota_period: 'weekly',
    price_rmb: 9.9,
    period_usd_quota: 20,
    price_cents: 990,
    settlement_usd_cents: 2000,
    quota_tokens: 200000,
    daily_quota_tokens: 200000,
    weekly_quota_tokens: 0,
    duration_days: 30,
    description: '',
    enabled: true
  }
}

function emptyUser() {
  return {
    id: null,
    username: '',
    email: '',
    password: '',
    role: 'user',
    status: 'pending',
    email_verified: true,
    plan_id: '',
    quota_tokens: 0,
    used_tokens: 0
  }
}

function emptyChannel() {
  return {
    id: null,
    name: '',
    base_url: '',
    enabled: true
  }
}

function emptyModel() {
  return {
    id: null,
    model: '',
    display_name: '',
    provider: 'openai',
    input_usd_per_million: 0,
    cached_input_usd_per_million: 0,
    output_usd_per_million: 0,
    billing_multiplier: 1,
    status: 'active',
    notes: ''
  }
}

function emptyDoc() {
  return {
    id: null,
    title: '',
    slug: '',
    group_name: '快速开始',
    description: '',
    content: '',
    sort_order: 100,
    enabled: true
  }
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [statsRes, ordersRes, usersRes, plansRes, modelsRes, channelsRes, docsRes, settingsRes] = await Promise.all([
      api.get('/admin/stats'),
      api.get('/admin/orders'),
      api.get('/admin/users'),
      api.get('/admin/plans'),
      api.get('/admin/models'),
      api.get('/admin/upstream-channels'),
      api.get('/admin/docs'),
      api.get('/admin/settings')
    ])
    stats.value = statsRes.data || {}
    orders.value = ordersRes.data || []
    users.value = usersRes.data || []
    plans.value = plansRes.data || []
    models.value = modelsRes.data?.items || []
    modelSource.value = modelsRes.data?.official_source || ''
    channels.value = channelsRes.data || []
    docs.value = docsRes.data || []
    Object.assign(settings, settingsRes.data, { smtp_password: '', epay_key: '' })
    setNavigationDraft(settings.navigation_items)
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

async function refreshAdminData() {
  notice.value = ''
  await loadAll()
}

function openPlanModal(plan = null) {
  Object.assign(planForm, emptyPlan())
  if (plan) {
    Object.assign(planForm, {
      id: plan.ID,
      name: plan.Name,
      code: plan.Code,
      badge_text: plan.BadgeText || '',
      plan_type: plan.PlanType || 'subscription',
      quota_period: plan.QuotaPeriod || 'weekly',
      price_rmb: centsToAmount(plan.PriceCents),
      period_usd_quota: centsToAmount(plan.SettlementUSDCents),
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
  showModal(plan ? 'edit-plan' : 'create-plan', plan ? '编辑套餐' : '新增套餐', plan ? '保存修改' : '创建套餐')
}

async function submitPlan() {
  const payload = normalizePlan(planForm)
  await runAction(async () => {
    if (planForm.id) {
      await api.put(`/admin/plans/${planForm.id}`, payload)
      notice.value = '套餐已更新'
    } else {
      await api.post('/admin/plans', payload)
      notice.value = '套餐已创建'
    }
  })
}

function confirmDeletePlan(plan) {
  showModal('delete-plan', '删除套餐', '确认删除', { plan }, true)
}

async function deletePlan() {
  await runAction(async () => {
    await api.delete(`/admin/plans/${modal.payload.plan.ID}`)
    notice.value = '套餐已删除'
  })
}

function openModelModal(model = null) {
  Object.assign(modelForm, emptyModel())
  if (model) {
    Object.assign(modelForm, {
      id: model.ID,
      model: model.ModelName || model.model || '',
      display_name: model.DisplayName || '',
      provider: model.Provider || 'openai',
      input_usd_per_million: model.InputUSDPerMillion || 0,
      cached_input_usd_per_million: model.CachedInputUSDPerMillion || 0,
      output_usd_per_million: model.OutputUSDPerMillion || 0,
      billing_multiplier: model.BillingMultiplier || 1,
      status: model.Status || 'active',
      notes: model.Notes || ''
    })
  }
  showModal(model ? 'edit-model' : 'create-model', model ? '编辑模型计费' : '新增模型计费', model ? '保存修改' : '创建模型')
}

async function submitModel() {
  const payload = normalizeModel(modelForm)
  await runAction(async () => {
    if (modelForm.id) {
      await api.put(`/admin/models/${modelForm.id}`, payload)
      notice.value = '模型计费已更新'
    } else {
      await api.post('/admin/models', payload)
      notice.value = '模型计费已创建'
    }
  })
}

function confirmDeleteModel(model) {
  showModal('delete-model', '删除模型计费', '确认删除', { model }, true)
}

async function deleteModel() {
  await runAction(async () => {
    await api.delete(`/admin/models/${modal.payload.model.ID}`)
    notice.value = '模型计费已删除'
  })
}

async function syncOfficialModels() {
  await runAction(async () => {
    const res = await api.post('/admin/models/sync-official')
    notice.value = `已同步 ${res.data?.synced || 0} 个官方模型价格`
  }, false)
}

function openChannelModal(channel = null) {
  Object.assign(channelForm, emptyChannel())
  if (channel) {
    Object.assign(channelForm, {
      id: channel.ID,
      name: channel.Name,
      base_url: channel.BaseURL,
      enabled: channel.Enabled
    })
  }
  showModal(channel ? 'edit-channel' : 'create-channel', channel ? '编辑渠道' : '新增渠道', channel ? '保存修改' : '创建渠道')
}

function openDocModal(doc = null) {
  Object.assign(docForm, emptyDoc())
  if (doc) {
    Object.assign(docForm, {
      id: doc.ID,
      title: doc.Title,
      slug: doc.Slug,
      group_name: doc.GroupName || '快速开始',
      description: doc.Description || '',
      content: doc.Content || '',
      sort_order: doc.SortOrder || 0,
      enabled: doc.Enabled
    })
  }
  showModal(doc ? 'edit-doc' : 'create-doc', doc ? '编辑配置文档' : '新增配置文档', doc ? '保存修改' : '创建文档')
}

async function submitDoc() {
  const payload = normalizeDoc(docForm)
  await runAction(async () => {
    if (docForm.id) {
      await api.put(`/admin/docs/${docForm.id}`, payload)
      notice.value = '文档已更新'
    } else {
      await api.post('/admin/docs', payload)
      notice.value = '文档已创建'
    }
  })
}

function confirmDeleteDoc(doc) {
  showModal('delete-doc', '删除配置文档', '确认删除', { doc }, true)
}

async function deleteDoc() {
  await runAction(async () => {
    await api.delete(`/admin/docs/${modal.payload.doc.ID}`)
    notice.value = '文档已删除'
  })
}

async function submitChannel() {
  const payload = normalizeChannel(channelForm)
  await runAction(async () => {
    if (channelForm.id) {
      await api.put(`/admin/upstream-channels/${channelForm.id}`, payload)
      notice.value = '渠道已更新'
    } else {
      await api.post('/admin/upstream-channels', payload)
      notice.value = '渠道已创建'
    }
  })
}

function confirmDeleteChannel(channel) {
  showModal('delete-channel', '删除渠道', '确认删除', { channel }, true)
}

async function deleteChannel() {
  await runAction(async () => {
    await api.delete(`/admin/upstream-channels/${modal.payload.channel.ID}`)
    notice.value = '渠道已删除'
  })
}

function openUserModal(user = null) {
  Object.assign(userForm, emptyUser())
  if (user) {
    Object.assign(userForm, {
      id: user.ID,
      username: user.Username,
      email: user.Email,
      password: '',
      role: user.Role || 'user',
      status: user.Status || 'pending',
      email_verified: Boolean(user.EmailVerified),
      plan_id: user.PlanID || '',
      quota_tokens: user.QuotaTokens || 0,
      used_tokens: user.UsedTokens || 0
    })
  }
  showModal(user ? 'edit-user' : 'create-user', user ? '编辑用户' : '新增用户', user ? '保存修改' : '创建用户')
}

async function submitUser() {
  const payload = normalizeUser(userForm)
  await runAction(async () => {
    if (userForm.id) {
      await api.patch(`/admin/users/${userForm.id}`, payload)
      notice.value = '用户已更新'
    } else {
      await api.post('/admin/users', payload)
      notice.value = '用户已创建'
    }
  })
}

function confirmDeleteUser(user) {
  showModal('delete-user', '删除用户', '确认删除', { user }, true)
}

async function deleteUser() {
  await runAction(async () => {
    await api.delete(`/admin/users/${modal.payload.user.ID}`)
    notice.value = '用户已删除'
  })
}

function openApproveModal(order) {
  const channel = channels.value.find((item) => item.Enabled) || null
  Object.assign(approve, {
    orderId: String(order.ID),
    channelId: channel?.ID || '',
    channel: channel?.Name || '',
    baseUrl: channel?.BaseURL || '',
    apiKey: '',
    adminNote: '',
    planId: order.PlanID || order.Plan?.ID || '',
    amountCents: order.AmountCents || 0,
    status: order.Status
  })
  showModal('approve-order', `审核订单 #${order.ID}`, '通过并开通')
}

function openEditOrderModal(order) {
  const upstream = order.Upstream || {}
  const channel = channels.value.find((item) => item.Name === upstream.Channel) || null
  Object.assign(approve, {
    orderId: String(order.ID),
    channelId: channel?.ID || '',
    channel: upstream.Channel || '',
    baseUrl: upstream.BaseURL || '',
    apiKey: '',
    adminNote: order.AdminNote || '',
    planId: order.PlanID || order.Plan?.ID || '',
    amountCents: order.AmountCents || 0,
    status: order.Status
  })
  showModal('edit-order', `编辑订单 #${order.ID}`, '保存修改')
}

function openRejectModal(order) {
  Object.assign(rejectForm, { orderId: String(order.ID), adminNote: '' })
  showModal('reject-order', `拒绝订单 #${order.ID}`, '确认拒绝', null, true)
}

function selectedApproveChannel() {
  return channels.value.find((channel) => String(channel.ID) === String(approve.channelId)) || null
}

function syncApproveChannel() {
  const channel = selectedApproveChannel()
  approve.channel = channel?.Name || ''
  approve.baseUrl = channel?.BaseURL || ''
}

async function approveOrder() {
  syncApproveChannel()
  await runAction(async () => {
    await api.post(`/admin/orders/${approve.orderId}/approve`, {
      channel_id: Number(approve.channelId) || undefined,
      channel: approve.channel,
      base_url: approve.baseUrl,
      api_key: approve.apiKey,
      admin_note: approve.adminNote
    })
    notice.value = '订单已审核通过'
  })
}

async function editOrder() {
  syncApproveChannel()
  const payload = {
    channel_id: Number(approve.channelId) || undefined,
    channel: approve.channel,
    base_url: approve.baseUrl,
    api_key: approve.apiKey,
    admin_note: approve.adminNote,
    amount_cents: Number(approve.amountCents) || undefined
  }
  if (approve.status !== 'approved') {
    payload.plan_id = Number(approve.planId) || undefined
  }
  await runAction(async () => {
    await api.put(`/admin/orders/${approve.orderId}`, payload)
    notice.value = '订单已保存'
  })
}

async function rejectOrder() {
  await runAction(async () => {
    await api.post(`/admin/orders/${rejectForm.orderId}/reject`, { admin_note: rejectForm.adminNote })
    notice.value = '订单已拒绝'
  })
}

async function saveSettings() {
  await runAction(async () => {
    await api.put('/admin/settings', {
      ...settings,
      smtp_port: Number(settings.smtp_port || 587)
    })
    settings.smtp_password = ''
    settings.epay_key = ''
    notice.value = '系统设置已保存'
  }, false)
}

async function saveNavigation() {
  syncNavigationSetting()
  await runAction(async () => {
    await api.put('/admin/settings', {
      ...settings,
      smtp_port: Number(settings.smtp_port || 587)
    })
    notice.value = '导航菜单已保存'
  }, false)
}

function createNavItem(overrides = {}) {
  return {
    label: '',
    path: '/',
    external: false,
    children: [],
    ...overrides
  }
}

function setNavigationDraft(value) {
  navDraft.value = parseNavigation(value).map((item) => ({
    ...createNavItem(item),
    children: (item.children || []).map((child) => createNavItem(child))
  }))
  syncNavigationSetting()
}

function parseNavigation(value) {
  try {
    const parsed = JSON.parse(value || '[]')
    return Array.isArray(parsed) && parsed.length ? parsed : cloneDefaultNavigation()
  } catch {
    return cloneDefaultNavigation()
  }
}

function cloneDefaultNavigation() {
  return JSON.parse(JSON.stringify(defaultNavigation))
}

function normalizeNavigation(items) {
  return items
    .map((item) => ({
      label: String(item.label || '').trim(),
      path: String(item.path || '#').trim() || '#',
      external: Boolean(item.external),
      children: (item.children || [])
        .map((child) => ({
          label: String(child.label || '').trim(),
          path: String(child.path || '#').trim() || '#',
          external: Boolean(child.external)
        }))
        .filter((child) => child.label)
    }))
    .filter((item) => item.label)
}

function syncNavigationSetting() {
  const normalized = normalizeNavigation(navDraft.value)
  settings.navigation_items = JSON.stringify(normalized.length ? normalized : cloneDefaultNavigation())
}

function addNavItem() {
  navDraft.value.push(createNavItem({ label: '新菜单', path: '/' }))
  syncNavigationSetting()
}

function addChildNavItem(index) {
  navDraft.value[index].children = navDraft.value[index].children || []
  navDraft.value[index].children.push(createNavItem({ label: '子菜单', path: '/' }))
  syncNavigationSetting()
}

function removeNavItem(index, childIndex = null) {
  if (childIndex === null) {
    navDraft.value.splice(index, 1)
  } else {
    navDraft.value[index].children.splice(childIndex, 1)
  }
  syncNavigationSetting()
}

function moveNavItem(index, direction) {
  const target = index + direction
  if (target < 0 || target >= navDraft.value.length) return
  const items = navDraft.value
  const [item] = items.splice(index, 1)
  items.splice(target, 0, item)
  syncNavigationSetting()
}

function resetNavigationDefault() {
  navDraft.value = cloneDefaultNavigation().map((item) => ({
    ...createNavItem(item),
    children: (item.children || []).map((child) => createNavItem(child))
  }))
  syncNavigationSetting()
}

async function runAction(action, close = true) {
  error.value = ''
  notice.value = ''
  try {
    await action()
    if (close) closeModal()
    await loadAll()
    window.dispatchEvent(new Event('app-data-updated'))
  } catch (err) {
    error.value = err.message
  }
}

function showModal(type, title, actionLabel, payload = null, danger = false) {
  Object.assign(modal, { open: true, type, title, actionLabel, payload, danger })
}

function closeModal() {
  Object.assign(modal, { open: false, type: '', title: '', actionLabel: '', payload: null, danger: false })
}

function normalizePlan(plan) {
  return {
    name: plan.name.trim(),
    code: plan.code.trim(),
    badge_text: plan.badge_text.trim(),
    plan_type: plan.plan_type,
    quota_period: plan.quota_period,
    price_cents: amountToCents(plan.price_rmb),
    settlement_usd_cents: amountToCents(plan.period_usd_quota),
    quota_tokens: 0,
    daily_quota_tokens: 0,
    weekly_quota_tokens: 0,
    duration_days: Number(plan.duration_days || 1),
    description: plan.description.trim(),
    enabled: Boolean(plan.enabled)
  }
}

function normalizeModel(item) {
  return {
    model: item.model.trim(),
    display_name: item.display_name.trim(),
    provider: item.provider.trim() || 'openai',
    input_usd_per_million: Number(item.input_usd_per_million || 0),
    cached_input_usd_per_million: Number(item.cached_input_usd_per_million || 0),
    output_usd_per_million: Number(item.output_usd_per_million || 0),
    billing_multiplier: Number(item.billing_multiplier || 1),
    status: item.status === 'disabled' ? 'disabled' : 'active',
    notes: item.notes.trim()
  }
}

function modelUnit(value) {
  return `$${Number(value || 0).toFixed(4)} / 1M Token`
}

function modelActualUnit(item, field) {
  return modelUnit((item[field] || 0) * (item.BillingMultiplier || 1))
}

function modelStatusLabel(value) {
  return value === 'disabled' ? '已停用' : '已启用'
}

function formatSyncTime(value) {
  if (!value) return '未同步'
  return formatDate(value)
}

function formatDate(value) {
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '-'
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}/${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function normalizeChannel(channel) {
  return {
    name: channel.name.trim(),
    base_url: channel.base_url.trim(),
    enabled: Boolean(channel.enabled)
  }
}

function normalizeDoc(doc) {
  return {
    title: doc.title.trim(),
    slug: doc.slug.trim(),
    group_name: doc.group_name.trim(),
    description: doc.description.trim(),
    content: doc.content,
    sort_order: Number(doc.sort_order || 0),
    enabled: Boolean(doc.enabled)
  }
}

function normalizeUser(user) {
  const payload = {
    username: user.username.trim(),
    email: user.email.trim(),
    role: user.role,
    status: user.status,
    email_verified: Boolean(user.email_verified),
    plan_id: user.plan_id === '' || user.plan_id === null ? null : Number(user.plan_id)
  }
  if (user.password) payload.password = user.password
  return payload
}

function money(cents, currency = '￥') {
  return `${currency}${((cents || 0) / 100).toFixed(2)}`
}

function amountToCents(value) {
  return Math.round(Number(value || 0) * 100)
}

function centsToAmount(value) {
  return Number(((value || 0) / 100).toFixed(2))
}

function rmb(value) {
  return `￥${((value || 0) / 100).toFixed(2)}`
}

function usd(value) {
  return `$${((value || 0) / 100).toFixed(2)}`
}

function quotaPeriodLabel(period) {
  return period === 'daily' ? '每日' : '每周'
}

function planWeeks(plan) {
  return Math.max(1, Math.round((plan.DurationDays || 30) / 7))
}

function totalUsd(plan) {
  const units = plan.QuotaPeriod === 'daily' ? (plan.DurationDays || 1) : planWeeks(plan)
  return `$${(((plan.SettlementUSDCents || 0) / 100) * units).toFixed(0)}`
}

function compactNumber(value) {
  return Number(value || 0).toLocaleString()
}

function roleLabel(value) {
  return roleOptions.find((item) => item.value === value)?.label || value
}

function statusLabel(value) {
  return statusOptions.find((item) => item.value === value)?.label || orderStatusMap[value] || value
}

function submitModal() {
  const actions = {
    'create-plan': submitPlan,
    'edit-plan': submitPlan,
    'delete-plan': deletePlan,
    'create-model': submitModel,
    'edit-model': submitModel,
    'delete-model': deleteModel,
    'create-channel': submitChannel,
    'edit-channel': submitChannel,
    'delete-channel': deleteChannel,
    'create-doc': submitDoc,
    'edit-doc': submitDoc,
    'delete-doc': deleteDoc,
    'create-user': submitUser,
    'edit-user': submitUser,
    'delete-user': deleteUser,
    'approve-order': approveOrder,
    'reject-order': rejectOrder,
    'edit-order': editOrder
  }
  actions[modal.type]?.()
}
</script>

<template>
  <section class="console-shell mx-auto max-w-7xl px-4 pb-12 sm:px-6">
    <div class="grid gap-5 lg:grid-cols-[250px_1fr]">
      <aside class="admin-sidebar">
        <div class="sidebar-glow"></div>
        <p class="section-kicker">Admin Center</p>
        <h2 class="mt-2 text-2xl font-black text-ink">管理后台</h2>
        <div class="mt-6 grid gap-2">
          <button
            v-for="item in menu"
            :key="item.key"
            class="nav-pill"
            :class="{ 'nav-pill-active': active === item.key }"
            @click="active = item.key"
          >
            <span>{{ item.label }}</span>
            <small>{{ item.hint }}</small>
          </button>
        </div>
      </aside>

      <div class="min-w-0">
        <div v-if="error" class="alert alert-danger">{{ error }}</div>
        <div v-if="notice" class="alert alert-success">{{ notice }}</div>

        <div v-if="active === 'overview'" class="space-y-6">
          <div class="admin-hero">
            <div>
              <p class="section-kicker">Overview</p>
              <h2 class="mt-2 text-3xl font-black text-white">运营总览</h2>
              <p class="mt-3 max-w-2xl text-sm leading-6 text-white/72">
                这里集中展示用户、订单、套餐和调用数据。待审核订单会优先露出，方便管理员直接进入审核流程。
              </p>
            </div>
            <div class="hero-orbit">
              <span>{{ pendingOrders }}</span>
              <small>待审核</small>
            </div>
          </div>

          <div class="stat-grid">
            <article class="stat-card">
              <span>用户总数</span>
              <strong>{{ stats.users || 0 }}</strong>
              <small>{{ approvedUsers }} 个已通过</small>
            </article>
            <article class="stat-card">
              <span>订单总数</span>
              <strong>{{ stats.orders || 0 }}</strong>
              <small>{{ pendingOrders }} 个待审核</small>
            </article>
            <article class="stat-card">
              <span>API Key</span>
              <strong>{{ stats.api_keys || 0 }}</strong>
              <small>用户自助创建</small>
            </article>
            <article class="stat-card">
              <span>调用次数</span>
              <strong>{{ stats.calls || 0 }}</strong>
              <small>网关请求日志</small>
            </article>
          </div>

          <div class="grid gap-5 xl:grid-cols-[1.2fr_0.8fr]">
            <section class="panel-surface p-5">
              <div class="section-head">
                <div>
                  <p class="section-kicker">Pending</p>
                  <h3>待处理订单</h3>
                </div>
                <button class="ghost-button" @click="active = 'orders'">查看全部</button>
              </div>
              <div class="mt-4 grid gap-3">
                <article v-for="order in orders.slice(0, 4)" :key="order.ID" class="list-row">
                  <div>
                    <strong>#{{ order.ID }} · {{ order.User?.Email || '未知用户' }}</strong>
                    <span>{{ order.Plan?.Name || '未关联套餐' }} · {{ money(order.AmountCents) }}</span>
                  </div>
                  <button v-if="order.Status === 'pending_review'" class="primary-button small" @click="openApproveModal(order)">审核</button>
                  <span v-else class="status-badge">{{ statusLabel(order.Status) }}</span>
                </article>
              </div>
            </section>

            <section class="panel-surface p-5">
              <div class="section-head">
                <div>
                  <p class="section-kicker">Plans</p>
                  <h3>套餐状态</h3>
                </div>
                <button class="ghost-button" @click="openPlanModal()">新增</button>
              </div>
              <div class="mt-4 grid gap-3">
                <article v-for="plan in plans.slice(0, 4)" :key="plan.ID" class="plan-mini">
                  <span :class="{ off: !plan.Enabled }"></span>
                  <div>
                    <strong>{{ plan.Name }}</strong>
                    <small>{{ rmb(plan.PriceCents) }} · {{ quotaPeriodLabel(plan.QuotaPeriod) }}额度 {{ usd(plan.SettlementUSDCents) }}</small>
                  </div>
                </article>
              </div>
            </section>
          </div>
        </div>

        <div v-if="active === 'plans'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Pricing</p>
              <h2>套餐管理</h2>
              <span>{{ enabledPlans }} 个启用套餐，{{ plans.length }} 个总套餐</span>
            </div>
            <div class="toolbar-actions">
              <button class="icon-button refresh-button" type="button" :disabled="loading" aria-label="刷新" title="刷新" @click="refreshAdminData">↻</button>
              <button class="primary-button" @click="openPlanModal()">新增套餐</button>
            </div>
          </div>

          <div class="plan-grid">
            <article v-for="plan in plans" :key="plan.ID" class="plan-card" :class="{ disabled: !plan.Enabled }">
              <div class="plan-card-top">
                <div>
                  <p>{{ plan.Code || '未设置编码' }}</p>
                  <h3>{{ plan.Name }}</h3>
                </div>
                <span class="status-badge" :class="{ muted: !plan.Enabled }">{{ plan.Enabled ? '已启用' : '已停用' }}</span>
              </div>
              <p class="plan-desc">{{ plan.Description || '暂无说明' }}</p>
              <div class="plan-price">
                <strong>{{ rmb(plan.PriceCents) }}</strong>
                <span>{{ plan.DurationDays }} 天</span>
              </div>
              <div class="quota-grid">
                <span><b>{{ usd(plan.SettlementUSDCents) }}</b>{{ quotaPeriodLabel(plan.QuotaPeriod) }}美元额度</span>
                <span><b>{{ totalUsd(plan) }}</b>预计总额度</span>
                <span><b>{{ plan.DurationDays }} 天</b>订阅周期</span>
              </div>
              <div class="card-actions">
                <button class="ghost-button" @click="openPlanModal(plan)">编辑</button>
                <button class="danger-button" @click="confirmDeletePlan(plan)">删除</button>
              </div>
            </article>
          </div>
        </div>

        <div v-if="active === 'orders'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Review</p>
              <h2>审核管理</h2>
              <span>订单审核、绑定上游账号和驳回原因都在弹窗内完成</span>
            </div>
            <button class="icon-button refresh-button" type="button" :disabled="loading" aria-label="刷新" title="刷新" @click="refreshAdminData">↻</button>
          </div>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr>
                    <th>订单</th>
                    <th>用户</th>
                    <th>套餐</th>
                    <th>上游渠道</th>
                    <th>金额</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="order in orders" :key="order.ID">
                    <td>#{{ order.ID }}</td>
                    <td>{{ order.User?.Email || '-' }}</td>
                    <td>{{ order.Plan?.Name || '-' }}</td>
                    <td>{{ order.Upstream?.Channel || '-' }}</td>
                    <td>{{ money(order.AmountCents) }}</td>
                    <td><span class="status-badge">{{ statusLabel(order.Status) }}</span></td>
                    <td>
                      <div class="table-actions">
                        <button class="ghost-button small" @click="openEditOrderModal(order)">编辑</button>
                        <button class="ghost-button small" :disabled="order.Status !== 'pending_review'" @click="openApproveModal(order)">审核</button>
                        <button class="danger-button small" :disabled="order.Status !== 'pending_review'" @click="openRejectModal(order)">拒绝</button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>

        <div v-if="active === 'models'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Model Billing</p>
              <h2>模型管理</h2>
              <span>{{ enabledModels }} 个启用模型，用户扣费按这里的单价和倍率计算</span>
            </div>
            <div class="toolbar-actions">
              <button class="ghost-button" type="button" :disabled="loading" @click="syncOfficialModels">同步官方倍率</button>
              <button class="icon-button refresh-button" type="button" :disabled="loading" aria-label="刷新" title="刷新" @click="refreshAdminData">↻</button>
              <button class="primary-button" @click="openModelModal()">新增模型</button>
            </div>
          </div>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <table class="data-table model-pricing-table">
                <thead>
                  <tr>
                    <th>模型</th>
                    <th>输入单价</th>
                    <th>缓存读取</th>
                    <th>输出单价</th>
                    <th>倍率</th>
                    <th>状态</th>
                    <th>同步时间</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in models" :key="item.ID">
                    <td class="model-cell">
                      <strong>{{ item.ModelName }}</strong>
                      <small>{{ item.DisplayName || item.Provider || '-' }}</small>
                    </td>
                    <td class="price-cell">
                      <strong>{{ modelActualUnit(item, 'InputUSDPerMillion') }}</strong>
                      <small>原价 {{ modelUnit(item.InputUSDPerMillion) }}</small>
                    </td>
                    <td class="price-cell">
                      <strong>{{ modelActualUnit(item, 'CachedInputUSDPerMillion') }}</strong>
                      <small>原价 {{ modelUnit(item.CachedInputUSDPerMillion) }}</small>
                    </td>
                    <td class="price-cell">
                      <strong>{{ modelActualUnit(item, 'OutputUSDPerMillion') }}</strong>
                      <small>原价 {{ modelUnit(item.OutputUSDPerMillion) }}</small>
                    </td>
                    <td class="multiplier-cell">{{ Number(item.BillingMultiplier || 1).toFixed(2) }}x</td>
                    <td class="status-cell"><span class="status-badge model-status-badge" :class="{ muted: item.Status !== 'active' }">{{ modelStatusLabel(item.Status) }}</span></td>
                    <td class="time-cell">{{ formatSyncTime(item.OfficialSyncedAt) }}</td>
                    <td class="actions-cell">
                      <div class="table-actions">
                        <button class="ghost-button small" @click="openModelModal(item)">编辑</button>
                        <button class="danger-button small" @click="confirmDeleteModel(item)">删除</button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="panel-surface p-5">
            <div class="section-head">
              <div>
                <p class="section-kicker">Official Snapshot</p>
                <h3>官方价格同步</h3>
                <span>同步会更新官方模型的输入、缓存读取和输出单价，但会保留你已设置的倍率。</span>
              </div>
              <a v-if="modelSource" class="ghost-button" :href="modelSource" target="_blank" rel="noreferrer">查看官方价格</a>
            </div>
          </section>
        </div>

        <div v-if="active === 'channels'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Channels</p>
              <h2>渠道管理</h2>
              <span>{{ enabledChannels }} 个启用渠道，{{ channels.length }} 个总渠道</span>
            </div>
            <div class="toolbar-actions">
              <button class="icon-button refresh-button" type="button" :disabled="loading" aria-label="刷新" title="刷新" @click="refreshAdminData">↻</button>
              <button class="primary-button" @click="openChannelModal()">新增渠道</button>
            </div>
          </div>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr>
                    <th>渠道名称</th>
                    <th>API 地址</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="channel in channels" :key="channel.ID">
                    <td>{{ channel.Name }}</td>
                    <td>{{ channel.BaseURL }}</td>
                    <td><span class="status-badge" :class="{ muted: !channel.Enabled }">{{ channel.Enabled ? '已启用' : '已停用' }}</span></td>
                    <td>
                      <div class="table-actions">
                        <button class="ghost-button small" @click="openChannelModal(channel)">编辑</button>
                        <button class="danger-button small" @click="confirmDeleteChannel(channel)">删除</button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>

        <div v-if="active === 'users'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Accounts</p>
              <h2>用户管理</h2>
              <span>新增、修改和删除用户都通过模态框完成，状态和角色使用中文选项</span>
            </div>
            <div class="toolbar-actions">
              <button class="icon-button refresh-button" type="button" :disabled="loading" aria-label="刷新" title="刷新" @click="refreshAdminData">↻</button>
              <button class="primary-button" @click="openUserModal()">新增用户</button>
            </div>
          </div>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr>
                    <th>用户</th>
                    <th>角色</th>
                    <th>状态</th>
                    <th>套餐</th>
                    <th>订阅额度</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="user in users" :key="user.ID">
                    <td>
                      <strong>{{ user.Email }}</strong>
                      <small>{{ user.Username }}</small>
                    </td>
                    <td>{{ roleLabel(user.Role) }}</td>
                    <td><span class="status-badge">{{ statusLabel(user.Status) }}</span></td>
                    <td>{{ user.Plan?.Name || '未分配' }}</td>
                    <td>{{ user.Plan ? `${usd(user.Plan.SettlementUSDCents)} / ${user.Plan.QuotaPeriod === 'daily' ? '日' : '周'}` : '未分配' }}</td>
                    <td>
                      <div class="table-actions">
                        <button class="ghost-button small" @click="openUserModal(user)">编辑</button>
                        <button class="danger-button small" @click="confirmDeleteUser(user)">删除</button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>

        <div v-if="active === 'docs'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Docs</p>
              <h2>配置文档</h2>
              <span>{{ enabledDocs }} 篇启用文档，{{ docs.length }} 篇总文档。左侧导航、排序和内容都可在这里维护。</span>
            </div>
            <div class="toolbar-actions">
              <button class="icon-button refresh-button" type="button" :disabled="loading" aria-label="刷新" title="刷新" @click="refreshAdminData">↻</button>
              <button class="primary-button" @click="openDocModal()">新增文档</button>
            </div>
          </div>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr>
                    <th>文档</th>
                    <th>分组</th>
                    <th>Slug</th>
                    <th>排序</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="doc in docs" :key="doc.ID">
                    <td>
                      <strong>{{ doc.Title }}</strong>
                      <small>{{ doc.Description || '暂无说明' }}</small>
                    </td>
                    <td>{{ doc.GroupName || '-' }}</td>
                    <td><code>{{ doc.Slug }}</code></td>
                    <td>{{ doc.SortOrder }}</td>
                    <td><span class="status-badge" :class="{ muted: !doc.Enabled }">{{ doc.Enabled ? '已启用' : '已停用' }}</span></td>
                    <td>
                      <div class="table-actions">
                        <button class="ghost-button small" @click="openDocModal(doc)">编辑</button>
                        <button class="danger-button small" @click="confirmDeleteDoc(doc)">删除</button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>

        <form v-if="active === 'navigation'" class="space-y-5" @submit.prevent="saveNavigation">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Navigation</p>
              <h2>导航菜单</h2>
              <span>维护首页顶部导航，支持一级菜单、下拉子菜单、排序和外链。</span>
            </div>
            <div class="toolbar-actions">
              <button type="button" class="icon-button refresh-button" :disabled="loading" aria-label="刷新" title="刷新" @click="refreshAdminData">↻</button>
              <button class="primary-button">保存导航</button>
            </div>
          </div>

          <section class="panel-surface p-5">
            <div class="nav-builder">
              <div class="nav-builder-head">
                <div>
                  <span>顶部导航配置</span>
                  <small>按顺序维护顶部导航，链接可填写 /plans、#tutorial 或完整网址。</small>
                </div>
                <div class="nav-builder-actions">
                  <button type="button" class="ghost-button small" @click="resetNavigationDefault">恢复默认</button>
                  <button type="button" class="primary-button small" @click="addNavItem">新增菜单</button>
                </div>
              </div>

              <div class="nav-editor-list">
                <article v-for="(item, index) in navDraft" :key="`nav-${index}`" class="nav-editor-card">
                  <div class="nav-editor-grid">
                    <label class="field">
                      <span>菜单名称</span>
                      <input v-model="item.label" placeholder="首页" @input="syncNavigationSetting" />
                    </label>
                    <label class="field">
                      <span>链接地址</span>
                      <input v-model="item.path" placeholder="/plans" @input="syncNavigationSetting" />
                    </label>
                    <label class="toggle-line nav-toggle">
                      <input v-model="item.external" type="checkbox" @change="syncNavigationSetting" />
                      新窗口打开
                    </label>
                    <div class="nav-row-actions">
                      <button type="button" class="ghost-button small" :disabled="index === 0" @click="moveNavItem(index, -1)">上移</button>
                      <button type="button" class="ghost-button small" :disabled="index === navDraft.length - 1" @click="moveNavItem(index, 1)">下移</button>
                      <button type="button" class="danger-button small" @click="removeNavItem(index)">删除</button>
                    </div>
                  </div>

                  <div class="child-nav-list">
                    <div v-for="(child, childIndex) in item.children" :key="`nav-${index}-child-${childIndex}`" class="child-nav-row">
                      <input v-model="child.label" placeholder="子菜单名称" @input="syncNavigationSetting" />
                      <input v-model="child.path" placeholder="/claude" @input="syncNavigationSetting" />
                      <label>
                        <input v-model="child.external" type="checkbox" @change="syncNavigationSetting" />
                        新窗口
                      </label>
                      <button type="button" class="danger-button small" @click="removeNavItem(index, childIndex)">删除</button>
                    </div>
                  </div>

                  <button type="button" class="ghost-button small" @click="addChildNavItem(index)">新增子菜单</button>
                </article>
              </div>
            </div>
          </section>
        </form>

        <form v-if="active === 'settings'" class="space-y-5" @submit.prevent="saveSettings">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Settings</p>
              <h2>系统设置</h2>
              <span>基础信息、SMTP 配置和易支付配置按类别维护</span>
            </div>
            <button class="primary-button">保存设置</button>
          </div>

          <div class="settings-tabs">
            <button type="button" :class="{ active: settingsTab === 'basic' }" @click="settingsTab = 'basic'">基础信息</button>
            <button type="button" :class="{ active: settingsTab === 'smtp' }" @click="settingsTab = 'smtp'">SMTP 配置</button>
            <button type="button" :class="{ active: settingsTab === 'epay' }" @click="settingsTab = 'epay'">易支付配置</button>
          </div>

          <section v-if="settingsTab === 'basic'" class="panel-surface p-5">
            <div class="form-grid">
              <label class="field">
                <span>网站标题</span>
                <input v-model="settings.site_title" placeholder="AI Gateway" />
              </label>
              <label class="field">
                <span>API 端点</span>
                <input v-model="settings.api_endpoint" placeholder="https://ai.itzkb.cn" />
              </label>
              <label class="field">
                <span>视频教程地址</span>
                <input v-model="settings.tutorial_video_url" placeholder="https://..." />
              </label>
              <label class="field">
                <span>定价页主标题</span>
                <input v-model="settings.pricing_title" placeholder="简单透明的定价" />
              </label>
              <label class="field">
                <span>定价页副标题</span>
                <input v-model="settings.pricing_subtitle" placeholder="保质保量无降智不掺假" />
              </label>
              <label class="field md:col-span-2">
                <span>定价页提示内容</span>
                <textarea v-model="settings.pricing_notice" rows="3" placeholder="展示在定价页顶部提示框中的说明文字"></textarea>
              </label>
            </div>
          </section>

          <section v-if="settingsTab === 'smtp'" class="panel-surface p-5">
            <div class="section-head mb-5">
              <div>
                <p class="section-kicker">Mail</p>
                <h3>SMTP 配置</h3>
              </div>
              <label class="toggle-line">
                <input v-model="settings.smtp_use_tls" type="checkbox" />
                使用 TLS
              </label>
            </div>
            <div class="form-grid">
              <label class="field"><span>SMTP 主机</span><input v-model="settings.smtp_host" placeholder="smtp.example.com" /></label>
              <label class="field"><span>SMTP 端口</span><input v-model.number="settings.smtp_port" type="number" min="1" /></label>
              <label class="field"><span>SMTP 用户名</span><input v-model="settings.smtp_username" /></label>
              <label class="field">
                <span>SMTP 密码</span>
                <input v-model="settings.smtp_password" type="password" :placeholder="settings.smtp_password_configured ? '已配置，留空不修改' : '请输入密码'" />
              </label>
              <label class="field"><span>发件邮箱</span><input v-model="settings.smtp_from_email" /></label>
              <label class="field"><span>发件名称</span><input v-model="settings.smtp_from_name" /></label>
            </div>
          </section>

          <section v-if="settingsTab === 'epay'" class="panel-surface p-5">
            <div class="section-head mb-5">
              <div>
                <p class="section-kicker">Payment</p>
                <h3>易支付配置</h3>
                <span>只需要填写接口网址、商户 ID 和商户 KEY，回调地址由系统自动生成。</span>
              </div>
            </div>
            <div class="form-grid">
              <label class="field md:col-span-2">
                <span>接口网址</span>
                <input v-model="settings.epay_submit_url" placeholder="https://mapi.example.com/" />
              </label>
              <label class="field"><span>商户 ID</span><input v-model="settings.epay_pid" placeholder="请输入商户 ID" /></label>
              <label class="field">
                <span>商户 KEY</span>
                <input v-model="settings.epay_key" type="password" :placeholder="settings.epay_key_configured ? '已配置，留空不修改' : '请输入商户 KEY'" />
              </label>
            </div>
          </section>
        </form>
      </div>
    </div>

    <div v-if="modal.open" class="modal-backdrop" @click.self="closeModal">
      <form class="modal-card" @submit.prevent="submitModal">
        <div class="modal-head">
          <h3>{{ modal.title }}</h3>
          <button type="button" class="icon-button" @click="closeModal">×</button>
        </div>

        <div v-if="modal.type === 'create-plan' || modal.type === 'edit-plan'" class="modal-body form-grid">
          <label class="field"><span>套餐名称</span><input v-model="planForm.name" required placeholder="月卡套餐" /></label>
          <label class="field"><span>套餐编码</span><input v-model="planForm.code" placeholder="monthly" /></label>
          <label class="field"><span>套餐角标文案</span><input v-model="planForm.badge_text" placeholder="热卖推荐" maxlength="16" /></label>
          <label class="field"><span>限额周期</span>
            <select v-model="planForm.quota_period">
              <option value="daily">日限额套餐</option>
              <option value="weekly">周限额套餐</option>
            </select>
          </label>
          <label class="field"><span>售价（RMB）</span><input v-model.number="planForm.price_rmb" type="number" min="0.01" step="0.01" required /></label>
          <label class="field"><span>{{ planForm.quota_period === 'daily' ? '每日美元额度' : '每周美元额度' }}</span><input v-model.number="planForm.period_usd_quota" type="number" min="0" step="0.01" /></label>
          <label class="field"><span>有效期（天）</span><input v-model.number="planForm.duration_days" type="number" min="1" required /></label>
          <label class="field"><span>预计总美元额度</span><input :value="totalUsd({ SettlementUSDCents: amountToCents(planForm.period_usd_quota), DurationDays: planForm.duration_days, QuotaPeriod: planForm.quota_period })" readonly /></label>
          <label class="field md:col-span-2"><span>套餐说明</span><textarea v-model="planForm.description" rows="3"></textarea></label>
          <label class="toggle-line md:col-span-2"><input v-model="planForm.enabled" type="checkbox" />启用套餐</label>
        </div>

        <div v-if="modal.type === 'create-channel' || modal.type === 'edit-channel'" class="modal-body form-grid">
          <label class="field"><span>渠道名称</span><input v-model="channelForm.name" required placeholder="OpenAI" /></label>
          <label class="field md:col-span-2"><span>API 地址</span><input v-model="channelForm.base_url" required placeholder="https://api.openai.com" /></label>
          <label class="toggle-line md:col-span-2"><input v-model="channelForm.enabled" type="checkbox" />启用渠道</label>
        </div>

        <div v-if="modal.type === 'create-model' || modal.type === 'edit-model'" class="modal-body form-grid">
          <label class="field"><span>模型 ID</span><input v-model="modelForm.model" required placeholder="gpt-5.5" /></label>
          <label class="field"><span>显示名称</span><input v-model="modelForm.display_name" placeholder="GPT-5.5" /></label>
          <label class="field"><span>服务商</span><input v-model="modelForm.provider" placeholder="openai" /></label>
          <label class="field"><span>输入单价 / 1M Token</span><input v-model.number="modelForm.input_usd_per_million" type="number" min="0" step="0.0001" /></label>
          <label class="field"><span>缓存读取单价 / 1M Token</span><input v-model.number="modelForm.cached_input_usd_per_million" type="number" min="0" step="0.0001" /></label>
          <label class="field"><span>输出单价 / 1M Token</span><input v-model.number="modelForm.output_usd_per_million" type="number" min="0" step="0.0001" /></label>
          <label class="field"><span>扣费倍率</span><input v-model.number="modelForm.billing_multiplier" type="number" min="0.0001" step="0.01" /></label>
          <label class="field"><span>状态</span>
            <select v-model="modelForm.status">
              <option value="active">启用</option>
              <option value="disabled">停用</option>
            </select>
          </label>
          <label class="field md:col-span-2"><span>备注</span><textarea v-model="modelForm.notes" rows="3"></textarea></label>
        </div>

        <div v-if="modal.type === 'create-doc' || modal.type === 'edit-doc'" class="modal-body form-grid">
          <label class="field">
            <span>文档标题</span>
            <input v-model="docForm.title" required placeholder="官方 API Base URL" />
          </label>
          <label class="field">
            <span>Slug</span>
            <input v-model="docForm.slug" required placeholder="api-base-url" />
          </label>
          <label class="field">
            <span>左侧分组</span>
            <input v-model="docForm.group_name" placeholder="快速开始" />
          </label>
          <label class="field">
            <span>排序</span>
            <input v-model.number="docForm.sort_order" type="number" />
          </label>
          <label class="field md:col-span-2">
            <span>说明</span>
            <input v-model="docForm.description" placeholder="展示在后台列表中的简短说明" />
          </label>
          <label class="field md:col-span-2">
            <span>Markdown 内容</span>
            <textarea v-model="docForm.content" class="markdown-editor" rows="18" placeholder="# 标题&#10;&#10;这里填写 Markdown 文档内容"></textarea>
          </label>
          <label class="toggle-line md:col-span-2"><input v-model="docForm.enabled" type="checkbox" />启用文档</label>
        </div>

        <div v-if="modal.type === 'create-user' || modal.type === 'edit-user'" class="modal-body form-grid">
          <label class="field"><span>用户名</span><input v-model="userForm.username" required /></label>
          <label class="field"><span>邮箱</span><input v-model="userForm.email" type="email" required /></label>
          <label class="field">
            <span>{{ userForm.id ? '新密码' : '登录密码' }}</span>
            <input v-model="userForm.password" type="password" :required="!userForm.id" minlength="8" :placeholder="userForm.id ? '留空不修改' : '至少 8 位'" />
          </label>
          <label class="field">
            <span>角色</span>
            <select v-model="userForm.role">
              <option v-for="option in roleOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <label class="field">
            <span>状态</span>
            <select v-model="userForm.status">
              <option v-for="option in statusOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <label class="field">
            <span>绑定套餐</span>
            <select v-model="userForm.plan_id">
              <option value="">不分配</option>
              <option v-for="plan in plans" :key="plan.ID" :value="plan.ID">{{ plan.Name }}</option>
            </select>
          </label>
          <label class="toggle-line md:col-span-2"><input v-model="userForm.email_verified" type="checkbox" />邮箱已验证</label>
        </div>

        <div v-if="modal.type === 'approve-order'" class="modal-body form-grid">
          <label class="field"><span>订单 ID</span><input v-model="approve.orderId" readonly /></label>
          <label class="field"><span>上游渠道</span>
            <select v-model="approve.channelId" required @change="syncApproveChannel">
              <option value="">请选择渠道</option>
              <option v-for="channel in channels.filter((item) => item.Enabled)" :key="channel.ID" :value="channel.ID">{{ channel.Name }}</option>
            </select>
          </label>
          <label class="field md:col-span-2"><span>上游 API Key</span><input v-model="approve.apiKey" type="password" required /></label>
          <label class="field md:col-span-2"><span>审核备注</span><textarea v-model="approve.adminNote" rows="3"></textarea></label>
        </div>

        <div v-if="modal.type === 'edit-order'" class="modal-body form-grid">
          <label class="field"><span>订单 ID</span><input v-model="approve.orderId" readonly /></label>
          <label class="field"><span>关联套餐</span>
            <select v-model="approve.planId" :disabled="approve.status === 'approved'">
              <option value="">不分配</option>
              <option v-for="plan in plans" :key="plan.ID" :value="plan.ID">{{ plan.Name }}</option>
            </select>
          </label>
          <label class="field"><span>金额（分）</span><input v-model.number="approve.amountCents" type="number" min="0" /></label>
          <label class="field"><span>上游渠道</span>
            <select v-model="approve.channelId" @change="syncApproveChannel">
              <option value="">不修改</option>
              <option v-for="channel in channels.filter((item) => item.Enabled)" :key="channel.ID" :value="channel.ID">{{ channel.Name }}</option>
            </select>
          </label>
          <label class="field md:col-span-2"><span>上游 API Key</span><input v-model="approve.apiKey" type="password" placeholder="留空不修改" /></label>
          <label class="field md:col-span-2"><span>审核备注</span><textarea v-model="approve.adminNote" rows="3"></textarea></label>
        </div>

        <div v-if="modal.type === 'reject-order'" class="modal-body">
          <label class="field"><span>拒绝原因</span><textarea v-model="rejectForm.adminNote" rows="4" placeholder="请输入给内部留档的拒绝原因"></textarea></label>
        </div>

        <div v-if="modal.type === 'delete-plan'" class="modal-body confirm-copy">
          <strong>确定删除「{{ modal.payload?.plan?.Name }}」吗？</strong>
          <p>删除后该套餐不会再出现在管理列表和用户可购套餐中，请确认没有正在依赖它的运营流程。</p>
        </div>

        <div v-if="modal.type === 'delete-channel'" class="modal-body confirm-copy">
          <strong>确定删除「{{ modal.payload?.channel?.Name }}」吗？</strong>
          <p>删除后审核弹窗不再提供该渠道，请确认没有新的开通流程依赖它。</p>
        </div>

        <div v-if="modal.type === 'delete-model'" class="modal-body confirm-copy">
          <strong>确定删除「{{ modal.payload?.model?.ModelName }}」吗？</strong>
          <p>删除后该模型会使用系统兜底价格计费，建议仅在确认不再使用该模型时删除。</p>
        </div>

        <div v-if="modal.type === 'delete-doc'" class="modal-body confirm-copy">
          <strong>确定删除「{{ modal.payload?.doc?.Title }}」吗？</strong>
          <p>删除后用户侧配置文档页面将不再展示这篇内容。</p>
        </div>

        <div v-if="modal.type === 'delete-user'" class="modal-body confirm-copy">
          <strong>确定删除「{{ modal.payload?.user?.Email }}」吗？</strong>
          <p>删除用户会移除账号本身，相关订单和密钥关系请在操作前确认。</p>
        </div>

        <div class="modal-actions">
          <button type="button" class="ghost-button" @click="closeModal">取消</button>
          <button :class="modal.danger ? 'danger-solid-button' : 'primary-button'">{{ modal.actionLabel }}</button>
        </div>
      </form>
    </div>
  </section>
</template>
