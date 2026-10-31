<script setup lang="ts">
import { ArrowLeftRight, ListChecks, Zap } from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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

const drawerContainer = ref<HTMLElement>()
const viewportWidth = ref(window.innerWidth)
const preferredWidth = ref(440)
const maxWidth = computed(() => Math.floor(viewportWidth.value / 3))
const minWidth = computed(() => Math.min(320, maxWidth.value))
const drawerWidth = computed(() => Math.min(maxWidth.value, Math.max(minWidth.value, preferredWidth.value)))
let resizing: { pointerId: number; startX: number; startWidth: number } | null = null

function setWidth(width: number) {
  preferredWidth.value = Math.min(maxWidth.value, Math.max(minWidth.value, width))
}

function startResize(e: PointerEvent) {
  if (e.button !== 0 || !e.isPrimary) return
  e.preventDefault()
  resizing = { pointerId: e.pointerId, startX: e.clientX, startWidth: drawerWidth.value }
  const handle = e.currentTarget as HTMLElement
  handle.focus({ preventScroll: true })
  handle.setPointerCapture(e.pointerId)
}

function moveResize(e: PointerEvent) {
  if (resizing?.pointerId !== e.pointerId) return
  setWidth(resizing.startWidth + resizing.startX - e.clientX)
}

function stopResize() {
  resizing = null
}

function resizeWithKey(e: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) return
  e.preventDefault()
  setWidth(e.key === 'Home' ? minWidth.value : e.key === 'End' ? maxWidth.value : drawerWidth.value + (e.key === 'ArrowLeft' ? 20 : -20))
}

function updateViewportWidth() {
  viewportWidth.value = window.innerWidth
}

onMounted(() => window.addEventListener('resize', updateViewportWidth))
onBeforeUnmount(() => window.removeEventListener('resize', updateViewportWidth))

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
  <div ref="drawerContainer" class="inspector-host">
    <a-drawer
      v-if="drawerContainer"
      :visible="!!head"
      :width="drawerWidth"
      :popup-container="drawerContainer"
      :render-to-body="false"
      :mask="false"
      :footer="false"
      :body-style="{ padding: 0 }"
      class="inspector"
      unmount-on-close
      @cancel="store.select(null)"
    >
      <template #title>
        <div v-if="head" class="inspector-head">
          <div
            class="inspector-resize"
            role="separator"
            tabindex="0"
            aria-orientation="vertical"
            :aria-label="t('tableSidebar.resize')"
            :aria-valuemin="minWidth"
            :aria-valuemax="maxWidth"
            :aria-valuenow="drawerWidth"
            @pointerdown="startResize"
            @pointermove="moveResize"
            @pointerup="stopResize"
            @pointercancel="stopResize"
            @lostpointercapture="stopResize"
            @keydown="resizeWithKey"
          />
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
        </div>
      </template>

      <template v-if="head">
        <a-alert v-if="issues.length" type="error" class="issue-box">
          <div v-for="(issue, i) in issues" :key="i">{{ issueMessage(issue) }}</div>
        </a-alert>

        <a-form layout="vertical" :model="{}" :disabled="store.readonly" class="inspector-form" auto-label-width>
          <TriggerPanel v-if="store.selection?.kind === 'trigger'" />
          <ParamsPanel v-else-if="store.selection?.kind === 'params'" />
          <ModulePanel v-else-if="step" :key="step.id" :step-id="step.id" />
          <AsyncPanel v-else-if="flow" :key="flow.id" :flow-id="flow.id" />
        </a-form>
      </template>
    </a-drawer>
  </div>
</template>

<style scoped>
.inspector-host {
  position: absolute;
  inset: 0;
  z-index: 5;
  pointer-events: none;
}
.inspector-host :deep(.arco-drawer) {
  border: 1px solid var(--color-border-2);
  border-top: none;
}
.inspector-host :deep(.arco-drawer-title) {
  flex: 1;
  min-width: 0;
}
.inspector-resize {
  position: absolute;
  inset: 0 auto 0 0;
  z-index: 1;
  width: 6px;
  cursor: col-resize;
  touch-action: none;
  user-select: none;
}
.inspector-host:has(.inspector-resize:hover, .inspector-resize:focus-visible) :deep(.arco-drawer) {
  border-left-color: var(--color-primary);
  box-shadow: inset 1px 0 0 var(--color-primary);
}
.inspector-head {
  display: flex;
  align-items: center;
  gap: 8px;
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
  width: 300px;
  padding: 8px 6px 6px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  background: var(--bg-popover);
  box-shadow: var(--shadow-popover);
}
.issue-box {
  width: auto;
  margin: 16px 20px 4px;
  flex: none;
}
.inspector-form :deep(.arco-form-item) {
  margin-bottom: 16px;
}
.inspector-form :deep(.arco-form-item-label-col) {
  margin-bottom: 4px;
}
</style>
