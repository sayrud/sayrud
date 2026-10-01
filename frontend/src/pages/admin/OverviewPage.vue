<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import {
  ChevronRight,
  Database,
  Layers,
  LockKeyhole,
  MonitorSmartphone,
  Table2,
  UserPlus,
  UsersRound,
} from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, onMounted, ref } from 'vue'
import { useRouter, type RouteLocationRaw } from 'vue-router'

import { adminApi, type AdminOverview, type AdminUser } from '@/api/admin'
import UserAvatar from '@/components/common/UserAvatar.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useAuthStore } from '@/stores/auth'

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
      label: '成员',
      value: d.users.total,
      note: d.users.disabled ? `其中 ${d.users.disabled} 人已停用` : `${d.users.admins} 位管理员`,
      icon: UsersRound,
      color: '#3370ff',
      to: { name: 'admin-users' },
    },
    { label: '近 7 天新增', value: d.users.newLast7Days, note: '新注册或新建的成员', icon: UserPlus, color: '#14c0a7' },
    {
      label: '多维表格',
      value: d.projects,
      note: '全站多维表格总数',
      icon: Table2,
      color: '#7f3bf5',
      to: { name: 'admin-projects' },
    },
    { label: '数据表', value: d.tables, note: '所有多维表格中的数据表', icon: Layers, color: '#ff8800' },
    { label: '记录', value: d.records, note: '所有数据表中的记录', icon: Database, color: '#f5319d' },
    { label: '有效会话', value: d.activeSessions, note: '未过期的登录会话', icon: MonitorSmartphone, color: '#00b2d6' },
  ]
})

const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '夜深了'
  if (h < 12) return '上午好'
  if (h < 18) return '下午好'
  return '晚上好'
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
    <PageHeader :title="`${greeting}，${auth.user?.userName ?? ''}`" description="这里是系统的整体情况" />

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
          <SettingsSection title="最近加入的成员" flush>
            <template #extra>
              <a-button type="text" size="small" @click="router.push({ name: 'admin-users' })">
                查看全部 <ChevronRight :size="14" />
              </a-button>
            </template>
            <a-list :data="data.recentUsers" :bordered="false" class="recent">
              <template #item="{ item }">
                <a-list-item :key="(item as AdminUser).id">
                  <a-list-item-meta>
                    <template #avatar>
                      <UserAvatar :name="(item as AdminUser).userName" :color="(item as AdminUser).color" :size="32" />
                    </template>
                    <template #title>
                      <a-space :size="6">
                        <span>{{ (item as AdminUser).userName }}</span>
                        <a-tag v-if="(item as AdminUser).isAdmin" color="arcoblue" size="small">管理员</a-tag>
                        <a-tag v-if="(item as AdminUser).disabled" size="small">已停用</a-tag>
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
          <SettingsSection title="系统信息">
            <a-descriptions :column="1" size="medium" class="info" :value-style="{ textAlign: 'right' }">
              <a-descriptions-item label="注册">
                <a-tag :color="data.system.settings.allowSignUp ? 'green' : 'gray'" size="small">
                  {{ data.system.settings.allowSignUp ? '开放注册' : '已关闭' }}
                </a-tag>
              </a-descriptions-item>
              <a-descriptions-item label="密码最小长度">{{ data.system.settings.passwordMinLength }} 位</a-descriptions-item>
              <a-descriptions-item label="登录有效期">{{ data.system.settings.sessionTTLDays }} 天</a-descriptions-item>
              <a-descriptions-item label="版本">
                {{ data.system.buildCommit ? data.system.buildCommit.slice(0, 7) : '开发版本' }}
              </a-descriptions-item>
              <a-descriptions-item label="Go">{{ data.system.goVersion }}</a-descriptions-item>
            </a-descriptions>
          </SettingsSection>
          <SettingsSection title="快捷入口">
            <a-space direction="vertical" fill class="shortcuts">
              <a-button long @click="router.push({ name: 'admin-users' })">
                <template #icon><UserPlus :size="15" /></template>
                新建成员
              </a-button>
              <a-button long @click="router.push({ name: 'admin-security' })">
                <template #icon><LockKeyhole :size="15" /></template>
                安全与注册
              </a-button>
            </a-space>
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
.side {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.side :deep(.settings-section + .settings-section) {
  margin-top: 0;
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
.shortcuts {
  padding: 8px 0 16px;
}
@media (max-width: 991px) {
  .side {
    margin-top: 16px;
  }
}
</style>
