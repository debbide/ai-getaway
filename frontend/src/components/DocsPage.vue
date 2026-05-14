<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { api } from '../api/client'

const docs = ref([])
const activeSlug = ref('')
const loading = ref(false)
const error = ref('')

const activeDoc = computed(() => docs.value.find((doc) => doc.Slug === activeSlug.value) || docs.value[0] || null)
const groupedDocs = computed(() => {
  const groups = []
  const byName = new Map()
  for (const doc of docs.value) {
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

function renderMarkdown(source) {
  const lines = String(source || '').split(/\r?\n/)
  const html = []
  let inCode = false
  let codeLang = ''
  let codeLines = []
  let inTable = false
  let tableRows = []
  let listItems = []

  const flushList = () => {
    if (!listItems.length) return
    html.push(`<ol>${listItems.map((item) => `<li>${inlineMarkdown(item)}</li>`).join('')}</ol>`)
    listItems = []
  }
  const flushTable = () => {
    if (!inTable) return
    const [head, , ...body] = tableRows
    const headers = splitTableRow(head)
    html.push('<table><thead><tr>')
    html.push(headers.map((cell) => `<th>${inlineMarkdown(cell)}</th>`).join(''))
    html.push('</tr></thead><tbody>')
    for (const row of body) {
      const cells = splitTableRow(row)
      html.push(`<tr>${cells.map((cell) => `<td>${inlineMarkdown(cell)}</td>`).join('')}</tr>`)
    }
    html.push('</tbody></table>')
    inTable = false
    tableRows = []
  }
  const flushCode = () => {
    if (!inCode) return
    html.push(`<pre><code class="language-${escapeAttr(codeLang)}">${escapeHtml(codeLines.join('\n'))}</code></pre>`)
    inCode = false
    codeLang = ''
    codeLines = []
  }

  for (const line of lines) {
    const codeMatch = line.match(/^```(\w+)?\s*$/)
    if (codeMatch) {
      if (inCode) {
        flushCode()
      } else {
        flushList()
        flushTable()
        inCode = true
        codeLang = codeMatch[1] || ''
      }
      continue
    }
    if (inCode) {
      codeLines.push(line)
      continue
    }
    if (/^\|.+\|$/.test(line)) {
      flushList()
      inTable = true
      tableRows.push(line)
      continue
    }
    flushTable()

    const ordered = line.match(/^\d+\.\s+(.+)$/)
    if (ordered) {
      listItems.push(ordered[1])
      continue
    }
    flushList()

    if (!line.trim()) continue
    if (line.startsWith('# ')) html.push(`<h1>${inlineMarkdown(line.slice(2))}</h1>`)
    else if (line.startsWith('## ')) html.push(`<h2>${inlineMarkdown(line.slice(3))}</h2>`)
    else if (line.startsWith('### ')) html.push(`<h3>${inlineMarkdown(line.slice(4))}</h3>`)
    else html.push(`<p>${inlineMarkdown(line)}</p>`)
  }
  flushCode()
  flushTable()
  flushList()
  return html.join('\n')
}

function splitTableRow(line) {
  return line.replace(/^\||\|$/g, '').split('|').map((cell) => cell.trim())
}

function inlineMarkdown(value) {
  return escapeHtml(value).replace(/`([^`]+)`/g, '<code>$1</code>').replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  })[char])
}

function escapeAttr(value) {
  return String(value || '').replace(/[^a-z0-9_-]/gi, '')
}
</script>

<template>
  <main class="docs-stage">
    <section class="docs-shell mx-auto max-w-7xl px-4 py-10 sm:px-6">
      <aside class="docs-sidebar">
        <div class="docs-search">输入关键词...</div>
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
        <div v-else-if="error" class="alert alert-danger">{{ error }}</div>
        <div v-else-if="!activeDoc" class="docs-empty">暂无可用文档</div>
        <div v-else>
          <p class="section-kicker">Configuration</p>
          <div class="docs-content" v-html="renderedContent"></div>
        </div>
      </article>
    </section>
  </main>
</template>
