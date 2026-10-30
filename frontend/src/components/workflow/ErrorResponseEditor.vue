<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import CodeEditor from '@/components/common/CodeEditor.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { WorkflowErrorResponse } from '@/types/workflow'

const props = defineProps<{ status: number; code: string; message: string; retryAfter?: boolean; template?: string; variables?: string[] }>()
const response = defineModel<WorkflowErrorResponse>()
const { t } = useI18n()
const store = useWorkflowEditorStore()

const defaultBody = computed(() => props.template ?? JSON.stringify({ error: { code: props.code, message: props.message } }, null, 2))
const placeholders = computed(() => props.variables?.map((v) => `{{${v}}}`) ?? [])
const body = computed({
  get: () => response.value?.body ?? defaultBody.value,
  set: (value) => {
    if (response.value && !store.readonly) response.value.body = value
  },
})
const invalidBody = computed(() => {
  try {
    JSON.parse(body.value)
    return false
  } catch {
    return true
  }
})
const invalidStatus = computed(() => {
  const status = response.value?.status ?? props.status
  return !Number.isInteger(status) || status < 200 || status > 599 || [204, 205, 304].includes(status)
})

function setCustom(custom: string | number | boolean) {
  if (store.readonly) return

  response.value = custom ? { status: props.status, body: defaultBody.value } : undefined
}

function onCollapseKeydown(event: KeyboardEvent) {
  const target = event.target as HTMLElement | null
  if (!target?.classList.contains('arco-collapse-item-header')) return

  // Arco collapse headers are focusable but do not handle keyboard activation.
  event.preventDefault()
  target.click()
}
</script>

<template>
  <a-collapse expand-icon-position="right" destroy-on-hide class="error-response" @keydown.enter="onCollapseKeydown" @keydown.space="onCollapseKeydown">
    <a-collapse-item key="response" :header="t('workflow.request.errorResponse')">
      <template #extra>
        <a-space :size="6">
          <a-tag v-if="response" size="small" color="arcoblue">{{ t('workflow.request.customResponse') }}</a-tag>
          <a-tag size="small" class="status-code">{{ response?.status ?? status }}</a-tag>
        </a-space>
      </template>

      <a-form-item :extra="variables ? t('workflow.request.templateHelp') : t('workflow.request.errorResponseHelp')">
        <a-radio-group :model-value="!!response" type="button" size="small" class="response-mode" :disabled="store.readonly" @change="setCustom">
          <a-radio :value="false">{{ t('workflow.request.defaultResponse') }}</a-radio>
          <a-radio :value="true">{{ t('workflow.request.customResponse') }}</a-radio>
        </a-radio-group>
      </a-form-item>

      <a-form-item v-if="response" :label="t('workflow.response.status')" :validate-status="invalidStatus ? 'error' : undefined" :help="invalidStatus ? t('workflow.issue.invalidSecurityResponseStatus') : undefined">
        <a-input-number v-model="response.status" :min="200" :max="599" :precision="0" :disabled="store.readonly" />
      </a-form-item>

      <a-form-item :label="t('workflow.response.body')" :validate-status="invalidBody ? 'error' : undefined" :extra="retryAfter ? t('workflow.request.retryAfterHelp') : undefined">
        <template v-if="invalidBody" #help>
          <a-typography-text type="danger" role="alert">{{ t('workflow.issue.invalidSecurityResponseBody') }}</a-typography-text>
        </template>
        <CodeEditor :key="`${!!response}-${store.readonly}`" v-model="body" language="json" :readonly="!response || store.readonly" :error="invalidBody" min-height="112px" max-height="280px" class="response-editor" :aria-label="t('workflow.response.body')" />
      </a-form-item>

      <a-form-item v-if="response && placeholders.length" :label="t('workflow.request.variables')">
        <div class="variables">
          <a-typography-text v-for="p in placeholders" :key="p" code copyable class="variable">{{ p }}</a-typography-text>
        </div>
      </a-form-item>
    </a-collapse-item>
  </a-collapse>
</template>

<style scoped>
.error-response {
  margin: 4px 0 12px;
}

.error-response :deep(.arco-collapse-item-header) {
  font-size: 12px;
  color: var(--text-caption);
}

.error-response :deep(.arco-form-item:last-child) {
  margin-bottom: 0;
}

.status-code {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}

.response-mode {
  display: flex;
  width: 100%;
}

.response-mode :deep(.arco-radio-button) {
  flex: 1;
  text-align: center;
}

.response-editor {
  width: 100%;
}

.variables {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 8px;
}

.variable {
  margin: 0;
  font-size: 12px;
}
</style>
