<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api/client'
import { renderMarkdown } from '../utils/markdown'

const docs = ref([])
const activeSlug = ref('')
const loading = ref(false)
const error = ref('')
const searchKeyword = ref('')

const normalizedKeyword = computed(() => searchKeyword.value.trim().toLowerCase())
const filteredDocs = computed(() => {
  if (!normalizedKeyword.value) return docs.value
  return docs.value.filter((doc) => {
    const content = [doc.Title, doc.GroupName, doc.Slug, doc.Content].filter(Boolean).join(' ').toLowerCase()
    return content.includes(normalizedKeyword.value)
  })
})
const activeDoc = computed(() => {
  const visibleActive = filteredDocs.value.find((doc) => doc.Slug === activeSlug.value)
  return visibleActive || filteredDocs.value[0] || null
})
const groupedDocs = computed(() => {
  const groups = []
  const byName = new Map()
  for (const doc of filteredDocs.value) {
    const name = doc.GroupName || '配置文档'
    if (!byName.has(name)) {
      byName.set(name, { name, items: [] })
      groups.push(byName.get(name))
    }
    byName.get(name).items.push(doc)
  }
  return groups
})
const renderedContent = computed(() => renderMarkdown(activeDoc.value?.Content || ''))

onMounted(loadDocs)

watch(activeDoc, (doc) => {
  if (!doc) return
  const url = `/docs/${doc.Slug}`
  if (window.location.pathname !== url) {
    window.history.replaceState({}, '', url)
  }
})

watch(searchKeyword, () => {
  if (!activeDoc.value && filteredDocs.value[0]) {
    activeSlug.value = filteredDocs.value[0].Slug
  }
})

watch(error, (message) => {
  if (message) ElMessage.error(message)
})

async function loadDocs() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/docs')
    docs.value = res.data || []
    const slug = window.location.pathname.replace(/^\/docs\/?/, '').replace(/^\/+/, '')
    activeSlug.value = docs.value.some((doc) => doc.Slug === slug) ? slug : docs.value[0]?.Slug || ''
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

function selectDoc(doc) {
  activeSlug.value = doc.Slug
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

</script>

<template>
  <main class="docs-stage">
    <section class="docs-shell mx-auto max-w-7xl px-4 py-10 sm:px-6">
      <aside class="docs-sidebar">
        <label class="docs-search">
          <span>搜索文档</span>
          <input v-model="searchKeyword" type="search" placeholder="输入关键词..." autocomplete="off" />
        </label>
        <div v-if="filteredDocs.length === 0" class="docs-no-results">没有找到匹配文档</div>
        <div v-for="group in groupedDocs" :key="group.name" class="docs-nav-group">
          <strong>{{ group.name }}</strong>
          <button
            v-for="doc in group.items"
            :key="doc.ID"
            class="docs-nav-item"
            :class="{ active: activeDoc?.ID === doc.ID }"
            @click="selectDoc(doc)"
          >
            {{ doc.Title }}
          </button>
        </div>
        <div class="docs-help-card">
          <strong>配置中未找到的问题，或者报错信息</strong>
          <span>请联系管理员，并附上客户端、模型 ID 和完整错误提示。</span>
        </div>
      </aside>

      <article class="docs-content-panel">
        <div v-if="loading" class="docs-empty">文档加载中...</div>
        <div v-else-if="error" class="docs-empty">文档暂时不可用</div>
        <div v-else-if="!activeDoc" class="docs-empty">暂无可用文档</div>
        <div v-else>
          <p class="section-kicker">Configuration</p>
          <div class="docs-content" v-html="renderedContent"></div>
        </div>
      </article>
    </section>
  </main>
</template>
