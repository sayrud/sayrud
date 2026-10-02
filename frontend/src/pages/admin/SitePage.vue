<script setup lang="ts">
import { X } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import logo from '@/assets/logo-avatar.png'
import PageHeader from '@/components/console/PageHeader.vue'
import SaveBar from '@/components/console/SaveBar.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useSystemSettings } from '@/composables/useSystemSettings'

const { t } = useI18n()

const { draft, loading, saving, dirty, save, reset } = useSystemSettings()
const origin = window.location.origin
</script>

<template>
  <div>
    <PageHeader :title="t('consoleNav.site')" :description="t('admin.site.description')" />

    <a-spin :loading="loading" class="spin">
      <SettingsSection :title="t('settings.profile.basicInfo')">
        <SettingRow :label="t('admin.site.siteName')" :description="t('admin.site.siteNameDescription')">
          <a-input v-model="draft.siteName" :max-length="32" show-word-limit placeholder="Sayrud" class="name-input" />
        </SettingRow>
        <SettingRow :label="t('sso.site.externalURL')" :description="t('sso.site.externalURLDescription')">
          <div class="url-row">
            <a-input v-model="draft.externalURL" :placeholder="origin" class="name-input" allow-clear />
            <a-button v-if="draft.externalURL !== origin" size="small" type="text" @click="draft.externalURL = origin">
              {{ t('sso.site.useCurrent') }}
            </a-button>
          </div>
        </SettingRow>
        <SettingRow :label="t('admin.site.preview')">
          <div class="tab-preview">
            <div class="tab">
              <img :src="logo" alt="" class="tab-icon" />
              <span class="ellipsis">{{ t('consoleNav.users') }} · {{ draft.siteName.trim() || 'Sayrud' }}</span>
              <X :size="12" class="tab-close" />
            </div>
            <div class="tab-bar" />
          </div>
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
.name-input {
  max-width: 360px;
}
.url-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.tab-preview {
  width: 280px;
  padding: 8px 8px 0;
  border-radius: 8px 8px 0 0;
  background: var(--bg-base);
  border: 1px solid var(--line-border);
  border-bottom: none;
}
.tab {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 220px;
  padding: 8px 10px;
  border-radius: 8px 8px 0 0;
  background: var(--bg-body);
  font-size: 12px;
  color: var(--text-title);
}
.tab-icon {
  width: 14px;
  height: 14px;
  border-radius: 3px;
}
.tab-close {
  flex: none;
  margin-left: auto;
  color: var(--text-placeholder);
}
.tab-bar {
  height: 6px;
  margin: 0 -8px;
  background: var(--bg-body);
}
</style>
