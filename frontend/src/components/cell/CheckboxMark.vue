<script setup lang="ts">
import { Check } from '@lucide/vue'

defineProps<{ checked: boolean; disabled?: boolean }>()
</script>

<template>
  <span class="checkbox-mark" :class="{ checked, disabled }">
    <Check v-if="checked" :size="12" :stroke-width="3" />
  </span>
</template>

<style scoped>
.checkbox-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border: 1.5px solid #bbbfc4;
  border-radius: 4px;
  background: #fff;
  color: #fff;
  flex: none;
  transition:
    background 0.15s,
    border-color 0.15s;
}
.checkbox-mark:hover {
  border-color: var(--color-primary);
}
.checkbox-mark.checked {
  background: var(--color-primary);
  border-color: var(--color-primary);
}
.checkbox-mark.disabled {
  opacity: 0.6;
}
</style>
