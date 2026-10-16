<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { CircleAlert, CloudOff, FlaskConical, GitCommitVertical, History, PanelLeftOpen, Rocket, Send, Workflow } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import DebugPanel from '@/components/workflow/DebugPanel.vue'
import TriggerDebugPanel from '@/components/workflow/TriggerDebugPanel.vue'
import FlowCanvas from '@/components/workflow/FlowCanvas.vue'
import NodeInspector from '@/components/workflow/NodeInspector.vue'
import RevisionsDrawer from '@/components/workflow/RevisionsDrawer.vue'
import RunsDrawer from '@/components/workflow/RunsDrawer.vue'
import StateIllustration from '@/components/common/StateIllustration.vue'
import { workflowsApi } from '@/api/workflow'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import { useProjectWorkflowsStore } from '@/stores/projectWorkflows'
import type { WorkflowIssue } from '@/types/workflow'
import { findStep, issueMessage, stepTitle } from '@/utils/workflow'

const props = defineProps<{ projectUid: string; workflowUid: string; sidebarCollapsed?: boolean }>()
const emit = defineEmits<{ expandSidebar: []; missing: [] }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const workflows = useProjectWorkflowsStore()

const debugOpen = ref(false)
const runsOpen = ref(false)
const revisionsOpen = ref(false)
const publishing = ref(false)
const issuesOpen = ref(false)
const editingName = ref(false)
const nameInput = ref<{ $el: HTMLElement }>()

async function load() {
  debugOpen.value = false
  try {
    await store.load(props.projectUid, props.workflowUid)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  }
}

// Save the current draft before switching workflows.
watch(() => [props.projectUid, props.workflowUid], async () => {
  await store.flush().catch(() => undefined)
  await load()
})
onMounted(load)

watch(() => store.notFound, (missing) => {
  if (missing) emit('missing')
})

watch(() => store.saveState, (state) => {
  if (state !== 'conflict') return
  Modal.warning({
    title: t('workflow.editor.conflictTitle'),
    content: t('workflow.editor.conflictContent'),
    okText: t('workflow.editor.reload'),
    maskClosable: false,
    escToClose: false,
    onOk: () => store.reload(),
  })
})

// The name, trigger type, and publish state shown in the sidebar follow the editor.
watch(
  () => [store.workflow, store.name, store.spec?.trigger.type, store.spec?.trigger.api?.method, store.spec?.trigger.api?.path] as const,
  () => {
    const wf = store.workflow
    if (!wf || wf.uid !== props.workflowUid || !store.spec) return
    const api = store.spec.trigger.api
    workflows.patch(wf.uid, {
      name: store.name.trim() || wf.name,
      triggerType: store.spec.trigger.type,
      method: api?.method ?? '',
      path: api?.path ?? '',
      enabled: wf.enabled,
      publishedVersion: wf.publishedVersion,
      publishedTriggerType: wf.publishedTriggerType,
      publishedMethod: wf.publishedMethod,
      publishedPath: wf.publishedPath,
      hasUnpublishedChanges: wf.hasUnpublishedChanges,
    })
  },
  { deep: true },
)

const saveFailed = computed(() => store.saveState === 'error' || store.saveState === 'conflict')

function issueTargetName(issue: WorkflowIssue) {
  const [kind, id] = issue.target.split(':')
  const spec = store.spec
  if (!spec) return ''
  if (kind === 'step' && id) {
    const step = findStep(spec, id)
    return step ? stepTitle(step) : id
  }
  if (kind === 'param' && id) return spec.trigger.api?.params.find((p) => p.id === id)?.key ?? ''
  if (kind === 'async' && id) return spec.async.find((f) => f.id === id)?.name || t('workflow.async.untitled')
  if (kind === 'trigger' || kind === 'route' || kind === 'auth' || kind === 'rate_limit') return t('workflow.trigger.title')
  return t('workflow.editor.flowTitle')
}

function locateIssue(issue: WorkflowIssue) {
  issuesOpen.value = false
  const [kind, id] = issue.target.split(':')
  if (kind === 'step' && id) store.select({ kind: 'step', id })
  else if (kind === 'param' || kind === 'params') store.select({ kind: 'params' })
  else if (kind === 'async' && id) store.select({ kind: 'async', id })
  else if (kind === 'flow') store.select(null)
  else store.select({ kind: 'trigger' })
}

async function setEnabled(enabled: boolean) {
  if (!store.workflow) return
  await workflowsApi.setEnabled(store.projectUID, store.workflow.uid, enabled)
  store.workflow.enabled = enabled
}

// Saves the draft, publishes it as a new version, and enables the workflow if it is disabled.
async function saveAndEnable() {
  if (!store.workflow || publishing.value) return
  publishing.value = true
  try {
    await store.flush()
    const resp = await workflowsApi.publish(store.projectUID, store.workflow.uid, store.draftVersion)
    if (resp.issues.length) {
      store.issues = resp.issues
      issuesOpen.value = true
      Message.warning(t('workflow.editor.publishBlocked', { n: resp.issues.length }))
      return
    }
    if (resp.workflow) store.setWorkflow(resp.workflow)
    if (!store.workflow.enabled) await setEnabled(true)
    Message.success(t('workflow.editor.publishSuccess'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    publishing.value = false
  }
}

// The switch shows whether the workflow is running: published and enabled. Turning it on publishes an unpublished workflow first.
const running = computed(() => !!store.workflow?.publishedVersion && !!store.workflow.enabled)
const toggling = ref(false)

async function toggleRunning(value: boolean | string | number) {
  if (!store.workflow || toggling.value) return
  const enabled = !!value
  toggling.value = true
  try {
    if (enabled && !store.workflow.publishedVersion) {
      await saveAndEnable()
      return
    }
    await setEnabled(enabled)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    toggling.value = false
  }
}

function startRename() {
  if (store.readonly) return
  editingName.value = true
  void nextTick(() => nameInput.value?.$el.querySelector('input')?.select())
}
function finishRename() {
  editingName.value = false
  store.name = store.name.trim() || store.workflow?.name || ''
}

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
    e.preventDefault()
    void store.flush().catch(() => undefined)
  }
}
function onBeforeUnload(e: BeforeUnloadEvent) {
  if (store.saveState === 'dirty' || store.saveState === 'saving') e.preventDefault()
}
onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('beforeunload', onBeforeUnload)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('beforeunload', onBeforeUnload)
  void store.flush().catch(() => undefined)
})
</script>

<template>
  <div class="workflow-editor">
    <header class="editor-head">
      <a-button v-if="sidebarCollapsed" type="text" size="small" class="btn-muted" :title="t('base.expandSidebar')" :aria-label="t('base.expandSidebar')" @click="emit('expandSidebar')"><template #icon><PanelLeftOpen :size="16" /></template></a-button>
      <span class="head-icon"><Workflow :size="15" /></span>
      <template v-if="store.workflow && store.workflow.uid === workflowUid">
        <a-input v-if="editingName" ref="nameInput" v-model="store.name" size="small" class="name-input" :max-length="64" @press-enter="finishRename" @keydown.esc="finishRename" @blur="finishRename" />
        <span v-else class="workflow-name" :class="{ readonly: store.readonly }" :title="store.readonly ? undefined : t('workflow.editor.renameHint')" @click="startRename">{{ store.name }}</span>
        <span class="head-divider" />
        <a-switch :model-value="running" :loading="toggling" :disabled="!store.canManage" @change="toggleRunning" />
        <span class="spacer" />
        <span v-if="saveFailed" class="save-error">
          <CloudOff :size="14" />
          {{ t('workflow.editor.saveFailed') }}
        </span>
        <a-popover v-if="store.issues.length" v-model:popup-visible="issuesOpen" trigger="click" position="br" :content-style="{ padding: 0 }">
          <a-button size="small" shape="round" status="danger">
            <template #icon><CircleAlert :size="15" /></template>
            {{ t('workflow.editor.issues', { n: store.issues.length }) }}
          </a-button>
          <template #content>
            <div class="issues-pop">
              <div class="issues-title">{{ t('workflow.editor.issuesTitle') }}</div>
              <a-list :bordered="false" size="small" hoverable :split="false">
                <a-list-item v-for="(issue, i) in store.issues" :key="i" class="issue-item" @click="locateIssue(issue)">
                  <a-list-item-meta :title="issueMessage(issue)" :description="issueTargetName(issue)" />
                </a-list-item>
              </a-list>
            </div>
          </template>
        </a-popover>
        <a-button size="small" :type="debugOpen ? 'primary' : 'secondary'" :disabled="!store.spec?.trigger.type" @click="debugOpen = !debugOpen">
          <template #icon><FlaskConical :size="14" /></template>
          {{ t('workflow.editor.debug') }}
        </a-button>
        <a-button size="small" @click="runsOpen = true">
          <template #icon><History :size="14" /></template>
          {{ t('workflow.editor.runs') }}
        </a-button>
        <a-button size="small" @click="revisionsOpen = true">
          <template #icon><GitCommitVertical :size="14" /></template>
          {{ t('workflow.revisions.title') }}
        </a-button>
        <a-button v-if="store.canManage" type="primary" size="small" :loading="publishing" @click="saveAndEnable">
          <template #icon><component :is="store.workflow.publishedVersion ? Send : Rocket" :size="14" /></template>
          {{ t('workflow.editor.publish') }}
        </a-button>
      </template>
    </header>

    <div v-if="store.loading && (!store.workflow || store.workflow.uid !== workflowUid)" class="center"><a-spin :size="28" /></div>
    <div v-else-if="store.notFound" class="center">
      <a-result :status="null" :title="t('workflow.editor.notFound')">
        <template #icon><StateIllustration name="not-found" /></template>
      </a-result>
    </div>
    <div v-else-if="store.spec && store.workflow?.uid === workflowUid" class="editor-body">
      <div class="canvas-col">
        <div class="canvas-area">
          <FlowCanvas />
          <NodeInspector />
        </div>
        <template v-if="debugOpen">
          <DebugPanel v-if="store.spec.trigger.type === 'api'" @close="debugOpen = false" />
          <TriggerDebugPanel v-else @close="debugOpen = false" @open-runs="runsOpen = true" />
        </template>
      </div>
    </div>

    <RunsDrawer v-model:visible="runsOpen" />
    <RevisionsDrawer v-model:visible="revisionsOpen" />
  </div>
</template>

<style scoped>
.workflow-editor {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--bg-body);
}
.editor-head {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 48px;
  padding: 0 16px 0 12px;
  border-bottom: 1px solid var(--line-border);
  flex: none;
}
.editor-head > * {
  flex: none;
}
.head-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  color: var(--text-title);
  opacity: 0.8;
}
.workflow-name {
  flex: 0 1 auto;
  min-width: 0;
  max-width: 280px;
  padding: 2px 6px;
  border-radius: 6px;
  font-size: 15px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: text;
}
.workflow-name:hover:not(.readonly) {
  background: var(--fill-hover);
}
.workflow-name.readonly {
  cursor: default;
}
:deep(.name-input) {
  width: 240px;
}
:deep(.name-input .arco-input) {
  font-size: 15px;
  font-weight: 600;
}
.head-divider {
  width: 1px;
  height: 16px;
  background: var(--line-border);
}
.save-error {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--color-danger);
}
.spacer {
  flex: 1;
}
.issues-pop {
  width: 380px;
  max-height: 420px;
  overflow: auto;
  padding: 8px;
}
.issues-title {
  padding: 6px 8px 8px;
  font-weight: 600;
  color: var(--text-title);
}
.issue-item {
  cursor: pointer;
  border-radius: 6px;
}
.center {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}
.editor-body {
  flex: 1;
  min-height: 0;
  display: flex;
}
.canvas-area {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
}
.canvas-col {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
</style>
