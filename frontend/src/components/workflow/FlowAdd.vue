<script setup lang="ts">
import { Plus } from '@lucide/vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import StepPicker from '@/components/workflow/StepPicker.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { StepType } from '@/types/workflow'
import { type ListAddress, moduleDef, newStep, resolveList, stepTable, varValue } from '@/utils/workflow'

const props = withDefaults(defineProps<{ address: ListAddress; index: number; types: StepType[]; tall?: boolean; hideLine?: boolean }>(), { tall: false, hideLine: false })
const { t } = useI18n()
const store = useWorkflowEditorStore()
const open = ref(false)

function add(type: StepType) {
  open.value = false
  const list = store.spec ? resolveList(store.spec, props.address) : null
  if (!list) return
  // Data steps default to the table the trigger listens on.
  const tableUID = store.spec!.trigger.record?.tableUID || (store.tables.length === 1 ? store.tables[0]!.uid : undefined)
  const step = newStep(store.spec!, type, tableUID)
  const trigger = store.spec!.trigger
  // Steps on the trigger table that need a record default to the triggering record.
  const targetsTrigger = trigger.record && trigger.type !== 'record_deleted' && stepTable(step) === trigger.record.tableUID
  if (targetsTrigger && 'recordUID' in (moduleDef(type)?.defaults ?? {})) step.config.recordUID = varValue('trigger.record.uid')
  list.splice(props.index, 0, step)
  void store.ensureFields(stepTable(step))
  store.select({ kind: 'step', id: step.id })
}
</script>

<template>
  <div class="flow-add" :class="{ tall, 'no-line': hideLine }">
    <a-popover
      v-if="!store.readonly"
      v-model:popup-visible="open"
      trigger="click"
      position="bottom"
      :content-style="{ padding: 0 }"
      :trigger-props="{ showArrow: false, popupOffset: 6, unmountOnClose: true }"
    >
      <a-tooltip :content="t('workflow.node.add')" mini position="right" :popup-visible="open ? false : undefined">
        <a-button shape="circle" size="mini" class="add-btn" :class="{ open }" :aria-label="t('workflow.node.add')">
          <template #icon><Plus :size="13" /></template>
        </a-button>
      </a-tooltip>
      <template #content>
        <StepPicker :types="types" @select="add" @close="open = false" />
      </template>
    </a-popover>
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
.add-btn:hover,
.add-btn.open {
  transform: scale(1.12);
  color: #fff;
  background: var(--color-primary);
  border-color: var(--color-primary);
  opacity: 1;
}
</style>
