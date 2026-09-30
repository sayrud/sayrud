<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'

import { useBaseStore } from '@/stores/base'
import type { SLField } from '@/types/bitable'
import { TableContext } from '@/utils/engine'
import { compileFormula, expFromDisplay, expToDisplay, FUNCTIONS } from '@/utils/formula'
import FieldTypeIcon from './FieldTypeIcon.vue'

const props = defineProps<{ visible: boolean; exp: string; fieldUID?: string }>()
const emit = defineEmits<{ 'update:visible': [boolean]; confirm: [exp: string] }>()

const store = useBaseStore()
const text = ref('')
const textarea = ref<HTMLTextAreaElement>()
const hover = ref<{ title: string; usage: string; desc: string } | null>(null)

const refFields = computed(() => store.fields.filter((f) => f.uid !== props.fieldUID))

watch(
  () => props.visible,
  (v) => {
    if (!v) return
    text.value = expToDisplay(props.exp, store.fields)
    nextTick(() => textarea.value?.focus())
  },
  { immediate: true },
)

const parsed = computed(() => expFromDisplay(text.value, store.fields))

const validation = computed(() => {
  if (!text.value.trim()) return { ok: false, message: '' }
  if (parsed.value.unknown.length) return { ok: false, message: `未知字段：${parsed.value.unknown.join('、')}` }
  const compiled = compileFormula(parsed.value.exp)
  if (compiled.error) return { ok: false, message: compiled.error }
  if (props.fieldUID && compiled.refs.includes(props.fieldUID)) return { ok: false, message: '公式不能引用自身' }
  return { ok: true, message: '' }
})

const preview = computed(() => {
  if (!validation.value.ok) return ''
  const record = store.records[0]
  if (!record) return ''
  const uid = props.fieldUID ?? '__preview__'
  const temp: SLField = {
    uid,
    tableUID: store.activeTableUID,
    label: '预览',
    type: 'formula',
    metadata: { exp: parsed.value.exp },
    position: 0,
    createdAt: '',
    updatedAt: '',
  }
  const ctx = new TableContext([...store.fields.filter((f) => f.uid !== uid), temp])
  return ctx.text(record, temp) || '（空）'
})

const groups = computed(() => {
  const map = new Map<string, string[]>()
  for (const [name, f] of Object.entries(FUNCTIONS)) {
    if (!map.has(f.category)) map.set(f.category, [])
    map.get(f.category)!.push(name)
  }
  return [...map.entries()]
})

function insert(snippet: string, cursorBack = 0) {
  const el = textarea.value
  if (!el) return
  const start = el.selectionStart
  const end = el.selectionEnd
  text.value = text.value.slice(0, start) + snippet + text.value.slice(end)
  nextTick(() => {
    el.focus()
    const pos = start + snippet.length - cursorBack
    el.setSelectionRange(pos, pos)
  })
}

function confirm() {
  if (text.value.trim() && !validation.value.ok) return
  emit('confirm', parsed.value.exp)
  emit('update:visible', false)
}
</script>

<template>
  <a-modal
    :visible="visible"
    title="编辑公式"
    :width="780"
    :mask-closable="false"
    ok-text="确定"
    :ok-button-props="{ disabled: !!text.trim() && !validation.ok }"
    unmount-on-close
    @ok="confirm"
    @cancel="emit('update:visible', false)"
  >
    <div class="formula-modal">
      <textarea
        ref="textarea"
        v-model="text"
        class="formula-input"
        spellcheck="false"
        placeholder="例如：[单价] * [数量]、IF([进度] >= 1, &quot;已完成&quot;, &quot;进行中&quot;)"
      />
      <div class="status">
        <span v-if="validation.message" class="error">{{ validation.message }}</span>
        <span v-else-if="validation.ok" class="ok">
          公式正确<template v-if="preview">，首条记录计算结果：<b>{{ preview }}</b></template>
        </span>
        <span v-else class="tip">使用 [字段名] 引用字段，点击下方列表快速插入</span>
      </div>
      <div class="picker">
        <div class="picker-list">
          <div class="group-title">字段</div>
          <div
            v-for="f in refFields"
            :key="f.uid"
            class="picker-item"
            @mouseenter="hover = { title: f.label, usage: `[${f.label}]`, desc: '引用该字段的值' }"
            @click="insert(`[${f.label}]`)"
          >
            <FieldTypeIcon :type="f.type" />
            <span class="ellipsis">{{ f.label }}</span>
          </div>
          <template v-for="[cat, names] in groups" :key="cat">
            <div class="group-title">{{ cat }}函数</div>
            <div
              v-for="n in names"
              :key="n"
              class="picker-item mono"
              @mouseenter="hover = { title: n, usage: FUNCTIONS[n]!.usage, desc: FUNCTIONS[n]!.desc }"
              @click="insert(`${n}()`, 1)"
            >
              {{ n }}
            </div>
          </template>
        </div>
        <div class="picker-doc">
          <template v-if="hover">
            <div class="doc-title">{{ hover.title }}</div>
            <div class="doc-desc">{{ hover.desc }}</div>
            <div class="doc-usage">{{ hover.usage }}</div>
          </template>
          <template v-else>
            <div class="doc-title">公式说明</div>
            <div class="doc-desc">
              支持运算符 + - * / % ^，文本连接 &amp;，比较 = != &lt; &gt; &lt;= &gt;=。日期相减得到天数，日期加数字得到新日期。
            </div>
          </template>
        </div>
      </div>
    </div>
  </a-modal>
</template>

<style scoped>
.formula-input {
  width: 100%;
  height: 96px;
  padding: 10px 12px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  outline: none;
  resize: none;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 14px;
  line-height: 1.6;
}
.formula-input:focus {
  border-color: var(--color-primary);
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
.ok b {
  color: var(--text-title);
}
.tip {
  color: var(--text-placeholder);
}
.picker {
  display: flex;
  height: 300px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  overflow: hidden;
}
.picker-list {
  width: 260px;
  overflow: auto;
  padding: 4px;
  border-right: 1px solid var(--line-border);
}
.group-title {
  padding: 8px 8px 4px;
  font-size: 12px;
  color: var(--text-placeholder);
}
.picker-item {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 8px;
  border-radius: 6px;
  cursor: pointer;
}
.picker-item:hover {
  background: var(--fill-hover);
}
.mono {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
}
.picker-doc {
  flex: 1;
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
  border-radius: 6px;
  background: var(--bg-body);
  border: 1px solid var(--line-border);
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
}
</style>
