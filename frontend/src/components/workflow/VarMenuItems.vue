<script setup lang="ts">
import { VAR_KIND_ICONS, type VarNode } from '@/utils/workflow'

/** Variables in the dropdown: variables with child properties expand into a submenu whose first item selects the variable itself. */
defineProps<{ nodes: VarNode[] }>()
</script>

<template>
  <template v-for="n in nodes" :key="n.path">
    <a-dsubmenu v-if="n.children?.length" trigger="hover" :popup-max-height="360">
      <template #icon><component :is="VAR_KIND_ICONS[n.kind]" :size="14" /></template>
      {{ n.label }}
      <template #content>
        <a-doption :value="n.path">
          <template #icon><component :is="VAR_KIND_ICONS[n.kind]" :size="14" /></template>
          {{ n.label }}
        </a-doption>
        <VarMenuItems :nodes="n.children" />
      </template>
    </a-dsubmenu>
    <a-doption v-else :value="n.path">
      <template #icon><component :is="VAR_KIND_ICONS[n.kind]" :size="14" /></template>
      {{ n.label }}
    </a-doption>
  </template>
</template>
