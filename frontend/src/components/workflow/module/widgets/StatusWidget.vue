<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { FieldDef } from '@/types/workflow'
import { findStep } from '@/utils/workflow'

/** HTTP response status code, picked from the common ones or entered directly. */
const props = defineProps<{ stepId: string; field: FieldDef }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const config = computed(() => findStep(store.spec!, props.stepId)!.config)

const STATUS_CODES = [200, 201, 202, 204, 400, 401, 403, 404, 409, 422, 500]

const STATUS_LABELS: Record<number, () => string> = {
  200: () => t('workflow.response.status200'),
  201: () => t('workflow.response.status201'),
  202: () => t('workflow.response.status202'),
  204: () => t('workflow.response.status204'),
  400: () => t('workflow.response.status400'),
  401: () => t('workflow.response.status401'),
  403: () => t('workflow.response.status403'),
  404: () => t('workflow.response.status404'),
  409: () => t('workflow.response.status409'),
  422: () => t('workflow.response.status422'),
  500: () => t('workflow.response.status500'),
}

const status = computed({
  get: () => config.value[props.field.key!] as number | undefined,
  set: (v) => {
    const n = Number(v)
    config.value[props.field.key!] = Number.isInteger(n) ? n : v
  },
})
const statusClass = computed(() => ((status.value ?? 200) >= 400 ? 'error' : 'ok'))
</script>

<template>
  <a-select v-model="status" allow-create :class="['status-select', statusClass]">
    <a-option v-for="code in STATUS_CODES" :key="code" :value="code">
      <a-space :size="8">
        <a-tag size="small" :color="code >= 400 ? 'red' : 'green'" class="status-code">{{ code }}</a-tag>
        {{ STATUS_LABELS[code]!() }}
      </a-space>
    </a-option>
  </a-select>
</template>

<style scoped>
.status-code {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-weight: 600;
}
</style>
