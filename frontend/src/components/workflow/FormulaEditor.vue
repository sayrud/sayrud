<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import TokenInput from '@/components/workflow/TokenInput.vue'
import VariablePicker from '@/components/workflow/VariablePicker.vue'
import { compileFormula, formulaCategoryLabel, FUNCTIONS, type FormulaCategory } from '@/utils/formula'
import type { RefEntry } from '@/utils/workflow'

const props = defineProps<{ visible: boolean; formula: string; entries: RefEntry[]; labelOf: (path: string) => string }>()
const emit = defineEmits<{ 'update:visible': [boolean]; confirm: [formula: string] }>()
const { t } = useI18n()

const text = ref('')
const editor = ref<InstanceType<typeof TokenInput>>()
const hover = ref<{ title: string; usage: string; desc: string } | null>(null)

watch(
  () => props.visible,
  (v) => {
    if (!v) return
    text.value = props.formula
    hover.value = null
    void nextTick(() => editor.value?.focus())
  },
  { immediate: true },
)

const groups = computed(() => props.entries.map((e) => ({ key: e.key, label: e.label, nodes: e.nodes })))

const functionTree = computed(() => {
  const map = new Map<FormulaCategory, string[]>()
  for (const [name, f] of Object.entries(FUNCTIONS)) {
    if (!map.has(f.category)) map.set(f.category, [])
    map.get(f.category)!.push(name)
  }
  return [...map.entries()].map(([cat, names]) => ({
    key: `cat:${cat}`,
    title: formulaCategoryLabel(cat),
    selectable: false,
    category: true,
    children: names.map((n) => ({ key: n, title: n })),
  }))
})
const functionKeys = computed(() => functionTree.value.map((c) => c.key))

function onFunctionSelect(_keys: (string | number)[], e: { node?: { key?: string | number } }) {
  const name = String(e.node?.key ?? '')
  if (FUNCTIONS[name]) editor.value?.insertText(`${name}()`, 1)
}

const error = computed(() => (text.value.trim() ? compileFormula(text.value).error ?? '' : ''))

function confirm() {
  if (error.value) return
  emit('confirm', text.value.trim())
  emit('update:visible', false)
}
</script>

<template>
  <a-modal
    :visible="visible"
    :title="t('workflow.formula.title')"
    :width="820"
    :ok-text="t('common.confirm')"
    :ok-button-props="{ disabled: !!error }"
    unmount-on-close
    @ok="confirm"
    @cancel="emit('update:visible', false)"
  >
    <div class="formula-editor">
      <div class="formula-box">
        <span class="formula-eq">=</span>
        <TokenInput ref="editor" v-model="text" ref-style="formula" :label-of="labelOf" multiline mono :placeholder="t('workflow.formula.placeholder')" />
      </div>
      <div class="status">
        <span v-if="error" class="error">{{ error }}</span>
        <span v-else-if="text.trim()" class="ok">{{ t('workflow.formula.valid') }}</span>
        <span v-else class="tip">{{ t('workflow.formula.tip') }}</span>
      </div>
      <div class="picker">
        <div class="picker-col vars">
          <VariablePicker :groups="groups" class="var-list" @select="(p) => editor?.insertRef(p)" />
        </div>
        <div class="picker-col fns">
          <a-tree :data="functionTree" :default-expanded-keys="functionKeys" :selected-keys="[]" block-node size="small" @select="onFunctionSelect">
            <template #title="node">
              <span v-if="node.category" class="group-title">{{ node.title }}</span>
              <span v-else class="fn-item" @mouseenter="hover = { title: node.title, usage: FUNCTIONS[node.title]!.usage, desc: FUNCTIONS[node.title]!.desc }">{{ node.title }}</span>
            </template>
          </a-tree>
        </div>
        <div class="picker-doc">
          <template v-if="hover">
            <div class="doc-title">{{ hover.title }}</div>
            <div class="doc-desc">{{ hover.desc }}</div>
            <div class="doc-usage">{{ hover.usage }}</div>
          </template>
          <template v-else>
            <div class="doc-title">{{ t('workflow.formula.docTitle') }}</div>
            <div class="doc-desc">{{ t('workflow.formula.doc') }}</div>
          </template>
        </div>
      </div>
    </div>
  </a-modal>
</template>

<style scoped>
.formula-box {
  display: flex;
  align-items: flex-start;
  gap: 4px;
  padding: 6px 8px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
}
.formula-box:focus-within {
  border-color: var(--color-primary);
}
.formula-eq {
  padding: 4px 2px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
  line-height: 22px;
  color: var(--text-placeholder);
}
.status {
  min-height: 22px;
  margin: 6px 0 12px;
  font-size: 13px;
}
.error {
  color: var(--color-danger);
}
.ok {
  color: var(--color-success);
}
.tip {
  color: var(--text-placeholder);
}
.picker {
  display: flex;
  height: 320px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  overflow: hidden;
}
.picker-col {
  overflow: auto;
  border-right: 1px solid var(--line-border);
}
.vars {
  width: 320px;
  flex: none;
}
.vars {
  display: flex;
  flex-direction: column;
}
.var-list {
  flex: 1;
  min-height: 0;
}
.fns {
  width: 170px;
  flex: none;
  padding: 4px;
}
.group-title {
  font-size: 12px;
  color: var(--text-placeholder);
}
.fn-item {
  display: block;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
}
.picker-doc {
  flex: 1;
  min-width: 0;
  padding: 16px;
  background: var(--bg-sidebar);
}
.doc-title {
  font-weight: 600;
  font-size: 15px;
}
.doc-desc {
  margin-top: 8px;
  color: var(--text-caption);
  line-height: 1.6;
}
.doc-usage {
  margin-top: 12px;
  padding: 8px 10px;
  border: 1px solid var(--line-border);
  border-radius: 6px;
  background: var(--bg-body);
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
}
</style>
