<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { adminBotsApi } from '@/api/bots'
import BotDetail from '@/components/bots/BotDetail.vue'
import { useSiteProjectSearch } from '@/composables/useProjectSearch'

const { t } = useI18n()

const route = useRoute()
const botId = computed(() => Number(route.params.botId))
const searchProjects = useSiteProjectSearch()
</script>

<template>
  <BotDetail
    :bot-id="botId"
    :api="adminBotsApi"
    :list-route="{ name: 'admin-bots' }"
    :list-label="t('admin.bots.title')"
    :search-projects="searchProjects"
    admin
  />
</template>
