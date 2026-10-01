<script setup lang="ts">
import { Check, Plus } from '@lucide/vue'
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { useBaseStore } from '@/stores/base'
import type { CellValue, SelectOption, SLField } from '@/types/bitable'
import { optionsOf } from '@/utils/format'
import SelectTag from './SelectTag.vue'

const { t } = useI18n()

const props = defineProps<{ field: SLField; value: CellValue; initialQuery?: string }>()
const emit = defineEmits<{ change: [value: CellValue]; close: [] }>()

const store = useBaseStore()
const multiple = computed(() => props.field.type === 'multi_select')
const query = ref(props.initialQuery ?? '')
const active = ref(0)
const input = ref<HTMLInputElement>()
const listEl = ref<HTMLElement>()

const selected = computed<string[]>(() => {
  const v = props.value
  if (Array.isArray(v)) return v
  return typeof v === 'string' && v ? [v] : []
})

const options = computed(() => optionsOf(store.fields.find((f) => f.uid === props.field.uid) ?? props.field))
const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  return q ? options.value.filter((o) => o.name.toLowerCase().includes(q)) : options.value
})
const canCreate = computed(() => {
  const q = query.value.trim()
  return q !== '' && !options.value.some((o) => o.name === q)
})
const itemCount = computed(() => filtered.value.length + (canCreate.value ? 1 : 0))
const selectedOptions = computed(
  () => selected.value.map((uid) => options.value.find((o) => o.uid === uid)).filter(Boolean) as SelectOption[],
)

watch(query, () => (active.value = 0))

function toggle(uid: string) {
  if (multiple.value) {
    const next = selected.value.includes(uid) ? selected.value.filter((x) => x !== uid) : [...selected.value, uid]
    emit('change', next)
    query.value = ''
  } else {
    emit('change', selected.value[0] === uid ? null : uid)
    emit('close')
  }
}

async function create() {
  const name = query.value.trim()
  if (!name) return
  const map = await store.ensureOptions(props.field.uid, [name])
  const uid = map.get(name)
  if (uid) toggle(uid)
}

function onKey(e: KeyboardEvent) {
  if (e.isComposing) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    active.value = (active.value + 1) % Math.max(1, itemCount.value)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    active.value = (active.value - 1 + itemCount.value) % Math.max(1, itemCount.value)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const opt = filtered.value[active.value]
    if (opt) toggle(opt.uid)
    else if (canCreate.value) create()
  } else if (e.key === 'Backspace' && !query.value && multiple.value && selected.value.length) {
    emit('change', selected.value.slice(0, -1))
  } else if (e.key === 'Escape') {
    e.preventDefault()
    e.stopPropagation()
    emit('close')
  }
  nextTick(() => listEl.value?.querySelector('.active')?.scrollIntoView({ block: 'nearest' }))
}

onMounted(() => input.value?.focus())
</script>

<template>
  <div class="select-panel" @keydown.stop>
    <div class="search-box">
      <SelectTag
        v-for="o in multiple ? selectedOptions : []"
        :key="o.uid"
        :name="o.name"
        :color="o.color"
        closable
        @close="toggle(o.uid)"
      />
      <input ref="input" v-model="query" :placeholder="t('select.searchOrCreate')" @keydown="onKey" />
    </div>
    <div ref="listEl" class="list">
      <div
        v-for="(o, i) in filtered"
        :key="o.uid"
        class="item"
        :class="{ active: i === active }"
        @mouseenter="active = i"
        @click="toggle(o.uid)"
      >
        <SelectTag :name="o.name" :color="o.color" />
        <Check v-if="selected.includes(o.uid)" :size="16" class="check" />
      </div>
      <div
        v-if="canCreate"
        class="item create"
        :class="{ active: active === filtered.length }"
        @mouseenter="active = filtered.length"
        @click="create"
      >
        <Plus :size="14" />
        <span>{{ t('select.create') }}</span>
        <SelectTag :name="query.trim()" :color="options.length" />
      </div>
      <div v-if="!itemCount" class="empty">{{ t('select.empty') }}</div>
    </div>
  </div>
</template>

<style scoped>
.select-panel {
  width: 100%;
  min-width: 240px;
}
.search-box {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--line-divider);
}
.search-box input {
  flex: 1;
  min-width: 80px;
  height: 24px;
  border: none;
  outline: none;
  font-size: 14px;
  background: transparent;
}
.list {
  max-height: 280px;
  overflow: auto;
  padding: 4px;
}
.item {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 8px;
  border-radius: 6px;
  cursor: pointer;
}
.item.active {
  background: var(--fill-hover);
}
.check {
  margin-left: auto;
  color: var(--color-primary);
  flex: none;
}
.create {
  color: var(--text-caption);
}
.empty {
  padding: 16px 8px;
  text-align: center;
  color: var(--text-placeholder);
  font-size: 13px;
}
</style>
