<script setup lang="ts">
import { Message, type TableColumnData } from '@arco-design/web-vue'
import { KeyRound, Plus } from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { AccessToken, AccessTokenApi } from '@/api/account'
import { tokenState, type TokenProjectOption } from '@/utils/accessToken'
import CreateTokenModal from './CreateTokenModal.vue'
import ScopeTags from './ScopeTags.vue'

const { t } = useI18n()

dayjs.extend(relativeTime)

const props = defineProps<{
  api: AccessTokenApi
  searchProjects: (keyword: string) => Promise<TokenProjectOption[]>
}>()
const emit = defineEmits<{ change: [] }>()

const tokens = ref<AccessToken[]>([])
const loading = ref(true)
const createVisible = ref(false)

const columns = computed<TableColumnData[]>(() => [
  { title: t('accessToken.colName'), slotName: 'name', width: 200 },
  { title: t('accessToken.colScopes'), slotName: 'scopes', width: 170 },
  { title: t('accessToken.colProjects'), slotName: 'projects', width: 200 },
  { title: t('accessToken.colExpires'), slotName: 'expires', width: 170 },
  { title: t('accessToken.colLastUsed'), slotName: 'lastUsed', width: 130 },
  { title: t('common.actions'), slotName: 'actions', width: 80, fixed: 'right' },
])

async function load() {
  loading.value = true
  try {
    tokens.value = await props.api.list()
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

async function revoke(token: AccessToken) {
  try {
    await props.api.revoke(token.id)
    Message.success(t('accessToken.revoked'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  }
  await load()
  emit('change')
}

function onCreated() {
  load()
  emit('change')
}

watch(() => props.api, load)
onMounted(load)

defineExpose({ openCreate: () => (createVisible.value = true) })
</script>

<template>
  <div>
    <a-table
      :columns="columns"
      :data="tokens"
      :loading="loading"
      row-key="id"
      :bordered="false"
      :scroll="{ x: 940 }"
      :pagination="false"
    >
      <template #name="{ record }">
        <div class="name-cell">
          <span class="token-name ellipsis">{{ record.name }}</span>
          <a-typography-text code class="prefix">{{ record.prefix }}…</a-typography-text>
        </div>
      </template>
      <template #scopes="{ record }">
        <ScopeTags :scopes="record.scopes" />
      </template>
      <template #projects="{ record }">
        <span v-if="!record.projects.length" class="text-caption">{{ t('accessToken.allProjects') }}</span>
        <a-space v-else :size="4" wrap>
          <a-tag v-for="(p, i) in record.projects.slice(0, 2)" :key="p.uid || i" size="small" :color="p.deleted ? 'gray' : undefined">
            {{ p.deleted ? t('accessToken.deletedProject') : p.name }}
          </a-tag>
          <a-tooltip v-if="record.projects.length > 2">
            <template #content>
              <div v-for="(p, i) in record.projects.slice(2)" :key="p.uid || i">
                {{ p.deleted ? t('accessToken.deletedProject') : p.name }}
              </div>
            </template>
            <a-tag size="small">+{{ record.projects.length - 2 }}</a-tag>
          </a-tooltip>
        </a-space>
      </template>
      <template #expires="{ record }">
        <a-tag v-if="tokenState(record) === 'expired'" color="red" size="small">{{ t('accessToken.expired') }}</a-tag>
        <span v-else-if="!record.expiresAt" class="text-caption">{{ t('accessToken.never') }}</span>
        <a-tooltip v-else :content="dayjs(record.expiresAt).format('YYYY-MM-DD HH:mm')">
          <a-space :size="4" class="nowrap">
            <span>{{ dayjs(record.expiresAt).format('YYYY-MM-DD') }}</span>
            <a-tag v-if="tokenState(record) === 'expiring'" color="orange" size="small">{{ t('accessToken.expiringSoon') }}</a-tag>
          </a-space>
        </a-tooltip>
      </template>
      <template #lastUsed="{ record }">
        <a-tooltip
          v-if="record.lastUsedAt"
          :content="t('accessToken.lastUsedDetail', { time: dayjs(record.lastUsedAt).format('YYYY-MM-DD HH:mm'), ip: record.lastUsedIP || t('common.unknown') })"
        >
          <span>{{ dayjs(record.lastUsedAt).fromNow() }}</span>
        </a-tooltip>
        <span v-else class="text-desc">{{ t('accessToken.neverUsed') }}</span>
      </template>
      <template #actions="{ record }">
        <a-popconfirm
          type="warning"
          :content="t('accessToken.revokeContent', { name: record.name })"
          :ok-text="t('accessToken.revoke')"
          :ok-button-props="{ status: 'danger' }"
          position="tr"
          @ok="revoke(record as AccessToken)"
        >
          <a-button type="text" status="danger" size="small">{{ t('accessToken.revoke') }}</a-button>
        </a-popconfirm>
      </template>
      <template #empty>
        <a-empty v-if="!loading" class="empty">
          <template #image><KeyRound :size="40" :stroke-width="1.5" /></template>
          <div class="empty-title">{{ t('accessToken.empty') }}</div>
          <div class="text-desc">{{ t('accessToken.emptyHint') }}</div>
          <a-button type="primary" class="empty-action" @click="createVisible = true">
            <template #icon><Plus :size="16" /></template>
            {{ t('accessToken.create') }}
          </a-button>
        </a-empty>
      </template>
    </a-table>

    <CreateTokenModal v-model:visible="createVisible" :api="api" :search-projects="searchProjects" @created="onCreated" />
  </div>
</template>

<style scoped>
.name-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  min-width: 0;
}
.token-name {
  max-width: 100%;
  color: var(--text-title);
  font-weight: 500;
}
.prefix {
  margin: 0;
  font-size: 12px;
}
.nowrap {
  white-space: nowrap;
}
:deep(.arco-table-th) {
  background: var(--bg-base);
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
</style>
