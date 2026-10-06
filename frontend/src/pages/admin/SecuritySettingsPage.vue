<script setup lang="ts">
import PageHeader from '@/components/console/PageHeader.vue'
import SaveBar from '@/components/console/SaveBar.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useSystemSettings } from '@/composables/useSystemSettings'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const { draft, loading, saving, dirty, save, reset } = useSystemSettings()
</script>

<template>
  <div>
    <PageHeader :title="t('consoleNav.adminSecurity')" :description="t('admin.security.description')" />

    <a-spin :loading="loading" class="spin">
      <SettingsSection :title="t('admin.overview.signUp')">
        <SettingRow :label="t('admin.security.allowSignUp')" :description="t('admin.security.allowSignUpDescription')">
          <template #action>
            <a-tag :color="draft.allowSignUp ? 'green' : 'gray'" size="small">
              {{ draft.allowSignUp ? t('admin.security.open') : t('admin.security.closed') }}
            </a-tag>
            <a-switch v-model="draft.allowSignUp" />
          </template>
        </SettingRow>
      </SettingsSection>

      <SettingsSection :title="t('admin.security.passwordPolicy')">
        <SettingRow :label="t('admin.security.passwordMinLength')" :description="t('admin.security.passwordMinLengthDescription')">
          <template #action>
            <a-input-number v-model="draft.passwordMinLength" :min="8" :max="64" :step="1" mode="button" class="num">
              <template #suffix>{{ t('admin.security.charactersUnit') }}</template>
            </a-input-number>
          </template>
        </SettingRow>
      </SettingsSection>

      <SettingsSection :title="t('admin.security.sessions')">
        <SettingRow :label="t('admin.security.sessionTTL')" :description="t('admin.security.sessionTTLDescription')">
          <template #action>
            <a-input-number v-model="draft.sessionTTLDays" :min="1" :max="365" :step="1" mode="button" class="num">
              <template #suffix>{{ t('admin.security.daysUnit') }}</template>
            </a-input-number>
          </template>
        </SettingRow>
      </SettingsSection>

      <SettingsSection :title="t('admin.security.networkRequests')">
        <SettingRow :label="t('admin.security.networkAllowlist')" :description="t('admin.security.networkAllowlistDescription')">
          <a-input-tag
            v-model="draft.networkAllowlist"
            :aria-label="t('admin.security.networkAllowlist')"
            allow-clear
            unique-value
            class="network-input"
          />
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
  width: 200px;
}
.network-input {
  max-width: 480px;
}
</style>
