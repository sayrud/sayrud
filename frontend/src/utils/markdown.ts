import DOMPurify from 'dompurify'
import { Marked } from 'marked'

const marked = new Marked({ gfm: true, breaks: false, async: false })

// The links of the third-party documents open aside, and their images do not tell the remote hosts where they are shown.
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A' && node.getAttribute('href')) {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer nofollow')
  }
  if (node.tagName === 'IMG') {
    node.setAttribute('loading', 'lazy')
    node.setAttribute('referrerpolicy', 'no-referrer')
  }
})

/** Renders the Markdown of a third-party document as sanitized HTML. */
export function renderMarkdown(source: string): string {
  const html = marked.parse(source) as string
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ['style', 'form', 'input', 'button', 'textarea', 'select'],
    FORBID_ATTR: ['style'],
  })
}
