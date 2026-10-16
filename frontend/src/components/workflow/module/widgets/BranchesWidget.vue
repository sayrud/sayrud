<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <div class="branch-note"><span class="dot then" />{{ t('workflow.condition.thenNote') }}</div>
  <div class="branch-note"><span class="dot else" />{{ t('workflow.condition.elseNote') }}</div>
</template>

<style scoped>
.branch-note {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 13px;
  line-height: 20px;
  color: var(--text-caption);
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
