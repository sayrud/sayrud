<script setup lang="ts">
import { indentWithTab } from '@codemirror/commands'
import { javascript } from '@codemirror/lang-javascript'
import { json, jsonParseLinter } from '@codemirror/lang-json'
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { linter } from '@codemirror/lint'
import { Compartment, EditorState, Prec } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'
import { tags } from '@lezer/highlight'
import { basicSetup } from 'codemirror'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { useThemeStore } from '@/stores/theme'

const props = withDefaults(
  defineProps<{
    language?: 'javascript' | 'json'
    /** Fixed height overrides minHeight and maxHeight. */
    height?: string
    minHeight?: string
    maxHeight?: string
    readonly?: boolean
    /** Display the border in the error state. */
    error?: boolean
    ariaLabel?: string
  }>(),
  {
    language: 'javascript',
    height: undefined,
    minHeight: '120px',
    maxHeight: '480px',
    readonly: false,
    error: false,
    ariaLabel: undefined,
  },
)
const model = defineModel<string>({ required: true })
// Mod-Enter submits from the editor instead of inserting a blank line.
const emit = defineEmits<{ submit: [] }>()

const themeStore = useThemeStore()
const root = ref<HTMLDivElement>()
const focused = ref(false)
let view: EditorView | undefined

const themeSlot = new Compartment()

// CSS variables supply colors, so switching them updates the light and dark themes.
const highlightStyle = HighlightStyle.define([
  { tag: [tags.keyword, tags.controlKeyword, tags.moduleKeyword, tags.operatorKeyword, tags.definitionKeyword, tags.modifier, tags.self], color: 'var(--code-keyword)' },
  { tag: [tags.string, tags.special(tags.string), tags.regexp, tags.escape], color: 'var(--code-string)' },
  { tag: [tags.number, tags.bool, tags.null, tags.atom, tags.unit], color: 'var(--code-number)' },
  { tag: [tags.comment, tags.lineComment, tags.blockComment, tags.docComment], color: 'var(--code-comment)', fontStyle: 'italic' },
  { tag: [tags.function(tags.variableName), tags.function(tags.propertyName), tags.function(tags.definition(tags.variableName))], color: 'var(--code-function)' },
  { tag: [tags.propertyName, tags.attributeName], color: 'var(--code-property)' },
  { tag: [tags.typeName, tags.className, tags.namespace], color: 'var(--code-type)' },
  { tag: [tags.definition(tags.variableName), tags.variableName, tags.labelName], color: 'var(--code-variable)' },
  { tag: [tags.operator, tags.punctuation, tags.separator, tags.bracket, tags.squareBracket, tags.brace, tags.paren], color: 'var(--code-punctuation)' },
  { tag: tags.invalid, color: 'var(--color-danger)' },
])

function editorTheme(dark: boolean) {
  return EditorView.theme(
    {
      '&': { height: '100%', color: 'var(--text-title)', backgroundColor: 'var(--bg-body)', fontSize: '12.5px' },
      '&.cm-focused': { outline: 'none' },
      '.cm-scroller': { fontFamily: "'SF Mono', Menlo, Consolas, 'Liberation Mono', monospace", lineHeight: '1.65' },
      '.cm-content': { padding: '8px 0', caretColor: 'var(--color-primary)' },
      '.cm-line': { padding: '0 12px 0 8px' },
      '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--color-primary)', borderLeftWidth: '2px' },
      '&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
        backgroundColor: 'var(--code-selection) !important',
      },
      '.cm-gutters': { backgroundColor: 'var(--bg-base)', color: 'var(--text-placeholder)', border: 'none', borderRight: '1px solid var(--line-border)' },
      '.cm-lineNumbers .cm-gutterElement': { minWidth: '32px', padding: '0 8px 0 12px' },
      '.cm-foldGutter .cm-gutterElement': { padding: '0 4px', color: 'var(--text-placeholder)' },
      '.cm-activeLine': { backgroundColor: 'var(--code-active-line)' },
      '.cm-activeLineGutter': { backgroundColor: 'var(--code-active-line)', color: 'var(--text-title)' },
      '&.cm-focused .cm-matchingBracket': { backgroundColor: 'var(--code-bracket)', outline: '1px solid var(--line-border-strong)' },
      '.cm-searchMatch': { backgroundColor: 'var(--grid-search)' },
      '.cm-searchMatch.cm-searchMatch-selected': { backgroundColor: 'var(--grid-search-current)' },
      '.cm-selectionMatch': { backgroundColor: 'var(--code-bracket)' },
      '.cm-foldPlaceholder': { backgroundColor: 'var(--fill-hover)', border: 'none', color: 'var(--text-caption)', padding: '0 6px' },
      '.cm-tooltip': {
        border: '1px solid var(--line-border)',
        borderRadius: '6px',
        backgroundColor: 'var(--bg-popover)',
        color: 'var(--text-title)',
        boxShadow: 'var(--shadow-popover)',
        overflow: 'hidden',
      },
      '.cm-tooltip-autocomplete > ul': { fontFamily: 'inherit', maxHeight: '240px' },
      '.cm-tooltip-autocomplete > ul > li': { padding: '2px 8px !important' },
      '.cm-tooltip-autocomplete > ul > li[aria-selected]': { backgroundColor: 'var(--bg-primary-soft)', color: 'var(--text-title)' },
      '.cm-completionDetail': { color: 'var(--text-caption)' },
      '.cm-diagnostic': { padding: '6px 10px', borderLeftWidth: '3px' },
      '.cm-diagnostic-error': { borderLeftColor: 'var(--color-danger)' },
      '.cm-panels': { backgroundColor: 'var(--bg-base)', color: 'var(--text-title)' },
      '.cm-panels.cm-panels-top': { borderBottom: '1px solid var(--line-border)' },
      '.cm-panels.cm-panels-bottom': { borderTop: '1px solid var(--line-border)' },
      '.cm-panel input, .cm-panel button': { fontSize: '12px' },
      '.cm-textfield': { border: '1px solid var(--line-border)', borderRadius: '4px', backgroundColor: 'var(--bg-body)', color: 'var(--text-title)' },
      '.cm-button': { border: '1px solid var(--line-border)', borderRadius: '4px', backgroundImage: 'none', backgroundColor: 'var(--bg-body)', color: 'var(--text-title)' },
    },
    { dark },
  )
}

onMounted(() => {
  view = new EditorView({
    parent: root.value!,
    state: EditorState.create({
      doc: model.value,
      extensions: [
        basicSetup,
        Prec.high(
          keymap.of([
            {
              key: 'Mod-Enter',
              run: () => {
                emit('submit')
                return true
              },
            },
            indentWithTab,
          ]),
        ),
        syntaxHighlighting(highlightStyle),
        EditorState.tabSize.of(2),
        props.language === 'json' ? [json(), linter(jsonParseLinter(), { delay: 300 })] : javascript(),
        EditorState.readOnly.of(props.readonly),
        EditorView.editable.of(!props.readonly),
        themeSlot.of(editorTheme(themeStore.theme === 'dark')),
        EditorView.contentAttributes.of(props.ariaLabel ? { 'aria-label': props.ariaLabel } : {}),
        EditorView.updateListener.of((u) => {
          if (u.docChanged) model.value = u.state.doc.toString()
          if (u.focusChanged) focused.value = u.view.hasFocus
        }),
      ],
    }),
  })
})

onBeforeUnmount(() => view?.destroy())

watch(model, (value) => {
  if (!view || value === view.state.doc.toString()) return
  view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } })
})
watch(
  () => themeStore.theme,
  (theme) => view?.dispatch({ effects: themeSlot.reconfigure(editorTheme(theme === 'dark')) }),
)

const sizeStyle = computed(() =>
  props.height ? { '--editor-height': props.height } : { '--editor-min-height': props.minHeight, '--editor-max-height': props.maxHeight },
)
</script>

<template>
  <div
    ref="root"
    class="code-editor"
    :class="{ fixed: !!height, focused, error, readonly }"
    :style="sizeStyle"
  />
</template>

<style scoped>
.code-editor {
  overflow: hidden;
  border: 1px solid var(--line-border);
  border-radius: 6px;
  background: var(--bg-body);
  transition: border-color 0.15s, box-shadow 0.15s;
}
.code-editor:hover:not(.readonly) {
  border-color: var(--line-border-strong);
}
.code-editor.error,
.code-editor.error:hover {
  border-color: var(--color-danger);
}
.code-editor.error.focused {
  border-color: var(--color-danger);
  box-shadow: 0 0 0 2px rgba(var(--danger-6), 0.15);
}
.code-editor.readonly {
  background: var(--bg-base);
}
.code-editor.readonly :deep(.cm-editor) {
  background: transparent;
}
.code-editor.readonly :deep(.cm-gutters),
.code-editor.readonly :deep(.cm-cursor) {
  display: none;
}
.code-editor.readonly :deep(.cm-activeLine) {
  background: transparent;
}
.code-editor.readonly :deep(.cm-line) {
  padding: 0 12px;
}
.code-editor.fixed :deep(.cm-editor) {
  height: var(--editor-height);
}
.code-editor:not(.fixed) :deep(.cm-editor) {
  max-height: var(--editor-max-height);
}
.code-editor:not(.fixed) :deep(.cm-content),
.code-editor:not(.fixed) :deep(.cm-gutter) {
  min-height: calc(var(--editor-min-height) - 2px);
}
</style>

<style>
body {
  --code-keyword: #cf222e;
  --code-string: #0a3069;
  --code-number: #0550ae;
  --code-comment: #6e7781;
  --code-function: #8250df;
  --code-property: #0550ae;
  --code-type: #953800;
  --code-variable: #1f2328;
  --code-punctuation: #57606a;
  --code-selection: rgba(51, 112, 255, 0.18);
  --code-active-line: rgba(31, 35, 41, 0.035);
  --code-bracket: rgba(51, 112, 255, 0.14);
}
body[arco-theme='dark'] {
  --code-keyword: #ff7b72;
  --code-string: #a5d6ff;
  --code-number: #79c0ff;
  --code-comment: #8b949e;
  --code-function: #d2a8ff;
  --code-property: #79c0ff;
  --code-type: #ffa657;
  --code-variable: #e6edf3;
  --code-punctuation: #a6a8ad;
  --code-selection: rgba(76, 136, 255, 0.3);
  --code-active-line: rgba(255, 255, 255, 0.04);
  --code-bracket: rgba(76, 136, 255, 0.22);
}
</style>
