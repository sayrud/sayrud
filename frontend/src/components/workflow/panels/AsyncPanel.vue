<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import PanelSection from '@/components/workflow/PanelSection.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import { eventDescription, eventLabel } from '@/utils/workflow'

const props = defineProps<{ flowId: string }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const flow = computed(() => store.spec!.async.find((f) => f.id === props.flowId)!)
</script>

<template>
  <PanelSection :title="t('workflow.async.title')">
    <a-form-item :label="t('workflow.async.event')">
      <a-radio-group v-model="flow.event" direction="vertical" class="events">
        <a-radio v-for="event in (['succeeded', 'request_received'] as const)" :key="event" :value="event" class="event" :class="{ active: flow.event === event }">
          <div class="event-title">{{ eventLabel(event) }}</div>
          <div class="event-desc">{{ eventDescription(event) }}</div>
        </a-radio>
      </a-radio-group>
    </a-form-item>
    <a-alert>{{ t('workflow.async.atLeastOnce') }}</a-alert>
  </PanelSection>
</template>

<style scoped>
:deep(.events) {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}
:deep(.event) {
  display: flex;
  align-items: flex-start;
  margin: 0;
  padding: 10px 12px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
}
:deep(.event.active) {
  border-color: var(--color-primary);
  background: var(--color-primary-lighter);
}
:deep(.event .arco-radio-label) {
  white-space: normal;
}
.event-title {
  font-weight: 500;
  color: var(--text-title);
}
.event-desc {
  margin-top: 2px;
  font-size: 12px;
  line-height: 18px;
  color: var(--text-placeholder);
}
</style>
