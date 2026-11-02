<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { AccessTokenScope } from '@/api/account'
import { sortScopes } from '@/utils/accessToken'

const { t } = useI18n()

const props = defineProps<{ scopes: AccessTokenScope[] }>()

const COLORS: Record<AccessTokenScope, string> = { read: 'arcoblue', write: 'orangered', events: 'purple' }
const labels = computed<Record<AccessTokenScope, string>>(() => ({
  read: t('accessToken.scopeRead'),
  write: t('accessToken.scopeWrite'),
  events: t('accessToken.scopeEvents'),
}))
const sorted = computed(() => sortScopes(props.scopes))
</script>

<template>
  <a-space :size="4" wrap>
    <a-tag v-for="s in sorted" :key="s" :color="COLORS[s]" size="small">{{ labels[s] }}</a-tag>
  </a-space>
</template>
