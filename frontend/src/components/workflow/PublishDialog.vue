<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { CircleCheck, Copy } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { apiEndpoint, workflowsApi } from '@/api/workflow'
import CodeEditor from '@/components/common/CodeEditor.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { TriggerType, WorkflowRevision } from '@/types/workflow'
import { curlExample, fetchExample, METHOD_COLORS, TRIGGER_META, triggerSummary } from '@/utils/workflow'

const visible = defineModel<boolean>('visible', { required: true })
const props = defineProps<{ justPublished: boolean }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()

const lang = ref<'curl' | 'js'>('curl')
const revisions = ref<WorkflowRevision[]>([])
const wf = computed(() => store.workflow)
const isApi = computed(() => wf.value?.publishedTriggerType === 'api')
const trigger = computed(() => TRIGGER_META[(wf.value?.publishedTriggerType || 'api') as TriggerType])
const endpoint = computed(() => (wf.value ? apiEndpoint(store.projectUID, wf.value.publishedPath) : ''))
const example = computed(() => {
  const draftApi = store.spec?.trigger.api
  if (!store.spec || !wf.value || !draftApi) return ''
  const spec = { ...store.spec, trigger: { ...store.spec.trigger, api: { ...draftApi, method: wf.value.publishedMethod as typeof draftApi.method } } }
  return lang.value === 'curl' ? curlExample(spec, endpoint.value) : fetchExample(spec, endpoint.value)
})

function revisionTrigger(r: WorkflowRevision) {
  return r.path ? `${r.method} ${r.path}` : TRIGGER_META[r.triggerType as TriggerType]?.label ?? r.triggerType
}

watch(visible, async (v) => {
  if (!v || !wf.value) return
  try {
    revisions.value = await workflowsApi.revisions(store.projectUID, wf.value.uid)
  } catch {
    revisions.value = []
  }
})

function copy(text: string) {
  navigator.clipboard?.writeText(text).then(() => Message.success(t('workflow.request.copied')), () => undefined)
}
</script>

<template>
  <a-modal v-model:visible="visible" :width="640" :footer="false" unmount-on-close>
    <template #title>
      <span class="title">
        <CircleCheck v-if="props.justPublished" :size="18" class="ok" />
        {{ props.justPublished ? t('workflow.publish.successTitle', { version: wf?.publishedVersion ?? 0 }) : t('workflow.publish.infoTitle') }}
      </span>
    </template>
    <template v-if="wf && wf.publishedVersion">
      <p v-if="props.justPublished" class="desc">{{ t('workflow.publish.successContent') }}</p>

      <template v-if="isApi">
        <div class="label">{{ t('workflow.request.endpoint') }}</div>
        <div class="endpoint">
          <a-tag size="small" :color="METHOD_COLORS[wf.publishedMethod]" class="method">{{ wf.publishedMethod }}</a-tag>
          <code class="url">{{ endpoint }}</code>
          <a-button type="text" size="mini" class="btn-muted" :title="t('workflow.request.copy')" :aria-label="t('workflow.request.copy')" @click="copy(endpoint)"><template #icon><Copy :size="13" /></template></a-button>
        </div>
      </template>
      <template v-else>
        <div class="label">{{ t('workflow.trigger.title') }}</div>
        <div class="endpoint">
          <span class="trigger-icon" :style="{ color: trigger.color, background: `${trigger.color}14` }"><component :is="trigger.icon" :size="14" /></span>
          <div class="trigger-text">
            <div class="trigger-label">{{ trigger.label }}</div>
            <div v-if="store.spec" class="trigger-summary">{{ triggerSummary(store.spec, store.fields) }}</div>
          </div>
        </div>
        <p class="desc hint">{{ t('workflow.publish.backgroundHint') }}</p>
      </template>
      <a-alert v-if="!wf.enabled" type="warning" class="alert">{{ t('workflow.publish.disabledHint') }}</a-alert>

      <div v-if="isApi" class="label row">
        <span>{{ t('workflow.publish.example') }}</span>
        <a-radio-group v-model="lang" type="button" size="mini">
          <a-radio value="curl">{{ t('workflow.publish.curl') }}</a-radio>
          <a-radio value="js">{{ t('workflow.publish.javascript') }}</a-radio>
        </a-radio-group>
      </div>
      <div v-if="isApi" class="example">
        <CodeEditor :model-value="example" language="javascript" readonly min-height="80px" max-height="220px" />
        <a-button type="text" size="mini" class="btn-muted copy-example" :title="t('workflow.request.copy')" :aria-label="t('workflow.request.copy')" @click="copy(example)"><template #icon><Copy :size="13" /></template></a-button>
      </div>

      <div class="label">{{ t('workflow.publish.revisions') }}</div>
      <div class="revisions">
        <div v-for="r in revisions" :key="r.version" class="revision">
          <span class="version">{{ t('workflow.editor.version', { version: r.version }) }}</span>
          <code class="rev-route">{{ revisionTrigger(r) }}</code>
          <span class="rev-meta">{{ t('workflow.publish.by', { user: r.createdBy?.userName ?? t('common.unknown'), time: dayjs(r.createdAt).format('YYYY-MM-DD HH:mm') }) }}</span>
        </div>
      </div>
    </template>
    <div v-else class="desc">{{ t('workflow.publish.never') }}</div>
  </a-modal>
</template>

<style scoped>
.title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.ok {
  color: var(--color-success);
}
.desc {
  margin: 0 0 8px;
  color: var(--text-caption);
}
.label {
  margin: 16px 0 6px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-title);
}
.label.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.endpoint {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 8px 8px 10px;
  border-radius: 6px;
  background: var(--bg-base);
}
.method {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-weight: 600;
}
.url {
  flex: 1;
  min-width: 0;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12.5px;
  overflow-wrap: anywhere;
}
.alert {
  margin-top: 8px;
}
.hint {
  margin-top: 8px;
  font-size: 12.5px;
}
.trigger-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 7px;
  flex: none;
}
.trigger-text {
  flex: 1;
  min-width: 0;
}
.trigger-label {
  font-weight: 500;
  color: var(--text-title);
}
.trigger-summary {
  font-size: 12px;
  color: var(--text-caption);
}
.example {
  position: relative;
}
.copy-example {
  position: absolute;
  top: 6px;
  right: 6px;
  background: var(--bg-body);
}
.revisions {
  max-height: 180px;
  overflow: auto;
}
.revision {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 0;
  border-bottom: 1px solid var(--line-border);
  font-size: 13px;
}
.version {
  font-weight: 600;
  min-width: 36px;
}
.rev-route {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  color: var(--text-caption);
}
.rev-meta {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-placeholder);
}
</style>
