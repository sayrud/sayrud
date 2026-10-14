<script setup lang="ts">
import { ArrowLeftRight, ListChecks, X, Zap } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import ModulePanel from '@/components/workflow/module/ModulePanel.vue'
import AsyncPanel from '@/components/workflow/panels/AsyncPanel.vue'
import ParamsPanel from '@/components/workflow/panels/ParamsPanel.vue'
import TriggerPanel from '@/components/workflow/panels/TriggerPanel.vue'
import TriggerList from '@/components/workflow/TriggerList.vue'
import { useTriggerSwitch } from '@/components/workflow/useTriggerSwitch'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { TriggerType } from '@/types/workflow'
import { findStep, issueMessage, stepMeta, triggerMetaOf } from '@/utils/workflow'

const { t } = useI18n()
const store = useWorkflowEditorStore()
const { change: changeTrigger } = useTriggerSwitch()

const step = computed(() => {
  const s = store.selection
  return s?.kind === 'step' && store.spec ? findStep(store.spec, s.id) : null
})
const flow = computed(() => {
  const s = store.selection
  return s?.kind === 'async' && store.spec ? store.spec.async.find((f) => f.id === s.id) ?? null : null
})
const triggerType = computed(() => store.spec?.trigger.type ?? '')

const head = computed(() => {
  const s = store.selection
  if (s?.kind === 'trigger' && store.spec) {
    const meta = triggerMetaOf(store.spec.trigger.type)
    return { icon: meta.icon, color: meta.color, fallback: meta.label, name: store.spec.trigger.name ?? '' }
  }
  if (s?.kind === 'params') return { icon: ListChecks, color: '#646a73', fallback: t('workflow.node.params'), name: '' }
  if (step.value) return { icon: stepMeta(step.value.type).icon, color: stepMeta(step.value.type).color, fallback: stepMeta(step.value.type).label, name: step.value.name ?? '' }
  if (flow.value) return { icon: Zap, color: flow.value.event === 'succeeded' ? '#14a37f' : '#3370ff', fallback: t('workflow.async.untitled'), name: flow.value.name ?? '' }
  return null
})
/** The request params node is fixed and cannot be renamed. */
const nameEditable = computed(() => store.selection?.kind !== 'params' && !store.readonly)

// The name is edited in the input and written back on blur or Enter; it is cleared when it equals the default name.
const draftName = ref('')
watch(
  () => [store.selection, head.value?.name, head.value?.fallback] as const,
  () => (draftName.value = head.value ? head.value.name.trim() || head.value.fallback : ''),
  { immediate: true },
)
function commitName() {
  const h = head.value
  if (!h) return
  const value = draftName.value.trim()
  const name = value && value !== h.fallback ? value : undefined
  if (store.selection?.kind === 'trigger') store.spec!.trigger.name = name
  else if (step.value) step.value.name = name
  else if (flow.value) flow.value.name = name
  draftName.value = name ?? h.fallback
}

function onNameEnter(e: KeyboardEvent) {
  if (e.isComposing) return
  const input = e.target as HTMLInputElement
  commitName()
  input.blur()
}

const switchOpen = ref(false)
function pickTrigger(to: TriggerType) {
  switchOpen.value = false
  changeTrigger(to)
}

const issues = computed(() => {
  const s = store.selection
  if (!s) return []
  if (s.kind === 'trigger') return ['trigger', 'route', 'auth', 'rate_limit'].flatMap((x) => store.issuesOf(x))
  if (s.kind === 'params') return store.issues.filter((i) => i.target.startsWith('param:') || i.target === 'params')
  if (s.kind === 'step') return store.issuesOf(`step:${s.id}`)
  return store.issuesOf(`async:${s.id}`)
})
</script>

<template>
  <aside v-if="head" class="inspector">
    <header class="inspector-head">
      <span class="head-icon" :style="{ background: head.color }"><component :is="head.icon" :size="16" /></span>
      <a-input
        v-if="nameEditable"
        v-model="draftName"
        class="head-name"
        :max-length="40"
        :placeholder="head.fallback"
        :aria-label="t('workflow.inspector.name')"
        @blur="commitName"
        @press-enter="onNameEnter"
      />
      <span v-else class="head-name static ellipsis">{{ head.name || head.fallback }}</span>
      <a-trigger
        v-if="store.selection?.kind === 'trigger' && triggerType && !store.readonly"
        v-model:popup-visible="switchOpen"
        trigger="click"
        position="br"
        :popup-offset="6"
      >
        <a-tooltip :content="t('workflow.node.changeType')" mini>
          <a-button type="text" class="btn-muted" :class="{ active: switchOpen }" :aria-label="t('workflow.node.changeType')">
            <template #icon><ArrowLeftRight :size="16" /></template>
          </a-button>
        </a-tooltip>
        <template #content>
          <div class="switch-menu"><TriggerList compact :selected="triggerType || undefined" @select="pickTrigger" /></div>
        </template>
      </a-trigger>
      <a-tooltip :content="t('workflow.inspector.close')" mini>
        <a-button type="text" class="btn-muted" :aria-label="t('workflow.inspector.close')" @click="store.select(null)">
          <template #icon><X :size="16" /></template>
        </a-button>
      </a-tooltip>
    </header>

    <a-alert v-if="issues.length" type="error" class="issue-box">
      <div v-for="(issue, i) in issues" :key="i">{{ issueMessage(issue) }}</div>
    </a-alert>

    <div class="inspector-body">
      <a-form layout="vertical" :model="{}" :disabled="store.readonly" class="inspector-form" auto-label-width>
        <TriggerPanel v-if="store.selection?.kind === 'trigger'" />
        <ParamsPanel v-else-if="store.selection?.kind === 'params'" />
        <ModulePanel v-else-if="step" :key="step.id" :step-id="step.id" />
        <AsyncPanel v-else-if="flow" :key="flow.id" :flow-id="flow.id" />
      </a-form>
    </div>
  </aside>
</template>

<style scoped>
/* Card floating on the right side of the canvas; hidden when no node is selected. */
.inspector {
  position: absolute;
  top: 12px;
  right: 12px;
  bottom: 12px;
  z-index: 5;
  display: flex;
  flex-direction: column;
  width: 440px;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--line-border);
  border-radius: 12px;
  background: var(--bg-body);
  box-shadow: var(--shadow-popover);
}
.inspector-head {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 60px;
  padding: 0 12px 0 20px;
  flex: none;
}
.head-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 7px;
  color: #fff;
  flex: none;
}
:deep(.head-name) {
  flex: 1;
  min-width: 0;
  padding: 0 6px;
  border-color: transparent;
  background: transparent;
}
:deep(.head-name:hover) {
  background: var(--fill-hover);
}
:deep(.head-name.arco-input-focus) {
  border-color: var(--color-primary);
  background: var(--bg-body);
}
:deep(.head-name .arco-input) {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-title);
}
.head-name.static {
  flex: 1;
  min-width: 0;
  padding: 0 7px;
  font-size: 16px;
  font-weight: 600;
  line-height: 32px;
  color: var(--text-title);
}

.switch-menu {
  border: 1px solid var(--line-border);
  border-radius: 8px;
  background: var(--bg-popover);
  box-shadow: var(--shadow-popover);
}
.issue-box {
  width: auto;
  margin: 0 20px 4px;
  flex: none;
}
.inspector-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.inspector-form :deep(.arco-form-item) {
  margin-bottom: 16px;
}
.inspector-form :deep(.arco-form-item-label-col) {
  margin-bottom: 4px;
}
.inspector-form :deep(.arco-form-item-label-col > .arco-form-item-label) {
  font-size: 13px;
  color: var(--text-caption);
}
</style>
