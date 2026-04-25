import { marked, type Tokens } from 'marked'
import DOMPurify from 'dompurify'
import { codeToHtml } from 'shiki'
import { transformerNotationDiff } from '@shikijs/transformers'

// Configure marked
marked.use({
  breaks: true,
  gfm: true,
  async: true,
})

// Language aliases
const langAliases: Record<string, string> = {
  sh: 'bash', shell: 'bash', zsh: 'bash',
  js: 'javascript', ts: 'typescript',
  py: 'python', rb: 'ruby',
  yml: 'yaml', dockerfile: 'docker',
  console: 'bash', terminal: 'bash',
  ps: 'powershell', ps1: 'powershell',
}

const supportedLangs = new Set([
  'javascript', 'typescript', 'python', 'go', 'rust', 'bash',
  'json', 'yaml', 'toml', 'html', 'css', 'vue', 'sql', 'docker',
  'markdown', 'diff', 'ini', 'xml', 'c', 'cpp', 'java', 'ruby',
  'php', 'swift', 'kotlin', 'lua', 'powershell', 'graphql', 'nginx',
])

function resolveLanguage(lang: string | undefined): string | null {
  if (!lang) return null
  const lower = lang.toLowerCase().trim()
  const resolved = langAliases[lower] || lower
  return supportedLangs.has(resolved) ? resolved : null
}

// Shiki-powered code renderer
const renderer = new marked.Renderer()

interface PendingHighlight {
  id: string
  code: string
  lang: string | null
}

let pendingHighlights: PendingHighlight[] = []
let highlightCounter = 0

renderer.code = function (this: unknown, token: Tokens.Code): string {
  const { text, lang } = token
  const resolvedLang = resolveLanguage(lang)
  const isDiff = lang === 'diff' || (!lang && looksLikeDiff(text))

  if (resolvedLang || isDiff) {
    const id = `shiki-${++highlightCounter}`
    pendingHighlights.push({
      id,
      code: text,
      lang: isDiff ? 'diff' : resolvedLang,
    })
    return `<div id="${id}" class="shiki-placeholder"><pre><code>${escapeHtml(text)}</code></pre></div>`
  }

  return `<pre class="code-block"><code>${escapeHtml(text)}</code></pre>`
}

marked.use({ renderer })

function looksLikeDiff(text: string): boolean {
  const lines = text.split('\n')
  let diffLines = 0
  for (const line of lines) {
    if (/^[+-](?![-+]{2})/.test(line) || /^@@/.test(line)) diffLines++
  }
  return diffLines >= 2 && diffLines / lines.length > 0.3
}

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

/**
 * Render markdown with shiki syntax highlighting.
 * Markdown is rendered as-is — no auto-detection of code blocks.
 * Content must use proper fenced code blocks (```lang ... ```) for highlighting.
 */
export async function renderMarkdownAsync(text: string): Promise<string> {
  if (!text) return ''

  pendingHighlights = []
  highlightCounter = 0

  let html = await marked.parse(text)

  // Replace placeholders with shiki-highlighted code
  for (const { id, code, lang } of pendingHighlights) {
    try {
      const highlighted = await codeToHtml(code, {
        lang: lang || 'text',
        themes: {
          light: 'github-light',
          dark: 'github-dark',
        },
        defaultColor: false,
        transformers: [
          transformerNotationDiff(),
        ],
      })
      html = html.replace(
        `<div id="${id}" class="shiki-placeholder"><pre><code>${escapeHtml(code)}</code></pre></div>`,
        `<div class="shiki-block">${highlighted}</div>`
      )
    } catch {
      // Shiki failed — keep fallback
    }
  }

  return DOMPurify.sanitize(html, {
    ADD_TAGS: ['span'],
    ADD_ATTR: ['style', 'class'],
  })
}

/**
 * Sync fallback (no shiki highlighting).
 */
export function renderMarkdown(text: string): string {
  if (!text) return ''
  const html = marked.parse(text) as string
  return DOMPurify.sanitize(html)
}
