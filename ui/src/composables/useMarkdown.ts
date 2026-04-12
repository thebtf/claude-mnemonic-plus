import { marked } from 'marked'
import DOMPurify from 'dompurify'

// Configure marked for safe defaults
marked.setOptions({
  breaks: true, // GFM line breaks
  gfm: true,    // GitHub Flavored Markdown
})

export function renderMarkdown(text: string): string {
  if (!text) return ''
  const html = marked.parse(text, { async: false }) as string
  return DOMPurify.sanitize(html)
}
