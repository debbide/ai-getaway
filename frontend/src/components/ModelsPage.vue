<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api/client'

const emit = defineEmits(['navigate', 'start'])

const models = ref([])
const loading = ref(false)
const error = ref('')
const activeProvider = ref('all')

const providers = computed(() => {
  const names = [...new Set(models.value.map((item) => providerName(item)).filter(Boolean))]
  return ['all', ...names]
})

const filteredModels = computed(() => {
  if (activeProvider.value === 'all') return models.value
  return models.value.filter((item) => providerName(item) === activeProvider.value)
})

const featuredModels = computed(() => {
  const source = filteredModels.value
  const featured = source.filter((item) => item.Featured || item.featured)
  return (featured.length ? featured : source).slice(0, 3)
})

const averageMultiplier = computed(() => {
  if (!models.value.length) return '1.00'
  const total = models.value.reduce((sum, item) => sum + multiplier(item), 0)
  return (total / models.value.length).toFixed(2)
})

onMounted(loadModels)

watch(error, (message) => {
  if (message) ElMessage.error(message)
})

async function loadModels() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/models')
    models.value = res.data || []
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

function providerName(item) {
  return item.Provider || item.provider || 'openai'
}

function displayName(item) {
  return item.DisplayName || item.ModelName || item.model || 'Untitled model'
}

function modelName(item) {
  return item.ModelName || item.model || displayName(item)
}

function multiplier(item) {
  const value = Number(item.BillingMultiplier || item.billing_multiplier || 1)
  return Number.isFinite(value) && value > 0 ? value : 1
}

function priceValue(item, key) {
  return Number(item[key] || 0) * multiplier(item)
}

function billingMode(item) {
  return (item.BillingMode || item.billing_mode) === 'request' ? 'request' : 'token'
}

function requestPriceValue(item) {
  return Number(item.RequestUSD || item.request_usd || 0) * multiplier(item)
}

function priceText(value) {
  const number = Number(value || 0)
  if (!Number.isFinite(number)) return '$0'
  if (number >= 1) return `$${number.toFixed(number % 1 === 0 ? 0 : 2)}`
  return `$${number.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')}`
}

function modelUnitPrice(item, key) {
  if (billingMode(item) === 'request') return '-'
  return priceText(priceValue(item, key))
}

function capabilityTags(item) {
  const name = modelName(item).toLowerCase()
  const tags = []
  if (name.includes('codex')) tags.push('代码任务')
  if (name.includes('mini') || name.includes('nano')) tags.push('轻量快速')
  if (name.includes('gpt-5') || name.includes('gpt-4')) tags.push('通用推理')
  if (!tags.length) tags.push('兼容调用')
  tags.push('OpenAI 格式')
  return tags.slice(0, 3)
}

function selectProvider(provider) {
  activeProvider.value = provider
}
</script>

<template>
  <main class="models-stage">
    <section class="models-hero mx-auto max-w-7xl px-4 py-14 sm:px-6">
      <div class="models-hero-copy">
        <p class="section-kicker">Model Catalog</p>
        <h1>模型列表</h1>
        <p>这里展示后台「模型管理」中已启用的模型，价格按实际扣费倍率计算，单位为每 1M Token。</p>
        <div class="models-actions">
          <el-button type="primary" size="large" @click="emit('start')">立即使用</el-button>
          <el-button size="large" plain @click="emit('navigate', '/docs')">查看接入文档</el-button>
        </div>
      </div>

      <div class="models-summary-panel">
        <div class="models-summary-grid">
          <div>
            <span>启用模型</span>
            <strong>{{ models.length }}</strong>
          </div>
          <div>
            <span>服务商</span>
            <strong>{{ Math.max(providers.length - 1, 0) }}</strong>
          </div>
          <div>
            <span>平均倍率</span>
            <strong>{{ averageMultiplier }}x</strong>
          </div>
        </div>
        <div class="models-signal">
          <span></span>
          <div>
            <strong>实时配置展示</strong>
            <small>管理员新增、停用或调价后，刷新本页即可同步。</small>
          </div>
        </div>
      </div>
    </section>

    <section class="models-shell mx-auto max-w-7xl px-4 pb-14 sm:px-6">
      <div class="models-toolbar">
        <div>
          <p class="section-kicker">Available Models</p>
          <h2>可用模型</h2>
          <span>只展示状态为启用的后台模型配置。</span>
        </div>
        <el-segmented
          class="models-filter"
          :model-value="activeProvider"
          :options="providers.map((provider) => ({ label: provider === 'all' ? '全部' : provider, value: provider }))"
          @update:model-value="selectProvider"
        />
      </div>

      <div v-if="loading" class="models-empty">模型加载中...</div>
      <div v-else-if="!models.length" class="models-empty">暂无启用模型，请先在后台模型管理中新增或启用模型。</div>
      <template v-else>
        <div class="models-feature-grid">
          <article v-for="item in featuredModels" :key="item.ID" class="model-feature-card">
            <div class="model-card-topline">
              <span>{{ providerName(item) }}</span>
              <b>{{ billingMode(item) === 'request' ? '按次' : `${multiplier(item).toFixed(2)}x` }}</b>
            </div>
            <h2>{{ displayName(item) }}</h2>
            <code>{{ modelName(item) }}</code>
            <div class="model-price-row">
              <div>
                <span>{{ billingMode(item) === 'request' ? '每次' : '输入' }}</span>
                <strong>{{ billingMode(item) === 'request' ? priceText(requestPriceValue(item)) : priceText(priceValue(item, 'InputUSDPerMillion')) }}</strong>
              </div>
              <div>
                <span>缓存</span>
                <strong>{{ modelUnitPrice(item, 'CachedInputUSDPerMillion') }}</strong>
              </div>
              <div>
                <span>输出</span>
                <strong>{{ modelUnitPrice(item, 'OutputUSDPerMillion') }}</strong>
              </div>
            </div>
            <div class="model-tags">
              <span v-for="tag in capabilityTags(item)" :key="tag">{{ tag }}</span>
            </div>
          </article>
        </div>

        <div class="models-table-card">
          <el-table :data="filteredModels" class="models-table" border>
            <el-table-column label="模型" min-width="220">
              <template #default="{ row: item }">
                  <div class="model-name-cell">
                    <strong>{{ displayName(item) }}</strong>
                    <small>{{ modelName(item) }}</small>
                  </div>
              </template>
            </el-table-column>
            <el-table-column label="服务商" min-width="120">
              <template #default="{ row: item }">
                <el-tag type="success">{{ providerName(item) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="计费" min-width="130">
              <template #default="{ row: item }">{{ billingMode(item) === 'request' ? `${priceText(requestPriceValue(item))} / 次` : '按 Token' }}</template>
            </el-table-column>
            <el-table-column label="输入 / 1M" min-width="130">
              <template #default="{ row: item }">{{ modelUnitPrice(item, 'InputUSDPerMillion') }}</template>
            </el-table-column>
            <el-table-column label="缓存读取 / 1M" min-width="150">
              <template #default="{ row: item }">{{ modelUnitPrice(item, 'CachedInputUSDPerMillion') }}</template>
            </el-table-column>
            <el-table-column label="输出 / 1M" min-width="130">
              <template #default="{ row: item }">{{ modelUnitPrice(item, 'OutputUSDPerMillion') }}</template>
            </el-table-column>
            <el-table-column label="倍率" min-width="100">
              <template #default="{ row: item }">{{ billingMode(item) === 'request' ? '-' : `${multiplier(item).toFixed(2)}x` }}</template>
            </el-table-column>
            <el-table-column label="说明" min-width="220">
              <template #default="{ row: item }">{{ item.Notes || '标准 OpenAI 兼容调用' }}</template>
            </el-table-column>
          </el-table>
        </div>
      </template>
    </section>
  </main>
</template>
