<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { Bot as BotIcon, Plus } from '@lucide/vue'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { myBotsApi, type Bot } from '@/api/bots'
import BotCreateModal from '@/components/bots/BotCreateModal.vue'
import BotTable from '@/components/bots/BotTable.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useMemberProjectSearch } from '@/composables/useProjectSearch'

const { t } = useI18n()
const router = useRouter()
const searchProjects = useMemberProjectSearch()

const bots = ref<Bot[]>([])
const limit = ref(0)
const canCreate = ref(false)
const loading = ref(true)
const createVisible = ref(false)

/** Why no more bots can be created, empty if one can. */
const blocked = computed(() => {
  if (loading.value || canCreate.value) return ''
  return limit.value && bots.value.length >= limit.value ? t('myBots.limitReached', { n: limit.value }, limit.value) : t('myBots.turnedOff')
})

async function load() {
  loading.value = true
  try {
    const resp = await myBotsApi.list()
    bots.value = resp.bots
    limit.value = resp.limit
    canCreate.value = resp.canCreate
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

function open(b: Bot) {
  router.push({ name: 'settings-bot', params: { botId: b.id } })
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader :title="t('myBots.title')" :description="t('myBots.description')">
      <template #extra>
        <a-tag v-if="limit && !loading">{{ t('myBots.quota', { n: bots.length, limit }) }}</a-tag>
        <a-tooltip :content="blocked" :disabled="!blocked">
          <a-button type="primary" :disabled="!!blocked || loading" @click="createVisible = true">
            <template #icon><Plus :size="16" /></template>
            {{ t('admin.bots.create') }}
          </a-button>
        </a-tooltip>
      </template>
    </PageHeader>

    <SettingsSection flush>
      <BotTable :bots="bots" :loading="loading" @open="open">
        <template #empty>
          <a-empty class="empty">
            <template #image><BotIcon :size="40" :stroke-width="1.5" /></template>
            <div class="empty-title">{{ t('admin.bots.empty') }}</div>
            <div class="text-desc">{{ blocked || t('admin.bots.emptyHint') }}</div>
            <a-button v-if="!blocked" type="primary" class="empty-action" @click="createVisible = true">
              <template #icon><Plus :size="16" /></template>
              {{ t('admin.bots.create') }}
            </a-button>
          </a-empty>
        </template>
      </BotTable>
    </SettingsSection>

    <BotCreateModal v-model:visible="createVisible" :api="myBotsApi" :search-projects="searchProjects" @done="open" />
  </div>
</template>

<style scoped>
.empty {
  padding: 24px 0;
  color: var(--text-placeholder);
}
.empty-title {
  color: var(--text-title);
  font-weight: 500;
}
.empty-action {
  margin-top: 16px;
}
</style>
