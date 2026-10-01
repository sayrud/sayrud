<script setup lang="ts">
import PageHeader from '@/components/console/PageHeader.vue'
import SaveBar from '@/components/console/SaveBar.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useSystemSettings } from '@/composables/useSystemSettings'

const { draft, loading, saving, dirty, save, reset } = useSystemSettings()
</script>

<template>
  <div>
    <PageHeader title="安全与注册" description="控制谁可以加入，以及账号密码与登录的安全策略" />

    <a-spin :loading="loading" class="spin">
      <SettingsSection title="注册">
        <SettingRow label="允许自行注册" description="关闭后只能由管理员在「成员管理」中新建成员">
          <template #action>
            <a-tag :color="draft.allowSignUp ? 'green' : 'gray'" size="small">
              {{ draft.allowSignUp ? '开放' : '已关闭' }}
            </a-tag>
            <a-switch v-model="draft.allowSignUp" />
          </template>
        </SettingRow>
      </SettingsSection>

      <SettingsSection title="密码策略">
        <SettingRow label="密码最小长度" description="对之后的注册、修改密码、新建成员和重置密码生效">
          <template #action>
            <a-input-number v-model="draft.passwordMinLength" :min="8" :max="64" :step="1" mode="button" class="num">
              <template #suffix>位</template>
            </a-input-number>
          </template>
        </SettingRow>
      </SettingsSection>

      <SettingsSection title="登录会话">
        <SettingRow label="登录有效期" description="超过有效期需重新登录，仅对之后的新登录生效">
          <template #action>
            <a-input-number v-model="draft.sessionTTLDays" :min="1" :max="365" :step="1" mode="button" class="num">
              <template #suffix>天</template>
            </a-input-number>
          </template>
        </SettingRow>
      </SettingsSection>

      <SaveBar :dirty="dirty" :saving="saving" @save="save" @reset="reset" />
    </a-spin>
  </div>
</template>

<style scoped>
.spin {
  display: block;
}
.num {
  width: 160px;
}
</style>
