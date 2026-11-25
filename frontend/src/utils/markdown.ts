import DOMPurify from 'dompurify'
import { Marked, type TokenizerAndRendererExtension, type Tokens } from 'marked'

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

const SANITIZE = {
  USE_PROFILES: { html: true },
  FORBID_TAGS: ['style', 'form', 'input', 'button', 'textarea', 'select'],
  FORBID_ATTR: ['style'],
}

/** Renders the Markdown of a third-party document as sanitized HTML. */
export function renderMarkdown(source: string): string {
  const html = marked.parse(source) as string
  return DOMPurify.sanitize(html, SANITIZE)
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!)
}

interface MentionToken extends Tokens.Generic {
  userId: number
}

/** Names the mentions while a comment is rendered, the parsing is synchronous. */
let mentionName: (id: number) => string = () => ''

const mention: TokenizerAndRendererExtension = {
  name: 'mention',
  level: 'inline',
  start: (src) => src.indexOf('<@'),
  tokenizer(src) {
    const m = /^<@(\d{1,16})>/.exec(src)
    return m ? { type: 'mention', raw: m[0], userId: Number(m[1]) } : undefined
  },
  renderer(token) {
    const id = (token as MentionToken).userId
    return `<span class="comment-mention" data-user-id="${id}">@${escapeHtml(mentionName(id) || String(id))}</span>`
  },
}

// Comments are written by any member, so their raw HTML is shown as text and only the mention tags are generated.
const commentMarked = new Marked({
  gfm: true,
  breaks: true,
  async: false,
  extensions: [mention],
  renderer: { html: ({ text }) => escapeHtml(text) },
})

/** Renders the Markdown of a comment as sanitized HTML, `<@userID>` becomes a mention tag named by nameOf. */
export function renderComment(source: string, nameOf: (id: number) => string): string {
  mentionName = nameOf
  try {
    return DOMPurify.sanitize(commentMarked.parse(source) as string, SANITIZE)
  } finally {
    mentionName = () => ''
  }
}
