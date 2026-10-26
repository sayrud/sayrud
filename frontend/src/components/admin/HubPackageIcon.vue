<script setup lang="ts">
import type { HubPackage } from '@/api/hub'
import WorkflowPluginIcon from '@/components/admin/WorkflowPluginIcon.vue'

/** Icon of a package of the Sayrud Hub catalog, the field shortcuts without one look like the installed shortcuts. */
withDefaults(defineProps<{ pkg: Pick<HubPackage, 'kind' | 'icon' | 'color'>; size?: number }>(), { size: 44 })
</script>

<template>
  <WorkflowPluginIcon
    :icon="pkg.icon || (pkg.kind === 'field_shortcut' ? 'zap' : undefined)"
    :color="pkg.color || (pkg.kind === 'field_shortcut' ? 'var(--color-primary)' : undefined)"
    :size="size"
  />
</template>
