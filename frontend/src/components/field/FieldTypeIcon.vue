<script setup lang="ts">
import { computed } from 'vue'

import type { FieldType } from '@/types/bitable'
import { fieldTypeInfo } from '@/utils/fieldTypes'

const props = withDefaults(defineProps<{ type: FieldType; size?: number }>(), { size: 14 })
const icon = computed(() => fieldTypeInfo(props.type).icon)
</script>

<template>
  <component :is="icon" :size="size" class="field-type-icon" />
</template>

<style scoped>
.field-type-icon {
  flex: none;
  color: var(--text-caption);
}
</style>
