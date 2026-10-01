<script setup lang="ts">
import { X } from '@lucide/vue'

import logo from '@/assets/logo-avatar.png'
import PageHeader from '@/components/console/PageHeader.vue'
import SaveBar from '@/components/console/SaveBar.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useSystemSettings } from '@/composables/useSystemSettings'

const { draft, loading, saving, dirty, save, reset } = useSystemSettings()
</script>

<template>
  <div>
    <PageHeader title="站点信息" description="设置站点的基础信息" />

    <a-spin :loading="loading" class="spin">
      <SettingsSection title="基础信息">
        <SettingRow label="站点名称" description="显示在网页标题和登录页">
          <a-input v-model="draft.siteName" :max-length="32" show-word-limit placeholder="Sayrud" class="name-input" />
        </SettingRow>
        <SettingRow label="预览">
          <div class="tab-preview">
            <div class="tab">
              <img :src="logo" alt="" class="tab-icon" />
              <span class="ellipsis">成员管理 · {{ draft.siteName.trim() || 'Sayrud' }}</span>
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
