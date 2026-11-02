<script setup lang="ts">
import type { TableColumnData } from '@arco-design/web-vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { Bot } from '@/api/bots'
import UserAvatar from '@/components/common/UserAvatar.vue'
import ScopeTags from '@/components/tokens/ScopeTags.vue'
import { tokenState } from '@/utils/accessToken'

const { t } = useI18n()

dayjs.extend(relativeTime)

const props = defineProps<{
  bots: Bot[]
  loading: boolean
  /** Shows who owns each bot, for the admin console. */
  showOwner?: boolean
}>()
const emit = defineEmits<{ open: [bot: Bot] }>()

const columns = computed<TableColumnData[]>(() => [
  { title: t('admin.bots.name'), slotName: 'bot', width: 260 },
  ...(props.showOwner ? [{ title: t('admin.bots.colOwner'), slotName: 'owner', width: 150 }] : []),
  { title: t('admin.users.colStatus'), slotName: 'status', width: 100 },
  { title: t('accessToken.colScopes'), slotName: 'scopes', width: 170 },
  { title: t('accessToken.colExpires'), slotName: 'expires', width: 130 },
  { title: t('admin.bots.colLastUsed'), slotName: 'lastUsed', width: 120 },
  { title: '', slotName: 'open', width: 72, align: 'right' as const },
])
</script>

<template>
  <a-table
    :columns="columns"
    :data="bots"
    :loading="loading"
    row-key="id"
    :bordered="false"
    :scroll="{ x: showOwner ? 1000 : 860 }"
    :pagination="false"
    row-class="bot-row"
    @row-click="(record: unknown) => emit('open', record as Bot)"
  >
    <template #bot="{ record }">
      <div class="bot-cell">
        <UserAvatar :name="record.userName" :color="record.color" :avatar-url="record.avatarUrl" :size="32" bot />
        <div class="bot-text">
          <div class="bot-name ellipsis">{{ record.userName }}</div>
          <div class="text-desc ellipsis">{{ record.email }}</div>
        </div>
      </div>
    </template>
    <template #owner="{ record }">
      <div v-if="record.owner" class="owner-cell">
        <UserAvatar :name="record.owner.userName" :color="record.owner.color" :avatar-url="record.owner.avatarUrl" :size="22" />
        <span class="ellipsis">{{ record.owner.userName }}</span>
      </div>
    </template>
    <template #status="{ record }">
      <a-badge
        :status="record.disabled ? 'normal' : 'success'"
        :text="record.disabled ? t('admin.users.statusDisabled') : t('admin.users.statusActive')"
      />
    </template>
    <template #scopes="{ record }">
      <ScopeTags v-if="record.token" :scopes="record.token.scopes" />
      <span v-else class="text-desc">{{ t('botDetail.tokenNone') }}</span>
    </template>
    <template #expires="{ record }">
      <template v-if="record.token">
        <a-tag v-if="tokenState(record.token) === 'expired'" color="red" size="small">{{ t('accessToken.expired') }}</a-tag>
        <span v-else-if="!record.token.expiresAt" class="text-caption">{{ t('accessToken.never') }}</span>
        <a-space v-else :size="4" class="nowrap">
          <span>{{ dayjs(record.token.expiresAt).format('YYYY-MM-DD') }}</span>
          <a-tag v-if="tokenState(record.token) === 'expiring'" color="orange" size="small">{{ t('accessToken.expiringSoon') }}</a-tag>
        </a-space>
      </template>
      <span v-else class="text-desc">-</span>
    </template>
    <template #lastUsed="{ record }">
      <a-tooltip v-if="record.token?.lastUsedAt" :content="dayjs(record.token.lastUsedAt).format('YYYY-MM-DD HH:mm')">
        <span>{{ dayjs(record.token.lastUsedAt).fromNow() }}</span>
      </a-tooltip>
      <span v-else class="text-desc">{{ t('accessToken.neverUsed') }}</span>
    </template>
    <template #open>
      <a-button type="text" size="small">{{ t('botDetail.manage') }}</a-button>
    </template>
    <template #empty>
      <slot v-if="!loading" name="empty" />
    </template>
  </a-table>
</template>

<style scoped>
.bot-cell,
.owner-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.owner-cell {
  gap: 8px;
}
.bot-text {
  min-width: 0;
}
.bot-name {
  color: var(--text-title);
  font-weight: 500;
}
.nowrap {
  white-space: nowrap;
}
:deep(.bot-row) {
  cursor: pointer;
}
:deep(.arco-badge-status-text) {
  font-size: 14px;
}
:deep(.arco-table-th) {
  background: var(--bg-base);
}
</style>
