<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import PanelSection from '@/components/workflow/PanelSection.vue'
import RecordTriggerPanel from '@/components/workflow/panels/RecordTriggerPanel.vue'
import RequestPanel from '@/components/workflow/panels/RequestPanel.vue'
import SchedulePanel from '@/components/workflow/panels/SchedulePanel.vue'
import TriggerList from '@/components/workflow/TriggerList.vue'
import { useTriggerSwitch } from '@/components/workflow/useTriggerSwitch'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import { needsTable } from '@/utils/workflow'

const { t } = useI18n()
const store = useWorkflowEditorStore()
const { change } = useTriggerSwitch()
const type = computed(() => store.spec!.trigger.type)
</script>

<template>
  <PanelSection v-if="!type" :title="t('workflow.trigger.pick')" :description="t('workflow.trigger.pickHelp')">
    <TriggerList v-if="!store.readonly" compact class="pick-list" @select="change" />
  </PanelSection>

  <template v-else>
    <RequestPanel v-if="type === 'api'" />
    <RecordTriggerPanel v-else-if="needsTable(type)" :key="type" />
    <SchedulePanel v-else />
  </template>
</template>

<style scoped>
/* The rows bleed into the section padding so that the icons and group titles align with the section title. */
.pick-list {
  margin: 4px -8px 0;
}
:global(.switch-loss) {
  margin: 8px 0 0;
  padding-left: 18px;
}
</style>
