<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { refText, splitRefs, type RefStyle } from '@/utils/workflow'

/**
 * Input that accepts inserted variable chips: templates write variables as {{path}} and formulas as {path}, displayed as non-editable chips.
 */
const props = withDefaults(
  defineProps<{
    modelValue: string
    refStyle?: RefStyle
    labelOf: (path: string) => string
    multiline?: boolean
    readonly?: boolean
    placeholder?: string
    mono?: boolean
  }>(),
  { refStyle: 'template', multiline: false, readonly: false, placeholder: '', mono: false },
)
const emit = defineEmits<{ 'update:modelValue': [value: string]; focus: []; blur: [] }>()

const root = ref<HTMLDivElement>()
/** The value most recently produced by input; when the external value matches it, skip re-rendering so the caret isn't disrupted. */
let lastValue: string | null = null
let savedRange: Range | null = null

const ZWSP = '\u200b'

function chip(path: string) {
  const el = document.createElement('span')
  el.className = 'token-chip'
  el.contentEditable = 'false'
  el.dataset.ref = path
  el.textContent = props.labelOf(path) || path
  el.title = path
  return el
}

function render(value: string) {
  const el = root.value
  if (!el) return
  const nodes = splitRefs(value, props.refStyle).map((t) => ('ref' in t ? chip(t.ref) : document.createTextNode(t.text)))
  // When it ends with a chip, append a zero-width character so the caret can rest after the chip.
  if (nodes.length && nodes[nodes.length - 1] instanceof HTMLElement) nodes.push(document.createTextNode(ZWSP))
  el.replaceChildren(...nodes)
  lastValue = value
}

function serialize(): string {
  const el = root.value
  if (!el) return ''
  let out = ''
  const walk = (parent: Node) => {
    parent.childNodes.forEach((n, i) => {
      if (n.nodeType === Node.TEXT_NODE) {
        out += (n.nodeValue ?? '').replaceAll(ZWSP, '').replaceAll('\u00a0', ' ')
      } else if (n instanceof HTMLElement) {
        if (n.dataset.ref !== undefined) out += refText(n.dataset.ref, props.refStyle)
        else if (n.tagName === 'BR') {
          // The placeholder line break the browser appends at the end does not count as content.
          if (!(parent === el && i === parent.childNodes.length - 1)) out += '\n'
        } else {
          if (out && !out.endsWith('\n')) out += '\n'
          walk(n)
        }
      }
    })
  }
  walk(el)
  return props.multiline ? out : out.replace(/\n/g, ' ')
}

function onInput() {
  const value = serialize()
  lastValue = value
  emit('update:modelValue', value)
}

watch(
  () => props.modelValue,
  (v) => {
    if (v !== lastValue) render(v)
  },
)
// Update chip text when variable labels change (e.g. a step is renamed).
const labels = computed(() => splitRefs(props.modelValue, props.refStyle).flatMap((t) => ('ref' in t ? [`${t.ref}\n${props.labelOf(t.ref)}`] : [])).join('\n'))
watch(labels, () => {
  root.value?.querySelectorAll<HTMLElement>('[data-ref]').forEach((el) => {
    el.textContent = props.labelOf(el.dataset.ref!) || el.dataset.ref!
  })
})

function onSelectionChange() {
  const sel = document.getSelection()
  if (sel && sel.rangeCount && root.value?.contains(sel.anchorNode)) savedRange = sel.getRangeAt(0).cloneRange()
}

onMounted(() => {
  render(props.modelValue)
  document.addEventListener('selectionchange', onSelectionChange)
})
onBeforeUnmount(() => document.removeEventListener('selectionchange', onSelectionChange))

/** Restore the last caret position, or place it at the end if there is none. */
function restoreSelection(): Range {
  const el = root.value!
  el.focus()
  const sel = document.getSelection()!
  let range = savedRange && el.contains(savedRange.startContainer) ? savedRange : null
  if (!range) {
    range = document.createRange()
    range.selectNodeContents(el)
    range.collapse(false)
  }
  sel.removeAllRanges()
  sel.addRange(range)
  return range
}

function insertRef(path: string) {
  if (props.readonly || !root.value) return
  const range = restoreSelection()
  range.deleteContents()
  const node = chip(path)
  range.insertNode(node)
  let after: Node = node
  if (!node.nextSibling) {
    after = document.createTextNode(ZWSP)
    node.after(after)
  }
  const caret = document.createRange()
  if (after === node) caret.setStartAfter(node)
  else caret.setStart(after, 1)
  caret.collapse(true)
  const sel = document.getSelection()!
  sel.removeAllRanges()
  sel.addRange(caret)
  savedRange = caret.cloneRange()
  onInput()
}

/** Insert text at the caret; back is the number of characters to move the caret back after inserting. */
function insertText(text: string, back = 0) {
  if (props.readonly || !root.value) return
  restoreSelection()
  document.execCommand('insertText', false, text)
  const sel = document.getSelection()
  for (let i = 0; i < back; i++) sel?.modify('move', 'backward', 'character')
  onInput()
}

function focus() {
  restoreSelection()
}

function onKeydown(e: KeyboardEvent) {
  if (e.key !== 'Enter' || e.isComposing) return
  e.preventDefault()
  if (props.multiline) document.execCommand('insertLineBreak')
}

function onPaste(e: ClipboardEvent) {
  e.preventDefault()
  let text = e.clipboardData?.getData('text/plain') ?? ''
  if (!props.multiline) text = text.replace(/\r?\n/g, ' ')
  document.execCommand('insertText', false, text)
  // Render pasted variable references as chips.
  const value = serialize()
  if (splitRefs(value, props.refStyle).some((t) => 'ref' in t)) {
    render(value)
    restoreCaretToEnd()
  }
  lastValue = value
  emit('update:modelValue', value)
}

function restoreCaretToEnd() {
  savedRange = null
  restoreSelection()
}

defineExpose({ insertRef, insertText, focus })
</script>

<template>
  <div
    ref="root"
    class="token-input"
    :class="{ multiline, mono, empty: !modelValue }"
    :contenteditable="readonly ? 'false' : 'true'"
    :data-placeholder="placeholder"
    spellcheck="false"
    role="textbox"
    :aria-multiline="multiline"
    @input="onInput"
    @keydown="onKeydown"
    @paste="onPaste"
    @drop.prevent
    @focus="emit('focus')"
    @blur="emit('blur')"
  />
</template>

<style scoped>
.token-input {
  flex: 1;
  min-width: 0;
  min-height: 30px;
  padding: 4px;
  outline: none;
  font-size: 13px;
  line-height: 22px;
  color: var(--text-title);
  white-space: pre;
  overflow: hidden;
  cursor: text;
}
.token-input.multiline {
  min-height: 72px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.token-input.mono {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12.5px;
}
.token-input[contenteditable='false'] {
  cursor: inherit;
}
.token-input.empty::before {
  content: attr(data-placeholder);
  color: var(--text-placeholder);
  pointer-events: none;
}
.token-input :deep(.token-chip) {
  display: inline-block;
  max-width: 220px;
  margin: 0 2px;
  padding: 0 6px;
  border-radius: 4px;
  /* Matches the arcoblue color scheme of a-tag. */
  background: rgb(var(--arcoblue-1));
  color: rgb(var(--arcoblue-6));
  font-family: var(--font-family, inherit);
  font-size: 12.5px;
  line-height: 20px;
  vertical-align: top;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  user-select: all;
  cursor: default;
}
</style>
