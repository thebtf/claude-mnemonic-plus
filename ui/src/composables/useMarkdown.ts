import { marked } from 'marked'
import DOMPurify from 'dompurify'

// Configure marked for safe defaults
marked.use({
  breaks: true, // GFM line breaks
  gfm: true,    // GitHub Flavored Markdown
})

/**
 * Detect if a text block looks like terminal/console output and wrap it
 * in a fenced code block so marked renders it as <pre><code>.
 *
 * Heuristics:
 * - Lines starting with / # (shell prompt), $ , > , or common command output patterns
 * - YAML-like blocks (key: value on consecutive lines)
 * - Lines that look like ip/network output (inet, link/, mtu, etc.)
 *
 * Only wraps blocks of 3+ consecutive "code-like" lines.
 */
function preProcessCodeBlocks(text: string): string {
  // Don't touch text that already has fenced code blocks
  if (text.includes('```')) return text

  const lines = text.split('\n')
  const result: string[] = []
  let codeBuffer: string[] = []

  const isCodeLine = (line: string): boolean => {
    const trimmed = line.trimStart()
    if (trimmed === '') return codeBuffer.length > 0 // blank inside code block = still code
    // Shell prompts
    if (/^[/$#>]/.test(trimmed)) return true
    // Network/system output
    if (/^(inet6?|link\/|valid_lft|mtu |scope |brd )/.test(trimmed)) return true
    // IP addresses, MACs
    if (/^\d+:\s+\S+@?\S*:?\s/.test(trimmed)) return true
    // Indented lines (4+ spaces or tab) that aren't markdown lists
    if (/^(\t|    )/.test(line) && !/^(\t|    )[-*+\d]/.test(line)) return true
    // YAML-like (key: "value" or key: value)
    if (/^\w[\w_-]*:\s*".+"$/.test(trimmed)) return true
    return false
  }

  function flushCode() {
    if (codeBuffer.length >= 2) {
      result.push('```')
      result.push(...codeBuffer)
      result.push('```')
    } else {
      result.push(...codeBuffer)
    }
    codeBuffer = []
  }

  for (const line of lines) {
    if (isCodeLine(line)) {
      codeBuffer.push(line)
    } else {
      if (codeBuffer.length > 0) flushCode()
      result.push(line)
    }
  }
  if (codeBuffer.length > 0) flushCode()

  return result.join('\n')
}

export function renderMarkdown(text: string): string {
  if (!text) return ''
  const processed = preProcessCodeBlocks(text)
  const html = marked.parse(processed) as string
  return DOMPurify.sanitize(html)
}
