<script setup lang="ts">
import { Check } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import type { TriggerType } from '@/types/workflow'
import { TRIGGER_META, TRIGGER_TYPES, type TriggerMeta } from '@/utils/workflow'

withDefaults(defineProps<{ selected?: TriggerType; compact?: boolean }>(), { selected: undefined, compact: false })
const emit = defineEmits<{ select: [type: TriggerType] }>()
const { t } = useI18n()

const GROUPS: { key: TriggerMeta['group']; label: () => string }[] = [
  { key: 'table', label: () => t('workflow.trigger.groupTable') },
  { key: 'other', label: () => t('workflow.trigger.groupOther') },
]
const typesOf = (group: TriggerMeta['group']) => TRIGGER_TYPES.filter((type) => TRIGGER_META[type].group === group)
</script>

<template>
  <div class="trigger-list" :class="{ compact }">
    <div v-for="g in GROUPS" :key="g.key" class="group">
      <div class="group-title">{{ g.label() }}</div>
      <a-radio-group :model-value="selected" class="items" @change="(v) => emit('select', v as TriggerType)">
        <a-radio v-for="type in typesOf(g.key)" :key="type" :value="type" class="item-radio">
          <template #radio="{ checked }">
            <span class="item" :class="{ active: checked }">
              <span class="icon" :style="{ background: TRIGGER_META[type].color }">
                <component :is="TRIGGER_META[type].icon" :size="compact ? 14 : 16" />
              </span>
              <span class="text">
                <span class="label">{{ TRIGGER_META[type].label }}</span>
                <span v-if="!compact" class="desc">{{ TRIGGER_META[type].description }}</span>
              </span>
              <Check v-if="compact && checked" :size="14" class="check" />
            </span>
          </template>
        </a-radio>
      </a-radio-group>
    </div>
  </div>
</template>

<style scoped>
.trigger-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.trigger-list.compact {
  gap: 4px;
  width: 300px;
  padding: 6px;
}
.group-title {
  margin-bottom: 8px;
  font-size: 12px;
  color: var(--text-placeholder);
}
.compact .group-title {
  margin: 4px 8px 2px;
}
.items {
  display: grid !important;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}
.compact .items {
  grid-template-columns: 1fr;
  gap: 0;
}
:deep(.item-radio) {
  margin: 0;
  padding: 0;
}
.item {
  display: flex;
  width: 100%;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  background: var(--bg-body);
  cursor: pointer;
  transition:
    border-color 0.15s,
    background-color 0.15s;
}
.item:hover {
  border-color: var(--line-border-strong);
}
.item.active {
  border-color: var(--color-primary);
  background: var(--color-primary-lighter);
}
.compact .item {
  align-items: center;
  padding: 6px 8px;
  border: none;
  border-radius: 6px;
}
.compact .item:hover {
  background: var(--fill-hover);
}
.compact .item.active {
  background: transparent;
}
.icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 7px;
  color: #fff;
  flex: none;
}
.compact .icon {
  width: 24px;
  height: 24px;
  border-radius: 6px;
}
.text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex: 1;
}
.label {
  font-weight: 500;
  color: var(--text-title);
}
.compact .label {
  font-weight: 400;
}
.desc {
  margin-top: 2px;
  font-size: 12px;
  line-height: 18px;
  color: var(--text-placeholder);
}
.check {
  color: var(--color-primary);
  flex: none;
}
</style>
