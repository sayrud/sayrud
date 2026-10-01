<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { Monitor, Smartphone, Tablet } from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { accountApi, type UserSession } from '@/api/account'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { parseUserAgent } from '@/utils/userAgent'

const { t } = useI18n()

dayjs.extend(relativeTime)

const DEVICE_ICONS = { desktop: Monitor, mobile: Smartphone, tablet: Tablet }

const sessions = ref<UserSession[]>([])
const loading = ref(true)

const items = computed(() =>
  sessions.value.map((s) => {
    const ua = parseUserAgent(s.userAgent)
    const known = ua.browser !== t('userAgent.unknownBrowser') || ua.os !== t('userAgent.unknownOS')
    // Shows the first token of the user agent when unrecognized, e.g. curl/8.7.1.
    const name = known ? `${ua.browser} · ${ua.os}` : s.userAgent.split(' ')[0] || t('settings.devices.unknownDevice')
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
    title: t('settings.devices.revokeTitle'),
    content: t('settings.devices.revokeContent', { name: s.name, ip: s.ip || t('settings.devices.unknownIP') }),
    hideCancel: false,
    okText: t('settings.devices.revoke'),
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      try {
        await accountApi.revokeSession(s.id)
        Message.success(t('settings.devices.revoked'))
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
      }
      load()
    },
  })
}

function revokeOthers() {
  Modal.warning({
    title: t('settings.devices.revokeOthersTitle'),
    content: t('settings.devices.revokeOthersContent', { n: othersCount.value }, othersCount.value),
    hideCancel: false,
    okText: t('settings.devices.revokeAll'),
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      try {
        await accountApi.revokeOthers()
        Message.success(t('settings.devices.othersRevoked'))
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
    <PageHeader :title="t('settings.devices.title')" :description="t('settings.devices.description')" />

    <SettingsSection :title="t('settings.devices.current')" :description="t('settings.devices.total', { n: sessions.length }, sessions.length)" flush>
      <template #extra>
        <a-button :disabled="!othersCount" @click="revokeOthers">{{ t('settings.devices.revokeOthers') }}</a-button>
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
                  <a-tooltip :content="(item as Item).userAgent || t('common.unknown')" position="tl">
                    <span>{{ (item as Item).name }}</span>
                  </a-tooltip>
                  <a-tag v-if="(item as Item).current" color="green" size="small">{{ t('settings.devices.currentDevice') }}</a-tag>
                </a-space>
              </template>
              <template #description>
                <a-space :size="16" wrap class="text-desc">
                  <span>{{ t('settings.devices.ip', { ip: (item as Item).ip || t('common.unknown') }) }}</span>
                  <span>{{ t('settings.devices.signedIn', { time: dayjs((item as Item).createdAt).fromNow() }) }}</span>
                  <span>{{ t('settings.devices.expires', { date: dayjs((item as Item).expiresAt).format('YYYY-MM-DD') }) }}</span>
                </a-space>
              </template>
            </a-list-item-meta>
            <template v-if="!(item as Item).current" #actions>
              <a-button type="text" status="danger" size="small" @click="revoke(item as Item)">{{ t('settings.devices.revoke') }}</a-button>
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
