<script setup lang="ts">
import { Check } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { type AppLocale, LOCALE_OPTIONS } from '@/i18n'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'
import { THEME_OPTIONS, type ThemeMode } from '@/utils/theme'

const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const { t } = useI18n()
</script>

<template>
  <div>
    <PageHeader :title="t('settings.preferences.title')" :description="t('settings.preferences.description')" />

    <SettingsSection :title="t('settings.preferences.appearance')" :description="t('settings.preferences.appearanceDescription')">
      <a-radio-group
        :model-value="themeStore.mode"
        class="themes"
        @change="(v) => themeStore.setMode(v as ThemeMode)"
      >
        <a-radio v-for="o in THEME_OPTIONS" :key="o.value" :value="o.value">
          <template #radio="{ checked }">
            <div class="theme-card" :class="{ selected: checked }">
              <div class="preview" :class="`preview-${o.value}`">
                <div class="mock mock-light">
                  <span class="mock-bar" />
                  <span class="mock-line" />
                  <span class="mock-line short" />
                </div>
                <div class="mock mock-dark">
                  <span class="mock-bar" />
                  <span class="mock-line" />
                  <span class="mock-line short" />
                </div>
                <span v-if="checked" class="check"><Check :size="12" :stroke-width="3" /></span>
              </div>
              <div class="theme-label">{{ o.label }}</div>
            </div>
          </template>
        </a-radio>
      </a-radio-group>
    </SettingsSection>

    <SettingsSection :title="t('common.language')" :description="t('settings.preferences.languageDescription')">
      <a-select
        class="locales"
        :model-value="localeStore.locale"
        @change="(v) => localeStore.setLocale(v as AppLocale)"
      >
        <a-option v-for="o in LOCALE_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</a-option>
      </a-select>
    </SettingsSection>
  </div>
</template>

<style scoped>
.themes {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  padding: 8px 0 16px;
}
.locales {
  width: 240px;
  margin: 8px 0 16px;
}
.themes :deep(.arco-radio) {
  margin: 0;
  padding: 0;
}
.theme-card {
  cursor: pointer;
  text-align: center;
}
.preview {
  position: relative;
  display: flex;
  width: 168px;
  height: 108px;
  overflow: hidden;
  border: 2px solid var(--line-border);
  border-radius: 10px;
  transition: border-color 0.15s;
}
.theme-card:hover .preview {
  border-color: var(--line-border-strong);
}
.theme-card.selected .preview {
  border-color: var(--color-primary);
}
.mock {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
}
.mock-light {
  background: #f5f6f7;
}
.mock-dark {
  background: #1e1f22;
}
.preview-light .mock-light,
.preview-dark .mock-dark,
.preview-system .mock {
  flex: 1;
}
.preview-light .mock-dark,
.preview-dark .mock-light {
  display: none;
}
.mock-bar {
  width: 60%;
  height: 14px;
  border-radius: 4px;
  background: #3370ff;
}
.mock-line {
  height: 8px;
  border-radius: 4px;
}
.mock-line.short {
  width: 70%;
}
.mock-light .mock-line {
  background: #dee0e3;
}
.mock-dark .mock-line {
  background: #36373b;
}
.check {
  position: absolute;
  right: 6px;
  top: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: var(--color-primary);
  color: #fff;
}
.theme-label {
  margin-top: 8px;
  color: var(--text-title);
}
.theme-card.selected .theme-label {
  color: var(--color-primary);
  font-weight: 500;
}
</style>
