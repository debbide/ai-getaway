<script setup>
import { computed, onBeforeUnmount, shallowRef } from 'vue'
import { Editor, Toolbar } from '@wangeditor/editor-for-vue'
import { Boot } from '@wangeditor/editor'
import markdownModule from '@wangeditor/plugin-md'
import '@wangeditor/editor/dist/css/style.css'
import { htmlToMarkdown, markdownToHtml, renderMarkdown } from '../utils/markdown'

let markdownModuleRegistered = false
try {
  Boot.registerModule(markdownModule)
  markdownModuleRegistered = true
} catch {
  markdownModuleRegistered = true
}

defineOptions({ name: 'MarkdownEditor' })

const props = defineProps({
  modelValue: { type: String, default: '' },
  minHeight: { type: Number, default: 360 },
  placeholder: { type: String, default: '输入 Markdown 内容' }
})

const emit = defineEmits(['update:modelValue'])

const editorRef = shallowRef(null)
const mode = shallowRef('rich')
const parsePasteMarkdown = shallowRef(true)

const richHtml = computed(() => markdownToHtml(props.modelValue))
const previewHtml = computed(() => renderMarkdown(props.modelValue))

const toolbarConfig = {
  toolbarKeys: [
    'headerSelect',
    'bold',
    'italic',
    'underline',
    'color',
    'bulletedList',
    'numberedList',
    'blockquote',
    'codeBlock',
    'insertLink',
    'divider',
    'undo',
    'redo'
  ]
}

const editorConfig = computed(() => ({
  placeholder: props.placeholder,
  MENU_CONF: {
    uploadImage: { base64LimitSize: 0 }
  }
}))

onBeforeUnmount(() => {
  const editor = editorRef.value
  if (editor) editor.destroy()
})

function handleCreated(editor) {
  editorRef.value = editor
}

function updateMarkdown(value) {
  emit('update:modelValue', value)
}

function handleRichChange(editor) {
  updateMarkdown(htmlToMarkdown(editor.getHtml()))
}

function handlePaste(editor, event, callback) {
  if (!parsePasteMarkdown.value) {
    callback(false)
    return
  }
  const text = event.clipboardData?.getData('text/plain') || ''
  if (!looksLikeMarkdown(text)) {
    callback(false)
    return
  }
  event.preventDefault()
  const next = [props.modelValue, text].filter(Boolean).join(props.modelValue ? '\n\n' : '')
  updateMarkdown(next)
  editor.setHtml(markdownToHtml(next))
  callback(true)
}

function looksLikeMarkdown(value) {
  const source = String(value || '').trim()
  if (!source) return false
  return /(^|\n)(#{1,6}\s|[-*+]\s|\d+\.\s|>\s|```|\|.+\|)/.test(source) || /\[[^\]]+\]\([^)]+\)|\*\*[^*]+\*\*|`[^`]+`/.test(source)
}
</script>

<template>
  <div class="md-editor-shell">
    <div class="md-editor-toolbar">
      <el-segmented v-model="mode" :options="[{ label: '富文本', value: 'rich' }, { label: 'Markdown 源码', value: 'source' }]" />
      <el-switch v-model="parsePasteMarkdown" active-text="粘贴解析 Markdown" inactive-text="原样粘贴" />
    </div>

    <div v-if="mode === 'rich'" class="md-rich-editor" :style="{ minHeight: `${minHeight}px` }">
      <Toolbar class="md-rich-toolbar" :editor="editorRef" :default-config="toolbarConfig" mode="default" />
      <Editor
        class="md-rich-body"
        :style="{ height: `${minHeight}px`, overflowY: 'hidden' }"
        :model-value="richHtml"
        :default-config="editorConfig"
        mode="default"
        @on-created="handleCreated"
        @on-change="handleRichChange"
        @custom-paste="handlePaste"
      />
    </div>

    <div v-else class="md-source-editor" :style="{ minHeight: `${minHeight}px` }">
      <el-input
        class="md-source-input"
        :model-value="modelValue"
        type="textarea"
        :autosize="{ minRows: 14, maxRows: 28 }"
        :placeholder="placeholder"
        @update:model-value="updateMarkdown"
      />
      <div class="md-source-preview">
        <div class="markdown-body" v-html="previewHtml"></div>
      </div>
    </div>
  </div>
</template>
