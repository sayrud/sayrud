<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <a-typography-text type="secondary" class="branch-note workflow-field-tip"><span class="dot then" />{{ t('workflow.condition.thenNote') }}</a-typography-text>
  <a-typography-text type="secondary" class="branch-note workflow-field-tip"><span class="dot else" />{{ t('workflow.condition.elseNote') }}</a-typography-text>
</template>

<style scoped>
.branch-note {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}
.branch-note + .branch-note {
  margin-top: 6px;
}
.dot {
  width: 8px;
  height: 8px;
  margin-top: 6px;
  border-radius: 50%;
  flex: none;
}
.dot.then {
  background: #14a37f;
}
.dot.else {
  background: var(--text-placeholder);
}
</style>
