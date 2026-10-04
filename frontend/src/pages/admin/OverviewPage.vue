<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import {
  ChevronRight,
  Database,
  Layers,
  MonitorSmartphone,
  Table2,
  UserPlus,
  UsersRound,
} from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, type RouteLocationRaw } from 'vue-router'

import { adminApi, type AdminOverview, type AdminUser } from '@/api/admin'
import UserAvatar from '@/components/common/UserAvatar.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()

dayjs.extend(relativeTime)

const auth = useAuthStore()
const router = useRouter()
const data = ref<AdminOverview | null>(null)
const loading = ref(true)

interface Stat {
  label: string
  value: number
  note: string
  icon: unknown
  color: string
  to?: RouteLocationRaw
}

const stats = computed<Stat[]>(() => {
  const d = data.value
  if (!d) return []
  return [
    {
      label: t('admin.overview.members'),
      value: d.users.total,
      note: d.users.disabled
        ? t('admin.overview.membersDisabled', { n: d.users.disabled }, d.users.disabled)
        : t('admin.overview.membersAdmins', { n: d.users.admins }, d.users.admins),
      icon: UsersRound,
      color: '#3370ff',
      to: { name: 'admin-users' },
    },
    {
      label: t('admin.overview.newMembers'),
      value: d.users.newLast7Days,
      note: t('admin.overview.newMembersNote'),
      icon: UserPlus,
      color: '#14c0a7',
    },
    {
      label: t('admin.overview.bases'),
      value: d.projects,
      note: t('admin.overview.basesNote'),
      icon: Table2,
      color: '#7f3bf5',
      to: { name: 'admin-projects' },
    },
    { label: t('admin.overview.tables'), value: d.tables, note: t('admin.overview.tablesNote'), icon: Layers, color: '#ff8800' },
    { label: t('admin.overview.records'), value: d.records, note: t('admin.overview.recordsNote'), icon: Database, color: '#f5319d' },
    {
      label: t('admin.overview.sessions'),
      value: d.activeSessions,
      note: t('admin.overview.sessionsNote'),
      icon: MonitorSmartphone,
      color: '#00b2d6',
    },
  ]
})

const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return t('admin.overview.lateNight')
  if (h < 12) return t('admin.overview.morning')
  if (h < 18) return t('admin.overview.afternoon')
  return t('admin.overview.evening')
})

onMounted(async () => {
  try {
    data.value = await adminApi.overview()
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div>
    <PageHeader
      :title="t('admin.overview.greeting', { greeting, name: auth.user?.userName ?? '' })"
      :description="t('admin.overview.description')"
    />

    <a-spin :loading="loading" class="spin">
      <a-grid :cols="{ xs: 1, sm: 2, lg: 3 }" :col-gap="16" :row-gap="16">
        <a-grid-item v-for="s in stats" :key="s.label">
          <a-card
            class="stat-card"
            :class="{ link: s.to }"
            :hoverable="!!s.to"
            :body-style="{ padding: '20px' }"
            @click="s.to && router.push(s.to)"
          >
            <div class="stat">
              <a-avatar shape="square" :size="40" :style="{ color: s.color, background: `${s.color}1a` }">
                <component :is="s.icon" :size="20" />
              </a-avatar>
              <a-statistic :title="s.label" :value="s.value" show-group-separator class="stat-value">
                <template #extra>
                  <span class="text-desc">{{ s.note }}</span>
                </template>
              </a-statistic>
            </div>
          </a-card>
        </a-grid-item>
      </a-grid>

      <a-row v-if="data" :gutter="16" class="columns">
        <a-col :xs="24" :lg="16">
          <SettingsSection :title="t('admin.overview.recentMembers')" flush>
            <template #extra>
              <a-button type="text" size="small" @click="router.push({ name: 'admin-users' })">
                {{ t('admin.overview.viewAll') }} <ChevronRight :size="14" />
              </a-button>
            </template>
            <a-list :data="data.recentUsers" :bordered="false" class="recent">
              <template #item="{ item }">
                <a-list-item :key="(item as AdminUser).id">
                  <a-list-item-meta>
                    <template #avatar>
                      <UserAvatar :name="(item as AdminUser).userName" :color="(item as AdminUser).color" :avatar-url="(item as AdminUser).avatarUrl" :size="32" />
                    </template>
                    <template #title>
                      <a-space :size="6">
                        <span>{{ (item as AdminUser).userName }}</span>
                        <a-tag v-if="(item as AdminUser).isAdmin" color="arcoblue" size="small">{{ t('admin.users.statusAdmin') }}</a-tag>
                        <a-tag v-if="(item as AdminUser).disabled" size="small">{{ t('admin.users.statusDisabled') }}</a-tag>
                      </a-space>
                    </template>
                    <template #description>
                      <span class="text-desc">{{ (item as AdminUser).email }}</span>
                    </template>
                  </a-list-item-meta>
                  <template #extra>
                    <span class="text-desc">{{ dayjs((item as AdminUser).createdAt).fromNow() }}</span>
                  </template>
                </a-list-item>
              </template>
            </a-list>
          </SettingsSection>
        </a-col>
        <a-col :xs="24" :lg="8" class="side">
          <SettingsSection :title="t('admin.overview.systemInfo')">
            <a-descriptions :column="1" size="medium" class="info" :value-style="{ textAlign: 'right' }">
              <a-descriptions-item :label="t('admin.overview.signUp')">
                <a-tag :color="data.system.settings.allowSignUp ? 'green' : 'gray'" size="small">
                  {{ data.system.settings.allowSignUp ? t('admin.overview.signUpOpen') : t('admin.security.closed') }}
                </a-tag>
              </a-descriptions-item>
              <a-descriptions-item :label="t('admin.security.passwordMinLength')">{{
                t('admin.overview.characters', { n: data.system.settings.passwordMinLength })
              }}</a-descriptions-item>
              <a-descriptions-item :label="t('admin.security.sessionTTL')">{{
                t('admin.overview.days', { n: data.system.settings.sessionTTLDays }, data.system.settings.sessionTTLDays)
              }}</a-descriptions-item>
              <a-descriptions-item :label="t('admin.overview.version')">
                {{ data.system.buildCommit ? data.system.buildCommit.slice(0, 7) : t('admin.overview.devBuild') }}
              </a-descriptions-item>
              <a-descriptions-item label="Go">{{ data.system.goVersion }}</a-descriptions-item>
            </a-descriptions>
          </SettingsSection>
        </a-col>
      </a-row>
    </a-spin>
  </div>
</template>

<style scoped>
.spin {
  display: block;
}
.stat-card {
  border-color: var(--line-border);
  border-radius: 8px;
}
.stat-card.link {
  cursor: pointer;
}
.stat {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}
.stat-value :deep(.arco-statistic-title) {
  margin-bottom: 2px;
  font-size: 14px;
  color: var(--text-caption);
}
.stat-value :deep(.arco-statistic-value) {
  font-size: 26px;
  line-height: 34px;
  color: var(--text-title);
}
.stat-value :deep(.arco-statistic-extra) {
  margin-top: 2px;
}
.columns {
  margin-top: 16px;
}
.recent :deep(.arco-list-item) {
  align-items: center;
  padding: 12px 24px !important;
}
.recent :deep(.arco-list-item-meta) {
  padding: 0;
}
.recent :deep(.arco-list-item-meta-title) {
  margin-bottom: 0;
  color: var(--text-title);
}
.info :deep(.arco-descriptions-item-label) {
  color: var(--text-caption);
}
@media (max-width: 991px) {
  .side {
    margin-top: 16px;
  }
}
</style>
