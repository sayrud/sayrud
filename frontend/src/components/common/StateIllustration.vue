<script setup lang="ts">
import addUser from '@/assets/illustrations/add-user.svg?raw'
import emptyFolder from '@/assets/illustrations/empty-folder.svg?raw'
import featureUpgrade from '@/assets/illustrations/feature-upgrade.svg?raw'
import noFiles from '@/assets/illustrations/no-files.svg?raw'
import noMembers from '@/assets/illustrations/no-members.svg?raw'
import noPermission from '@/assets/illustrations/no-permission.svg?raw'
import noResults from '@/assets/illustrations/no-results.svg?raw'
import notFound from '@/assets/illustrations/not-found.svg?raw'

const illustrations = {
  'add-user': addUser,
  'empty-folder': emptyFolder,
  'feature-upgrade': featureUpgrade,
  'no-files': noFiles,
  'no-members': noMembers,
  'no-permission': noPermission,
  'no-results': noResults,
  'not-found': notFound,
}

withDefaults(defineProps<{ name: keyof typeof illustrations; width?: number }>(), { width: 180 })
</script>

<template>
  <!-- Bundled SVGs inherit the current theme; surrounding text describes the state. -->
  <span class="state-illustration" :style="{ width: `${width}px` }" aria-hidden="true" v-html="illustrations[name]" />
</template>

<style scoped>
.state-illustration {
  display: inline-block;
  max-width: 100%;
  flex: none;
  line-height: 0;
  vertical-align: middle;
}
.state-illustration :deep(svg) {
  display: block;
  width: 100%;
  height: auto;
}
</style>
