<script setup lang="ts">
defineProps<{ dirty: boolean; saving: boolean }>()
defineEmits<{ save: []; reset: [] }>()
</script>

<template>
  <div class="save-bar">
    <span v-if="dirty" class="text-desc save-hint">有未保存的修改</span>
    <a-space>
      <a-button :disabled="!dirty || saving" @click="$emit('reset')">重置</a-button>
      <a-button type="primary" :disabled="!dirty" :loading="saving" @click="$emit('save')">保存</a-button>
    </a-space>
  </div>
</template>

<style scoped>
.save-bar {
  position: sticky;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  margin-top: 16px;
  padding: 12px 24px;
  background: var(--bg-body);
  border: 1px solid var(--line-border);
  border-radius: 8px;
}
.save-hint {
  margin-right: auto;
  color: var(--color-warning);
}
</style>
