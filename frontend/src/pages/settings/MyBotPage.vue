<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { myBotsApi } from '@/api/bots'
import BotDetail from '@/components/bots/BotDetail.vue'
import { useMemberProjectSearch } from '@/composables/useProjectSearch'

const { t } = useI18n()

const route = useRoute()
const botId = computed(() => Number(route.params.botId))
const searchProjects = useMemberProjectSearch()
</script>

<template>
  <BotDetail
    :bot-id="botId"
    :api="myBotsApi"
    :list-route="{ name: 'settings-bots' }"
    :list-label="t('myBots.title')"
    :search-projects="searchProjects"
    :remove="myBotsApi.remove"
  />
</template>
