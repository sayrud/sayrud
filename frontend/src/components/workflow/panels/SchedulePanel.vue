<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import PanelSection from '@/components/workflow/PanelSection.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { ScheduleRepeat, ScheduleTrigger } from '@/types/workflow'
import { scheduleSummary, weekdayName } from '@/utils/workflow'

const { t } = useI18n()
const store = useWorkflowEditorStore()
const schedule = computed(() => store.spec!.trigger.schedule as ScheduleTrigger)

const REPEATS: { value: ScheduleRepeat; label: () => string }[] = [
  { value: 'none', label: () => t('workflow.schedule.none') },
  { value: 'minutes', label: () => t('workflow.schedule.minutes') },
  { value: 'hours', label: () => t('workflow.schedule.hours') },
  { value: 'daily', label: () => t('workflow.schedule.daily') },
  { value: 'workdays', label: () => t('workflow.schedule.workdays') },
  { value: 'weekly', label: () => t('workflow.schedule.weekly') },
  { value: 'monthly', label: () => t('workflow.schedule.monthly') },
]
/** Weekdays starting with Monday; value 0 is Sunday. */
const WEEKDAYS = [1, 2, 3, 4, 5, 6, 0]

const timezones = computed(() => {
  const list = (Intl as unknown as { supportedValuesOf?: (key: string) => string[] }).supportedValuesOf?.('timeZone') ?? []
  return list.includes(schedule.value.timezone) ? list : [schedule.value.timezone, ...list]
})

function changeRepeat(repeat: ScheduleRepeat) {
  const s = schedule.value
  s.repeat = repeat
  s.interval = repeat === 'minutes' ? Math.max(s.interval ?? 0, 15) : repeat === 'hours' ? Math.max(s.interval ?? 0, 1) : undefined
  if (repeat === 'weekly' && !s.weekdays?.length) s.weekdays = [new Date(s.start || Date.now()).getDay()]
  if (repeat !== 'weekly') s.weekdays = undefined
}
</script>

<template>
  <PanelSection :title="t('workflow.schedule.title')">
    <a-form-item :label="t('workflow.schedule.start')" :extra="schedule.repeat === 'none' ? t('workflow.schedule.startOnceHelp') : t('workflow.schedule.startHelp')">
      <a-date-picker v-model="schedule.start" show-time format="YYYY-MM-DD HH:mm" value-format="YYYY-MM-DDTHH:mm" :time-picker-props="{ format: 'HH:mm' }" style="width: 100%" />
    </a-form-item>
    <a-form-item :label="t('workflow.schedule.repeat')">
      <a-select :model-value="schedule.repeat" @change="(v) => changeRepeat(v as ScheduleRepeat)">
        <a-option v-for="r in REPEATS" :key="r.value" :value="r.value">{{ r.label() }}</a-option>
      </a-select>
    </a-form-item>
    <a-form-item v-if="schedule.repeat === 'minutes' || schedule.repeat === 'hours'" :label="t('workflow.schedule.interval')">
      <a-input-number v-model="schedule.interval" :min="schedule.repeat === 'minutes' ? 5 : 1" :max="schedule.repeat === 'minutes' ? 1440 : 168">
        <template #prefix>{{ t('workflow.schedule.every') }}</template>
        <template #suffix>{{ schedule.repeat === 'minutes' ? t('workflow.date.minutes') : t('workflow.date.hours') }}</template>
      </a-input-number>
    </a-form-item>
    <a-form-item v-if="schedule.repeat === 'weekly'" :label="t('workflow.schedule.weekdays')">
      <a-checkbox-group v-model="schedule.weekdays" class="weekdays">
        <a-checkbox v-for="d in WEEKDAYS" :key="d" :value="d">{{ weekdayName(d) }}</a-checkbox>
      </a-checkbox-group>
    </a-form-item>
    <a-form-item :label="t('workflow.schedule.timezone')">
      <a-select v-model="schedule.timezone" allow-search>
        <a-option v-for="tz in timezones" :key="tz" :value="tz">{{ tz }}</a-option>
      </a-select>
    </a-form-item>
    <div class="summary">{{ scheduleSummary(schedule) }}</div>
  </PanelSection>

  <a-alert class="note">{{ t('workflow.schedule.note') }}</a-alert>
</template>

<style scoped>
.weekdays {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 0;
}
.summary {
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--color-primary-lighter);
  color: var(--color-primary);
  font-size: 13px;
}
.note {
  width: auto;
  margin: 4px 20px 20px;
}
</style>
