<script setup lang="ts">
import { GripVertical, Plus, Trash } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { useBaseStore } from '@/stores/base'
import type { SLView, SortOrder } from '@/types/bitable'
import { isQueryable } from '@/utils/fieldTypes'
import { sortOrderLabels } from '@/utils/filters'
import FieldSelect from './FieldSelect.vue'

const { t } = useI18n()

/** Shared by sort and group, both are ordered lists of field and direction. */
const props = defineProps<{ view: SLView; kind: 'sort' | 'group' }>()
const store = useBaseStore()

const MAX = computed(() => (props.kind === 'group' ? 3 : 10))
const items = computed(() =>
  props.kind === 'sort'
    ? props.view.config.sort
    : props.view.config.group.map((g) => ({ fieldUID: g.fieldUID, order: g.order ?? ('asc' as SortOrder) })),
)
const fields = computed(() => store.fields.filter(isQueryable))
const used = computed(() => items.value.map((i) => i.fieldUID))

function save(list: { fieldUID: string; order: SortOrder }[]) {
  if (props.kind === 'sort') store.updateViewConfig({ sort: list })
  else store.updateViewConfig({ group: list })
}

function add() {
  const f = fields.value.find((x) => !used.value.includes(x.uid))
  if (f) save([...items.value, { fieldUID: f.uid, order: 'asc' }])
}

const dragIndex = ref(-1)
const overIndex = ref(-1)
function onDrop() {
  const from = dragIndex.value
  const to = overIndex.value
  if (from >= 0 && to >= 0 && from !== to) {
    const list = [...items.value]
    const [x] = list.splice(from, 1)
    list.splice(to, 0, x!)
    save(list)
  }
  dragIndex.value = overIndex.value = -1
}
</script>

<template>
  <div class="sg-panel">
    <div class="panel-head">
      <span class="panel-title">{{ kind === 'sort' ? t('toolbar.sortTitle') : t('toolbar.groupTitle') }}</span>
      <span v-if="kind === 'group'" class="text-caption tip">{{ t('toolbar.groupMax') }}</span>
    </div>
    <div v-if="!items.length" class="empty">{{ kind === 'sort' ? t('toolbar.sortEmpty') : t('toolbar.groupEmpty') }}</div>
    <div
      v-for="(item, i) in items"
      :key="item.fieldUID"
      class="row"
      :class="{ over: overIndex === i && dragIndex !== i }"
      @dragover.prevent="overIndex = i"
      @drop.prevent="onDrop"
    >
      <span class="grip" draggable="true" @dragstart="dragIndex = i" @dragend="onDrop"><GripVertical :size="14" /></span>
      <div class="col-field">
        <FieldSelect
          :fields="fields"
          :model-value="item.fieldUID"
          :disabled-u-i-ds="used"
          size="small"
          @update:model-value="(v) => save(items.map((x, j) => (j === i ? { ...x, fieldUID: v } : x)))"
        />
      </div>
      <a-radio-group
        type="button"
        size="small"
        :model-value="item.order"
        @change="(v) => save(items.map((x, j) => (j === i ? { ...x, order: v as SortOrder } : x)))"
      >
        <a-radio value="asc">{{ sortOrderLabels(store.fields.find((f) => f.uid === item.fieldUID)).asc }}</a-radio>
        <a-radio value="desc">{{ sortOrderLabels(store.fields.find((f) => f.uid === item.fieldUID)).desc }}</a-radio>
      </a-radio-group>
      <button class="icon-btn" @click="save(items.filter((_x, j) => j !== i))"><Trash :size="14" /></button>
    </div>
    <button class="add" :disabled="items.length >= MAX || used.length >= fields.length" @click="add">
      <Plus :size="14" /> {{ kind === 'sort' ? t('toolbar.addSort') : t('toolbar.addGroup') }}
    </button>
    <div v-if="fields.length < store.fields.length" class="note">{{ kind === 'sort' ? t('toolbar.formulaNoSort') : t('toolbar.formulaNoGroup') }}</div>
  </div>
</template>

<style scoped>
.sg-panel {
  width: 460px;
  padding: 14px 16px;
}
.panel-head {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 10px;
}
.tip {
  font-size: 12px;
}
.empty {
  padding: 8px 0 12px;
  color: var(--text-placeholder);
}
.row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  border-top: 2px solid transparent;
}
.row.over {
  border-top-color: var(--color-primary);
}
.grip {
  display: flex;
  color: var(--text-placeholder);
  cursor: grab;
}
.col-field {
  flex: 1;
  min-width: 0;
}
.add {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 6px;
  border: none;
  background: transparent;
  color: var(--color-primary);
  border-radius: 6px;
  cursor: pointer;
}
.add:hover {
  background: var(--color-primary-lighter);
}
.add:disabled {
  color: var(--text-disabled);
  cursor: not-allowed;
  background: transparent;
}
.note {
  margin-top: 8px;
  font-size: 12px;
  color: var(--text-placeholder);
}
</style>
