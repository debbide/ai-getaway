<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Bell,
  Connection,
  Cpu,
  CreditCard,
  DataAnalysis,
  Document,
  Menu as MenuIcon,
  Monitor,
  Refresh,
  Setting,
  ShoppingCart,
  Tickets,
  User
} from '@element-plus/icons-vue'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'

const menu = [
  { key: 'overview', label: '总览', hint: '运营数据', icon: DataAnalysis },
  { key: 'plans', label: '套餐管理', hint: '价格与额度', icon: CreditCard },
  { key: 'orders', label: '审核管理', hint: '订单开通', icon: ShoppingCart },
  { key: 'models', label: '模型管理', hint: '计费倍率', icon: Cpu },
  { key: 'channels', label: '渠道管理', hint: '上游接口', icon: Connection },
  { key: 'users', label: '用户管理', hint: '账号与权限', icon: User },
  { key: 'usageRecords', label: '使用记录', hint: '调用日志', icon: Tickets },
  { key: 'announcements', label: '公告管理', hint: '控制台公告', icon: Bell },
  { key: 'docs', label: '配置文档', hint: 'Markdown 内容', icon: Document },
  { key: 'emailTemplates', label: '邮件模板', hint: '通知文案', icon: Monitor },
  { key: 'navigation', label: '导航菜单', hint: '顶部菜单', icon: MenuIcon },
  { key: 'settings', label: '系统设置', hint: '邮件与支付', icon: Setting }
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
  rejected: '已拒绝',
  payment_timeout: '支付超时',
  paid_late: '超时已支付',
  pending_manual_review: '待人工处理'
}

const reviewableOrderStatuses = ['pending_review', 'pending_manual_review', 'paid_late']

const active = ref('overview')
const auth = useAuthStore()
const settingsTab = ref('basic')
const usersTab = ref('users')
const channelsTab = ref('upstream')
const stats = ref({})
const orders = ref([])
const users = ref([])
const apiKeys = ref([])
const usageRecords = ref([])
const usageSummary = ref(null)
const plans = ref([])
const models = ref([])
const modelSource = ref('')
const channels = ref([])
const publicChannels = ref([])
const pollingPools = ref([])
const docs = ref([])
const announcements = ref([])
const emailTemplates = ref([])
const emailTemplateVariables = ref([])
const error = ref('')
const notice = ref('')
const navDraft = ref([])
const apiEndpointDraft = ref([])
const loading = ref(false)
const smtpTesting = ref(false)
const filterTimers = {}
let overviewMetricsTimer = null
const modal = reactive({ open: false, type: '', title: '', actionLabel: '', danger: false, payload: null })
const approve = reactive({ orderId: '', channelId: '', channel: '', baseUrl: '', username: '', password: '', apiKey: '', adminNote: '', planId: '', amountRmb: 0, status: '', planType: '', quotaPeriod: '' })
const rejectForm = reactive({ orderId: '', adminNote: '' })
const planForm = reactive(emptyPlan())
const modelForm = reactive(emptyModel())
const channelForm = reactive(emptyChannel())
const publicChannelForm = reactive(emptyPublicChannel())
const pollingPoolForm = reactive(emptyPollingPool())
const userForm = reactive(emptyUser())
const apiKeyForm = reactive(emptyApiKey())
const docForm = reactive(emptyDoc())
const announcementForm = reactive(emptyAnnouncement())
const emailTemplateForm = reactive(emptyEmailTemplate())
const userSearch = reactive({ keyword: '', role: '', status: '', plan: '' })
const planSearch = reactive({ keyword: '', status: '', category: 'daily' })
const orderSearch = reactive({ keyword: '', status: '', planId: '', paymentMethod: '' })
const channelSearch = reactive({ keyword: '', status: '' })
const publicChannelSearch = reactive({ keyword: '', status: '' })
const pollingPoolSearch = reactive({ keyword: '', status: '' })
const apiKeySearch = reactive({ keyword: '', status: '' })
const usageSearch = reactive({ userKeyword: '', apiKeyKeyword: '', range: '7d' })
const announcementSearch = reactive({ keyword: '', status: '' })
const docSearch = reactive({ keyword: '', status: '', groupName: '' })
const pagination = reactive({
  plans: { page: 1, pageSize: 10 },
  orders: { page: 1, pageSize: 10 },
  models: { page: 1, pageSize: 10 },
  upstreamChannels: { page: 1, pageSize: 10 },
  publicChannels: { page: 1, pageSize: 10 },
  pollingPools: { page: 1, pageSize: 10 },
  users: { page: 1, pageSize: 10 },
  apiKeys: { page: 1, pageSize: 10 },
  usageRecords: { page: 1, pageSize: 20 },
  announcements: { page: 1, pageSize: 10 },
  docs: { page: 1, pageSize: 10 }
})
const listTotals = reactive({
  plans: 0,
  orders: 0,
  upstreamChannels: 0,
  publicChannels: 0,
  pollingPools: 0,
  users: 0,
  usageRecords: 0,
  announcements: 0,
  docs: 0
})
const settings = reactive({
  site_title: '',
  contact_email: '',
  api_endpoints: '',
  navigation_items: '',
  pricing_title: '',
  pricing_subtitle: '',
  pricing_notice: '',
  allow_registration: true,
  smtp_host: '',
  smtp_port: 587,
  smtp_username: '',
  smtp_password: '',
  smtp_from_email: '',
  smtp_from_name: '',
  smtp_use_tls: true,
  order_payment_admin_email_enabled: false,
  order_approved_user_email_enabled: false,
  subscription_expire_email_enabled: false,
  subscription_expire_remind_days: 3,
  epay_pid: '',
  epay_key: '',
  epay_notify_url: '',
  epay_return_url: '',
  epay_submit_url: '',
  online_payment_enabled: true,
  manual_payment_enabled: true,
  mock_api_online_enabled: false,
  mock_api_online_base: 0,
  manual_payment_qr_code: '',
  smtp_password_configured: false,
  epay_key_configured: false,
  smtp_test_email: ''
})

const pendingOrders = computed(() => stats.value.pending_orders ?? orders.value.filter((order) => reviewableOrderStatuses.includes(order.Status)).length)
const currentMenu = computed(() => menu.find((item) => item.key === active.value) || menu[0])
const adminDisplayName = computed(() => auth.user?.email || auth.user?.username || 'Admin')
const modalDialogWidth = computed(() => {
  if (modal.type === 'create-polling-pool' || modal.type === 'edit-polling-pool') return '980px'
  return '760px'
})
const enabledPlans = computed(() => stats.value.enabled_plans ?? plans.value.filter((plan) => plan.Enabled).length)
const enabledModels = computed(() => models.value.filter((item) => item.Status === 'active').length)
const approvedUsers = computed(() => stats.value.approved_users ?? users.value.filter((user) => user.Status === 'approved').length)
const enabledChannels = computed(() => channels.value.filter((channel) => channel.Enabled).length)
const enabledPublicChannels = computed(() => publicChannels.value.filter((channel) => channel.Enabled).length)
const enabledPollingPools = computed(() => pollingPools.value.filter((pool) => pool.Enabled).length)
const enabledDocs = computed(() => docs.value.filter((doc) => doc.Enabled).length)
const enabledAnnouncements = computed(() => announcements.value.filter((item) => item.Enabled).length)
const enabledEmailTemplates = computed(() => emailTemplates.value.filter((item) => item.Enabled).length)
const pendingReviewOrders = computed(() => orders.value.filter((order) => reviewableOrderStatuses.includes(order.Status)))
const overviewPlans = computed(() => plans.value.slice(0, 4))
const hasMorePlans = computed(() => plans.value.length > 4)
const filteredApiKeys = computed(() => {
  const keyword = String(apiKeySearch.keyword || '').trim().toLowerCase()
  const status = String(apiKeySearch.status || '')
  return apiKeys.value.filter((key) => {
    const matchesKeyword = !keyword || [key.ID, key.Name, key.KeyPrefix, key.User?.Username, key.User?.Email].some((value) => String(value || '').toLowerCase().includes(keyword))
    const matchesStatus = !status || key.Status === status
    return matchesKeyword && matchesStatus
  })
})
const pagedPlans = computed(() => plans.value)
const pagedOrders = computed(() => orders.value)
const pagedModels = computed(() => paginateItems(models.value, pagination.models))
const pagedUsers = computed(() => users.value)
const pagedApiKeys = computed(() => paginateItems(filteredApiKeys.value, pagination.apiKeys))
const pagedUsageRecords = computed(() => usageRecords.value)
const pagedAnnouncements = computed(() => announcements.value)
const pagedDocs = computed(() => docs.value)
const pagedUpstreamChannels = computed(() => channels.value)
const pagedPublicChannels = computed(() => publicChannels.value)
const pagedPollingPools = computed(() => pollingPools.value)

function responseData(result, fallback) {
  return result.status === 'fulfilled' ? result.value.data : fallback
}

function unwrapListData(payload, fallback = []) {
  if (Array.isArray(payload)) return payload
  if (Array.isArray(payload?.items)) return payload.items
  return fallback
}

function applyListData(key, target, payload, fallback = []) {
  const items = unwrapListData(payload, fallback)
  target.value = items
  if (payload && !Array.isArray(payload)) {
    listTotals[key] = Number(payload.total ?? items.length)
    pagination[key].page = Number(payload.page || pagination[key].page)
    pagination[key].pageSize = Number(payload.page_size || pagination[key].pageSize)
  } else {
    listTotals[key] = items.length
  }
}

function collectLoadErrors(results) {
  return results.filter((item) => item.status === 'rejected').map((item) => item.reason?.message).filter(Boolean)
}

onMounted(async () => {
  await loadAll()
  startOverviewMetricsPolling()
})

watch(() => [userSearch.keyword, userSearch.role, userSearch.status, userSearch.plan], () => scheduleFilterRefresh('users'))
watch(() => [planSearch.keyword, planSearch.status, planSearch.category], () => scheduleFilterRefresh('plans'))
watch(() => [orderSearch.keyword, orderSearch.status, orderSearch.planId, orderSearch.paymentMethod], () => scheduleFilterRefresh('orders'))
watch(() => [channelSearch.keyword, channelSearch.status], () => scheduleFilterRefresh('upstreamChannels'))
watch(() => [publicChannelSearch.keyword, publicChannelSearch.status], () => scheduleFilterRefresh('publicChannels'))
watch(() => [pollingPoolSearch.keyword, pollingPoolSearch.status], () => scheduleFilterRefresh('pollingPools'))
watch(() => [usageSearch.userKeyword, usageSearch.apiKeyKeyword, usageSearch.range], () => scheduleFilterRefresh('usageRecords'))
watch(() => [announcementSearch.keyword, announcementSearch.status], () => scheduleFilterRefresh('announcements'))
watch(() => [docSearch.keyword, docSearch.groupName, docSearch.status], () => scheduleFilterRefresh('docs'))
watch(usersTab, () => {
  if (active.value === 'users') refreshActiveData()
})
watch(channelsTab, () => {
  if (active.value === 'channels') refreshActiveData()
})

onBeforeUnmount(() => {
  Object.values(filterTimers).forEach((timer) => clearTimeout(timer))
  stopOverviewMetricsPolling()
})

watch(active, (value) => {
  refreshActiveData()
  if (value === 'overview') {
    refreshOverviewMetrics()
    startOverviewMetricsPolling()
    return
  }
  stopOverviewMetricsPolling()
})

watch(error, (message) => {
  if (message) ElMessage.error(message)
})

watch(notice, (message) => {
  if (message) ElMessage.success(message)
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
    polling_pool_id: '',
    delivery_source: 'public',
    price_rmb: 9.9,
    is_free: false,
    free_per_user_limit: 1,
    free_total_limit: 0,
    period_usd_quota: 20,
    price_cents: 990,
    settlement_usd_cents: 2000,
    duration_days: 30,
    description: '',
    is_lottery: false,
    lottery_url: '',
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
    has_upstream: false,
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
    supports_gpt: true,
    supports_claude: false,
    enabled: true
  }
}

function emptyPublicChannel() {
  return {
    id: null,
    name: '',
    base_url: '',
    api_key: '',
    supports_gpt: true,
    supports_claude: false,
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

function emptyEmailTemplate() {
  return {
    type: '',
    name: '',
    description: '',
    subject: '',
    body: '',
    enabled: true
  }
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const results = await Promise.allSettled([
      api.get('/admin/stats'),
      api.get('/admin/orders', orderFilterParams()),
      api.get('/admin/users', userFilterParams()),
      api.get('/admin/plans', planFilterParams()),
      api.get('/admin/models'),
      api.get('/admin/upstream-channels', upstreamChannelFilterParams()),
      api.get('/admin/public-channels', publicChannelFilterParams()),
      api.get('/admin/polling-pools', pollingPoolFilterParams()),
      api.get('/admin/keys'),
      api.get('/admin/usage/logs', usageRecordFilterParams()),
      api.get('/admin/docs', docFilterParams()),
      api.get('/admin/announcements', announcementFilterParams()),
      api.get('/admin/email-templates'),
      api.get('/admin/settings')
    ])
    const [statsRes, ordersRes, usersRes, plansRes, modelsRes, channelsRes, publicChannelsRes, pollingPoolsRes, keysRes, usageRecordsRes, docsRes, announcementsRes, emailTemplatesRes, settingsRes] = results
    const modelData = responseData(modelsRes, { items: [], official_source: '' })
    const templateData = responseData(emailTemplatesRes, { items: [], variables: [] })
    stats.value = responseData(statsRes, {})
    applyListData('orders', orders, responseData(ordersRes, { items: [] }))
    applyListData('users', users, responseData(usersRes, { items: [] }))
    applyListData('plans', plans, responseData(plansRes, { items: [] }))
    models.value = modelData?.items || []
    modelSource.value = modelData?.official_source || ''
    applyListData('upstreamChannels', channels, responseData(channelsRes, { items: [] }))
    applyListData('publicChannels', publicChannels, responseData(publicChannelsRes, { items: [] }))
    applyListData('pollingPools', pollingPools, responseData(pollingPoolsRes, { items: [] }))
    apiKeys.value = unwrapListData(responseData(keysRes, []))
    applyUsageRecordData(responseData(usageRecordsRes, { items: [] }))
    applyListData('docs', docs, responseData(docsRes, { items: [] }))
    applyListData('announcements', announcements, responseData(announcementsRes, { items: [] }))
    emailTemplates.value = templateData?.items || []
    emailTemplateVariables.value = templateData?.variables || []
    Object.assign(settings, responseData(settingsRes, {}), { smtp_password: '', epay_key: '' })
    setNavigationDraft(settings.navigation_items)
    setAPIEndpointDraft(settings.api_endpoints)
    const loadErrors = collectLoadErrors(results)
    if (loadErrors.length) error.value = `部分数据暂时加载失败：${loadErrors[0]}`
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

async function refreshAdminData() {
  notice.value = ''
  await refreshActiveData()
}

function setActiveSection(section) {
  if (active.value === section) {
    refreshAdminData()
    return
  }
  active.value = section
}

async function refreshActiveData() {
  loading.value = true
  error.value = ''
  try {
    await loadAdminSection(active.value)
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

async function loadAdminSection(section) {
  switch (section) {
    case 'overview':
      await loadOverviewData()
      break
    case 'plans':
      await loadPlansData()
      break
    case 'orders':
      await loadOrdersData()
      break
    case 'models':
      await loadModelsData()
      break
    case 'channels':
      await loadChannelsData()
      break
    case 'users':
      await loadUsersData()
      break
    case 'usageRecords':
      await loadUsageRecordsData()
      break
    case 'announcements':
      await loadAnnouncementsData()
      break
    case 'docs':
      await loadDocsData()
      break
    case 'emailTemplates':
      await loadEmailTemplatesData()
      break
    case 'navigation':
    case 'settings':
      await loadSettingsData()
      break
    default:
      await loadAll()
  }
}

async function loadOverviewData() {
  const [statsRes, ordersRes, usersRes, plansRes] = await Promise.all([
    api.get('/admin/stats'),
    api.get('/admin/orders', orderFilterParams()),
    api.get('/admin/users', userFilterParams()),
    api.get('/admin/plans', planFilterParams())
  ])
  stats.value = statsRes.data || {}
  applyListData('orders', orders, ordersRes.data || { items: [] })
  applyListData('users', users, usersRes.data || { items: [] })
  applyListData('plans', plans, plansRes.data || { items: [] })
}

async function loadPlansData() {
  const [plansRes, publicChannelsRes, pollingPoolsRes] = await Promise.all([
    api.get('/admin/plans', planFilterParams()),
    api.get('/admin/public-channels', publicChannelFilterParams()),
    api.get('/admin/polling-pools', pollingPoolFilterParams())
  ])
  applyListData('plans', plans, plansRes.data || { items: [] })
  applyListData('publicChannels', publicChannels, publicChannelsRes.data || { items: [] })
  applyListData('pollingPools', pollingPools, pollingPoolsRes.data || { items: [] })
}

async function loadOrdersData() {
  const [ordersRes, plansRes, channelsRes] = await Promise.all([
    api.get('/admin/orders', orderFilterParams()),
    api.get('/admin/plans', allPlansParams()),
    api.get('/admin/upstream-channels', upstreamChannelFilterParams())
  ])
  applyListData('orders', orders, ordersRes.data || { items: [] })
  plans.value = unwrapListData(plansRes.data || [])
  applyListData('upstreamChannels', channels, channelsRes.data || { items: [] })
}

async function loadModelsData() {
  const res = await api.get('/admin/models')
  models.value = res.data?.items || []
  modelSource.value = res.data?.official_source || ''
}

async function loadChannelsData() {
  const [channelsRes, publicChannelsRes, pollingPoolsRes] = await Promise.all([
    api.get('/admin/upstream-channels', upstreamChannelFilterParams()),
    api.get('/admin/public-channels', publicChannelFilterParams()),
    api.get('/admin/polling-pools', pollingPoolFilterParams())
  ])
  applyListData('upstreamChannels', channels, channelsRes.data || { items: [] })
  applyListData('publicChannels', publicChannels, publicChannelsRes.data || { items: [] })
  applyListData('pollingPools', pollingPools, pollingPoolsRes.data || { items: [] })
}

async function loadUsersData() {
  const [usersRes, plansRes, channelsRes, keysRes] = await Promise.all([
    api.get('/admin/users', userFilterParams()),
    api.get('/admin/plans', allPlansParams()),
    api.get('/admin/upstream-channels', upstreamChannelFilterParams()),
    api.get('/admin/keys')
  ])
  applyListData('users', users, usersRes.data || { items: [] })
  plans.value = unwrapListData(plansRes.data || [])
  applyListData('upstreamChannels', channels, channelsRes.data || { items: [] })
  apiKeys.value = unwrapListData(keysRes.data || [])
}

async function loadUsageRecordsData() {
  const res = await api.get('/admin/usage/logs', usageRecordFilterParams())
  applyUsageRecordData(res.data || { items: [] })
}

async function loadAnnouncementsData() {
  const res = await api.get('/admin/announcements', announcementFilterParams())
  applyListData('announcements', announcements, res.data || { items: [] })
}

async function loadDocsData() {
  const res = await api.get('/admin/docs', docFilterParams())
  applyListData('docs', docs, res.data || { items: [] })
}

async function loadEmailTemplatesData() {
  const res = await api.get('/admin/email-templates')
  emailTemplates.value = res.data?.items || []
  emailTemplateVariables.value = res.data?.variables || []
}

async function loadSettingsData() {
  const res = await api.get('/admin/settings')
  Object.assign(settings, res.data, { smtp_password: '', epay_key: '' })
  setNavigationDraft(settings.navigation_items)
  setAPIEndpointDraft(settings.api_endpoints)
}

function userFilterParams() {
  return listRequestParams('users', {
    q: userSearch.keyword,
    role: userSearch.role,
    status: userSearch.status,
    plan: userSearch.plan
  })
}

function planFilterParams() {
  return listRequestParams('plans', {
    q: planSearch.keyword,
    status: planSearch.status,
    category: planSearch.category
  })
}

function allPlansParams() {
  return { params: { page: 1, page_size: 200 } }
}

function orderFilterParams() {
  return listRequestParams('orders', {
    q: orderSearch.keyword,
    status: orderSearch.status,
    plan_id: orderSearch.planId,
    payment_method: orderSearch.paymentMethod
  })
}

function upstreamChannelFilterParams() {
  return listRequestParams('upstreamChannels', {
    q: channelSearch.keyword,
    status: channelSearch.status
  })
}

function publicChannelFilterParams() {
  return listRequestParams('publicChannels', {
    q: publicChannelSearch.keyword,
    status: publicChannelSearch.status
  })
}

function pollingPoolFilterParams() {
  return listRequestParams('pollingPools', {
    q: pollingPoolSearch.keyword,
    status: pollingPoolSearch.status
  })
}

function announcementFilterParams() {
  return listRequestParams('announcements', {
    q: announcementSearch.keyword,
    status: announcementSearch.status
  })
}

function docFilterParams() {
  return listRequestParams('docs', {
    q: docSearch.keyword,
    status: docSearch.status,
    group_name: docSearch.groupName
  })
}

function usageRecordFilterParams() {
  return listRequestParams('usageRecords', {
    user_keyword: usageSearch.userKeyword,
    api_key_keyword: usageSearch.apiKeyKeyword,
    range: usageSearch.range
  })
}

function listRequestParams(key, filters = {}) {
  return {
    params: cleanParams({
      ...filters,
      page: pagination[key].page,
      page_size: pagination[key].pageSize
    })
  }
}

function emptyPollingPool() {
  return {
    id: null,
    name: '',
    supports_gpt: true,
    supports_claude: false,
    enabled: true,
    accounts: [emptyPollingPoolAccount()]
  }
}

function emptyPollingPoolAccount() {
  return {
    id: null,
    name: '',
    base_url: '',
    api_key: '',
    total_usd_quota: 300,
    remaining_usd_quota: 300,
    enabled: true,
    sort_order: 0
  }
}

function cleanParams(params) {
  return Object.fromEntries(Object.entries(params).filter(([, value]) => value !== '' && value !== null && value !== undefined))
}

function applyUsageRecordData(payload = {}) {
  const items = unwrapListData(payload, [])
  usageRecords.value = items
  usageSummary.value = payload?.summary || null
  listTotals.usageRecords = Number(payload?.total ?? items.length)
  pagination.usageRecords.page = Number(payload?.page || pagination.usageRecords.page)
  pagination.usageRecords.pageSize = Number(payload?.page_size || pagination.usageRecords.pageSize)
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
      polling_pool_id: plan.PollingPoolID || plan.PollingPool?.ID || '',
      delivery_source: plan.PollingPoolID || plan.PollingPool?.ID ? 'pool' : 'public',
      price_rmb: centsToAmount(plan.PriceCents),
      is_free: Number(plan.PriceCents || 0) === 0 && !plan.IsLottery,
      free_per_user_limit: plan.FreePerUserLimit || 1,
      free_total_limit: plan.FreeTotalLimit || 0,
      period_usd_quota: centsToAmount(plan.SettlementUSDCents),
      price_cents: plan.PriceCents,
      settlement_usd_cents: plan.SettlementUSDCents,
      duration_days: plan.DurationDays,
      description: plan.Description,
      is_lottery: Boolean(plan.IsLottery),
      lottery_url: plan.LotteryURL || '',
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
      supports_gpt: channel.SupportsGPT !== false,
      supports_claude: Boolean(channel.SupportsClaude),
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
      supports_gpt: channel.SupportsGPT !== false,
      supports_claude: Boolean(channel.SupportsClaude),
      total_usd_quota: centsToAmount(channel.TotalUSDCents),
      remaining_usd_quota: centsToAmount(channel.RemainingUSDCents),
      enabled: channel.Enabled
    })
  }
  showModal(channel ? 'edit-public-channel' : 'create-public-channel', channel ? '编辑公共渠道' : '新增公共渠道', channel ? '保存修改' : '创建公共渠道')
}

function openPollingPoolModal(pool = null) {
  Object.assign(pollingPoolForm, emptyPollingPool())
  if (pool) {
    Object.assign(pollingPoolForm, {
      id: pool.ID,
      name: pool.Name,
      supports_gpt: pool.SupportsGPT !== false,
      supports_claude: Boolean(pool.SupportsClaude),
      enabled: pool.Enabled,
      accounts: (pool.Accounts || []).map((account) => ({
        id: account.ID,
        name: account.Name || '',
        base_url: account.BaseURL || '',
        api_key: account.APIKey || '',
        total_usd_quota: centsToAmount(account.TotalUSDCents),
        remaining_usd_quota: centsToAmount(account.RemainingUSDCents),
        enabled: account.Enabled,
        sort_order: account.SortOrder || 0
      }))
    })
    if (!pollingPoolForm.accounts.length) pollingPoolForm.accounts = [emptyPollingPoolAccount()]
  }
  showModal(pool ? 'edit-polling-pool' : 'create-polling-pool', pool ? '编辑轮询号池' : '新增轮询号池', pool ? '保存修改' : '创建轮询号池')
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

function openEmailTemplateModal(item) {
  Object.assign(emailTemplateForm, emptyEmailTemplate(), {
    type: item.Type,
    name: item.Name,
    description: item.Description || '',
    subject: item.Subject || '',
    body: item.Body || '',
    enabled: Boolean(item.Enabled)
  })
  showModal('edit-email-template', `编辑邮件模板：${item.Name}`, '保存模板')
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

async function submitEmailTemplate() {
  const payload = {
    name: emailTemplateForm.name,
    description: emailTemplateForm.description,
    subject: emailTemplateForm.subject,
    body: emailTemplateForm.body,
    enabled: emailTemplateForm.enabled
  }
  await runAction(async () => {
    await api.put(`/admin/email-templates/${emailTemplateForm.type}`, payload)
    notice.value = '邮件模板已保存'
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

async function submitPollingPool() {
  const payload = normalizePollingPool(pollingPoolForm)
  await runAction(async () => {
    if (pollingPoolForm.id) {
      await api.put(`/admin/polling-pools/${pollingPoolForm.id}`, payload)
      notice.value = '轮询号池已更新'
    } else {
      await api.post('/admin/polling-pools', payload)
      notice.value = '轮询号池已创建'
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

function confirmDeletePollingPool(pool) {
  showModal('delete-polling-pool', '删除轮询号池', '确认删除', { pool }, true)
}

async function deletePublicChannel() {
  await runAction(async () => {
    await api.delete(`/admin/public-channels/${modal.payload.channel.ID}`)
    notice.value = '公共渠道已删除'
  })
}

async function deletePollingPool() {
  await runAction(async () => {
    await api.delete(`/admin/polling-pools/${modal.payload.pool.ID}`)
    notice.value = '轮询号池已删除'
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
      has_upstream: Boolean(user.Upstream),
      channel_id: channel?.ID || '',
      upstream_username: upstream.Username || '',
      upstream_password: upstream.Password || '',
      api_key: upstream.APIKey || ''
    })
  }
  showModal(user ? 'edit-user' : 'create-user', user ? '编辑用户' : '新增用户', user ? '保存修改' : '创建用户')
}

async function submitUser() {
  const validationMessage = validateUserForm(userForm)
  if (validationMessage) {
    error.value = validationMessage
    return
  }
  if (shouldEditUserUpstream(userForm) && (!Number(userForm.channel_id) || !String(userForm.upstream_username || '').trim() || !String(userForm.upstream_password || '').trim() || !String(userForm.api_key || '').trim())) {
    error.value = '编辑上游渠道时，必须填写渠道、上游账号、上游密码和 API Key'
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
    if (err.status === 404 && err.rawMessage === 'upstream account not found') {
      showModal('user-upstream', `渠道 #${user.ID}`, '关闭', { user, upstream: null })
      return
    }
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
  const isPublic = isPublicOrder(order)
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
    amountRmb: centsToAmount(order.AmountCents),
    status: order.Status,
    planType: order.Plan?.PlanType || '',
    quotaPeriod: order.Plan?.QuotaPeriod || '',
  })
  showModal('approve-order', `审核订单 #${order.ID}`, isPublic ? '确认通过' : '通过并开通', { order })
}

function openEditOrderModal(order) {
  const upstream = order.Upstream || {}
  const channel = channels.value.find((item) => item.Name === upstream.Channel) || null
  const isPublic = isPublicOrder(order)
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
    amountRmb: centsToAmount(order.AmountCents),
    status: order.Status,
    planType: order.Plan?.PlanType || '',
    quotaPeriod: order.Plan?.QuotaPeriod || '',
  })
  showModal('edit-order', `编辑订单 #${order.ID}`, '保存修改', { order })
}

function openRejectModal(order) {
  Object.assign(rejectForm, { orderId: String(order.ID), adminNote: '' })
  showModal('reject-order', `拒绝订单 #${order.ID}`, '确认拒绝', null, true)
}

function selectedApproveChannel() {
  return channels.value.find((channel) => String(channel.ID) === String(approve.channelId)) || null
}

function syncApproveChannel() {
  if (approveOrderUsesPublicChannel()) {
    approve.channel = ''
    approve.baseUrl = ''
    approve.channelId = ''
    return
  }
  const channel = selectedApproveChannel()
  approve.channel = channel?.Name || ''
  approve.baseUrl = channel?.BaseURL || ''
}

async function approveOrder() {
  syncApproveChannel()
  if (approveOrderUsesPublicChannel()) {
    await runAction(async () => {
      await api.post(`/admin/orders/${approve.orderId}/complete-payment`)
      notice.value = '订单已确认并开通'
    })
    return
  }
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
    admin_note: approve.adminNote,
    amount_cents: amountToCents(approve.amountRmb)
  }
  if (!approveOrderUsesPublicChannel()) {
    Object.assign(payload, {
      channel_id: Number(approve.channelId) || undefined,
      channel: approve.channel,
      base_url: approve.baseUrl,
      username: approve.username,
      password: approve.password,
      api_key: approve.apiKey
    })
  }
  if (approve.status !== 'approved') {
    payload.plan_id = Number(approve.planId) || undefined
  }
  await runAction(async () => {
    await api.put(`/admin/orders/${approve.orderId}`, payload)
    notice.value = '订单已保存'
  })
}

async function completeOrderPayment(order) {
  await runAction(async () => {
    await api.post(`/admin/orders/${order.ID}/complete-payment`)
    notice.value = '订单已手动确认支付'
  }, false)
}

async function rejectOrder() {
  await runAction(async () => {
    await api.post(`/admin/orders/${rejectForm.orderId}/reject`, { admin_note: rejectForm.adminNote })
    notice.value = '订单已拒绝'
  })
}

function confirmCloseOrder(order) {
  showModal('close-order', `关闭订单 #${order.ID}`, '确认关闭', { order }, true)
}

async function closeOrder() {
  await runAction(async () => {
    await api.post(`/admin/orders/${modal.payload.order.ID}/close`, { admin_note: '管理员关闭订单' })
    notice.value = '订单已关闭'
  })
}

function confirmDeleteOrder(order) {
  showModal('delete-order', `删除订单 #${order.ID}`, '确认删除', { order }, true)
}

async function deleteOrder() {
  await runAction(async () => {
    await api.delete(`/admin/orders/${modal.payload.order.ID}`)
    notice.value = '订单已删除'
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

function handleManualPaymentQRUpload(event) {
  const file = event.target.files?.[0]
  if (!file) return
  if (!file.type.startsWith('image/')) {
    error.value = '请上传图片格式的付款二维码'
    event.target.value = ''
    return
  }
  if (file.size > 1024 * 1024) {
    error.value = '付款二维码图片不能超过 1MB'
    event.target.value = ''
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    settings.manual_payment_qr_code = String(reader.result || '')
  }
  reader.onerror = () => {
    error.value = '二维码读取失败，请重新选择图片'
  }
  reader.readAsDataURL(file)
}

function clearManualPaymentQR() {
  settings.manual_payment_qr_code = ''
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
  const usePool = isPublic && plan.delivery_source === 'pool'
  return {
    name: plan.name.trim(),
    code: plan.code.trim(),
    badge_text: plan.badge_text.trim(),
    plan_type: isPublic ? 'public' : 'subscription',
    quota_period: isPublic ? 'public' : plan.quota_period,
    public_channel_id: isPublic && !usePool ? Number(plan.public_channel_id || 0) : null,
    polling_pool_id: usePool ? Number(plan.polling_pool_id || 0) : null,
    price_cents: plan.is_lottery || plan.is_free ? 0 : amountToCents(plan.price_rmb),
    settlement_usd_cents: amountToCents(plan.period_usd_quota),
    duration_days: isPublic ? 1 : Number(plan.duration_days || 1),
    description: plan.description.trim(),
    is_lottery: Boolean(plan.is_lottery),
    lottery_url: plan.lottery_url.trim(),
    free_per_user_limit: plan.is_free ? Number(plan.free_per_user_limit || 1) : 0,
    free_total_limit: plan.is_free ? Number(plan.free_total_limit || 0) : 0,
    enabled: Boolean(plan.enabled)
  }
}

function paginateItems(items, pager) {
  const page = Math.max(1, Number(pager.page || 1))
  const pageSize = Math.max(1, Number(pager.pageSize || 10))
  const start = (page - 1) * pageSize
  return items.slice(start, start + pageSize)
}

function handlePageChange(key, page) {
  pagination[key].page = page
  if (isServerPaginatedKey(key)) refreshPaginatedList(key)
}

function handlePageSizeChange(key, pageSize) {
  pagination[key].pageSize = pageSize
  pagination[key].page = 1
  if (isServerPaginatedKey(key)) refreshPaginatedList(key)
}

function resetPager(key) {
  pagination[key].page = 1
}

function scheduleFilterRefresh(key) {
  resetPager(key)
  if (!isListActive(key)) return
  if (filterTimers[key]) clearTimeout(filterTimers[key])
  filterTimers[key] = setTimeout(() => {
    refreshPaginatedList(key)
  }, 250)
}

function isServerPaginatedKey(key) {
  return ['plans', 'orders', 'upstreamChannels', 'publicChannels', 'pollingPools', 'users', 'usageRecords', 'announcements', 'docs'].includes(key)
}

function isListActive(key) {
  if (key === 'users') return active.value === 'users' && usersTab.value === 'users'
  if (key === 'upstreamChannels') return active.value === 'channels' && channelsTab.value === 'upstream'
  if (key === 'publicChannels') return active.value === 'channels' && channelsTab.value === 'public'
  if (key === 'pollingPools') return active.value === 'channels' && channelsTab.value === 'pool'
  return active.value === key
}

async function refreshPaginatedList(key) {
  if (!isListActive(key)) return
  await refreshActiveData()
}

function normalizePublicChannel(channel) {
  return {
    name: channel.name.trim(),
    base_url: channel.base_url.trim(),
    api_key: channel.api_key.trim(),
    supports_gpt: Boolean(channel.supports_gpt),
    supports_claude: Boolean(channel.supports_claude),
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
    supports_gpt: Boolean(channel.supports_gpt),
    supports_claude: Boolean(channel.supports_claude),
    enabled: Boolean(channel.enabled)
  }
}

function normalizePollingPool(pool) {
  return {
    name: pool.name.trim(),
    supports_gpt: Boolean(pool.supports_gpt),
    supports_claude: Boolean(pool.supports_claude),
    enabled: Boolean(pool.enabled),
    accounts: (pool.accounts || []).map((account, index) => ({
      id: Number(account.id || 0),
      name: String(account.name || '').trim() || `账号${index + 1}`,
      base_url: String(account.base_url || '').trim(),
      api_key: String(account.api_key || '').trim(),
      total_usd_cents: amountToCents(account.total_usd_quota),
      remaining_usd_cents: amountToCents(account.remaining_usd_quota),
      enabled: Boolean(account.enabled),
      sort_order: Number(account.sort_order || index)
    }))
  }
}

function addPollingPoolAccount() {
  pollingPoolForm.accounts.push({ ...emptyPollingPoolAccount(), sort_order: pollingPoolForm.accounts.length })
}

function removePollingPoolAccount(index) {
  if (pollingPoolForm.accounts.length <= 1) return
  pollingPoolForm.accounts.splice(index, 1)
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
  if (shouldEditUserUpstream(user)) {
    payload.channel_id = Number(user.channel_id || 0)
    payload.upstream_username = user.upstream_username.trim()
    payload.upstream_password = user.upstream_password
    payload.api_key = user.api_key
  }
  if (user.password) payload.password = user.password
  return payload
}

function validateUserForm(user) {
  const username = String(user.username || '').trim()
  const email = String(user.email || '').trim()
  const password = String(user.password || '')
  if (username.length < 2 || username.length > 64) return '用户名长度需为 2-64 位'
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) return '请填写正确的邮箱地址'
  if (!user.id && password.length < 8) return '登录密码至少需要 8 位'
  if (user.id && password && password.length < 8) return '新密码至少需要 8 位，留空则不修改'
  return ''
}

function requiresUserUpstreamRebind(user) {
  return user.id && String(user.plan_id || '') !== String(user.original_plan_id || '') && String(user.plan_id || '') !== ''
}

function shouldEditUserUpstream(user) {
  return Boolean(user.id) && (Boolean(user.has_upstream) || requiresUserUpstreamRebind(user))
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

function usdMicros(value) {
  return `$${(Number(value || 0) / 1000000).toFixed(6)}`
}

function usageCost(record) {
  return record?.estimated_usd_micros ? usdMicros(record.estimated_usd_micros) : usd(record?.estimated_usd_cents || 0)
}

function usageUserLabel(record) {
  return record?.user_email || record?.username || `User #${record?.user_id || '-'}`
}

function usageApiKeyLabel(record) {
  const prefix = record?.api_key_prefix ? `${record.api_key_prefix}...` : `Key #${record?.api_key_id || '-'}`
  return record?.api_key_name ? `${record.api_key_name} / ${prefix}` : prefix
}

function usageEndpoint(record) {
  return record?.endpoint || record?.path || '-'
}

function requestTypeLabel(value) {
  return { chat: '对话', stream: '流式' }[value] || '调用'
}

function latency(value) {
  const ms = Number(value || 0)
  if (!ms) return '-'
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${ms}ms`
}

function quotaPeriodLabel(period) {
  if (period === 'public') return '公共'
  return period === 'daily' ? '每日' : '每周'
}

function isPublicPlan(plan) {
  return plan?.PlanType === 'public' || plan?.QuotaPeriod === 'public'
}

function isPublicOrder(order) {
  return isPublicPlan(order?.Plan)
}

function approveOrderUsesPublicChannel() {
  return approve.planType === 'public' || approve.quotaPeriod === 'public'
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
  if (plan.PollingPoolID || plan.PollingPool) {
    return plan.PollingPool?.Name || pollingPools.value.find((pool) => pool.ID === plan.PollingPoolID)?.Name || '未绑定轮询号池'
  }
  return plan.PublicChannel?.Name || publicChannels.value.find((channel) => channel.ID === plan.PublicChannelID)?.Name || '未绑定公共渠道'
}

function protocolTags(item) {
  const tags = []
  if (item?.SupportsGPT !== false) tags.push('GPT')
  if (item?.SupportsClaude) tags.push('Claude')
  return tags.length ? tags : ['未启用']
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
    'create-polling-pool': submitPollingPool,
    'edit-polling-pool': submitPollingPool,
    'delete-polling-pool': deletePollingPool,
    'create-doc': submitDoc,
    'edit-doc': submitDoc,
    'delete-doc': deleteDoc,
    'create-announcement': submitAnnouncement,
    'edit-announcement': submitAnnouncement,
    'delete-announcement': deleteAnnouncement,
    'edit-email-template': submitEmailTemplate,
    'create-user': submitUser,
    'edit-user': submitUser,
    'edit-api-key': submitApiKey,
    'user-upstream': closeModal,
    'delete-user': deleteUser,
    'delete-api-key': deleteApiKey,
    'approve-order': approveOrder,
    'reject-order': rejectOrder,
    'edit-order': editOrder,
    'close-order': closeOrder,
    'delete-order': deleteOrder
  }
  actions[modal.type]?.()
}
</script>

<template>
  <section class="admin-app-shell">
    <aside class="admin-sidebar">
      <div class="admin-brand">
        <span class="admin-brand-mark">AI</span>
        <div>
          <strong>管理后台</strong>
          <small>AI Gateway</small>
        </div>
      </div>
      <nav class="admin-menu-list" aria-label="管理后台菜单">
        <button v-for="item in menu" :key="item.key" class="admin-menu-button" :class="{ active: active === item.key }" type="button" @click="setActiveSection(item.key)">
          <el-icon class="admin-menu-icon"><component :is="item.icon" /></el-icon>
          <div class="admin-menu-item">
            <span>{{ item.label }}</span>
            <small>{{ item.hint }}</small>
          </div>
        </button>
      </nav>
    </aside>

    <div class="admin-main">
      <header class="admin-topbar">
        <div>
          <p class="section-kicker">Admin Center</p>
          <h1>{{ currentMenu.label }}</h1>
          <span>{{ currentMenu.hint }}</span>
        </div>
        <div class="admin-topbar-actions">
          <el-button circle :icon="Refresh" :loading="loading" aria-label="刷新" title="刷新" @click="refreshAdminData" />
          <div class="admin-status-chip">
            <span>{{ pendingOrders }}</span>
            <small>待审核</small>
          </div>
          <div class="admin-user-chip">
            <strong>{{ adminDisplayName }}</strong>
            <small>Administrator</small>
          </div>
        </div>
      </header>

      <main class="admin-content">
        <div class="min-w-0">
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
                <el-button @click="setActiveSection('orders')">查看全部</el-button>
              </div>
              <div class="mt-4 grid gap-3">
                <article v-for="order in pendingReviewOrders.slice(0, 4)" :key="order.ID" class="list-row">
                  <div>
                    <strong>#{{ order.ID }} · {{ order.User?.Email || '未知用户' }}</strong>
                    <span>{{ order.Plan?.Name || '未关联套餐' }} · {{ money(order.AmountCents) }}</span>
                  </div>
                  <el-button type="primary" size="small" @click="openApproveModal(order)">审核</el-button>
                </article>
              </div>
            </section>

            <section class="panel-surface p-5">
              <div class="section-head">
                <div>
                  <p class="section-kicker">Plans</p>
                  <h3>套餐状态</h3>
                </div>
                <el-button @click="openPlanModal()">新增</el-button>
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
                <el-button @click="setActiveSection('plans')">更多</el-button>
              </div>
            </section>
          </div>
        </div>

        <div v-if="active === 'plans'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Pricing</p>
              <h2>套餐管理</h2>
              <span>{{ enabledPlans }} 个启用套餐，{{ listTotals.plans }} 个筛选结果</span>
            </div>
            <div class="toolbar-actions">
              <el-button circle :icon="Refresh" :loading="loading" aria-label="刷新" title="刷新" @click="refreshAdminData" />
              <el-button type="primary" @click="openPlanModal()">新增套餐</el-button>
            </div>
          </div>

          <section class="panel-surface p-4">
            <el-segmented
              v-model="planSearch.category"
              class="settings-tabs mb-4"
              :options="[
                { label: '日套餐', value: 'daily' },
                { label: '周套餐', value: 'weekly' },
                { label: '活动套餐', value: 'public' },
                { label: '免费套餐', value: 'free' },
                { label: '抽奖套餐', value: 'lottery' }
              ]"
            />
            <el-form class="form-grid user-filter-grid" label-position="top">
              <el-form-item label="搜索">
                <el-input v-model="planSearch.keyword" clearable placeholder="套餐名 / 编码 / 描述 / ID" @input="resetPager('plans')" />
              </el-form-item>
              <el-form-item label="状态">
                <el-select v-model="planSearch.status" clearable placeholder="全部" @change="resetPager('plans')">
                  <el-option label="全部" value="" />
                  <el-option label="已启用" value="enabled" />
                  <el-option label="已停用" value="disabled" />
                </el-select>
              </el-form-item>
            </el-form>
          </section>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <el-table :data="pagedPlans" class="plans-table" border>
                <el-table-column label="套餐" min-width="260">
                  <template #default="{ row: plan }">
                    <div class="plan-main-cell">
                      <strong>{{ plan.Name }}</strong>
                      <small>{{ plan.Code || '未设置编码' }}</small>
                      <span>{{ plan.Description || '暂无说明' }}</span>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="价格" width="150">
                  <template #default="{ row: plan }">
                    <div class="plan-price-cell">
                      <strong>{{ plan.IsLottery ? '抽奖' : (plan.PriceCents === 0 ? '免费' : rmb(plan.PriceCents)) }}</strong>
                      <el-tag :type="plan.IsLottery ? 'warning' : 'info'">{{ plan.IsLottery ? '抽奖套餐' : '购买套餐' }}</el-tag>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="额度" min-width="210">
                  <template #default="{ row: plan }">
                    <div class="plan-quota-cell">
                      <span>{{ quotaPeriodLabel(plan.QuotaPeriod) }}美元额度</span>
                      <strong>{{ usd(plan.SettlementUSDCents) }}</strong>
                      <small>预计总额 {{ totalUsd(plan) }}</small>
                      <small v-if="plan.PriceCents === 0">已领取 {{ plan.FreeClaimedCount || 0 }} / {{ plan.FreeTotalLimit || '不限' }}</small>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="交付" min-width="180">
                  <template #default="{ row: plan }">
                    <div class="plan-delivery-cell">
                      <span v-if="plan.IsLottery">{{ plan.LotteryURL || '未设置跳转地址' }}</span>
                      <span v-else-if="plan.QuotaPeriod === 'public'">{{ publicChannelName(plan) }}</span>
                      <span v-else>{{ plan.DurationDays }} 天</span>
                      <el-tag :type="plan.Enabled ? 'success' : 'info'">{{ plan.Enabled ? '已启用' : '已停用' }}</el-tag>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="150" fixed="right">
                  <template #default="{ row: plan }">
                    <div class="table-actions">
                      <el-button size="small" @click="openPlanModal(plan)">编辑</el-button>
                      <el-button type="danger" size="small" @click="confirmDeletePlan(plan)">删除</el-button>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </div>
            <div class="p-4 flex justify-end">
              <el-pagination
                :current-page="pagination.plans.page"
                :page-size="pagination.plans.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="listTotals.plans"
                background
                layout="total, sizes, prev, pager, next"
                @current-change="handlePageChange('plans', $event)"
                @size-change="handlePageSizeChange('plans', $event)"
              />
            </div>
          </section>
        </div>

        <div v-if="active === 'orders'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Review</p>
              <h2>审核管理</h2>
              <span>订单审核、搜索、关闭和删除都在这里处理</span>
            </div>
            <el-button circle :icon="Refresh" :loading="loading" aria-label="刷新" title="刷新" @click="refreshAdminData" />
          </div>

          <section class="panel-surface p-4">
            <el-form class="form-grid user-filter-grid" label-position="top">
              <el-form-item label="搜索">
                <el-input v-model="orderSearch.keyword" clearable placeholder="订单号 / 用户 / 邮箱 / 支付单号" @input="resetPager('orders')" />
              </el-form-item>
              <el-form-item label="状态">
                <el-select v-model="orderSearch.status" clearable placeholder="全部" @change="resetPager('orders')">
                  <el-option label="全部" value="" />
                  <el-option v-for="(label, value) in orderStatusMap" :key="value" :label="label" :value="value" />
                </el-select>
              </el-form-item>
              <el-form-item label="套餐">
                <el-select v-model="orderSearch.planId" clearable filterable placeholder="全部" @change="resetPager('orders')">
                  <el-option label="全部" value="" />
                  <el-option v-for="plan in plans" :key="plan.ID" :label="plan.Name" :value="String(plan.ID)" />
                </el-select>
              </el-form-item>
              <el-form-item label="支付方式">
                <el-select v-model="orderSearch.paymentMethod" clearable placeholder="全部" @change="resetPager('orders')">
                  <el-option label="全部" value="" />
                  <el-option label="在线支付" value="online" />
                  <el-option label="人工支付" value="manual" />
                  <el-option label="免费订单" value="free" />
                </el-select>
              </el-form-item>
            </el-form>
          </section>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <el-table :data="pagedOrders" border>
                <el-table-column label="订单" width="90">
                  <template #default="{ row: order }">#{{ order.ID }}</template>
                </el-table-column>
                <el-table-column label="用户" min-width="220">
                  <template #default="{ row: order }">
                    <strong>{{ order.User?.Email || '-' }}</strong>
                    <small v-if="order.UserPaymentNote">付款备注：{{ order.UserPaymentNote }}</small>
                  </template>
                </el-table-column>
                <el-table-column label="套餐" min-width="150">
                  <template #default="{ row: order }">{{ order.Plan?.Name || '-' }}</template>
                </el-table-column>
                <el-table-column label="上游渠道" min-width="170">
                  <template #default="{ row: order }">{{ isPublicOrder(order) ? (order.Plan?.PublicChannel?.Name || '公共渠道') : (order.Upstream?.Channel || '-') }}</template>
                </el-table-column>
                <el-table-column label="金额" width="110">
                  <template #default="{ row: order }">{{ money(order.AmountCents) }}</template>
                </el-table-column>
                <el-table-column label="状态" width="130">
                  <template #default="{ row: order }"><el-tag>{{ statusLabel(order.Status) }}</el-tag></template>
                </el-table-column>
                <el-table-column label="操作" width="360" fixed="right">
                  <template #default="{ row: order }">
                    <div class="table-actions">
                      <el-button size="small" @click="openEditOrderModal(order)">编辑</el-button>
                      <el-button v-if="order.Status === 'pending_payment'" type="primary" size="small" @click="completeOrderPayment(order)">完成支付</el-button>
                      <el-button v-if="reviewableOrderStatuses.includes(order.Status)" size="small" @click="openApproveModal(order)">审核</el-button>
                      <el-button v-if="reviewableOrderStatuses.includes(order.Status)" type="danger" size="small" @click="openRejectModal(order)">拒绝</el-button>
                      <el-button v-if="order.Status !== 'approved' && order.Status !== 'rejected' && order.Status !== 'payment_timeout'" type="warning" size="small" @click="confirmCloseOrder(order)">关闭</el-button>
                      <el-button v-if="order.Status !== 'approved'" type="danger" plain size="small" @click="confirmDeleteOrder(order)">删除</el-button>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </div>
            <div class="p-4 flex justify-end">
              <el-pagination
                :current-page="pagination.orders.page"
                :page-size="pagination.orders.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="listTotals.orders"
                background
                layout="total, sizes, prev, pager, next"
                @current-change="handlePageChange('orders', $event)"
                @size-change="handlePageSizeChange('orders', $event)"
              />
            </div>
          </section>
        </div>

        <div v-if="active === 'models'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Model Billing</p>
              <h2>模型管理</h2>
              <span>{{ enabledModels }} 个启用模型，{{ models.length }} 个总模型</span>
            </div>
            <div class="toolbar-actions">
              <el-button :loading="loading" @click="syncOfficialModels">同步官方倍率</el-button>
              <el-button circle :icon="Refresh" :loading="loading" aria-label="刷新" title="刷新" @click="refreshAdminData" />
              <el-button type="primary" @click="openModelModal()">新增模型</el-button>
            </div>
          </div>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <el-table :data="pagedModels" class="model-pricing-table" border>
                <el-table-column label="模型" min-width="210"><template #default="{ row: item }"><div class="model-cell"><strong>{{ item.ModelName }}</strong><small>{{ item.DisplayName || item.Provider || '-' }}</small></div></template></el-table-column>
                <el-table-column label="输入单价" min-width="150"><template #default="{ row: item }"><div class="price-cell"><strong>{{ modelActualUnit(item, 'InputUSDPerMillion') }}</strong><small>原价 {{ modelUnit(item.InputUSDPerMillion) }}</small></div></template></el-table-column>
                <el-table-column label="缓存读取" min-width="150"><template #default="{ row: item }"><div class="price-cell"><strong>{{ modelActualUnit(item, 'CachedInputUSDPerMillion') }}</strong><small>原价 {{ modelUnit(item.CachedInputUSDPerMillion) }}</small></div></template></el-table-column>
                <el-table-column label="输出单价" min-width="150"><template #default="{ row: item }"><div class="price-cell"><strong>{{ modelActualUnit(item, 'OutputUSDPerMillion') }}</strong><small>原价 {{ modelUnit(item.OutputUSDPerMillion) }}</small></div></template></el-table-column>
                <el-table-column label="倍率" width="90"><template #default="{ row: item }">{{ Number(item.BillingMultiplier || 1).toFixed(2) }}x</template></el-table-column>
                <el-table-column label="展示卡片" width="110"><template #default="{ row: item }"><el-tag :type="item.Featured ? 'success' : 'info'">{{ item.Featured ? '展示' : '不展示' }}</el-tag></template></el-table-column>
                <el-table-column label="状态" width="110"><template #default="{ row: item }"><el-tag :type="item.Status === 'active' ? 'success' : 'info'">{{ modelStatusLabel(item.Status) }}</el-tag></template></el-table-column>
                <el-table-column label="同步时间" min-width="150"><template #default="{ row: item }">{{ formatSyncTime(item.OfficialSyncedAt) }}</template></el-table-column>
                <el-table-column label="操作" width="150" fixed="right"><template #default="{ row: item }"><div class="table-actions"><el-button size="small" @click="openModelModal(item)">编辑</el-button><el-button type="danger" size="small" @click="confirmDeleteModel(item)">删除</el-button></div></template></el-table-column>
              </el-table>
            </div>
            <div class="p-4 flex justify-end">
              <el-pagination
                :current-page="pagination.models.page"
                :page-size="pagination.models.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="models.length"
                background
                layout="total, sizes, prev, pager, next"
                @current-change="handlePageChange('models', $event)"
                @size-change="handlePageSizeChange('models', $event)"
              />
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
              <span>普通渠道 {{ enabledChannels }}/{{ listTotals.upstreamChannels }}，公共渠道 {{ enabledPublicChannels }}/{{ listTotals.publicChannels }}，轮询号池 {{ enabledPollingPools }}/{{ listTotals.pollingPools }}</span>
            </div>
            <div class="toolbar-actions">
              <el-button circle :icon="Refresh" :loading="loading" aria-label="刷新" title="刷新" @click="refreshAdminData" />
              <el-button v-if="channelsTab === 'upstream'" type="primary" @click="openChannelModal()">新增渠道</el-button>
              <el-button v-else-if="channelsTab === 'public'" type="primary" @click="openPublicChannelModal()">新增公共渠道</el-button>
              <el-button v-else type="primary" @click="openPollingPoolModal()">新增轮询号池</el-button>
            </div>
          </div>

          <el-segmented
            v-model="channelsTab"
            class="settings-tabs"
            :options="[
              { label: '上游渠道', value: 'upstream' },
              { label: '公共渠道', value: 'public' },
              { label: '轮询号池', value: 'pool' }
            ]"
          />

          <section v-if="channelsTab === 'upstream'" class="panel-surface overflow-hidden">
            <div class="p-4 border-b border-slate-100">
              <el-form class="form-grid user-filter-grid" label-position="top">
                <el-form-item label="搜索">
                  <el-input v-model="channelSearch.keyword" clearable placeholder="名称 / 地址 / ID" @input="resetPager('upstreamChannels')" />
                </el-form-item>
                <el-form-item label="状态">
                  <el-select v-model="channelSearch.status" clearable placeholder="全部" @change="resetPager('upstreamChannels')">
                    <el-option label="全部" value="" />
                    <el-option label="已启用" value="enabled" />
                    <el-option label="已停用" value="disabled" />
                  </el-select>
                </el-form-item>
              </el-form>
            </div>
            <div class="table-wrap">
              <el-table :data="pagedUpstreamChannels" border>
                <el-table-column label="渠道名称" min-width="160" prop="Name" />
                <el-table-column label="API 地址" min-width="260" prop="BaseURL" />
                <el-table-column label="支持协议" min-width="150"><template #default="{ row: channel }"><div class="table-actions"><el-tag v-for="tag in protocolTags(channel)" :key="tag" size="small">{{ tag }}</el-tag></div></template></el-table-column>
                <el-table-column label="状态" width="110"><template #default="{ row: channel }"><el-tag :type="channel.Enabled ? 'success' : 'info'">{{ channel.Enabled ? '已启用' : '已停用' }}</el-tag></template></el-table-column>
                <el-table-column label="操作" width="150"><template #default="{ row: channel }"><div class="table-actions"><el-button size="small" @click="openChannelModal(channel)">编辑</el-button><el-button type="danger" size="small" @click="confirmDeleteChannel(channel)">删除</el-button></div></template></el-table-column>
              </el-table>
            </div>
            <div class="p-4 flex justify-end">
              <el-pagination
                :current-page="pagination.upstreamChannels.page"
                :page-size="pagination.upstreamChannels.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="listTotals.upstreamChannels"
                background
                layout="total, sizes, prev, pager, next"
                @current-change="handlePageChange('upstreamChannels', $event)"
                @size-change="handlePageSizeChange('upstreamChannels', $event)"
              />
            </div>
          </section>

          <section v-else-if="channelsTab === 'public'" class="panel-surface overflow-hidden">
            <div class="p-4 border-b border-slate-100">
              <el-form class="form-grid user-filter-grid" label-position="top">
                <el-form-item label="搜索">
                  <el-input v-model="publicChannelSearch.keyword" clearable placeholder="名称 / 地址 / ID" @input="resetPager('publicChannels')" />
                </el-form-item>
                <el-form-item label="状态">
                  <el-select v-model="publicChannelSearch.status" clearable placeholder="全部" @change="resetPager('publicChannels')">
                    <el-option label="全部" value="" />
                    <el-option label="已启用" value="enabled" />
                    <el-option label="已停用" value="disabled" />
                  </el-select>
                </el-form-item>
              </el-form>
            </div>
            <div class="table-wrap">
              <el-table :data="pagedPublicChannels" border>
                <el-table-column label="渠道名称" min-width="160" prop="Name" />
                <el-table-column label="API 地址" min-width="260" prop="BaseURL" />
                <el-table-column label="支持协议" min-width="150"><template #default="{ row: channel }"><div class="table-actions"><el-tag v-for="tag in protocolTags(channel)" :key="tag" size="small">{{ tag }}</el-tag></div></template></el-table-column>
                <el-table-column label="剩余额度 / 总额度" min-width="160"><template #default="{ row: channel }">{{ channelQuotaText(channel) }}</template></el-table-column>
                <el-table-column label="状态" width="110"><template #default="{ row: channel }"><el-tag :type="channel.Enabled && channel.RemainingUSDCents > 0 ? 'success' : 'info'">{{ channel.RemainingUSDCents <= 0 ? '售罄' : (channel.Enabled ? '已启用' : '已停用') }}</el-tag></template></el-table-column>
                <el-table-column label="操作" width="150"><template #default="{ row: channel }"><div class="table-actions"><el-button size="small" @click="openPublicChannelModal(channel)">编辑</el-button><el-button type="danger" size="small" @click="confirmDeletePublicChannel(channel)">删除</el-button></div></template></el-table-column>
              </el-table>
            </div>
            <div class="p-4 flex justify-end">
              <el-pagination
                :current-page="pagination.publicChannels.page"
                :page-size="pagination.publicChannels.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="listTotals.publicChannels"
                background
                layout="total, sizes, prev, pager, next"
                @current-change="handlePageChange('publicChannels', $event)"
                @size-change="handlePageSizeChange('publicChannels', $event)"
              />
            </div>
          </section>

          <section v-else class="panel-surface overflow-hidden">
            <div class="p-4 border-b border-slate-100">
              <el-form class="form-grid user-filter-grid" label-position="top">
                <el-form-item label="搜索">
                  <el-input v-model="pollingPoolSearch.keyword" clearable placeholder="名称 / ID" @input="resetPager('pollingPools')" />
                </el-form-item>
                <el-form-item label="状态">
                  <el-select v-model="pollingPoolSearch.status" clearable placeholder="全部" @change="resetPager('pollingPools')">
                    <el-option label="全部" value="" />
                    <el-option label="已启用" value="enabled" />
                    <el-option label="已停用" value="disabled" />
                  </el-select>
                </el-form-item>
              </el-form>
            </div>
            <div class="table-wrap">
              <el-table :data="pagedPollingPools" border>
                <el-table-column label="号池名称" min-width="160" prop="Name" />
                <el-table-column label="支持协议" min-width="150"><template #default="{ row: pool }"><div class="table-actions"><el-tag v-for="tag in protocolTags(pool)" :key="tag" size="small">{{ tag }}</el-tag></div></template></el-table-column>
                <el-table-column label="账号数量" width="110"><template #default="{ row: pool }">{{ pool.Accounts?.length || 0 }}</template></el-table-column>
                <el-table-column label="剩余额度 / 总额度" min-width="160"><template #default="{ row: pool }">{{ channelQuotaText(pool) }}</template></el-table-column>
                <el-table-column label="状态" width="110"><template #default="{ row: pool }"><el-tag :type="pool.Enabled && pool.RemainingUSDCents > 0 ? 'success' : 'info'">{{ pool.RemainingUSDCents <= 0 ? '售罄' : (pool.Enabled ? '已启用' : '已停用') }}</el-tag></template></el-table-column>
                <el-table-column label="操作" width="150"><template #default="{ row: pool }"><div class="table-actions"><el-button size="small" @click="openPollingPoolModal(pool)">编辑</el-button><el-button type="danger" size="small" @click="confirmDeletePollingPool(pool)">删除</el-button></div></template></el-table-column>
              </el-table>
            </div>
            <div class="p-4 flex justify-end">
              <el-pagination
                :current-page="pagination.pollingPools.page"
                :page-size="pagination.pollingPools.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="listTotals.pollingPools"
                background
                layout="total, sizes, prev, pager, next"
                @current-change="handlePageChange('pollingPools', $event)"
                @size-change="handlePageSizeChange('pollingPools', $event)"
              />
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
              <el-button circle :icon="Refresh" :loading="loading" aria-label="刷新" title="刷新" @click="refreshAdminData" />
              <el-button type="primary" @click="openUserModal()">新增用户</el-button>
            </div>
          </div>

          <el-segmented
            v-model="usersTab"
            class="settings-tabs"
            :options="[
              { label: '用户列表', value: 'users' },
              { label: 'API Key', value: 'api-keys' }
            ]"
          />

          <section v-if="usersTab === 'users'" class="panel-surface p-4">
            <el-form class="form-grid user-filter-grid" label-position="top">
              <el-form-item label="搜索">
                <el-input v-model="userSearch.keyword" clearable placeholder="用户名 / 邮箱 / ID" />
              </el-form-item>
              <el-form-item label="角色">
                <el-select v-model="userSearch.role" clearable placeholder="全部">
                  <el-option label="全部" value="" />
                  <el-option v-for="option in roleOptions" :key="option.value" :label="option.label" :value="option.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="状态">
                <el-select v-model="userSearch.status" clearable placeholder="全部">
                  <el-option label="全部" value="" />
                  <el-option v-for="option in statusOptions" :key="option.value" :label="option.label" :value="option.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="套餐">
                <el-select v-model="userSearch.plan" clearable filterable placeholder="全部">
                  <el-option label="全部" value="" />
                  <el-option v-for="plan in plans" :key="plan.ID" :label="plan.Name" :value="String(plan.ID)" />
                </el-select>
              </el-form-item>
            </el-form>
          </section>

          <section v-if="usersTab === 'users'" class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <el-table :data="pagedUsers" border>
                <el-table-column label="用户" min-width="220"><template #default="{ row: user }"><strong>{{ user.Email }}</strong><small>{{ user.Username }}</small></template></el-table-column>
                <el-table-column label="角色" width="110"><template #default="{ row: user }">{{ roleLabel(user.Role) }}</template></el-table-column>
                <el-table-column label="状态" width="110"><template #default="{ row: user }"><el-tag>{{ statusLabel(user.Status) }}</el-tag></template></el-table-column>
                <el-table-column label="套餐" min-width="150"><template #default="{ row: user }">{{ planLabel(user) }}</template></el-table-column>
                <el-table-column label="订阅额度" min-width="160"><template #default="{ row: user }">{{ user.Plan ? `${usd(user.Plan.SettlementUSDCents)} / ${user.Plan.QuotaPeriod === 'daily' ? '日' : '周'}` : '未分配' }}</template></el-table-column>
                <el-table-column label="操作" width="190"><template #default="{ row: user }"><div class="table-actions"><el-button size="small" @click="openUserModal(user)">编辑</el-button><el-button size="small" @click="openUserUpstreamModal(user)">渠道</el-button><el-button type="danger" size="small" @click="confirmDeleteUser(user)">删除</el-button></div></template></el-table-column>
              </el-table>
            </div>
            <div class="p-4 flex justify-end">
              <el-pagination
                :current-page="pagination.users.page"
                :page-size="pagination.users.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="listTotals.users"
                background
                layout="total, sizes, prev, pager, next"
                @current-change="handlePageChange('users', $event)"
                @size-change="handlePageSizeChange('users', $event)"
              />
            </div>
          </section>

          <section v-if="usersTab === 'api-keys'" class="panel-surface overflow-hidden">
            <div class="p-4 border-b border-slate-100">
              <el-form class="form-grid user-filter-grid" label-position="top">
                <el-form-item label="搜索">
                  <el-input v-model="apiKeySearch.keyword" clearable placeholder="用户 / 名称 / 前缀 / ID" @input="resetPager('apiKeys')" />
                </el-form-item>
                <el-form-item label="状态">
                  <el-select v-model="apiKeySearch.status" clearable placeholder="全部" @change="resetPager('apiKeys')">
                    <el-option label="全部" value="" />
                    <el-option label="已启用" value="active" />
                    <el-option label="已停用" value="disabled" />
                  </el-select>
                </el-form-item>
              </el-form>
            </div>
            <div class="table-wrap">
              <el-table :data="pagedApiKeys" border>
                <el-table-column label="用户" min-width="220"><template #default="{ row: key }">{{ key.User?.Email || key.User?.Username || '-' }}</template></el-table-column>
                <el-table-column label="名称" min-width="140" prop="Name" />
                <el-table-column label="前缀" min-width="120"><template #default="{ row: key }">{{ apiKeyPrefix(key.KeyPrefix) }}</template></el-table-column>
                <el-table-column label="状态" width="110"><template #default="{ row: key }"><el-tag>{{ apiKeyStatusLabel(key.Status) }}</el-tag></template></el-table-column>
                <el-table-column label="更新时间" min-width="160"><template #default="{ row: key }">{{ formatDate(key.UpdatedAt || key.CreatedAt) }}</template></el-table-column>
                <el-table-column label="操作" width="200"><template #default="{ row: key }"><div class="table-actions"><el-button size="small" @click="openApiKeyModal(key)">编辑</el-button><el-button size="small" @click="toggleApiKeyStatus(key)">{{ key.Status === 'active' ? '停用' : '启用' }}</el-button><el-button type="danger" size="small" @click="confirmDeleteApiKey(key)">删除</el-button></div></template></el-table-column>
              </el-table>
            </div>
            <div class="p-4 flex justify-end">
              <el-pagination
                :current-page="pagination.apiKeys.page"
                :page-size="pagination.apiKeys.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="filteredApiKeys.length"
                background
                layout="total, sizes, prev, pager, next"
                @current-change="handlePageChange('apiKeys', $event)"
                @size-change="handlePageSizeChange('apiKeys', $event)"
              />
            </div>
          </section>
        </div>

        <div v-if="active === 'usageRecords'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Usage Logs</p>
              <h2>使用记录</h2>
              <span>查看用户 API Key 调用日志，可按用户和 API Key 搜索</span>
            </div>
            <el-button circle :icon="Refresh" :loading="loading" aria-label="刷新" title="刷新" @click="refreshAdminData" />
          </div>

          <div class="stat-grid">
            <article class="stat-card">
              <span>请求数</span>
              <strong>{{ usageSummary?.total_requests || 0 }}</strong>
              <small>{{ listTotals.usageRecords }} 条筛选结果</small>
            </article>
            <article class="stat-card">
              <span>总 Token</span>
              <strong>{{ compactNumber(usageSummary?.total_tokens || 0) }}</strong>
              <small>输入 {{ compactNumber(usageSummary?.prompt_tokens || 0) }} / 输出 {{ compactNumber(usageSummary?.completion_tokens || 0) }}</small>
            </article>
            <article class="stat-card">
              <span>总费用</span>
              <strong>{{ usageSummary?.total_usd_micros ? usdMicros(usageSummary.total_usd_micros) : usd(usageSummary?.total_usd_cents || 0) }}</strong>
              <small>按调用日志估算</small>
            </article>
            <article class="stat-card">
              <span>平均耗时</span>
              <strong>{{ latency(usageSummary?.average_latency_ms) }}</strong>
              <small>每次请求</small>
            </article>
          </div>

          <section class="panel-surface p-4">
            <el-form class="form-grid user-filter-grid" label-position="top">
              <el-form-item label="用户">
                <el-input v-model="usageSearch.userKeyword" clearable placeholder="用户名 / 邮箱 / 用户 ID" @input="resetPager('usageRecords')" />
              </el-form-item>
              <el-form-item label="API Key">
                <el-input v-model="usageSearch.apiKeyKeyword" clearable placeholder="名称 / 前缀 / Key ID" @input="resetPager('usageRecords')" />
              </el-form-item>
              <el-form-item label="时间范围">
                <el-select v-model="usageSearch.range" placeholder="时间范围" @change="resetPager('usageRecords')">
                  <el-option label="最近 24 小时" value="24h" />
                  <el-option label="最近 7 天" value="7d" />
                  <el-option label="最近 30 天" value="30d" />
                  <el-option label="全部时间" value="all" />
                </el-select>
              </el-form-item>
            </el-form>
          </section>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <el-table :data="pagedUsageRecords" border>
                <el-table-column label="用户" min-width="220">
                  <template #default="{ row: record }">
                    <strong>{{ usageUserLabel(record) }}</strong>
                    <small>{{ record.username || `ID: ${record.user_id || '-'}` }}</small>
                  </template>
                </el-table-column>
                <el-table-column label="API Key" min-width="190">
                  <template #default="{ row: record }">{{ usageApiKeyLabel(record) }}</template>
                </el-table-column>
                <el-table-column label="模型" min-width="150">
                  <template #default="{ row: record }">{{ record.model || '-' }}</template>
                </el-table-column>
                <el-table-column label="端点" min-width="220">
                  <template #default="{ row: record }">
                    <code>{{ usageEndpoint(record) }}</code>
                    <small>{{ record.method || 'POST' }} · {{ requestTypeLabel(record.request_type) }}</small>
                  </template>
                </el-table-column>
                <el-table-column label="Token" min-width="130">
                  <template #default="{ row: record }">
                    <strong>{{ compactNumber(record.total_tokens || 0) }}</strong>
                    <small>入 {{ compactNumber(record.prompt_tokens || 0) }} / 出 {{ compactNumber(record.completion_tokens || 0) }}</small>
                  </template>
                </el-table-column>
                <el-table-column label="费用" width="120">
                  <template #default="{ row: record }">{{ usageCost(record) }}</template>
                </el-table-column>
                <el-table-column label="状态" width="110">
                  <template #default="{ row: record }">
                    <el-tag :type="record.status_code >= 400 ? 'danger' : 'success'">{{ record.status_code || '-' }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="耗时" width="120">
                  <template #default="{ row: record }">{{ latency(record.latency_ms) }}</template>
                </el-table-column>
                <el-table-column label="时间" min-width="160">
                  <template #default="{ row: record }">{{ formatDate(record.created_at) }}</template>
                </el-table-column>
              </el-table>
            </div>
            <div class="p-4 flex justify-end">
              <el-pagination
                :current-page="pagination.usageRecords.page"
                :page-size="pagination.usageRecords.pageSize"
                :page-sizes="[20, 50, 100]"
                :total="listTotals.usageRecords"
                background
                layout="total, sizes, prev, pager, next"
                @current-change="handlePageChange('usageRecords', $event)"
                @size-change="handlePageSizeChange('usageRecords', $event)"
              />
            </div>
          </section>
        </div>

        <div v-if="active === 'announcements'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Announcements</p>
              <h2>公告管理</h2>
              <span>{{ enabledAnnouncements }} 条启用公告，{{ listTotals.announcements }} 条筛选结果。用户控制台默认展示最新启用公告。</span>
            </div>
            <div class="toolbar-actions">
              <el-button circle :icon="Refresh" :loading="loading" aria-label="刷新" title="刷新" @click="refreshAdminData" />
              <el-button type="primary" @click="openAnnouncementModal()">发布公告</el-button>
            </div>
          </div>

          <section class="panel-surface p-4">
            <el-form class="form-grid user-filter-grid" label-position="top">
              <el-form-item label="搜索">
                <el-input v-model="announcementSearch.keyword" clearable placeholder="标题 / 摘要 / 内容 / ID" @input="resetPager('announcements')" />
              </el-form-item>
              <el-form-item label="状态">
                <el-select v-model="announcementSearch.status" clearable placeholder="全部" @change="resetPager('announcements')">
                  <el-option label="全部" value="" />
                  <el-option label="已启用" value="enabled" />
                  <el-option label="已停用" value="disabled" />
                </el-select>
              </el-form-item>
            </el-form>
          </section>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <el-table :data="pagedAnnouncements" border>
                <el-table-column label="公告" min-width="260"><template #default="{ row: item }"><strong>{{ item.Title }}</strong><small>{{ item.Summary || item.Content }}</small></template></el-table-column>
                <el-table-column label="发布时间" min-width="160"><template #default="{ row: item }">{{ formatDate(item.PublishedAt || item.CreatedAt) }}</template></el-table-column>
                <el-table-column label="排序" width="90" prop="SortOrder" />
                <el-table-column label="状态" width="150"><template #default="{ row: item }"><el-tag :type="item.Enabled ? 'success' : 'info'">{{ item.Enabled ? '已启用' : '已停用' }}</el-tag><el-tag v-if="item.Pinned" class="ml-1">置顶</el-tag></template></el-table-column>
                <el-table-column label="操作" width="150"><template #default="{ row: item }"><div class="table-actions"><el-button size="small" @click="openAnnouncementModal(item)">编辑</el-button><el-button type="danger" size="small" @click="confirmDeleteAnnouncement(item)">删除</el-button></div></template></el-table-column>
              </el-table>
            </div>
            <div class="p-4 flex justify-end">
              <el-pagination
                :current-page="pagination.announcements.page"
                :page-size="pagination.announcements.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="listTotals.announcements"
                background
                layout="total, sizes, prev, pager, next"
                @current-change="handlePageChange('announcements', $event)"
                @size-change="handlePageSizeChange('announcements', $event)"
              />
            </div>
          </section>
        </div>

        <div v-if="active === 'docs'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Docs</p>
              <h2>配置文档</h2>
              <span>{{ enabledDocs }} 篇启用文档，{{ listTotals.docs }} 篇筛选结果。左侧导航、排序和内容都可在这里维护。</span>
            </div>
            <div class="toolbar-actions">
              <el-button circle :icon="Refresh" :loading="loading" aria-label="刷新" title="刷新" @click="refreshAdminData" />
              <el-button type="primary" @click="openDocModal()">新增文档</el-button>
            </div>
          </div>

          <section class="panel-surface p-4">
            <el-form class="form-grid user-filter-grid" label-position="top">
              <el-form-item label="搜索">
                <el-input v-model="docSearch.keyword" clearable placeholder="标题 / Slug / 描述 / ID" @input="resetPager('docs')" />
              </el-form-item>
              <el-form-item label="分组">
                <el-input v-model="docSearch.groupName" clearable placeholder="分组名" @input="resetPager('docs')" />
              </el-form-item>
              <el-form-item label="状态">
                <el-select v-model="docSearch.status" clearable placeholder="全部" @change="resetPager('docs')">
                  <el-option label="全部" value="" />
                  <el-option label="已启用" value="enabled" />
                  <el-option label="已停用" value="disabled" />
                </el-select>
              </el-form-item>
            </el-form>
          </section>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <el-table :data="pagedDocs" border>
                <el-table-column label="文档" min-width="240"><template #default="{ row: doc }"><strong>{{ doc.Title }}</strong><small>{{ doc.Description || '暂无说明' }}</small></template></el-table-column>
                <el-table-column label="分组" min-width="120"><template #default="{ row: doc }">{{ doc.GroupName || '-' }}</template></el-table-column>
                <el-table-column label="Slug" min-width="150"><template #default="{ row: doc }"><code>{{ doc.Slug }}</code></template></el-table-column>
                <el-table-column label="排序" width="90" prop="SortOrder" />
                <el-table-column label="状态" width="110"><template #default="{ row: doc }"><el-tag :type="doc.Enabled ? 'success' : 'info'">{{ doc.Enabled ? '已启用' : '已停用' }}</el-tag></template></el-table-column>
                <el-table-column label="操作" width="150"><template #default="{ row: doc }"><div class="table-actions"><el-button size="small" @click="openDocModal(doc)">编辑</el-button><el-button type="danger" size="small" @click="confirmDeleteDoc(doc)">删除</el-button></div></template></el-table-column>
              </el-table>
            </div>
            <div class="p-4 flex justify-end">
              <el-pagination
                :current-page="pagination.docs.page"
                :page-size="pagination.docs.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="listTotals.docs"
                background
                layout="total, sizes, prev, pager, next"
                @current-change="handlePageChange('docs', $event)"
                @size-change="handlePageSizeChange('docs', $event)"
              />
            </div>
          </section>
        </div>

        <div v-if="active === 'emailTemplates'" class="space-y-5">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Email Templates</p>
              <h2>邮件模板</h2>
              <span>{{ enabledEmailTemplates }} 个模板启用，{{ emailTemplates.length }} 个总模板。模板变量会在发送时自动替换。</span>
            </div>
            <el-button circle :icon="Refresh" :loading="loading" aria-label="刷新" title="刷新" @click="refreshAdminData" />
          </div>

          <section class="panel-surface overflow-hidden">
            <div class="table-wrap">
              <el-table :data="emailTemplates" border>
                <el-table-column label="模板" min-width="220"><template #default="{ row: item }"><strong>{{ item.Name }}</strong><small>{{ item.Description || item.Type }}</small></template></el-table-column>
                <el-table-column label="邮件标题" min-width="260" prop="Subject" />
                <el-table-column label="状态" width="110"><template #default="{ row: item }"><el-tag :type="item.Enabled ? 'success' : 'info'">{{ item.Enabled ? '已启用' : '已停用' }}</el-tag></template></el-table-column>
                <el-table-column label="操作" width="100"><template #default="{ row: item }"><el-button size="small" @click="openEmailTemplateModal(item)">编辑</el-button></template></el-table-column>
              </el-table>
            </div>
          </section>

          <section class="panel-surface p-5">
            <div class="section-head">
              <div>
                <p class="section-kicker">Variables</p>
                <h3>可用变量</h3>
                <span>例如填写 {username}你好，发送时会替换成真实用户名。</span>
              </div>
            </div>
            <div class="template-variable-list mt-4">
              <code v-for="item in emailTemplateVariables" :key="item">{{ item }}</code>
            </div>
          </section>
        </div>

        <el-form v-if="active === 'navigation'" class="space-y-5" label-position="top" @submit.prevent="saveNavigation">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Navigation</p>
              <h2>导航菜单</h2>
              <span>维护首页顶部导航，支持一级菜单、下拉子菜单、排序和外链。</span>
            </div>
            <div class="toolbar-actions">
              <el-button circle :icon="Refresh" :loading="loading" aria-label="刷新" title="刷新" @click="refreshAdminData" />
              <el-button type="primary" native-type="submit">保存导航</el-button>
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
                  <el-button size="small" @click="resetNavigationDefault">恢复默认</el-button>
                  <el-button type="primary" size="small" @click="addNavItem">新增菜单</el-button>
                </div>
              </div>

              <div class="nav-editor-list">
                <article v-for="(item, index) in navDraft" :key="`nav-${index}`" class="nav-editor-card">
                  <div class="nav-editor-grid">
                    <el-form-item label="菜单名称">
                      <el-input v-model="item.label" placeholder="首页" @input="syncNavigationSetting" />
                    </el-form-item>
                    <el-form-item label="链接地址">
                      <el-input v-model="item.path" placeholder="/plans" @input="syncNavigationSetting" />
                    </el-form-item>
                    <el-form-item class="nav-toggle" label="新窗口打开">
                      <el-switch v-model="item.external" @change="syncNavigationSetting" />
                    </el-form-item>
                    <div class="nav-row-actions">
                      <el-button size="small" :disabled="index === 0" @click="moveNavItem(index, -1)">上移</el-button>
                      <el-button size="small" :disabled="index === navDraft.length - 1" @click="moveNavItem(index, 1)">下移</el-button>
                      <el-button type="danger" size="small" @click="removeNavItem(index)">删除</el-button>
                    </div>
                  </div>

                  <div class="child-nav-list">
                    <div v-for="(child, childIndex) in item.children" :key="`nav-${index}-child-${childIndex}`" class="child-nav-row">
                      <el-input v-model="child.label" placeholder="子菜单名称" @input="syncNavigationSetting" />
                      <el-input v-model="child.path" placeholder="/claude" @input="syncNavigationSetting" />
                      <el-switch v-model="child.external" active-text="新窗口" @change="syncNavigationSetting" />
                      <el-button type="danger" size="small" @click="removeNavItem(index, childIndex)">删除</el-button>
                    </div>
                  </div>

                  <el-button size="small" @click="addChildNavItem(index)">新增子菜单</el-button>
                </article>
              </div>
            </div>
          </section>
        </el-form>

        <el-form v-if="active === 'settings'" class="space-y-5" label-position="top" @submit.prevent="saveSettings">
          <div class="page-toolbar">
            <div>
              <p class="section-kicker">Settings</p>
              <h2>系统设置</h2>
              <span>基础信息、SMTP 配置和易支付配置按类别维护</span>
            </div>
            <el-button type="primary" native-type="submit">保存设置</el-button>
          </div>

          <el-segmented
            v-model="settingsTab"
            class="settings-tabs"
            :options="[
              { label: '基础信息', value: 'basic' },
              { label: 'API 端点', value: 'endpoints' },
              { label: 'SMTP 配置', value: 'smtp' },
              { label: '通知开关', value: 'notifications' },
              { label: '易支付配置', value: 'epay' },
              { label: '人工支付', value: 'manualPayment' }
            ]"
          />

          <section v-if="settingsTab === 'basic'" class="panel-surface p-5">
            <div class="form-grid">
              <el-form-item label="网站标题">
                <el-input v-model="settings.site_title" placeholder="AI Gateway" />
              </el-form-item>
              <el-form-item label="联系邮箱">
                <el-input v-model="settings.contact_email" type="email" placeholder="support@example.com" />
              </el-form-item>
              <el-form-item label="定价页主标题">
                <el-input v-model="settings.pricing_title" placeholder="简单透明的定价" />
              </el-form-item>
              <el-form-item label="定价页副标题">
                <el-input v-model="settings.pricing_subtitle" placeholder="保质保量无降智不掺假" />
              </el-form-item>
              <el-form-item class="md:col-span-2" label="定价页提示内容">
                <el-input v-model="settings.pricing_notice" type="textarea" :rows="3" placeholder="展示在定价页顶部提示框中的说明文字" />
              </el-form-item>
              <el-form-item class="md:col-span-2" label="允许新用户注册">
                <el-switch v-model="settings.allow_registration" />
              </el-form-item>
              <el-form-item label="模拟在线API人数">
                <el-switch v-model="settings.mock_api_online_enabled" />
              </el-form-item>
              <el-form-item label="起始模拟在线人数">
                <el-input-number v-model="settings.mock_api_online_base" :min="0" :max="1000000" class="w-full" />
              </el-form-item>
            </div>
          </section>

          <section v-if="settingsTab === 'endpoints'" class="panel-surface p-5">
            <div class="section-head mb-5">
              <div>
                <p class="section-kicker">API Endpoints</p>
                <h3>API 端点</h3>
                <span>配置用户控制台展示的 API 接入地址、标签和线路说明。</span>
              </div>
              <el-button type="primary" size="small" @click="addAPIEndpoint">新增端点</el-button>
            </div>
            <div class="endpoint-admin-list">
              <article v-for="(endpoint, index) in apiEndpointDraft" :key="index" class="endpoint-admin-item">
                <div class="form-grid">
                  <el-form-item label="展示标签">
                    <el-input v-model="endpoint.label" placeholder="CN2 优化" @input="syncAPIEndpointSetting" />
                  </el-form-item>
                  <el-form-item label="线路说明">
                    <el-input v-model="endpoint.description" placeholder="国内直连优化线路" @input="syncAPIEndpointSetting" />
                  </el-form-item>
                  <el-form-item class="md:col-span-2" label="API 地址">
                    <el-input v-model="endpoint.url" placeholder="https://api.example.com/v1" @input="syncAPIEndpointSetting" />
                  </el-form-item>
                </div>
                <el-button type="danger" size="small" :disabled="apiEndpointDraft.length <= 1" @click="removeAPIEndpoint(index)">删除</el-button>
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
                <el-switch v-model="settings.smtp_use_tls" active-text="使用 TLS" />
                <el-button size="small" :loading="smtpTesting" @click="sendSMTPTest">
                  {{ smtpTesting ? '发送中...' : '发送测试邮件' }}
                </el-button>
              </div>
            </div>
            <div class="form-grid">
              <el-form-item label="SMTP 主机"><el-input v-model="settings.smtp_host" placeholder="smtp.example.com" /></el-form-item>
              <el-form-item label="SMTP 端口"><el-input-number v-model="settings.smtp_port" :min="1" class="w-full" /></el-form-item>
              <el-form-item label="SMTP 用户名"><el-input v-model="settings.smtp_username" /></el-form-item>
              <el-form-item label="SMTP 密码"><el-input v-model="settings.smtp_password" type="password" show-password :placeholder="settings.smtp_password_configured ? '已配置，留空不修改' : '请输入密码'" /></el-form-item>
              <el-form-item label="发件邮箱"><el-input v-model="settings.smtp_from_email" /></el-form-item>
              <el-form-item label="发件名称"><el-input v-model="settings.smtp_from_name" /></el-form-item>
              <el-form-item class="md:col-span-2" label="测试收件邮箱"><el-input v-model="settings.smtp_test_email" type="email" placeholder="输入一个邮箱用于接收测试邮件" /></el-form-item>
            </div>
          </section>

          <section v-if="settingsTab === 'notifications'" class="panel-surface p-5">
            <div class="section-head mb-5">
              <div>
                <p class="section-kicker">Notifications</p>
                <h3>邮件通知开关</h3>
                <span>开启后按邮件模板发送，SMTP 未配置时会记录后端日志但不会阻塞订单流程。</span>
              </div>
            </div>
            <div class="form-grid">
              <el-form-item class="md:col-span-2" label="用户支付成功且订单待审核时通知管理员">
                <el-switch v-model="settings.order_payment_admin_email_enabled" />
              </el-form-item>
              <el-form-item class="md:col-span-2" label="审核通过并开通套餐后通知用户">
                <el-switch v-model="settings.order_approved_user_email_enabled" />
              </el-form-item>
              <el-form-item label="套餐到期提醒用户">
                <el-switch v-model="settings.subscription_expire_email_enabled" />
              </el-form-item>
              <el-form-item label="到期前提醒天数">
                <el-input-number v-model="settings.subscription_expire_remind_days" :min="1" :max="365" class="w-full" />
              </el-form-item>
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
              <el-form-item class="md:col-span-2" label="启用在线支付">
                <el-switch v-model="settings.online_payment_enabled" active-text="前端展示在线支付" />
              </el-form-item>
              <el-form-item class="md:col-span-2" label="接口网址">
                <el-input v-model="settings.epay_submit_url" placeholder="https://mapi.example.com/" />
              </el-form-item>
              <el-form-item label="商户 ID">
                <el-input v-model="settings.epay_pid" placeholder="请输入商户 ID" />
              </el-form-item>
              <el-form-item label="商户 KEY">
                <el-input v-model="settings.epay_key" type="password" show-password :placeholder="settings.epay_key_configured ? '已配置，留空不修改' : '请输入商户 KEY'" />
              </el-form-item>
            </div>
          </section>

          <section v-if="settingsTab === 'manualPayment'" class="panel-surface p-5">
            <div class="section-head mb-5">
              <div>
                <p class="section-kicker">Manual Payment</p>
                <h3>人工支付二维码</h3>
                <span>上传后，用户选择人工支付时会展示该二维码，并引导用户备注当前账号。</span>
              </div>
            </div>
            <div class="manual-payment-admin">
              <div class="manual-payment-preview">
                <img v-if="settings.manual_payment_qr_code" :src="settings.manual_payment_qr_code" alt="人工支付付款二维码预览" />
                <span v-else>尚未上传付款二维码</span>
              </div>
              <div class="form-grid">
                <el-form-item class="md:col-span-2" label="启用人工支付">
                  <el-switch v-model="settings.manual_payment_enabled" active-text="前端展示人工支付" />
                </el-form-item>
                <el-form-item class="md:col-span-2" label="上传付款二维码">
                  <el-upload :auto-upload="false" accept="image/*" :show-file-list="false" :on-change="(file) => handleManualPaymentQRUpload({ target: { files: [file.raw] } })">
                    <el-button>选择图片</el-button>
                  </el-upload>
                </el-form-item>
                <div class="order-flow-note md:col-span-2">
                  <strong>用户侧提示</strong>
                  <span>用户点击人工支付后会看到二维码，并被要求填写当前账号或转账留言；提交后订单进入待审核。</span>
                </div>
                <el-button type="danger" size="small" :disabled="!settings.manual_payment_qr_code" @click="clearManualPaymentQR">清空二维码</el-button>
              </div>
            </div>
          </section>
        </el-form>
        </div>
      </main>
    </div>

    <el-dialog v-model="modal.open" class="admin-modal-dialog" :title="modal.title" :width="modalDialogWidth" align-center @close="closeModal">
      <el-form class="admin-modal-form" label-position="top" @submit.prevent="submitModal">

        <div v-if="modal.type === 'create-plan' || modal.type === 'edit-plan'" class="modal-body form-grid">
          <el-form-item label="套餐名称" required><el-input v-model="planForm.name" placeholder="月卡套餐" /></el-form-item>
          <el-form-item label="套餐编码"><el-input v-model="planForm.code" placeholder="monthly" /></el-form-item>
          <el-form-item label="套餐角标文案"><el-input v-model="planForm.badge_text" placeholder="热卖推荐" maxlength="16" /></el-form-item>
          <el-form-item label="抽奖套餐"><el-switch v-model="planForm.is_lottery" active-text="参与抽奖" @change="planForm.is_free = false" /></el-form-item>
          <el-form-item v-if="!planForm.is_lottery" label="免费套餐"><el-switch v-model="planForm.is_free" active-text="免费领取" /></el-form-item>
          <el-form-item label="限额周期">
            <el-select v-model="planForm.quota_period">
              <el-option label="日限额套餐" value="daily" />
              <el-option label="周限额套餐" value="weekly" />
              <el-option label="公共渠道" value="public" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="planForm.quota_period === 'public'" label="供给来源" required>
            <el-select v-model="planForm.delivery_source">
              <el-option value="public" label="公共渠道" />
              <el-option value="pool" label="轮询号池" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="planForm.quota_period === 'public' && planForm.delivery_source !== 'pool'" label="绑定公共渠道" required>
            <el-select v-model="planForm.public_channel_id" placeholder="请选择公共渠道">
              <el-option label="请选择公共渠道" value="" />
              <el-option v-for="channel in publicChannels.filter((item) => item.Enabled)" :key="channel.ID" :label="`${channel.Name}（剩余 ${usd(channel.RemainingUSDCents)}）`" :value="channel.ID" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="planForm.quota_period === 'public' && planForm.delivery_source === 'pool'" label="绑定轮询号池" required>
            <el-select v-model="planForm.polling_pool_id" placeholder="请选择轮询号池">
              <el-option label="请选择轮询号池" value="" />
              <el-option v-for="pool in pollingPools.filter((item) => item.Enabled)" :key="pool.ID" :label="`${pool.Name}（剩余 ${usd(pool.RemainingUSDCents)}）`" :value="pool.ID" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="planForm.is_lottery" class="md:col-span-2" label="参与抽奖按钮跳转地址" required><el-input v-model="planForm.lottery_url" placeholder="https://example.com/lottery" /></el-form-item>
          <el-form-item v-if="!planForm.is_lottery && !planForm.is_free" label="售价（RMB）"><el-input v-model.number="planForm.price_rmb" type="number" min="0.01" step="0.01" required /></el-form-item>
          <el-form-item v-if="planForm.is_free" label="每人领取上限"><el-input v-model.number="planForm.free_per_user_limit" type="number" min="1" step="1" required /></el-form-item>
          <el-form-item v-if="planForm.is_free" label="总领取上限"><el-input v-model.number="planForm.free_total_limit" type="number" min="0" step="1" placeholder="0 表示不限" /></el-form-item>
          <el-form-item :label="planForm.quota_period === 'public' ? '预计总美元额度' : (planForm.quota_period === 'daily' ? '每日美元额度' : '每周美元额度')"><el-input v-model.number="planForm.period_usd_quota" type="number" min="0" step="0.01" /></el-form-item>
          <el-form-item v-if="planForm.quota_period !== 'public'" label="有效期（天）"><el-input v-model.number="planForm.duration_days" type="number" min="1" required /></el-form-item>
          <el-form-item v-if="planForm.quota_period !== 'public'" label="预计总美元额度"><el-input :model-value="totalUsd({ SettlementUSDCents: amountToCents(planForm.period_usd_quota), DurationDays: planForm.duration_days, QuotaPeriod: planForm.quota_period })" readonly /></el-form-item>
          <el-form-item class="md:col-span-2" label="套餐说明"><el-input v-model="planForm.description" type="textarea" :rows="3" /></el-form-item>
          <el-form-item class="md:col-span-2" label="启用套餐"><el-switch v-model="planForm.enabled" /></el-form-item>
        </div>

        <div v-if="modal.type === 'create-channel' || modal.type === 'edit-channel'" class="modal-body form-grid">
          <el-form-item label="渠道名称" required><el-input v-model="channelForm.name" placeholder="OpenAI" /></el-form-item>
          <el-form-item class="md:col-span-2" label="API 地址" required><el-input v-model="channelForm.base_url" placeholder="https://api.openai.com" /></el-form-item>
          <el-form-item label="GPT 协议"><el-switch v-model="channelForm.supports_gpt" active-text="支持" /></el-form-item>
          <el-form-item label="Claude 协议"><el-switch v-model="channelForm.supports_claude" active-text="支持" /></el-form-item>
          <el-form-item class="md:col-span-2" label="启用渠道"><el-switch v-model="channelForm.enabled" /></el-form-item>
        </div>

        <div v-if="modal.type === 'create-public-channel' || modal.type === 'edit-public-channel'" class="modal-body form-grid">
          <el-form-item label="渠道名称" required><el-input v-model="publicChannelForm.name" placeholder="公共 OpenAI" /></el-form-item>
          <el-form-item class="md:col-span-2" label="API 地址" required><el-input v-model="publicChannelForm.base_url" placeholder="https://api.openai.com" /></el-form-item>
          <el-form-item class="md:col-span-2" label="API Key" :required="!publicChannelForm.id">
            <el-input v-model="publicChannelForm.api_key" :placeholder="publicChannelForm.id ? '留空则不修改' : '请输入公共渠道 API Key'" />
          </el-form-item>
          <el-form-item label="GPT 协议"><el-switch v-model="publicChannelForm.supports_gpt" active-text="支持" /></el-form-item>
          <el-form-item label="Claude 协议"><el-switch v-model="publicChannelForm.supports_claude" active-text="支持" /></el-form-item>
          <el-form-item label="渠道总额度（美元）"><el-input v-model.number="publicChannelForm.total_usd_quota" type="number" min="0" step="0.01" required /></el-form-item>
          <el-form-item label="剩余美元额度"><el-input v-model.number="publicChannelForm.remaining_usd_quota" type="number" min="0" step="0.01" required /></el-form-item>
          <el-form-item class="md:col-span-2" label="启用公共渠道"><el-switch v-model="publicChannelForm.enabled" /></el-form-item>
        </div>

        <div v-if="modal.type === 'create-polling-pool' || modal.type === 'edit-polling-pool'" class="modal-body form-grid">
          <el-form-item label="号池名称" required><el-input v-model="pollingPoolForm.name" placeholder="轮询号池 A" /></el-form-item>
          <el-form-item label="GPT 协议"><el-switch v-model="pollingPoolForm.supports_gpt" active-text="支持" /></el-form-item>
          <el-form-item label="Claude 协议"><el-switch v-model="pollingPoolForm.supports_claude" active-text="支持" /></el-form-item>
          <el-form-item label="启用号池"><el-switch v-model="pollingPoolForm.enabled" /></el-form-item>
          <div class="md:col-span-2 pool-account-editor">
            <div class="section-head">
              <div>
                <h3>号池账号</h3>
                <span>按排序从小到大扣减额度，第一个账号额度用完后自动使用下一个账号。</span>
              </div>
              <el-button size="small" type="primary" @click="addPollingPoolAccount">新增账号</el-button>
            </div>
            <div v-for="(account, index) in pollingPoolForm.accounts" :key="index" class="pool-account-row">
              <div class="pool-account-field pool-account-name">
                <span>账号名称</span>
                <el-input v-model="account.name" placeholder="例如：OpenAI 主账号" />
              </div>
              <div class="pool-account-field pool-account-base">
                <span>API 地址</span>
                <el-input v-model="account.base_url" placeholder="https://api.openai.com" />
              </div>
              <div class="pool-account-field pool-account-key">
                <span>API Key</span>
                <el-input v-model="account.api_key" placeholder="请输入上游 API Key" />
              </div>
              <div class="pool-account-field pool-account-quota">
                <span>总额度（美元）</span>
                <el-input v-model.number="account.total_usd_quota" type="number" min="0" step="0.01" placeholder="总额度" />
              </div>
              <div class="pool-account-field pool-account-remaining">
                <span>剩余额度（美元）</span>
                <el-input v-model.number="account.remaining_usd_quota" type="number" min="0" step="0.01" placeholder="剩余额度" />
              </div>
              <div class="pool-account-field pool-account-sort">
                <span>排序</span>
                <el-input v-model.number="account.sort_order" type="number" placeholder="数字越小越先使用" />
              </div>
              <div class="pool-account-enabled">
                <span>启用</span>
                <el-switch v-model="account.enabled" />
              </div>
              <el-button class="pool-account-delete" size="small" type="danger" :disabled="pollingPoolForm.accounts.length <= 1" @click="removePollingPoolAccount(index)">删除</el-button>
            </div>
          </div>
        </div>

        <div v-if="modal.type === 'create-model' || modal.type === 'edit-model'" class="modal-body form-grid">
          <el-form-item label="模型 ID" required><el-input v-model="modelForm.model" placeholder="gpt-5.5" /></el-form-item>
          <el-form-item label="显示名称"><el-input v-model="modelForm.display_name" placeholder="GPT-5.5" /></el-form-item>
          <el-form-item label="服务商"><el-input v-model="modelForm.provider" placeholder="openai" /></el-form-item>
          <el-form-item label="输入单价 / 1M Token"><el-input v-model.number="modelForm.input_usd_per_million" type="number" min="0" step="0.0001" /></el-form-item>
          <el-form-item label="缓存读取单价 / 1M Token"><el-input v-model.number="modelForm.cached_input_usd_per_million" type="number" min="0" step="0.0001" /></el-form-item>
          <el-form-item label="输出单价 / 1M Token"><el-input v-model.number="modelForm.output_usd_per_million" type="number" min="0" step="0.0001" /></el-form-item>
          <el-form-item label="扣费倍率"><el-input v-model.number="modelForm.billing_multiplier" type="number" min="0.0001" step="0.01" /></el-form-item>
          <el-form-item label="状态">
            <el-select v-model="modelForm.status">
              <el-option value="active" label="启用" />
              <el-option value="disabled" label="停用" />
            </el-select>
          </el-form-item>
          <el-form-item class="md:col-span-2" label="展示在 /models 顶部卡片中"><el-switch v-model="modelForm.featured" /></el-form-item>
          <el-form-item class="md:col-span-2" label="备注"><el-input v-model="modelForm.notes" type="textarea" :rows="3" /></el-form-item>
        </div>

        <div v-if="modal.type === 'create-doc' || modal.type === 'edit-doc'" class="modal-body form-grid">
          <el-form-item label="文档标题">
            <el-input v-model="docForm.title" required placeholder="官方 API Base URL" />
          </el-form-item>
          <el-form-item label="Slug" required>
            <el-input v-model="docForm.slug" placeholder="api-base-url" />
          </el-form-item>
          <el-form-item label="左侧分组">
            <el-input v-model="docForm.group_name" placeholder="快速开始" />
          </el-form-item>
          <el-form-item label="排序">
            <el-input-number v-model="docForm.sort_order" class="w-full" />
          </el-form-item>
          <el-form-item class="md:col-span-2" label="说明">
            <el-input v-model="docForm.description" placeholder="展示在后台列表中的简短说明" />
          </el-form-item>
          <el-form-item class="md:col-span-2" label="Markdown 内容">
            <el-input v-model="docForm.content" type="textarea" :rows="18" placeholder="# 标题&#10;&#10;这里填写 Markdown 文档内容" />
          </el-form-item>
          <el-form-item class="md:col-span-2" label="启用文档"><el-switch v-model="docForm.enabled" /></el-form-item>
        </div>

        <div v-if="modal.type === 'create-announcement' || modal.type === 'edit-announcement'" class="modal-body form-grid">
          <el-form-item class="md:col-span-2" label="公告标题">
            <el-input v-model="announcementForm.title" required placeholder="【2026-05-14】服务更新说明" />
          </el-form-item>
          <el-form-item class="md:col-span-2" label="摘要">
            <el-input v-model="announcementForm.summary" type="textarea" :rows="3" placeholder="收起状态下展示的短内容，留空时自动使用正文前段" />
          </el-form-item>
          <el-form-item class="md:col-span-2" label="公告正文">
            <el-input v-model="announcementForm.content" type="textarea" :rows="8" placeholder="支持换行展示。可填写更新说明、使用提醒、教程地址等内容。" />
          </el-form-item>
          <el-form-item label="链接文案">
            <el-input v-model="announcementForm.link_text" placeholder="教程地址" />
          </el-form-item>
          <el-form-item label="链接地址">
            <el-input v-model="announcementForm.link_url" placeholder="https://docs.example.com/..." />
          </el-form-item>
          <el-form-item label="发布时间">
            <el-input v-model="announcementForm.published_at" type="datetime-local" />
          </el-form-item>
          <el-form-item label="排序">
            <el-input v-model.number="announcementForm.sort_order" type="number" />
          </el-form-item>
          <el-form-item label="置顶公告"><el-switch v-model="announcementForm.pinned" /></el-form-item>
          <el-form-item label="启用公告"><el-switch v-model="announcementForm.enabled" /></el-form-item>
        </div>

        <div v-if="modal.type === 'edit-email-template'" class="modal-body form-grid">
          <el-form-item class="md:col-span-2" label="模板名称">
            <el-input v-model="emailTemplateForm.name" required />
          </el-form-item>
          <el-form-item class="md:col-span-2" label="说明">
            <el-input v-model="emailTemplateForm.description" />
          </el-form-item>
          <el-form-item class="md:col-span-2" label="邮件标题">
            <el-input v-model="emailTemplateForm.subject" required placeholder="{site_title} 通知" />
          </el-form-item>
          <el-form-item class="md:col-span-2" label="邮件内容 HTML">
            <el-input v-model="emailTemplateForm.body" type="textarea" :rows="12" placeholder="<p>{username}你好：</p>" />
          </el-form-item>
          <el-form-item class="md:col-span-2" label="启用模板"><el-switch v-model="emailTemplateForm.enabled" /></el-form-item>
          <div class="template-variable-list md:col-span-2">
            <code v-for="item in emailTemplateVariables" :key="item">{{ item }}</code>
          </div>
        </div>

        <div v-if="modal.type === 'create-user' || modal.type === 'edit-user'" class="modal-body form-grid">
          <el-form-item label="用户名"><el-input v-model="userForm.username" required /></el-form-item>
          <el-form-item label="邮箱"><el-input v-model="userForm.email" type="email" required /></el-form-item>
          <el-form-item :label="userForm.id ? '新密码' : '登录密码'">
            <el-input v-model="userForm.password" type="password" :required="!userForm.id" minlength="8" :placeholder="userForm.id ? '留空不修改' : '至少 8 位'" />
          </el-form-item>
          <el-form-item label="角色">
            <el-select v-model="userForm.role">
              <el-option v-for="option in roleOptions" :key="option.value" :value="option.value" :label="option.label" />
            </el-select>
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="userForm.status">
              <el-option v-for="option in statusOptions" :key="option.value" :value="option.value" :label="option.label" />
            </el-select>
          </el-form-item>
          <el-form-item label="绑定套餐">
            <el-select v-model="userForm.plan_id">
              <el-option value="" label="不分配" />
              <el-option v-for="plan in plans" :key="plan.ID" :value="plan.ID" :label="plan.Name" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="shouldEditUserUpstream(userForm)" :label="requiresUserUpstreamRebind(userForm) ? '重新绑定上游渠道' : '上游渠道'">
            <el-select v-model="userForm.channel_id" required>
              <el-option value="" label="请选择渠道" />
              <el-option v-for="channel in channels.filter((item) => item.Enabled)" :key="channel.ID" :value="channel.ID" :label="channel.Name" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="shouldEditUserUpstream(userForm)" class="md:col-span-2" label="上游账号">
            <el-input v-model="userForm.upstream_username" required placeholder="请输入上游账号" />
          </el-form-item>
          <el-form-item v-if="shouldEditUserUpstream(userForm)" label="上游密码">
            <el-input v-model="userForm.upstream_password" type="text" required placeholder="请输入上游密码" />
          </el-form-item>
          <el-form-item v-if="shouldEditUserUpstream(userForm)" label="上游 API Key">
            <el-input v-model="userForm.api_key" type="text" required placeholder="请输入上游 API Key" />
          </el-form-item>
          <el-form-item class="md:col-span-2" label="邮箱已验证"><el-switch v-model="userForm.email_verified" /></el-form-item>
        </div>

        <div v-if="modal.type === 'edit-api-key'" class="modal-body form-grid">
          <el-form-item class="md:col-span-2" label="名称">
            <el-input v-model="apiKeyForm.name" required placeholder="默认名称" />
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="apiKeyForm.status">
              <el-option value="active" label="启用" />
              <el-option value="disabled" label="停用" />
            </el-select>
          </el-form-item>
        </div>

        <div v-if="modal.type === 'user-upstream'" class="modal-body form-grid">
          <div v-if="!modal.payload?.upstream" class="order-flow-note md:col-span-2">
            <strong>尚未绑定上游渠道</strong>
            <span>当前用户没有独立上游账号。需要开通时，请在编辑用户时分配套餐并填写上游渠道、账号、密码和 API Key。</span>
          </div>
          <el-form-item label="用户"><el-input :model-value="modal.payload?.user?.Email || '-'" readonly /></el-form-item>
          <el-form-item label="状态"><el-input :model-value="modal.payload?.upstream ? statusLabel(modal.payload.upstream.Status) : '未绑定'" readonly /></el-form-item>
          <el-form-item label="上游渠道"><el-input :model-value="modal.payload?.upstream?.Channel || '-'" readonly /></el-form-item>
          <el-form-item class="md:col-span-2" label="API 地址"><el-input :model-value="modal.payload?.upstream?.BaseURL || '-'" readonly /></el-form-item>
          <el-form-item label="上游账号"><el-input :model-value="modal.payload?.upstream?.Username || '-'" readonly /></el-form-item>
          <el-form-item label="上游密码"><el-input :model-value="modal.payload?.upstream?.Password || '-'" readonly /></el-form-item>
          <el-form-item class="md:col-span-2" label="API Key"><el-input :model-value="modal.payload?.upstream?.APIKey || '-'" type="textarea" :rows="4" readonly /></el-form-item>
          <el-form-item label="最后使用"><el-input :model-value="formatDate(modal.payload?.upstream?.LastUsedAt)" readonly /></el-form-item>
          <el-form-item label="更新时间"><el-input :model-value="formatDate(modal.payload?.upstream?.UpdatedAt)" readonly /></el-form-item>
        </div>

        <div v-if="modal.type === 'approve-order'" class="modal-body form-grid">
          <el-form-item label="订单 ID"><el-input v-model="approve.orderId" readonly /></el-form-item>
          <div v-if="modal.payload?.order?.UserPaymentNote" class="order-flow-note md:col-span-2">
            <strong>用户付款备注</strong>
            <span>{{ modal.payload.order.UserPaymentNote }}</span>
          </div>
          <div v-if="approveOrderUsesPublicChannel()" class="order-flow-note md:col-span-2">
            <strong>公共套餐无需审核绑定上游</strong>
            <span>公共套餐在支付完成时会自动扣减公共渠道额度并开通，此处不需要填写上游账号、密码或 API Key。</span>
          </div>
          <el-form-item v-if="!approveOrderUsesPublicChannel()" label="上游渠道">
            <el-select v-model="approve.channelId" required @change="syncApproveChannel">
              <el-option value="" label="请选择渠道" />
              <el-option v-for="channel in channels.filter((item) => item.Enabled)" :key="channel.ID" :value="channel.ID" :label="channel.Name" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="!approveOrderUsesPublicChannel()" label="上游账号"><el-input v-model="approve.username" required /></el-form-item>
          <el-form-item v-if="!approveOrderUsesPublicChannel()" label="上游密码"><el-input v-model="approve.password" type="text" required /></el-form-item>
          <el-form-item v-if="!approveOrderUsesPublicChannel()" class="md:col-span-2" label="上游 API Key"><el-input v-model="approve.apiKey" type="text" required /></el-form-item>
          <el-form-item class="md:col-span-2" label="审核备注"><el-input v-model="approve.adminNote" type="textarea" :rows="3" /></el-form-item>
        </div>

        <div v-if="modal.type === 'edit-order'" class="modal-body form-grid">
          <el-form-item label="订单 ID"><el-input v-model="approve.orderId" readonly /></el-form-item>
          <div v-if="modal.payload?.order?.UserPaymentNote" class="order-flow-note md:col-span-2">
            <strong>用户付款备注</strong>
            <span>{{ modal.payload.order.UserPaymentNote }}</span>
          </div>
          <el-form-item label="关联套餐">
            <el-select v-model="approve.planId" :disabled="approve.status === 'approved'">
              <el-option value="" label="不分配" />
              <el-option v-for="plan in plans" :key="plan.ID" :value="plan.ID" :label="plan.Name" />
            </el-select>
          </el-form-item>
          <el-form-item label="金额（元）"><el-input v-model.number="approve.amountRmb" type="number" min="0" step="0.01" /></el-form-item>
          <div v-if="approveOrderUsesPublicChannel()" class="order-flow-note md:col-span-2">
            <strong>公共套餐使用已绑定的公共渠道</strong>
            <span>编辑公共套餐订单时不需要维护独立上游渠道、账号、密码或 API Key。</span>
          </div>
          <el-form-item v-if="!approveOrderUsesPublicChannel()" label="上游渠道">
            <el-select v-model="approve.channelId" @change="syncApproveChannel">
              <el-option value="" label="不修改" />
              <el-option v-for="channel in channels.filter((item) => item.Enabled)" :key="channel.ID" :value="channel.ID" :label="channel.Name" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="!approveOrderUsesPublicChannel()" label="上游账号"><el-input v-model="approve.username" placeholder="留空不修改" /></el-form-item>
          <el-form-item v-if="!approveOrderUsesPublicChannel()" label="上游密码"><el-input v-model="approve.password" type="text" placeholder="留空不修改" /></el-form-item>
          <el-form-item v-if="!approveOrderUsesPublicChannel()" class="md:col-span-2" label="上游 API Key"><el-input v-model="approve.apiKey" type="text" placeholder="留空不修改" /></el-form-item>
          <el-form-item class="md:col-span-2" label="审核备注"><el-input v-model="approve.adminNote" type="textarea" :rows="3" /></el-form-item>
        </div>

        <div v-if="modal.type === 'reject-order'" class="modal-body">
          <el-form-item label="拒绝原因"><el-input v-model="rejectForm.adminNote" type="textarea" :rows="4" placeholder="请输入给内部留档的拒绝原因" /></el-form-item>
        </div>

        <div v-if="modal.type === 'close-order'" class="modal-body confirm-copy">
          <strong>确定关闭订单 #{{ modal.payload?.order?.ID }} 吗？</strong>
          <p>未支付订单会标记为支付超时，待审核订单会标记为已拒绝。</p>
        </div>

        <div v-if="modal.type === 'delete-order'" class="modal-body confirm-copy">
          <strong>确定删除订单 #{{ modal.payload?.order?.ID }} 吗？</strong>
          <p>删除后无法恢复，已通过订单不允许删除。</p>
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

        <div v-if="modal.type === 'delete-polling-pool'" class="modal-body confirm-copy">
          <strong>确定删除「{{ modal.payload?.pool?.Name }}」吗？</strong>
          <p>删除后绑定到该轮询号池的套餐将无法继续售卖，请先确认没有启用中的套餐依赖它。</p>
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

      </el-form>
      <template #footer>
        <div class="modal-actions">
          <el-button @click="closeModal">取消</el-button>
          <el-button :type="modal.danger ? 'danger' : 'primary'" @click="submitModal">{{ modal.actionLabel }}</el-button>
        </div>
      </template>
    </el-dialog>
  </section>
</template>
