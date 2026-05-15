<script setup>
import { computed, ref } from 'vue'

const emit = defineEmits(['navigate', 'start'])

const search = ref('')
const activeCategory = ref('all')

const categories = [
  { id: 'all', label: '全部问题' },
  { id: 'getting-started', label: '接入配置' },
  { id: 'account', label: '账号套餐' },
  { id: 'request', label: '请求报错' },
  { id: 'billing', label: '扣费额度' },
  { id: 'client', label: '客户端' }
]

const faqItems = [
  {
    category: 'getting-started',
    question: 'API 地址应该填哪里？',
    answer:
      '在支持 OpenAI 格式的客户端里，把 Base URL 填为本站提供的 API 端点。如果客户端要求填写完整路径，通常使用 /v1；如果客户端会自动拼接 /v1，就只填写域名根地址。以后台「API 端点」页面展示的地址为准。',
    solution: [
      '先复制控制台或公告中给出的 API 端点。',
      '确认客户端里没有重复写两次 /v1。',
      '用 /v1/chat/completions 做一次最小请求验证连通性。'
    ]
  },
  {
    category: 'getting-started',
    question: 'API Key 从哪里获取？为什么不能用上游官方 Key？',
    answer:
      '用户应该使用本站控制台生成的平台 API Key。平台 Key 用于识别你的账号、套餐、额度和调用日志；系统会在服务端自动替换为已绑定的上游 Key 后再转发。',
    solution: [
      '登录后进入控制台，创建或查看 API Key。',
      '请求时使用 Authorization: Bearer 加平台 Key。',
      '不要把上游官方 Key 填到普通客户端里，也不要把 Key 发给他人。'
    ]
  },
  {
    category: 'getting-started',
    question: '中转站兼容哪些接口？',
    answer:
      '本站按 OpenAI 兼容格式转发 /v1/* 请求，常用的 chat completions、responses、embeddings 等接口取决于上游和后台模型配置是否支持。普通响应和 SSE 流式输出都可以透传。',
    solution: [
      '优先使用 OpenAI 兼容客户端或 SDK。',
      '模型 ID 必须填写 /models 页面展示的可用模型。',
      '如果某个接口返回 404 或模型不存在，先换成已展示的模型 ID 测试。'
    ]
  },
  {
    category: 'account',
    question: '注册后为什么还不能调用？',
    answer:
      '新账号可能需要购买套餐、完成支付或等待管理员审核开通。未开通、已到期、无套餐或未绑定上游渠道时，请求会被拒绝。',
    solution: [
      '先在定价页选择套餐并完成下单。',
      '支付后如仍未开通，等待管理员审核或联系站点支持。',
      '在控制台检查套餐状态、到期时间和 API Key 状态。'
    ]
  },
  {
    category: 'account',
    question: '套餐到期或额度用完会发生什么？',
    answer:
      '套餐过期、日额度/周额度耗尽、公共渠道余额不足时，系统会停止继续转发请求，避免产生不可控费用。',
    solution: [
      '进入控制台查看套餐有效期和额度消耗。',
      '日限额套餐等待下一个自然日额度恢复，周限额套餐等待下一周期恢复。',
      '需要立即继续使用时，续费或购买更高额度套餐。'
    ]
  },
  {
    category: 'request',
    question: '401 Unauthorized 怎么处理？',
    answer:
      '401 通常表示 API Key 缺失、格式错误、Key 已被禁用、复制时多了空格，或者把登录密码、上游 Key 当成平台 Key 使用。',
    solution: [
      '确认请求头是 Authorization: Bearer ag_xxx。',
      '重新复制控制台里的完整 API Key，去掉前后空格和换行。',
      '检查 API Key 状态是否为启用；必要时轮换生成新 Key。'
    ]
  },
  {
    category: 'request',
    question: '403 Forbidden 或账号未审核是什么意思？',
    answer:
      '403 多半是账号未开通、套餐不可用、当前用户状态不允许调用，或管理员尚未完成订单审核与上游绑定。',
    solution: [
      '确认订单状态是否已完成并审核通过。',
      '查看控制台是否显示有效套餐。',
      '把账号邮箱、订单号和报错时间发给管理员排查。'
    ]
  },
  {
    category: 'request',
    question: '404 model not found 或模型不存在怎么办？',
    answer:
      '模型 ID 必须与后台启用模型完全一致。大小写、后缀、供应商别名写错，或调用了本站未开放的模型，都会导致模型不存在。',
    solution: [
      '打开 /models 复制模型 ID，不要只复制展示名称。',
      '把客户端里的默认模型改成本站可用模型。',
      '如果需要新模型，联系管理员在后台模型管理中添加并启用。'
    ]
  },
  {
    category: 'request',
    question: '429 Too Many Requests 是限流还是额度问题？',
    answer:
      '429 可能来自本站分钟级限流，也可能来自上游模型服务的限速、并发限制或临时拥塞。',
    solution: [
      '降低并发数、请求频率和上下文长度。',
      '开启客户端重试，但要使用指数退避，不要立刻高频重试。',
      '如果持续出现，换低负载模型或联系管理员查看上游状态。'
    ]
  },
  {
    category: 'request',
    question: '500、502、503、504 这类服务错误怎么排查？',
    answer:
      '5xx 一般表示上游暂时不可用、网络超时、渠道配置异常、上游 Key 失效，或请求体超过上游限制。',
    solution: [
      '先用短提示词和轻量模型重试，排除请求过大。',
      '检查是否只有某个模型异常，还是所有模型都异常。',
      '保留完整错误、模型 ID、请求时间和 trace 信息后联系管理员。'
    ]
  },
  {
    category: 'request',
    question: '流式输出断开、卡住或没有逐字返回怎么办？',
    answer:
      '流式输出依赖客户端、浏览器代理、网络和上游 SSE 支持。某些客户端会把流式响应缓存后一次性展示，看起来像没有流式。',
    solution: [
      '确认请求里 stream: true，并且客户端支持 SSE。',
      '尝试关闭本地代理、公司网关或浏览器插件后重试。',
      '如果普通响应正常但流式异常，换客户端或改用非流式模式临时处理。'
    ]
  },
  {
    category: 'billing',
    question: '为什么一次对话扣费比预期高？',
    answer:
      '计费通常按输入、缓存读取和输出 Token 综合计算。长上下文、多轮历史、系统提示词、工具调用和大段文件都会增加输入 Token；模型倍率也会影响最终消耗。',
    solution: [
      '在使用记录里查看输入、输出 Token 和模型倍率。',
      '减少历史消息、压缩上下文，避免把无关文件整段发送。',
      '对简单任务使用低成本模型，对复杂任务再切换高阶模型。'
    ]
  },
  {
    category: 'billing',
    question: '公共套餐和订阅套餐有什么区别？',
    answer:
      '订阅套餐通常按账号周期额度使用；公共套餐使用站点维护的公共渠道余额，适合一次性或活动额度。实际售卖方式以定价页展示为准。',
    solution: [
      '长期稳定使用优先选订阅套餐。',
      '短期试用或临时需求可以选择公共套餐。',
      '公共渠道售罄或余额不足时，需要等待补货或改购订阅套餐。'
    ]
  },
  {
    category: 'client',
    question: 'Chatbox、Cherry Studio、Cursor、Open WebUI 应该怎么填？',
    answer:
      '核心配置都一样：供应商选择 OpenAI 或 OpenAI Compatible，Base URL 填本站 API 地址，API Key 填平台 Key，模型填写 /models 中的模型 ID。',
    solution: [
      '供应商：OpenAI Compatible 或自定义 OpenAI。',
      'Base URL：使用站点 API 端点，按客户端要求决定是否带 /v1。',
      'Model：复制 /models 的模型 ID，例如后台开放的 gpt 系列模型。'
    ]
  },
  {
    category: 'client',
    question: '为什么浏览器能打开网站，但客户端请求失败？',
    answer:
      '网页能打开只代表前端可访问，不代表客户端的 API 地址、代理、DNS、证书和请求头都正确。',
    solution: [
      '在同一台机器用 curl 测试 /v1/chat/completions。',
      '检查客户端是否走了错误代理，或代理拦截了 Authorization 请求头。',
      '确认系统时间正确，证书校验没有被本地安全软件阻断。'
    ]
  },
  {
    category: 'client',
    question: '请求很慢应该先看哪里？',
    answer:
      '响应慢通常与模型负载、输出长度、网络线路、上游排队和客户端代理有关。高阶推理模型、长输出任务天然更慢。',
    solution: [
      '减少 max_tokens 或限制输出长度。',
      '换轻量模型测试是否明显变快。',
      '在使用记录里对比响应耗时，并把慢请求时间点反馈给管理员。'
    ]
  }
]

const quickChecks = [
  'Base URL 是否正确，是否重复或遗漏 /v1',
  '请求头是否为 Authorization: Bearer 平台 API Key',
  '模型 ID 是否来自 /models 页面并且完全一致',
  '账号套餐是否已开通、未到期且额度未耗尽',
  '客户端代理、网络和证书是否正常',
  '错误发生时的模型、时间、请求 ID 和完整报错是否已保留'
]

const filteredItems = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  return faqItems.filter((item) => {
    const inCategory = activeCategory.value === 'all' || item.category === activeCategory.value
    if (!keyword) return inCategory
    const haystack = `${item.question} ${item.answer} ${item.solution.join(' ')}`.toLowerCase()
    return inCategory && haystack.includes(keyword)
  })
})

function categoryCount(id) {
  if (id === 'all') return faqItems.length
  return faqItems.filter((item) => item.category === id).length
}
</script>

<template>
  <main class="faq-stage">
    <section class="faq-hero mx-auto max-w-7xl px-4 py-14 sm:px-6">
      <div class="faq-hero-copy">
        <p class="section-kicker">FAQ</p>
        <h1>常见问题与排查方案</h1>
        <p>
          汇集中转站用户最常见的接入、账号、报错、扣费和客户端配置问题。先按下面的清单排查，仍不能解决时，把关键信息发给管理员会更快定位。
        </p>
        <div class="faq-actions">
          <button class="hero-primary" type="button" @click="emit('start')">进入控制台 <span>→</span></button>
          <button class="hero-secondary" type="button" @click="emit('navigate', '/docs')">查看教程</button>
        </div>
      </div>

      <aside class="faq-check-panel">
        <strong>报错前先确认</strong>
        <ul>
          <li v-for="item in quickChecks" :key="item">{{ item }}</li>
        </ul>
      </aside>
    </section>

    <section class="faq-shell mx-auto max-w-7xl px-4 pb-14 sm:px-6">
      <div class="faq-toolbar">
        <label class="faq-search">
          <span>搜索问题</span>
          <input v-model="search" type="search" placeholder="输入 401、模型不存在、额度、Base URL..." />
        </label>
        <div class="faq-category-tabs" aria-label="FAQ 分类">
          <button
            v-for="category in categories"
            :key="category.id"
            type="button"
            :class="{ active: activeCategory === category.id }"
            @click="activeCategory = category.id"
          >
            {{ category.label }}
            <small>{{ categoryCount(category.id) }}</small>
          </button>
        </div>
      </div>

      <div class="faq-content-grid">
        <article v-for="item in filteredItems" :key="item.question" class="faq-card">
          <div class="faq-card-head">
            <span>{{ categories.find((category) => category.id === item.category)?.label }}</span>
            <h2>{{ item.question }}</h2>
          </div>
          <p>{{ item.answer }}</p>
          <div class="faq-solution">
            <strong>解决方案</strong>
            <ol>
              <li v-for="step in item.solution" :key="step">{{ step }}</li>
            </ol>
          </div>
        </article>

        <div v-if="!filteredItems.length" class="faq-empty">
          <strong>没有找到匹配问题</strong>
          <span>换一个关键词，或按「全部问题」浏览完整排查清单。</span>
        </div>
      </div>

      <section class="faq-support-strip">
        <div>
          <p class="section-kicker">Need Support</p>
          <h2>联系管理员时请附上这些信息</h2>
          <p>账号邮箱、API Key 前缀、模型 ID、请求时间、完整错误、客户端名称、是否流式请求，以及控制台使用记录里的耗时和 Token 信息。</p>
        </div>
        <button class="primary-button" type="button" @click="emit('navigate', '/usage-records')">查看使用记录</button>
      </section>
    </section>
  </main>
</template>
