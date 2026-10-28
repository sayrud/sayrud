<script setup lang="ts">
import { CircleCheck, CloudDownload, Download } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { HubPackage, HubReleaseDetail } from '@/api/hub'
import { hubAction } from '@/utils/hub'
import { versionLabel } from '@/utils/workflowPlugin'

/** The button of a package of the Sayrud Hub catalog: install, update, open the installation, or why it can not be. */
const props = withDefaults(defineProps<{ pkg: HubPackage; latest?: HubReleaseDetail; size?: 'small' | 'medium' | 'large'; long?: boolean }>(), {
  latest: undefined,
  size: 'small',
  long: false,
})
const emit = defineEmits<{ install: []; update: []; open: [] }>()
const { t } = useI18n()

const action = computed(() => hubAction(props.pkg, props.latest))
const disabledHint = computed(() => {
  switch (action.value) {
    case 'conflict':
      return t('hubCatalog.conflictHint', { version: versionLabel(props.pkg.installed!.version) })
    case 'incompatible':
      return t('hubCatalog.incompatibleHint', { version: props.latest!.minAppVersion })
    case 'unavailable':
      return t('hubCatalog.unavailableHint')
  }
  return ''
})
</script>

<template>
  <span class="hub-action" :class="{ long }" @click.stop>
    <a-button v-if="action === 'install'" type="primary" :size="size" :long="long" @click="emit('install')">
      <template #icon><Download :size="14" /></template>
      {{ t('hubCatalog.install') }}
    </a-button>
    <a-button v-else-if="action === 'update'" type="primary" :size="size" :long="long" @click="emit('update')">
      <template #icon><CloudDownload :size="14" /></template>
      {{ t('hub.updateTo', { version: versionLabel(pkg.latestVersion) }) }}
    </a-button>
    <a-tooltip v-else-if="action === 'open'" :content="t('hubCatalog.openHint')">
      <a-button status="success" :size="size" :long="long" @click="emit('open')">
        <template #icon><CircleCheck :size="14" /></template>
        {{ t('hubCatalog.installed') }}
      </a-button>
    </a-tooltip>
    <a-tooltip v-else :content="disabledHint">
      <!-- The wrapper shows the tooltip, a disabled button does not receive the pointer events. -->
      <span class="disabled-wrapper" :class="{ long }" tabindex="0">
        <a-button disabled :size="size" :long="long">
          {{ action === 'conflict' ? t('hubCatalog.installedLocally') : action === 'incompatible' ? t('hubCatalog.incompatible') : t('hubCatalog.unavailable') }}
        </a-button>
      </span>
    </a-tooltip>
  </span>
</template>

<style scoped>
.hub-action {
  display: inline-flex;
  flex: none;
  cursor: default;
}
.hub-action.long,
.disabled-wrapper.long {
  display: flex;
  width: 100%;
}
.disabled-wrapper {
  display: inline-flex;
  border-radius: var(--border-radius-small);
}
.disabled-wrapper:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
.disabled-wrapper :deep(.arco-btn) {
  pointer-events: none;
}
</style>
