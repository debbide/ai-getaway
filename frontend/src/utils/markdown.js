import DOMPurify from 'dompurify'
import { marked } from 'marked'
import TurndownService from 'turndown'

marked.setOptions({
  breaks: true,
  gfm: true
})

const turndownService = new TurndownService({
  codeBlockStyle: 'fenced',
  headingStyle: 'atx'
})

export function markdownToHtml(source) {
  return marked.parse(String(source || ''))
}

export function sanitizeHtml(html) {
  return DOMPurify.sanitize(String(html || ''), {
    ADD_ATTR: ['target', 'rel']
  })
}

export function renderMarkdown(source) {
  return sanitizeHtml(markdownToHtml(source))
}

export function htmlToMarkdown(html) {
  return turndownService.turndown(String(html || '')).trim()
}

export function plainTextFromMarkdown(source) {
  if (typeof window === 'undefined') return String(source || '').replace(/[#*_`>\-[\]()]/g, '').trim()
  const div = window.document.createElement('div')
  div.innerHTML = renderMarkdown(source)
  return (div.textContent || '').trim()
}
