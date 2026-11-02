<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { Bot as BotIcon, Plus } from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { adminApi, type BotStatusFilter } from '@/api/admin'
import { adminBotsApi, type Bot } from '@/api/bots'
import BotCreateModal from '@/components/bots/BotCreateModal.vue'
import BotTable from '@/components/bots/BotTable.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useSiteProjectSearch } from '@/composables/useProjectSearch'

const { t } = useI18n()
const router = useRouter()
const searchProjects = useSiteProjectSearch()

const statusOptions = computed<{ value: BotStatusFilter; label: string }[]>(() => [
  { value: '', label: t('admin.users.statusAll') },
  { value: 'active', label: t('admin.users.statusActive') },
  { value: 'disabled', label: t('admin.users.statusDisabled') },
])

const bots = ref<Bot[]>([])
const loading = ref(false)
const query = reactive({ keyword: '', status: '' as BotStatusFilter })
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })
const createVisible = ref(false)

async function load() {
  loading.value = true
  try {
    const resp = await adminApi.bots({
      page: pagination.current,
      pageSize: pagination.pageSize,
      keyword: query.keyword.trim() || undefined,
      status: query.status,
    })
    bots.value = resp.bots
    pagination.total = resp.total
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

function reload() {
  pagination.current = 1
  load()
}

let timer: ReturnType<typeof setTimeout> | undefined
watch(
  () => query.keyword,
  () => {
    clearTimeout(timer)
    timer = setTimeout(reload, 300)
  },
)
watch(() => query.status, reload)
onBeforeUnmount(() => clearTimeout(timer))

function onPageChange(page: number) {
  pagination.current = page
  load()
}

function onPageSizeChange(pageSize: number) {
  pagination.pageSize = pageSize
  reload()
}

function open(b: Bot) {
  router.push({ name: 'admin-bot', params: { botId: b.id } })
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader :title="t('admin.bots.title')" :description="t('admin.bots.description')">
      <template #extra>
        <a-button type="primary" @click="createVisible = true">
          <template #icon><Plus :size="16" /></template>
          {{ t('admin.bots.create') }}
        </a-button>
      </template>
    </PageHeader>

    <SettingsSection flush>
      <div v-if="pagination.total > 0 || query.keyword.trim() || query.status" class="toolbar">
        <a-radio-group v-model="query.status" type="button">
          <a-radio v-for="o in statusOptions" :key="o.value" :value="o.value">{{ o.label }}</a-radio>
        </a-radio-group>
        <a-input-search v-model="query.keyword" :placeholder="t('admin.bots.search')" allow-clear class="search" />
      </div>

      <BotTable :bots="bots" :loading="loading" show-owner @open="open">
        <template #empty>
          <a-empty class="empty">
            <template #image><BotIcon :size="40" :stroke-width="1.5" /></template>
            <template v-if="query.keyword.trim() || query.status">{{ t('admin.bots.noMatch') }}</template>
            <template v-else>
              <div class="empty-title">{{ t('admin.bots.empty') }}</div>
              <div class="text-desc">{{ t('admin.bots.emptyHint') }}</div>
              <a-button type="primary" class="empty-action" @click="createVisible = true">
                <template #icon><Plus :size="16" /></template>
                {{ t('admin.bots.create') }}
              </a-button>
            </template>
          </a-empty>
        </template>
      </BotTable>

      <div v-if="pagination.total > pagination.pageSize" class="pagination-footer">
        <a-pagination
          :current="pagination.current"
          :page-size="pagination.pageSize"
          :total="pagination.total"
          :disabled="loading"
          :page-size-options="[10, 20, 50, 100]"
          show-total
          show-page-size
          @change="onPageChange"
          @page-size-change="onPageSizeChange"
        >
          <template #total="{ total }">
            {{ t('admin.bots.total', { n: total }, total) }}
          </template>
        </a-pagination>
      </div>
    </SettingsSection>

    <BotCreateModal v-model:visible="createVisible" :api="adminBotsApi" :search-projects="searchProjects" pick-owner @done="open" />
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 24px;
}
.search {
  width: 260px;
}
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
.pagination-footer {
  padding: 16px 24px;
}
.pagination-footer :deep(.arco-pagination) {
  width: 100%;
  flex-wrap: wrap;
  row-gap: 12px;
}
.pagination-footer :deep(.arco-pagination-total) {
  margin-right: auto;
  color: var(--text-caption);
}
@media (max-width: 768px) {
  .toolbar,
  .pagination-footer {
    padding: 12px 16px;
  }
  .search {
    width: 100%;
  }
}
</style>
