<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { api } from '../api/client'

const menu = [
  { key: 'overview', label: '总览', hint: '运营数据' },
  { key: 'plans', label: '套餐管理', hint: '价格与额度' },
  { key: 'orders', label: '审核管理', hint: '订单开通' },
  { key: 'models', label: '模型管理', hint: '计费倍率' },
  { key: 'channels', label: '渠道管理', hint: '上游接口' },
  { key: 'users', label: '用户管理', hint: '账号与权限' },
  { key: 'announcements', label: '公告管理', hint: '控制台公告' },
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
const usersTab = ref('users')
const channelsTab = ref('upstream')
const stats = ref({})
const orders = ref([])
const users = ref([])
const apiKeys = ref([])
const plans = ref([])
const models = ref([])
const modelSource = ref('')
const channels = ref([])
const publicChannels = ref([])
const docs = ref([])
const announcements = ref([])
const error = ref('')
const notice = ref('')
const navDraft = ref([])
const apiEndpointDraft = ref([])
const loading = ref(false)
const smtpTesting = ref(false)
let userSearchTimer = null
let overviewMetricsTimer = null
const modal = reactive({ open: false, type: '', title: '', actionLabel: '', danger: false, payload: null })
const approve = reactive({ orderId: '', channelId: '', channel: '', baseUrl: '', username: '', password: '', apiKey: '', adminNote: '', planId: '', amountCents: 0, status: '' })
const rejectForm = reactive({ orderId: '', adminNote: '' })
const planForm = reactive(emptyPlan())
const modelForm = reactive(emptyModel())
const channelForm = reactive(emptyChannel())
const publicChannelForm = reactive(emptyPublicChannel())
const userForm = reactive(emptyUser())
const apiKeyForm = reactive(emptyApiKey())
const docForm = reactive(emptyDoc())
const announcementForm = reactive(emptyAnnouncement())
const userSearch = reactive({ keyword: '', role: '', status: '', plan: '' })
const settings = reactive({
  site_title: '',
  contact_email: '',
  api_endpoints: '',
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
  epay_key_configured: false,
  smtp_test_email: ''
})

const pendingOrders = computed(() => orders.value.filter((order) => order.Status === 'pending_review').length)
const enabledPlans = computed(() => plans.value.filter((plan) => plan.Enabled).length)
const enabledModels = computed(() => models.value.filter((item) => item.Status === 'active').length)
const approvedUsers = computed(() => users.value.filter((user) => user.Status === 'approved').length)
const enabledChannels = computed(() => channels.value.filter((channel) => channel.Enabled).length)
const enabledPublicChannels = computed(() => publicChannels.value.filter((channel) => channel.Enabled).length)
const enabledDocs = computed(() => docs.value.filter((doc) => doc.Enabled).length)
const enabledAnnouncements = computed(() => announcements.value.filter((item) => item.Enabled).length)
const pendingReviewOrders = computed(() => orders.value.filter((order) => order.Status === 'pending_review'))
const overviewPlans = computed(() => plans.value.slice(0, 4))
const hasMorePlans = computed(() => plans.value.length > 4)
const filteredUsers = computed(() => {
  const keyword = String(userSearch.keyword || '').trim().toLowerCase()
  const role = String(userSearch.role || '')
  const status = String(userSearch.status || '')
  const plan = String(userSearch.plan || '')
  return users.value.filter((user) => {
    const matchesKeyword = !keyword || [user.Username, user.Email, user.ID].some((value) => String(value || '').toLowerCase().includes(keyword))
    const matchesRole = !role || user.Role === role
    const matchesStatus = !status || user.Status === status
    const matchesPlan = !plan || String(user.PlanID || '') === plan || String(user.Plan?.ID || '') === plan || String(user.Plan?.Name || '').toLowerCase().includes(plan.toLowerCase())
    return matchesKeyword && matchesRole && matchesStatus && matchesPlan
  })
})
const filteredApiKeys = computed(() => apiKeys.value)

onMounted(async () => {
  await loadAll()
  startOverviewMetricsPolling()
})

watch(
  () => [userSearch.keyword, userSearch.role, userSearch.status, userSearch.plan],
  () => {
    if (active.value !== 'users') return
    if (userSearchTimer) clearTimeout(userSearchTimer)
    userSearchTimer = setTimeout(() => {
      loadAll()
    }, 250)
  }
)

onBeforeUnmount(() => {
  if (userSearchTimer) clearTimeout(userSearchTimer)
  stopOverviewMetricsPolling()
})

watch(active, (value) => {
  if (value === 'overview') {
    refreshOverviewMetrics()
    startOverviewMetricsPolling()
    return
  }
  stopOverviewMetricsPolling()
})

function emptyPlan() {
  return {
    id: null,
    name: '',
    code: '',
    badge_text: '',
    plan_type: 'subscription',
    quota_period: 'weekly',
    public_channel_id: '',
    price_rmb: 9.9,
    period_usd_quota: 20,
    price_cents: 990,
    settlement_usd_cents: 2000,
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
    original_plan_id: '',
    channel_id: '',
    upstream_username: '',
    upstream_password: '',
    api_key: ''
  }
}

function emptyApiKey() {
  return {
    id: null,
    name: '',
    status: 'active'
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

function emptyPublicChannel() {
  return {
    id: null,
    name: '',
    base_url: '',
    api_key: '',
    total_usd_quota: 400,
    remaining_usd_quota: 400,
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
    featured: false,
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

function emptyAnnouncement() {
  return {
    id: null,
    title: '',
    summary: '',
    content: '',
    link_text: '',
    link_url: '',
    sort_order: 100,
    pinned: false,
    enabled: true,
    published_at: ''
  }
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const userParams = {
      params: {
        q: userSearch.keyword || undefined,
        role: userSearch.role || undefined,
        status: userSearch.status || undefined,
        plan: userSearch.plan || undefined
      }
    }
    const [statsRes, ordersRes, usersRes, plansRes, modelsRes, channelsRes, publicChannelsRes, keysRes, docsRes, announcementsRes, settingsRes] = await Promise.all([
      api.get('/admin/stats'),
      api.get('/admin/orders'),
      api.get('/admin/users', userParams),
      api.get('/admin/plans'),
      api.get('/admin/models'),
      api.get('/admin/upstream-channels'),
      api.get('/admin/public-channels'),
      api.get('/admin/keys'),
      api.get('/admin/docs'),
      api.get('/admin/announcements'),
      api.get('/admin/settings')
    ])
    stats.value = statsRes.data || {}
    orders.value = ordersRes.data || []
    users.value = usersRes.data || []
    plans.value = plansRes.data || []
    models.value = modelsRes.data?.items || []
    modelSource.value = modelsRes.data?.official_source || ''
    channels.value = channelsRes.data || []
    publicChannels.value = publicChannelsRes.data || []
    apiKeys.value = keysRes.data || []
    docs.value = docsRes.data || []
    announcements.value = announcementsRes.data || []
    Object.assign(settings, settingsRes.data, { smtp_password: '', epay_key: '' })
    setNavigationDraft(settings.navigation_items)
    setAPIEndpointDraft(settings.api_endpoints)
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

async function refreshOverviewMetrics() {
  if (active.value !== 'overview') return
  try {
    const res = await api.get('/admin/stats')
    stats.value = res.data || {}
  } catch (err) {
    if (!error.value) error.value = err.message
  }
}

function startOverviewMetricsPolling() {
  if (overviewMetricsTimer || active.value !== 'overview') return
  overviewMetricsTimer = setInterval(refreshOverviewMetrics, 3000)
}

function stopOverviewMetricsPolling() {
  if (!overviewMetricsTimer) return
  clearInterval(overviewMetricsTimer)
  overviewMetricsTimer = null
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
      public_channel_id: plan.PublicChannelID || plan.PublicChannel?.ID || '',
      price_rmb: centsToAmount(plan.PriceCents),
      period_usd_quota: centsToAmount(plan.SettlementUSDCents),
      price_cents: plan.PriceCents,
      settlement_usd_cents: plan.SettlementUSDCents,
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
      featured: Boolean(model.Featured),
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

function openPublicChannelModal(channel = null) {
  Object.assign(publicChannelForm, emptyPublicChannel())
  if (channel) {
    Object.assign(publicChannelForm, {
      id: channel.ID,
      name: channel.Name,
      base_url: channel.BaseURL,
      api_key: channel.APIKey || '',
      total_usd_quota: centsToAmount(channel.TotalUSDCents),
      remaining_usd_quota: centsToAmount(channel.RemainingUSDCents),
      enabled: channel.Enabled
    })
  }
  showModal(channel ? 'edit-public-channel' : 'create-public-channel', channel ? '编辑公共渠道' : '新增公共渠道', channel ? '保存修改' : '创建公共渠道')
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

function openAnnouncementModal(item = null) {
  Object.assign(announcementForm, emptyAnnouncement())
  if (item) {
    Object.assign(announcementForm, {
      id: item.ID,
      title: item.Title,
      summary: item.Summary || '',
      content: item.Content || '',
      link_text: item.LinkText || '',
      link_url: item.LinkURL || '',
      sort_order: item.SortOrder || 0,
      pinned: Boolean(item.Pinned),
      enabled: Boolean(item.Enabled),
      published_at: toDateTimeLocal(item.PublishedAt)
    })
  }
  showModal(item ? 'edit-announcement' : 'create-announcement', item ? '编辑公告' : '发布公告', item ? '保存修改' : '发布公告')
}

async function submitAnnouncement() {
  const payload = normalizeAnnouncement(announcementForm)
  await runAction(async () => {
    if (announcementForm.id) {
      await api.put(`/admin/announcements/${announcementForm.id}`, payload)
      notice.value = '公告已更新'
    } else {
      await api.post('/admin/announcements', payload)
      notice.value = '公告已发布'
    }
  })
}

function confirmDeleteAnnouncement(item) {
  showModal('delete-announcement', '删除公告', '确认删除', { announcement: item }, true)
}

async function deleteAnnouncement() {
  await runAction(async () => {
    await api.delete(`/admin/announcements/${modal.payload.announcement.ID}`)
    notice.value = '公告已删除'
  })
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

async function submitPublicChannel() {
  const payload = normalizePublicChannel(publicChannelForm)
  await runAction(async () => {
    if (publicChannelForm.id) {
      await api.put(`/admin/public-channels/${publicChannelForm.id}`, payload)
      notice.value = '公共渠道已更新'
    } else {
      await api.post('/admin/public-channels', payload)
      notice.value = '公共渠道已创建'
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

function confirmDeletePublicChannel(channel) {
  showModal('delete-public-channel', '删除公共渠道', '确认删除', { channel }, true)
}

async function deletePublicChannel() {
  await runAction(async () => {
    await api.delete(`/admin/public-channels/${modal.payload.channel.ID}`)
    notice.value = '公共渠道已删除'
  })
}

function openUserModal(user = null) {
  Object.assign(userForm, emptyUser())
  const upstream = user?.Upstream || {}
  const channel = channels.value.find((item) => item.Name === upstream.Channel) || null
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
      original_plan_id: user.PlanID || '',
      channel_id: channel?.ID || '',
      upstream_username: upstream.Username || '',
      upstream_password: upstream.Password || '',
      api_key: upstream.APIKey || ''
    })
  }
  showModal(user ? 'edit-user' : 'create-user', user ? '编辑用户' : '新增用户', user ? '保存修改' : '创建用户')
}

async function submitUser() {
  if (requiresUserUpstreamRebind(userForm) && (!Number(userForm.channel_id) || !String(userForm.upstream_username || '').trim() || !String(userForm.upstream_password || '').trim() || !String(userForm.api_key || '').trim())) {
    error.value = '修改用户套餐后，必须重新绑定上游渠道并填写上游账号、上游密码和 API Key'
    return
  }
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

async function openUserUpstreamModal(user) {
  error.value = ''
  const cached = user.Upstream
  if (cached) {
    showModal('user-upstream', `渠道 #${user.ID}`, '关闭', { user, upstream: cached })
    return
  }
  try {
    const res = await api.get(`/admin/users/${user.ID}/upstream`)
    showModal('user-upstream', `渠道 #${user.ID}`, '关闭', { user, upstream: res.data })
  } catch (err) {
    error.value = err.message
  }
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

function openApiKeyModal(key = null) {
  Object.assign(apiKeyForm, emptyApiKey())
  if (key) {
    Object.assign(apiKeyForm, {
      id: key.ID,
      name: key.Name || '',
      status: key.Status || 'active'
    })
  }
  showModal('edit-api-key', '编辑 API Key', '保存修改', { key }, false)
}

async function submitApiKey() {
  await runAction(async () => {
    await api.patch(`/admin/keys/${modal.payload.key.ID}`, {
      name: apiKeyForm.name.trim(),
      status: apiKeyForm.status
    })
    notice.value = 'API Key 已更新'
  })
}

async function toggleApiKeyStatus(key) {
  await runAction(async () => {
    await api.patch(`/admin/keys/${key.ID}`, {
      status: key.Status === 'active' ? 'disabled' : 'active'
    })
    notice.value = key.Status === 'active' ? 'API Key 已停用' : 'API Key 已启用'
  })
}

function confirmDeleteApiKey(key) {
  showModal('delete-api-key', '删除 API Key', '确认删除', { key }, true)
}

async function deleteApiKey() {
  await runAction(async () => {
    await api.delete(`/admin/keys/${modal.payload.key.ID}`)
    notice.value = 'API Key 已删除'
  })
}

function openApproveModal(order) {
  const channel = channels.value.find((item) => item.Enabled) || null
  Object.assign(approve, {
    orderId: String(order.ID),
    channelId: channel?.ID || '',
    channel: channel?.Name || '',
    baseUrl: channel?.BaseURL || '',
    username: '',
    password: '',
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
    username: upstream.Username || '',
    password: upstream.Password || '',
    apiKey: upstream.APIKey || '',
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
      username: approve.username,
      password: approve.password,
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
    username: approve.username,
    password: approve.password,
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
  syncAPIEndpointSetting()
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

async function sendSMTPTest() {
  error.value = ''
  notice.value = ''
  if (!String(settings.smtp_test_email || '').trim()) {
    error.value = '请先填写测试收件邮箱'
    return
  }
  smtpTesting.value = true
  try {
    await api.post('/admin/settings/test-smtp', {
      ...settings,
      smtp_port: Number(settings.smtp_port || 587),
      to_email: settings.smtp_test_email.trim()
    })
    notice.value = `测试邮件已发送到 ${settings.smtp_test_email.trim()}`
  } catch (err) {
    error.value = err.message
  } finally {
    smtpTesting.value = false
  }
}

async function saveNavigation() {
  syncNavigationSetting()
  syncAPIEndpointSetting()
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

function createAPIEndpoint(overrides = {}) {
  return {
    label: '',
    description: '',
    url: '',
    ...overrides
  }
}

function setAPIEndpointDraft(value) {
  apiEndpointDraft.value = parseAPIEndpoints(value).map((item) => createAPIEndpoint(item))
  syncAPIEndpointSetting()
}

function parseAPIEndpoints(value) {
  try {
    const parsed = JSON.parse(value || '[]')
    return Array.isArray(parsed) && parsed.length ? normalizeAPIEndpoints(parsed) : defaultAPIEndpoints()
  } catch {
    return defaultAPIEndpoints()
  }
}

function defaultAPIEndpoints() {
  return [{ label: '默认', description: '主线路', url: 'https://ai.itzkb.cn' }]
}

function normalizeAPIEndpoints(items) {
  return items
    .map((item) => ({
      label: String(item.label || '').trim() || 'API',
      description: String(item.description || '').trim(),
      url: String(item.url || '').trim()
    }))
    .filter((item) => item.url)
}

function syncAPIEndpointSetting() {
  const normalized = normalizeAPIEndpoints(apiEndpointDraft.value)
  settings.api_endpoints = JSON.stringify(normalized.length ? normalized : defaultAPIEndpoints())
}

function addAPIEndpoint() {
  apiEndpointDraft.value.push(createAPIEndpoint({ label: '新线路', description: '备用线路', url: 'https://' }))
  syncAPIEndpointSetting()
}

function removeAPIEndpoint(index) {
  apiEndpointDraft.value.splice(index, 1)
  syncAPIEndpointSetting()
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
  const isPublic = plan.quota_period === 'public'
  return {
    name: plan.name.trim(),
    code: plan.code.trim(),
    badge_text: plan.badge_text.trim(),
    plan_type: isPublic ? 'public' : 'subscription',
    quota_period: isPublic ? 'public' : plan.quota_period,
    public_channel_id: isPublic ? Number(plan.public_channel_id || 0) : null,
    price_cents: amountToCents(plan.price_rmb),
    settlement_usd_cents: amountToCents(plan.period_usd_quota),
    duration_days: isPublic ? 1 : Number(plan.duration_days || 1),
    description: plan.description.trim(),
    enabled: Boolean(plan.enabled)
  }
}

function normalizePublicChannel(channel) {
  return {
    name: channel.name.trim(),
    base_url: channel.base_url.trim(),
    api_key: channel.api_key.trim(),
    total_usd_cents: amountToCents(channel.total_usd_quota),
    remaining_usd_cents: amountToCents(channel.remaining_usd_quota),
    enabled: Boolean(channel.enabled)
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
    featured: Boolean(item.featured),
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

function normalizeAnnouncement(item) {
  return {
    title: item.title.trim(),
    summary: item.summary.trim(),
    content: item.content.trim(),
    link_text: item.link_text.trim(),
    link_url: item.link_url.trim(),
    sort_order: Number(item.sort_order || 0),
    pinned: Boolean(item.pinned),
    enabled: Boolean(item.enabled),
    published_at: item.published_at
  }
}

function toDateTimeLocal(value) {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
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
  if (requiresUserUpstreamRebind(user)) {
    payload.channel_id = Number(user.channel_id || 0)
    payload.upstream_username = user.upstream_username.trim()
    payload.upstream_password = user.upstream_password
    payload.api_key = user.api_key
  }
  if (user.password) payload.password = user.password
  return payload
}

function requiresUserUpstreamRebind(user) {
  return user.id && String(user.plan_id || '') !== String(user.original_plan_id || '') && String(user.plan_id || '') !== ''
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
  if (period === 'public') return '公共'
  return period === 'daily' ? '每日' : '每周'
}

function planWeeks(plan) {
  return Math.max(1, Math.round((plan.DurationDays || 30) / 7))
}

function totalUsd(plan) {
  if (plan.QuotaPeriod === 'public') return usd(plan.SettlementUSDCents)
  const units = plan.QuotaPeriod === 'daily' ? (plan.DurationDays || 1) : planWeeks(plan)
  return `$${(((plan.SettlementUSDCents || 0) / 100) * units).toFixed(0)}`
}

function channelQuotaText(channel) {
  const remaining = channel?.RemainingUSDCents || 0
  const total = channel?.TotalUSDCents || 0
  return `${usd(remaining)} / ${usd(total)}`
}

function publicChannelName(plan) {
  return plan.PublicChannel?.Name || publicChannels.value.find((channel) => channel.ID === plan.PublicChannelID)?.Name || '未绑定公共渠道'
}

function compactNumber(value) {
  return Number(value || 0).toLocaleString()
}

function percent(value) {
  return `${Number(value || 0).toFixed(1)}%`
}

function bytes(value) {
  const size = Number(value || 0)
  if (size <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let current = size
  let unit = 0
  while (current >= 1024 && unit < units.length - 1) {
    current /= 1024
    unit += 1
  }
  return `${current >= 10 || unit === 0 ? current.toFixed(0) : current.toFixed(1)} ${units[unit]}`
}

function systemLoad() {
  return stats.value.system_load || {}
}

function systemLoadText() {
  const load = systemLoad()
  if (!load.load_average_1) return `${load.go_routines || load.goroutines || 0} goroutines`
  return `Load ${Number(load.load_average_1 || 0).toFixed(2)} / ${Number(load.load_average_5 || 0).toFixed(2)}`
}

function memoryText() {
  const load = systemLoad()
  if (!load.memory_total_bytes) return `进程 ${bytes(load.process_memory_bytes)}`
  return `${bytes(load.memory_used_bytes)} / ${bytes(load.memory_total_bytes)}`
}

function roleLabel(value) {
  return roleOptions.find((item) => item.value === value)?.label || value
}

function planLabel(user) {
  return user.Plan?.Name || '未分配'
}

function planSearchValue(plan) {
  return String(plan || '').toLowerCase()
}

function apiKeyStatusLabel(value) {
  return value === 'disabled' ? '已停用' : '已启用'
}

function apiKeyPrefix(value) {
  return value || '-'
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
    'create-public-channel': submitPublicChannel,
    'edit-public-channel': submitPublicChannel,
    'delete-public-channel': deletePublicChannel,
    'create-doc': submitDoc,
    'edit-doc': submitDoc,
    'delete-doc': deleteDoc,
    'create-announcement': submitAnnouncement,
    'edit-announcement': submitAnnouncement,
    'delete-announcement': deleteAnnouncement,
    'create-user': submitUser,
    'edit-user': submitUser,
    'edit-api-key': submitApiKey,
    'user-upstream': closeModal,
    'delete-user': deleteUser,
    'delete-api-key': deleteApiKey,
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
            <article class="stat-card">
              <span>活动 API 连接</span>
              <strong>{{ stats.active_api_connections || 0 }}</strong>
              <small>实时接入中，请求结束自动 -1</small>
            </article>
            <article class="stat-card">
              <span>CPU 负载</span>
              <strong>{{ percent(systemLoad().cpu_percent) }}</strong>
              <small>{{ systemLoadText() }}</small>
            </article>
            <article class="stat-card">
              <span>内存占用</span>
              <strong>{{ percent(systemLoad().memory_used_percent) }}</strong>
              <small>{{ memoryText() }}</small>
            </article>
            <article class="stat-card">
              <span>运行状态</span>
              <strong>{{ systemLoad().cpu_count || 0 }} 核</strong>
              <small>{{ systemLoad().system_metrics_provider || 'runtime' }} · {{ formatDate(systemLoad().sampled_at) }}</small>
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
                <article v-for="order in pendingReviewOrders.slice(0, 4)" :key="order.ID" class="list-row">
                  <div>
                    <strong>#{{ order.ID }} · {{ order.User?.Email || '未知用户' }}</strong>
                    <span>{{ order.Plan?.Name || '未关联套餐' }} · {{ money(order.AmountCents) }}</span>
                  </div>
                  <button class="primary-button small" @click="openApproveModal(order)">审核</button>
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
                <article v-for="plan in overviewPlans" :key="plan.ID" class="plan-mini">
                  <span :class="{ off: !plan.Enabled }"></span>
                  <div>
                    <strong>{{ plan.Name }}</strong>
                    <small>{{ rmb(plan.PriceCents) }} · {{ quotaPeriodLabel(plan.QuotaPeriod) }}额度 {{ usd(plan.SettlementUSDCents) }}</small>
                  </div>
                </article>
              </div>
              <div v-if="hasMorePlans" class="mt-4 flex justify-end">
                <button class="ghost-button" @click="active = 'plans'">更多</button>
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
                <span v-if="plan.QuotaPeriod === 'public'"><b>{{ publicChannelName(plan) }}</b>公共渠道</span>
                <span v-else><b>{{ plan.DurationDays }} 天</b>订阅周期</span>
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
                    <th>展示卡片</th>
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
                    <td class="status-cell"><span class="status-badge" :class="{ muted: !item.Featured }">{{ item.Featured ? '展示' : '不展示' }}</span></td>
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
              <span>普通渠道 {{ enabledChannels }}/{{ channels.length }}，公共渠道 {{ enabledPublicChannels }}/{{ publicChannels.length }}</span>
            </div>
            <div class="toolbar-actions">
              <button class="icon-button refresh-button" type="button" :disabled="loading" aria-label="刷新" title="刷新" @click="refreshAdminData">↻</button>
              <button v-if="channelsTab === 'upstream'" class="primary-button" @click="openChannelModal()">新增渠道</button>
              <button v-else class="primary-button" @click="openPublicChannelModal()">新增公共渠道</button>
            </div>
          </div>

          <div class="settings-tabs">
            <button :class="{ active: channelsTab === 'upstream' }" @click="channelsTab = 'upstream'">上游渠道</button>
            <button :class="{ active: channelsTab === 'public' }" @click="channelsTab = 'public'">公共渠道</button>
          </div>

          <section v-if="channelsTab === 'upstream'" class="panel-surface overflow-hidden">
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

          <section v-else class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr>
                    <th>渠道名称</th>
                    <th>API 地址</th>
                    <th>剩余额度 / 总额度</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="channel in publicChannels" :key="channel.ID">
                    <td>{{ channel.Name }}</td>
                    <td>{{ channel.BaseURL }}</td>
                    <td>{{ channelQuotaText(channel) }}</td>
                    <td><span class="status-badge" :class="{ muted: !channel.Enabled || channel.RemainingUSDCents <= 0 }">{{ channel.RemainingUSDCents <= 0 ? '售罄' : (channel.Enabled ? '已启用' : '已停用') }}</span></td>
                    <td>
                      <div class="table-actions">
                        <button class="ghost-button small" @click="openPublicChannelModal(channel)">编辑</button>
                        <button class="danger-button small" @click="confirmDeletePublicChannel(channel)">删除</button>
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

          <div class="settings-tabs">
            <button type="button" :class="{ active: usersTab === 'users' }" @click="usersTab = 'users'">用户列表</button>
            <button type="button" :class="{ active: usersTab === 'api-keys' }" @click="usersTab = 'api-keys'">API Key</button>
          </div>

          <section v-if="usersTab === 'users'" class="panel-surface p-4">
            <div class="form-grid user-filter-grid">
              <label class="field">
                <span>搜索</span>
                <input v-model="userSearch.keyword" placeholder="用户名 / 邮箱 / ID" />
              </label>
              <label class="field">
                <span>角色</span>
                <select v-model="userSearch.role">
                  <option value="">全部</option>
                  <option v-for="option in roleOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                </select>
              </label>
              <label class="field">
                <span>状态</span>
                <select v-model="userSearch.status">
                  <option value="">全部</option>
                  <option v-for="option in statusOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                </select>
              </label>
              <label class="field">
                <span>套餐</span>
                <select v-model="userSearch.plan">
                  <option value="">全部</option>
                  <option v-for="plan in plans" :key="plan.ID" :value="String(plan.ID)">{{ plan.Name }}</option>
                </select>
              </label>
            </div>
          </section>

          <section v-if="usersTab === 'users'" class="panel-surface overflow-hidden">
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
                  <tr v-for="user in filteredUsers" :key="user.ID">
                    <td>
                      <strong>{{ user.Email }}</strong>
                      <small>{{ user.Username }}</small>
                    </td>
                    <td>{{ roleLabel(user.Role) }}</td>
                    <td><span class="status-badge">{{ statusLabel(user.Status) }}</span></td>
                    <td>{{ planLabel(user) }}</td>
                    <td>{{ user.Plan ? `${usd(user.Plan.SettlementUSDCents)} / ${user.Plan.QuotaPeriod === 'daily' ? '日' : '周'}` : '未分配' }}</td>
                    <td>
                      <div class="table-actions">
                        <button class="ghost-button small" @click="openUserModal(user)">编辑</button>
                        <button class="ghost-button small" @click="openUserUpstreamModal(user)">渠道</button>
                        <button class="danger-button small" @click="confirmDeleteUser(user)">删除</button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section v-if="usersTab === 'api-keys'" class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr>
                    <th>用户</th>
                    <th>名称</th>
                    <th>前缀</th>
                    <th>状态</th>
                    <th>更新时间</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="key in filteredApiKeys" :key="key.ID">
                    <td>{{ key.User?.Email || key.User?.Username || '-' }}</td>
                    <td>{{ key.Name }}</td>
                    <td>{{ apiKeyPrefix(key.KeyPrefix) }}</td>
                    <td><span class="status-badge">{{ apiKeyStatusLabel(key.Status) }}</span></td>
                    <td>{{ formatDate(key.UpdatedAt || key.CreatedAt) }}</td>
                    <td>
                      <div class="table-actions">
                        <button class="ghost-button small" @click="openApiKeyModal(key)">编辑</button>
                        <button class="ghost-button small" @click="toggleApiKeyStatus(key)">{{ key.Status === 'active' ? '停用' : '启用' }}</button>
                        <button class="danger-button small" @click="confirmDeleteApiKey(key)">删除</button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>

        <div v-if="active === 'announcements'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Announcements</p>
              <h2>公告管理</h2>
              <span>{{ enabledAnnouncements }} 条启用公告，{{ announcements.length }} 条总公告。用户控制台默认展示最新启用公告。</span>
            </div>
            <div class="toolbar-actions">
              <button class="icon-button refresh-button" type="button" :disabled="loading" aria-label="刷新" title="刷新" @click="refreshAdminData">↻</button>
              <button class="primary-button" @click="openAnnouncementModal()">发布公告</button>
            </div>
          </div>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr>
                    <th>公告</th>
                    <th>发布时间</th>
                    <th>排序</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in announcements" :key="item.ID">
                    <td>
                      <strong>{{ item.Title }}</strong>
                      <small>{{ item.Summary || item.Content }}</small>
                    </td>
                    <td>{{ formatDate(item.PublishedAt || item.CreatedAt) }}</td>
                    <td>{{ item.SortOrder }}</td>
                    <td>
                      <span class="status-badge" :class="{ muted: !item.Enabled }">{{ item.Enabled ? '已启用' : '已停用' }}</span>
                      <span v-if="item.Pinned" class="status-badge">置顶</span>
                    </td>
                    <td>
                      <div class="table-actions">
                        <button class="ghost-button small" @click="openAnnouncementModal(item)">编辑</button>
                        <button class="danger-button small" @click="confirmDeleteAnnouncement(item)">删除</button>
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
                  <small>按顺序维护顶部导航，链接可填写 /plans、/models 或完整网址。</small>
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
            <button type="button" :class="{ active: settingsTab === 'endpoints' }" @click="settingsTab = 'endpoints'">API 端点</button>
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
                <span>联系邮箱</span>
                <input v-model="settings.contact_email" type="email" placeholder="support@example.com" />
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

          <section v-if="settingsTab === 'endpoints'" class="panel-surface p-5">
            <div class="section-head mb-5">
              <div>
                <p class="section-kicker">API Endpoints</p>
                <h3>API 端点</h3>
                <span>配置用户控制台展示的 API 接入地址、标签和线路说明。</span>
              </div>
              <button type="button" class="primary-button small" @click="addAPIEndpoint">新增端点</button>
            </div>
            <div class="endpoint-admin-list">
              <article v-for="(endpoint, index) in apiEndpointDraft" :key="index" class="endpoint-admin-item">
                <div class="form-grid">
                  <label class="field">
                    <span>展示标签</span>
                    <input v-model="endpoint.label" placeholder="CN2 优化" @input="syncAPIEndpointSetting" />
                  </label>
                  <label class="field">
                    <span>线路说明</span>
                    <input v-model="endpoint.description" placeholder="国内直连优化线路" @input="syncAPIEndpointSetting" />
                  </label>
                  <label class="field md:col-span-2">
                    <span>API 地址</span>
                    <input v-model="endpoint.url" placeholder="https://api.example.com/v1" @input="syncAPIEndpointSetting" />
                  </label>
                </div>
                <button type="button" class="danger-button small" :disabled="apiEndpointDraft.length <= 1" @click="removeAPIEndpoint(index)">删除</button>
              </article>
            </div>
          </section>

          <section v-if="settingsTab === 'smtp'" class="panel-surface p-5">
            <div class="section-head mb-5">
              <div>
                <p class="section-kicker">Mail</p>
                <h3>SMTP 配置</h3>
              </div>
              <div class="toolbar-actions">
                <label class="toggle-line">
                  <input v-model="settings.smtp_use_tls" type="checkbox" />
                  使用 TLS
                </label>
                <button type="button" class="ghost-button small" :disabled="smtpTesting" @click="sendSMTPTest">
                  {{ smtpTesting ? '发送中...' : '发送测试邮件' }}
                </button>
              </div>
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
              <label class="field md:col-span-2">
                <span>测试收件邮箱</span>
                <input v-model="settings.smtp_test_email" type="email" placeholder="输入一个邮箱用于接收测试邮件" />
              </label>
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
              <option value="public">公共渠道</option>
            </select>
          </label>
          <label v-if="planForm.quota_period === 'public'" class="field">
            <span>绑定公共渠道</span>
            <select v-model="planForm.public_channel_id" required>
              <option value="">请选择公共渠道</option>
              <option v-for="channel in publicChannels.filter((item) => item.Enabled)" :key="channel.ID" :value="channel.ID">{{ channel.Name }}（剩余 {{ usd(channel.RemainingUSDCents) }}）</option>
            </select>
          </label>
          <label class="field"><span>售价（RMB）</span><input v-model.number="planForm.price_rmb" type="number" min="0.01" step="0.01" required /></label>
          <label class="field"><span>{{ planForm.quota_period === 'public' ? '预计总美元额度' : (planForm.quota_period === 'daily' ? '每日美元额度' : '每周美元额度') }}</span><input v-model.number="planForm.period_usd_quota" type="number" min="0" step="0.01" /></label>
          <label v-if="planForm.quota_period !== 'public'" class="field"><span>有效期（天）</span><input v-model.number="planForm.duration_days" type="number" min="1" required /></label>
          <label v-if="planForm.quota_period !== 'public'" class="field"><span>预计总美元额度</span><input :value="totalUsd({ SettlementUSDCents: amountToCents(planForm.period_usd_quota), DurationDays: planForm.duration_days, QuotaPeriod: planForm.quota_period })" readonly /></label>
          <label class="field md:col-span-2"><span>套餐说明</span><textarea v-model="planForm.description" rows="3"></textarea></label>
          <label class="toggle-line md:col-span-2"><input v-model="planForm.enabled" type="checkbox" />启用套餐</label>
        </div>

        <div v-if="modal.type === 'create-channel' || modal.type === 'edit-channel'" class="modal-body form-grid">
          <label class="field"><span>渠道名称</span><input v-model="channelForm.name" required placeholder="OpenAI" /></label>
          <label class="field md:col-span-2"><span>API 地址</span><input v-model="channelForm.base_url" required placeholder="https://api.openai.com" /></label>
          <label class="toggle-line md:col-span-2"><input v-model="channelForm.enabled" type="checkbox" />启用渠道</label>
        </div>

        <div v-if="modal.type === 'create-public-channel' || modal.type === 'edit-public-channel'" class="modal-body form-grid">
          <label class="field"><span>渠道名称</span><input v-model="publicChannelForm.name" required placeholder="公共 OpenAI" /></label>
          <label class="field md:col-span-2"><span>API 地址</span><input v-model="publicChannelForm.base_url" required placeholder="https://api.openai.com" /></label>
          <label class="field md:col-span-2">
            <span>API Key</span>
            <input v-model="publicChannelForm.api_key" type="text" :required="!publicChannelForm.id" :placeholder="publicChannelForm.id ? '留空则不修改' : '请输入公共渠道 API Key'" />
          </label>
          <label class="field"><span>渠道总额度（美元）</span><input v-model.number="publicChannelForm.total_usd_quota" type="number" min="0" step="0.01" required /></label>
          <label class="field"><span>剩余美元额度</span><input v-model.number="publicChannelForm.remaining_usd_quota" type="number" min="0" step="0.01" required /></label>
          <label class="toggle-line md:col-span-2"><input v-model="publicChannelForm.enabled" type="checkbox" />启用公共渠道</label>
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
          <label class="toggle-line md:col-span-2"><input v-model="modelForm.featured" type="checkbox" />展示在 /models 顶部卡片中</label>
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

        <div v-if="modal.type === 'create-announcement' || modal.type === 'edit-announcement'" class="modal-body form-grid">
          <label class="field md:col-span-2">
            <span>公告标题</span>
            <input v-model="announcementForm.title" required placeholder="【2026-05-14】服务更新说明" />
          </label>
          <label class="field md:col-span-2">
            <span>摘要</span>
            <textarea v-model="announcementForm.summary" rows="3" placeholder="收起状态下展示的短内容，留空时自动使用正文前段"></textarea>
          </label>
          <label class="field md:col-span-2">
            <span>公告正文</span>
            <textarea v-model="announcementForm.content" class="markdown-editor announcement-editor" rows="8" required placeholder="支持换行展示。可填写更新说明、使用提醒、教程地址等内容。"></textarea>
          </label>
          <label class="field">
            <span>链接文案</span>
            <input v-model="announcementForm.link_text" placeholder="教程地址" />
          </label>
          <label class="field">
            <span>链接地址</span>
            <input v-model="announcementForm.link_url" placeholder="https://docs.example.com/..." />
          </label>
          <label class="field">
            <span>发布时间</span>
            <input v-model="announcementForm.published_at" type="datetime-local" />
          </label>
          <label class="field">
            <span>排序</span>
            <input v-model.number="announcementForm.sort_order" type="number" />
          </label>
          <label class="toggle-line"><input v-model="announcementForm.pinned" type="checkbox" />置顶公告</label>
          <label class="toggle-line"><input v-model="announcementForm.enabled" type="checkbox" />启用公告</label>
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
          <label v-if="requiresUserUpstreamRebind(userForm)" class="field">
            <span>重新绑定上游渠道</span>
            <select v-model="userForm.channel_id" required>
              <option value="">请选择渠道</option>
              <option v-for="channel in channels.filter((item) => item.Enabled)" :key="channel.ID" :value="channel.ID">{{ channel.Name }}</option>
            </select>
          </label>
          <label v-if="requiresUserUpstreamRebind(userForm)" class="field md:col-span-2">
            <span>上游账号</span>
            <input v-model="userForm.upstream_username" required placeholder="请输入上游账号" />
          </label>
          <label v-if="requiresUserUpstreamRebind(userForm)" class="field">
            <span>上游密码</span>
            <input v-model="userForm.upstream_password" type="text" required placeholder="请输入上游密码" />
          </label>
          <label v-if="requiresUserUpstreamRebind(userForm)" class="field">
            <span>新的上游 API Key</span>
            <input v-model="userForm.api_key" type="text" required placeholder="修改套餐后必须重新绑定" />
          </label>
          <label class="toggle-line md:col-span-2"><input v-model="userForm.email_verified" type="checkbox" />邮箱已验证</label>
        </div>

        <div v-if="modal.type === 'edit-api-key'" class="modal-body form-grid">
          <label class="field md:col-span-2">
            <span>名称</span>
            <input v-model="apiKeyForm.name" required placeholder="默认名称" />
          </label>
          <label class="field">
            <span>状态</span>
            <select v-model="apiKeyForm.status">
              <option value="active">启用</option>
              <option value="disabled">停用</option>
            </select>
          </label>
        </div>

        <div v-if="modal.type === 'user-upstream'" class="modal-body form-grid">
          <label class="field"><span>用户</span><input :value="modal.payload?.user?.Email || '-'" readonly /></label>
          <label class="field"><span>状态</span><input :value="statusLabel(modal.payload?.upstream?.Status || '-')" readonly /></label>
          <label class="field"><span>上游渠道</span><input :value="modal.payload?.upstream?.Channel || '-'" readonly /></label>
          <label class="field md:col-span-2"><span>API 地址</span><input :value="modal.payload?.upstream?.BaseURL || '-'" readonly /></label>
          <label class="field"><span>上游账号</span><input :value="modal.payload?.upstream?.Username || '-'" readonly /></label>
          <label class="field"><span>上游密码</span><input :value="modal.payload?.upstream?.Password || '-'" readonly /></label>
          <label class="field md:col-span-2"><span>API Key</span><textarea :value="modal.payload?.upstream?.APIKey || '-'" rows="4" readonly></textarea></label>
          <label class="field"><span>最后使用</span><input :value="formatDate(modal.payload?.upstream?.LastUsedAt)" readonly /></label>
          <label class="field"><span>更新时间</span><input :value="formatDate(modal.payload?.upstream?.UpdatedAt)" readonly /></label>
        </div>

        <div v-if="modal.type === 'approve-order'" class="modal-body form-grid">
          <label class="field"><span>订单 ID</span><input v-model="approve.orderId" readonly /></label>
          <label class="field"><span>上游渠道</span>
            <select v-model="approve.channelId" required @change="syncApproveChannel">
              <option value="">请选择渠道</option>
              <option v-for="channel in channels.filter((item) => item.Enabled)" :key="channel.ID" :value="channel.ID">{{ channel.Name }}</option>
            </select>
          </label>
          <label class="field"><span>上游账号</span><input v-model="approve.username" required /></label>
          <label class="field"><span>上游密码</span><input v-model="approve.password" type="text" required /></label>
          <label class="field md:col-span-2"><span>上游 API Key</span><input v-model="approve.apiKey" type="text" required /></label>
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
          <label class="field"><span>上游账号</span><input v-model="approve.username" placeholder="留空不修改" /></label>
          <label class="field"><span>上游密码</span><input v-model="approve.password" type="text" placeholder="留空不修改" /></label>
          <label class="field md:col-span-2"><span>上游 API Key</span><input v-model="approve.apiKey" type="text" placeholder="留空不修改" /></label>
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

        <div v-if="modal.type === 'delete-public-channel'" class="modal-body confirm-copy">
          <strong>确定删除「{{ modal.payload?.channel?.Name }}」吗？</strong>
          <p>删除后绑定到该公共渠道的套餐将无法继续售卖，请先确认没有启用中的公共套餐依赖它。</p>
        </div>

        <div v-if="modal.type === 'delete-model'" class="modal-body confirm-copy">
          <strong>确定删除「{{ modal.payload?.model?.ModelName }}」吗？</strong>
          <p>删除后该模型会使用系统兜底价格计费，建议仅在确认不再使用该模型时删除。</p>
        </div>

        <div v-if="modal.type === 'delete-doc'" class="modal-body confirm-copy">
          <strong>确定删除「{{ modal.payload?.doc?.Title }}」吗？</strong>
          <p>删除后用户侧配置文档页面将不再展示这篇内容。</p>
        </div>

        <div v-if="modal.type === 'delete-announcement'" class="modal-body confirm-copy">
          <strong>确定删除「{{ modal.payload?.announcement?.Title }}」吗？</strong>
          <p>删除后用户控制台和历史公告中都不会再展示这条内容。</p>
        </div>

        <div v-if="modal.type === 'delete-user'" class="modal-body confirm-copy">
          <strong>确定删除「{{ modal.payload?.user?.Email }}」吗？</strong>
          <p>删除用户会移除账号本身，相关订单和密钥关系请在操作前确认。</p>
        </div>

        <div v-if="modal.type === 'delete-api-key'" class="modal-body confirm-copy">
          <strong>确定删除这个 API Key 吗？</strong>
          <p>{{ modal.payload?.key?.Name || '-' }}，删除后将立即失效。</p>
        </div>

        <div class="modal-actions">
          <button type="button" class="ghost-button" @click="closeModal">取消</button>
          <button :class="modal.danger ? 'danger-solid-button' : 'primary-button'">{{ modal.actionLabel }}</button>
        </div>
      </form>
    </div>
  </section>
</template>
