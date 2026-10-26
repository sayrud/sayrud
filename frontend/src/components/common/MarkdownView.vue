<script setup lang="ts">
import { computed } from 'vue'

import { renderMarkdown } from '@/utils/markdown'

/** Shows a Markdown document written by a third party, such as the readme of a Sayrud Hub package. */
const props = defineProps<{ source: string }>()

const html = computed(() => renderMarkdown(props.source))
</script>

<template>
  <!-- renderMarkdown sanitizes the HTML. -->
  <div class="markdown" v-html="html" />
</template>

<style scoped>
.markdown {
  color: var(--text-body);
  font-size: 14px;
  line-height: 1.75;
  overflow-wrap: anywhere;
}
.markdown :deep(> :first-child) {
  margin-top: 0;
}
.markdown :deep(> :last-child) {
  margin-bottom: 0;
}
.markdown :deep(h1),
.markdown :deep(h2),
.markdown :deep(h3),
.markdown :deep(h4) {
  margin: 24px 0 12px;
  color: var(--text-title);
  font-weight: 600;
  line-height: 1.4;
}
.markdown :deep(h1) {
  font-size: 20px;
}
.markdown :deep(h2) {
  padding-bottom: 8px;
  border-bottom: 1px solid var(--line-border);
  font-size: 17px;
}
.markdown :deep(h3) {
  font-size: 15px;
}
.markdown :deep(h4) {
  font-size: 14px;
}
.markdown :deep(p),
.markdown :deep(ul),
.markdown :deep(ol),
.markdown :deep(blockquote),
.markdown :deep(pre),
.markdown :deep(table) {
  margin: 0 0 14px;
}
.markdown :deep(ul),
.markdown :deep(ol) {
  padding-left: 22px;
}
.markdown :deep(a) {
  color: var(--color-primary);
  text-decoration: none;
}
.markdown :deep(a:hover) {
  text-decoration: underline;
}
.markdown :deep(code) {
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--fill-hover);
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 0.88em;
}
.markdown :deep(pre) {
  overflow-x: auto;
  padding: 12px 16px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  background: var(--bg-base);
  line-height: 1.6;
}
.markdown :deep(pre code) {
  padding: 0;
  background: transparent;
  font-size: 12.5px;
}
.markdown :deep(blockquote) {
  padding: 4px 16px;
  border-left: 3px solid var(--line-border-strong);
  color: var(--text-caption);
}
.markdown :deep(table) {
  display: block;
  overflow-x: auto;
  border-collapse: collapse;
}
.markdown :deep(th),
.markdown :deep(td) {
  padding: 6px 12px;
  border: 1px solid var(--line-border);
}
.markdown :deep(th) {
  background: var(--bg-base);
  font-weight: 600;
}
.markdown :deep(img) {
  max-width: 100%;
  border-radius: 6px;
}
.markdown :deep(hr) {
  margin: 20px 0;
  border: none;
  border-top: 1px solid var(--line-border);
}
</style>
