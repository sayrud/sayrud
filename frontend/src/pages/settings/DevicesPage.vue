<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { Monitor, Smartphone, Tablet } from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, onMounted, ref } from 'vue'

import { accountApi, type UserSession } from '@/api/account'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { parseUserAgent } from '@/utils/userAgent'

dayjs.extend(relativeTime)

const DEVICE_ICONS = { desktop: Monitor, mobile: Smartphone, tablet: Tablet }

const sessions = ref<UserSession[]>([])
const loading = ref(true)

const items = computed(() =>
  sessions.value.map((s) => {
    const ua = parseUserAgent(s.userAgent)
    const known = ua.browser !== '未知浏览器' || ua.os !== '未知系统'
    // Shows the first token of the user agent when unrecognized, e.g. curl/8.7.1.
    const name = known ? `${ua.browser} · ${ua.os}` : s.userAgent.split(' ')[0] || '未知设备'
    return { ...s, ua, name }
  }),
)
type Item = (typeof items.value)[number]
const othersCount = computed(() => sessions.value.filter((s) => !s.current).length)

async function load() {
  loading.value = true
  try {
    sessions.value = await accountApi.sessions()
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

function revoke(s: Item) {
  Modal.warning({
    title: '下线该设备？',
    content: `${s.name}（${s.ip || '未知 IP'}）需要重新登录才能继续使用。`,
    hideCancel: false,
    okText: '下线',
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      try {
        await accountApi.revokeSession(s.id)
        Message.success('已下线')
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
      }
      load()
    },
  })
}

function revokeOthers() {
  Modal.warning({
    title: '退出其他全部设备？',
    content: `除当前设备外的 ${othersCount.value} 台设备需要重新登录。`,
    hideCancel: false,
    okText: '全部退出',
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      try {
        await accountApi.revokeOthers()
        Message.success('已退出其他设备')
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
      }
      load()
    },
  })
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader title="登录设备" description="查看登录过你账号的设备，发现异常时可及时下线" />

    <SettingsSection title="当前登录的设备" :description="`共 ${sessions.length} 台设备`" flush>
      <template #extra>
        <a-button :disabled="!othersCount" @click="revokeOthers">退出其他全部设备</a-button>
      </template>
      <a-list :data="items" :loading="loading" :bordered="false" class="devices">
        <template #item="{ item }">
          <a-list-item :key="item.id">
            <a-list-item-meta>
              <template #avatar>
                <a-avatar shape="square" :size="40" class="device-icon">
                  <component :is="DEVICE_ICONS[(item as Item).ua.device]" :size="20" />
                </a-avatar>
              </template>
              <template #title>
                <a-space :size="8">
                  <a-tooltip :content="(item as Item).userAgent || '未知'" position="tl">
                    <span>{{ (item as Item).name }}</span>
                  </a-tooltip>
                  <a-tag v-if="(item as Item).current" color="green" size="small">当前设备</a-tag>
                </a-space>
              </template>
              <template #description>
                <a-space :size="16" wrap class="text-desc">
                  <span>IP：{{ (item as Item).ip || '未知' }}</span>
                  <span>登录于 {{ dayjs((item as Item).createdAt).fromNow() }}</span>
                  <span>{{ dayjs((item as Item).expiresAt).format('YYYY-MM-DD') }} 到期</span>
                </a-space>
              </template>
            </a-list-item-meta>
            <template v-if="!(item as Item).current" #actions>
              <a-button type="text" status="danger" size="small" @click="revoke(item as Item)">下线</a-button>
            </template>
          </a-list-item>
        </template>
      </a-list>
    </SettingsSection>
  </div>
</template>

<style scoped>
.devices :deep(.arco-list-item) {
  align-items: center;
  padding: 14px 24px !important;
}
.devices :deep(.arco-list-item-meta) {
  padding: 0;
}
.devices :deep(.arco-list-item-meta-title) {
  margin-bottom: 2px;
  font-weight: 500;
  color: var(--text-title);
}
.device-icon {
  background: var(--bg-base);
  color: var(--text-caption);
}
@media (max-width: 768px) {
  .devices :deep(.arco-list-item) {
    padding: 12px 16px !important;
  }
}
</style>
