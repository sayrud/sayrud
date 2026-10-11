<script setup lang="ts">
import { Plus } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { StepType } from '@/types/workflow'
import { type ListAddress, newStep, resolveList, STEP_META, varValue } from '@/utils/workflow'

const props = withDefaults(defineProps<{ address: ListAddress; index: number; types: StepType[]; tall?: boolean; hideLine?: boolean }>(), { tall: false, hideLine: false })
const { t } = useI18n()
const store = useWorkflowEditorStore()

function add(type: StepType) {
  const list = store.spec ? resolveList(store.spec, props.address) : null
  if (!list) return
  // Data steps default to the table the trigger listens on.
  const tableUID = store.spec!.trigger.record?.tableUID || (store.tables.length === 1 ? store.tables[0]!.uid : undefined)
  const step = newStep(store.spec!, type, tableUID)
  const trigger = store.spec!.trigger
  if (type === 'update' && trigger.record && trigger.type !== 'record_deleted' && step.tableUID === trigger.record.tableUID) step.recordUID = varValue('trigger.record.uid')
  list.splice(props.index, 0, step)
  void store.ensureFields(step.tableUID)
  store.select({ kind: 'step', id: step.id })
}
</script>

<template>
  <div class="flow-add" :class="{ tall, 'no-line': hideLine }">
    <a-dropdown v-if="!store.readonly" trigger="click" position="bl" class="add-dropdown">
      <a-tooltip :content="t('workflow.node.add')" mini position="right">
        <a-button shape="circle" size="mini" class="add-btn" :aria-label="t('workflow.node.add')">
          <template #icon><Plus :size="13" /></template>
        </a-button>
      </a-tooltip>
      <template #content>
        <a-doption v-for="type in types" :key="type" class="step-option" @click="add(type)">
          <div class="option-row">
            <span class="option-icon" :style="{ background: STEP_META[type].color }">
              <component :is="STEP_META[type].icon" :size="15" />
            </span>
            <span class="option-text">
              <span class="option-label">{{ STEP_META[type].label }}</span>
              <span class="option-desc">{{ STEP_META[type].description }}</span>
            </span>
          </div>
        </a-doption>
      </template>
    </a-dropdown>
  </div>
</template>

<style scoped>
.flow-add {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 40px;
  flex: none;
}
.flow-add.tall {
  height: 48px;
}
.flow-add::before {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  left: 50%;
  width: 2px;
  margin-left: -1px;
  background: var(--line-border-strong);
}
.flow-add.no-line::before {
  display: none;
}
.add-btn {
  position: relative;
  z-index: 1;
  width: 22px;
  height: 22px;
  border: 1px solid var(--line-border-strong);
  background: var(--bg-body);
  color: var(--text-caption);
  opacity: 0.85;
  transition:
    transform 0.15s,
    color 0.15s,
    border-color 0.15s,
    opacity 0.15s;
}
.add-btn:hover {
  transform: scale(1.12);
  color: #fff;
  background: var(--color-primary);
  border-color: var(--color-primary);
  opacity: 1;
}
.step-option :deep(.arco-dropdown-option-content) {
  width: 100%;
}
.option-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 0;
  min-width: 220px;
}
.option-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  color: #fff;
  flex: none;
}
.option-text {
  display: flex;
  flex-direction: column;
  line-height: 18px;
}
.option-label {
  color: var(--text-title);
}
.option-desc {
  font-size: 12px;
  color: var(--text-placeholder);
}
</style>
