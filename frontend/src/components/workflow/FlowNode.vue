<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { ArrowLeftRight, Check, CircleAlert, CircleCheck, CircleX, Copy, Ellipsis, GitFork, ListChecks, PencilLine, Trash2, Zap } from '@lucide/vue'
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import FlowFork from '@/components/workflow/FlowFork.vue'
import { useTriggerSwitch } from '@/components/workflow/useTriggerSwitch'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { AsyncEvent, WorkflowStep } from '@/types/workflow'
import {
  apiOf,
  cloneStep,
  countSteps,
  eventLabel,
  findStep,
  issueMessage,
  locate,
  METHOD_COLORS,
  newAsyncFlow,
  STEP_META,
  stepSummary,
  stepTitle,
  successAnchor,
  TRIGGER_META,
  TRIGGER_TYPES,
  triggerMetaOf,
  triggerSummary,
  triggerTitle,
} from '@/utils/workflow'

const props = defineProps<{
  step?: WorkflowStep
  /** Fixed node: the trigger, or the validated params of an API trigger. */
  fixed?: 'trigger' | 'params'
  /** Start node of an async flow. */
  asyncFlowId?: string
  /** Whether to extend the main lane downward when the node's row grows taller because of a fork. */
  tail?: boolean
  /** The main flow ends at this node; draw an end marker below it. */
  end?: boolean
}>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const { change: changeTrigger } = useTriggerSwitch()

const kind = computed(() => (props.fixed ? props.fixed : props.asyncFlowId ? 'async' : 'step'))
const flow = computed(() => (props.asyncFlowId ? store.spec!.async.find((f) => f.id === props.asyncFlowId) : undefined))
const target = computed(() => {
  if (kind.value === 'step') return `step:${props.step!.id}`
  if (kind.value === 'async') return `async:${props.asyncFlowId}`
  return kind.value
})
const issues = computed(() => {
  if (kind.value === 'trigger') return ['trigger', 'route', 'auth', 'rate_limit'].flatMap((x) => store.issuesOf(x))
  if (kind.value === 'params') return store.issues.filter((i) => i.target.startsWith('param:') || i.target === 'params')
  return store.issuesOf(target.value)
})
const selected = computed(() => {
  const s = store.selection
  if (!s) return false
  if (kind.value === 'step') return s.kind === 'step' && s.id === props.step!.id
  if (kind.value === 'async') return s.kind === 'async' && s.id === props.asyncFlowId
  return s.kind === kind.value
})
const trace = computed(() => (props.step ? store.lastTrace.get(props.step.id) : undefined))

const triggerMeta = computed(() => triggerMetaOf(store.spec!.trigger.type))
const api = computed(() => apiOf(store.spec!))
const title = computed(() => {
  if (kind.value === 'trigger') return triggerTitle(store.spec!)
  if (kind.value === 'params') return t('workflow.node.params')
  if (kind.value === 'async') return flow.value?.name?.trim() || t('workflow.async.untitled')
  return stepTitle(props.step!)
})
const summary = computed(() => {
  const spec = store.spec!
  if (kind.value === 'trigger') return triggerSummary(spec, store.fields)
  if (kind.value === 'params') {
    const params = api.value?.params ?? []
    if (!params.length) return t('workflow.node.noParams')
    return t('workflow.node.paramsSummary', { n: params.length, required: params.filter((p) => p.required).length })
  }
  if (kind.value === 'async') return flow.value ? eventLabel(flow.value.event) : ''
  return stepSummary(spec, props.step!, store.fields)
})
const icon = computed(() => {
  if (kind.value === 'trigger') return triggerMeta.value.icon
  if (kind.value === 'params') return ListChecks
  if (kind.value === 'async') return Zap
  return STEP_META[props.step!.type].icon
})
const color = computed(() => {
  if (kind.value === 'trigger') return triggerMeta.value.color
  if (kind.value === 'params') return '#646a73'
  if (kind.value === 'async') return flow.value?.event === 'request_received' ? '#3370ff' : '#14a37f'
  return STEP_META[props.step!.type].color
})

// ---- Async flow forks ----

const isSuccessResponse = computed(() => props.step?.type === 'response' && (props.step.status ?? 200) < 400)
/** Async flow event this node can fork: request received for an API trigger node, succeeded for a successful response node. */
const forkEvent = computed<AsyncEvent | null>(() => {
  if (kind.value === 'trigger') return api.value ? 'request_received' : null
  return isSuccessResponse.value ? 'succeeded' : null
})
/** Async flows drawn as forks from this node: succeeded flows hang off the successful response at the end of the main lane, or off the trigger node when there is none. */
const forkFlows = computed(() => {
  const spec = store.spec!
  if (!api.value) return []
  const anchor = successAnchor(spec)
  if (kind.value === 'trigger') return spec.async.filter((f) => f.event === 'request_received' || (f.event === 'succeeded' && !anchor))
  if (props.step && props.step.id === anchor) return spec.async.filter((f) => f.event === 'succeeded')
  return []
})
/** Other successful response nodes only show tags linking to the succeeded-event flows. */
const linkedFlows = computed(() => (isSuccessResponse.value && !forkFlows.value.length ? store.spec!.async.filter((f) => f.event === 'succeeded') : []))

function addAsync(event: AsyncEvent) {
  const spec = store.spec!
  const f = newAsyncFlow(spec, event)
  spec.async.push(f)
  store.select({ kind: 'async', id: f.id })
}

function select() {
  if (kind.value === 'step') store.select({ kind: 'step', id: props.step!.id })
  else if (kind.value === 'async') store.select({ kind: 'async', id: props.asyncFlowId! })
  else store.select({ kind: kind.value })
}

// ---- Top-right actions ----

const menuOpen = ref(false)
const hasMenu = computed(() => !store.readonly && kind.value !== 'params')
const location = computed(() => (props.step && store.spec ? locate(store.spec, props.step.id) : null))

const editing = ref(false)
const draftName = ref('')
const nameInput = ref<{ $el: HTMLElement }>()

function startRename() {
  menuOpen.value = false
  draftName.value = title.value
  editing.value = true
  void nextTick(() => nameInput.value?.$el.querySelector('input')?.select())
}

function commitRename() {
  if (!editing.value) return
  editing.value = false
  const name = draftName.value.trim()
  if (kind.value === 'trigger') store.spec!.trigger.name = name && name !== triggerMeta.value.label ? name : undefined
  else if (kind.value === 'async' && flow.value) flow.value.name = name || undefined
  else if (props.step) {
    const step = findStep(store.spec!, props.step.id)
    if (step) step.name = name && name !== STEP_META[step.type].label ? name : undefined
  }
}

function duplicate() {
  menuOpen.value = false
  const loc = location.value
  if (!loc || !store.spec) return
  const copy = cloneStep(store.spec, loc.list[loc.index]!)
  loc.list.splice(loc.index + 1, 0, copy)
  store.select({ kind: 'step', id: copy.id })
  Message.success(t('workflow.node.duplicated'))
}

function removeStep() {
  const loc = location.value
  if (!loc) return
  const step = loc.list[loc.index]!
  const nested = countSteps(step.then ?? []) + countSteps(step.else ?? [])
  const doRemove = () => {
    loc.list.splice(loc.index, 1)
    store.select(null)
  }
  if (!nested) return doRemove()
  Modal.warning({
    title: t('workflow.node.deleteTitle', { name: stepTitle(step) }),
    content: t('workflow.node.deleteBranchContent', { n: nested }),
    hideCancel: false,
    okText: t('common.delete'),
    okButtonProps: { status: 'danger' },
    onOk: doRemove,
  })
}

function removeFlow() {
  const f = flow.value
  if (!f) return
  Modal.warning({
    title: t('workflow.async.deleteTitle'),
    content: t('workflow.async.deleteContent'),
    hideCancel: false,
    okText: t('common.delete'),
    okButtonProps: { status: 'danger' },
    onOk: () => {
      store.spec!.async = store.spec!.async.filter((x) => x.id !== f.id)
      store.select(null)
    },
  })
}

function remove() {
  menuOpen.value = false
  if (kind.value === 'async') removeFlow()
  else removeStep()
}
</script>

<template>
  <div class="node-row" :class="{ forked: forkFlows.length }">
    <div class="node-col">
      <div class="flow-node" :data-node="target" :class="{ selected, failed: trace?.status === 'failed' }" tabindex="0" @click="select" @keydown.enter.self="select">
        <span class="node-icon" :style="{ background: color }"><component :is="icon" :size="18" /></span>
        <div class="node-text">
          <a-input
            v-if="editing"
            ref="nameInput"
            v-model="draftName"
            size="mini"
            class="name-input"
            :max-length="40"
            @click.stop
            @pointerdown.stop
            @keydown.enter.prevent="commitRename"
            @keydown.esc.prevent="editing = false"
            @blur="commitRename"
          />
          <div v-else class="node-title ellipsis">{{ title }}</div>
          <div class="node-summary ellipsis">
            <a-tag v-if="kind === 'trigger' && api" size="small" :color="METHOD_COLORS[api.method]" class="method-tag">{{ api.method }}</a-tag>
            <span class="ellipsis">{{ summary }}</span>
          </div>
        </div>
        <div class="node-badges">
          <a-tooltip v-if="issues.length" position="top">
            <template #content>
              <div v-for="(issue, i) in issues" :key="i">{{ issueMessage(issue) }}</div>
            </template>
            <span class="badge issue"><CircleAlert :size="15" /></span>
          </a-tooltip>
          <a-tooltip v-else-if="trace" :content="trace.status === 'failed' ? trace.error || t('workflow.node.lastRunFailed') : t('workflow.node.lastRunSucceeded')">
            <span class="badge" :class="trace.status"><component :is="trace.status === 'failed' ? CircleX : CircleCheck" :size="15" /></span>
          </a-tooltip>
        </div>

        <div v-if="hasMenu" class="node-toolbar" :class="{ open: menuOpen }" @click.stop @pointerdown.stop>
          <a-button v-if="kind === 'step' || kind === 'async'" type="text" size="mini" class="btn-muted" :title="t('workflow.node.delete')" :aria-label="t('workflow.node.delete')" @click="remove">
            <template #icon><Trash2 :size="14" /></template>
          </a-button>
          <a-dropdown v-model:popup-visible="menuOpen" trigger="click" position="br">
            <a-button type="text" size="mini" class="btn-muted" :class="{ active: menuOpen }" :title="t('common.more')" :aria-label="t('common.more')">
              <template #icon><Ellipsis :size="15" /></template>
            </a-button>
            <template #content>
              <a-doption @click="startRename"><template #icon><PencilLine :size="15" /></template>{{ t('workflow.node.rename') }}</a-doption>
              <a-dsubmenu v-if="kind === 'trigger'" value="type" trigger="hover">
                <template #icon><ArrowLeftRight :size="15" /></template>
                {{ t('workflow.node.changeType') }}
                <template #content>
                  <a-doption v-for="type in TRIGGER_TYPES" :key="type" @click="changeTrigger(type)">
                    <template #icon><component :is="TRIGGER_META[type].icon" :size="15" :style="{ color: TRIGGER_META[type].color }" /></template>
                    <span class="type-option">{{ TRIGGER_META[type].label }}<Check v-if="store.spec!.trigger.type === type" :size="14" class="type-check" /></span>
                  </a-doption>
                </template>
              </a-dsubmenu>
              <a-doption v-if="kind === 'step'" @click="duplicate"><template #icon><Copy :size="15" /></template>{{ t('workflow.node.duplicate') }}</a-doption>
              <a-doption v-if="kind === 'step' || kind === 'async'" @click="remove"><template #icon><Trash2 :size="15" /></template>{{ t('workflow.node.delete') }}</a-doption>
            </template>
          </a-dropdown>
        </div>

        <div v-if="linkedFlows.length" class="async-tags" @click.stop>
          <a-tag v-for="f in linkedFlows" :key="f.id" size="small" bordered class="async-tag" @click="store.select({ kind: 'async', id: f.id })">
            <template #icon><Zap :size="11" /></template>
            <span class="ellipsis">{{ f.name || eventLabel(f.event) }}</span>
          </a-tag>
        </div>
        <a-tooltip v-else-if="forkEvent && !store.readonly" :content="t('workflow.editor.forkAsync', { event: eventLabel(forkEvent) })" mini>
          <a-button shape="circle" size="mini" class="fork-handle" :aria-label="t('workflow.editor.forkAsync', { event: eventLabel(forkEvent) })" @click.stop="addAsync(forkEvent)">
            <template #icon><GitFork :size="12" /></template>
          </a-button>
        </a-tooltip>
      </div>
      <template v-if="end">
        <div class="end-link" />
        <span class="end-dot" />
      </template>
      <div v-else-if="forkFlows.length && tail" class="node-tail" />
    </div>
    <FlowFork v-if="forkFlows.length" class="node-fork" :flows="forkFlows" />
  </div>
</template>

<style scoped>
.node-row {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: none;
}
/* When forked, the left and right columns are equal width so the node stays centered on the main lane, and the fork takes up real space without overlapping other nodes. */
.node-row.forked {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: stretch;
}
.node-col {
  display: flex;
  flex-direction: column;
  align-items: center;
  grid-column: 2;
}
.node-fork {
  grid-column: 3;
  justify-self: start;
}
.end-link {
  width: 2px;
  height: 20px;
  background: var(--line-border-strong);
}
.end-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--line-border-strong);
}
.node-tail {
  flex: 1;
  width: 2px;
  background: var(--line-border-strong);
}
.flow-node {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  width: 280px;
  height: 64px;
  padding: 0 14px;
  border: 1px solid var(--line-border);
  border-radius: 12px;
  background: var(--bg-body);
  cursor: pointer;
  outline: none;
  transition:
    border-color 0.15s,
    background-color 0.15s;
  flex: none;
}
.flow-node:hover {
  border-color: var(--line-border-strong);
}
.flow-node.selected {
  border-color: var(--color-primary);
  background: var(--color-primary-lighter);
}
.flow-node.failed:not(.selected) {
  border-color: rgba(245, 74, 69, 0.55);
}
.node-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  color: #fff;
  flex: none;
}
.node-text {
  flex: 1;
  min-width: 0;
}
.node-title {
  font-weight: 600;
  font-size: 14px;
  line-height: 20px;
  color: var(--text-title);
}
.name-input {
  width: 100%;
  margin: -1px 0 -1px -4px;
  padding: 0 4px;
}
.name-input :deep(.arco-input) {
  font-size: 14px;
  font-weight: 600;
}
.node-summary {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 2px;
  font-size: 12px;
  line-height: 18px;
  color: var(--text-caption);
}
.method-tag {
  flex: none;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 11px;
  font-weight: 600;
}
.node-badges {
  display: flex;
  align-items: center;
  flex: none;
}
.badge {
  display: inline-flex;
}
.badge.issue,
.badge.failed {
  color: var(--color-danger);
}
.badge.succeeded {
  color: var(--color-success);
}
/* Action buttons float above the card's top-right corner and only show on hover or while the menu is open; the bottom padding bridges to the card so they don't disappear while moving the mouse. */
.node-toolbar {
  position: absolute;
  right: 0;
  bottom: 100%;
  display: flex;
  gap: 2px;
  padding-bottom: 6px;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.12s;
}
.flow-node:hover .node-toolbar,
.node-toolbar.open {
  opacity: 1;
  pointer-events: auto;
}
.type-option {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.type-check {
  color: var(--color-primary);
}
.fork-handle {
  position: absolute;
  top: 50%;
  right: -11px;
  z-index: 1;
  width: 22px;
  height: 22px;
  margin-top: -11px;
  border: 1px solid var(--line-border-strong);
  background: var(--bg-body);
  color: var(--text-caption);
  opacity: 0;
  transition:
    opacity 0.15s,
    color 0.15s,
    border-color 0.15s;
}
.flow-node:hover .fork-handle {
  opacity: 1;
}
.fork-handle:hover {
  border-color: var(--color-primary);
  background: var(--bg-body);
  color: var(--color-primary);
}
.async-tags {
  position: absolute;
  left: calc(100% + 14px);
  top: 50%;
  transform: translateY(-50%);
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.async-tags::before {
  content: '';
  position: absolute;
  right: 100%;
  top: 50%;
  width: 14px;
  border-top: 1px solid var(--line-border-strong);
}
.async-tag {
  max-width: 160px;
  border-radius: 10px;
  background: var(--bg-body);
  cursor: pointer;
}
.async-tag:hover {
  color: var(--text-title);
}
</style>
