<script setup lang="ts">
import { BadgeCheck, Download } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import type { HubPackage } from '@/api/hub'
import HubPackageAction from '@/components/admin/HubPackageAction.vue'
import HubPackageIcon from '@/components/admin/HubPackageIcon.vue'
import { compactCount } from '@/utils/hub'
import { versionLabel } from '@/utils/workflowPlugin'

/** A package of the Sayrud Hub catalog in the grid; the card opens its details. */
defineProps<{ pkg: HubPackage }>()
const emit = defineEmits<{ details: []; install: []; update: []; open: [] }>()
const { t, locale } = useI18n()

const MAX_TAGS = 3
</script>

<template>
  <a-card
    class="hub-card"
    :bordered="true"
    role="button"
    tabindex="0"
    :aria-label="pkg.name"
    @click="emit('details')"
    @keydown.enter.self="emit('details')"
  >
    <div class="layout">
      <HubPackageIcon :pkg="pkg" :size="48" />
      <div class="main">
        <div class="head">
          <div class="name-row">
            <span class="name ellipsis" :title="pkg.name">{{ pkg.name }}</span>
            <a-tooltip v-if="pkg.official" :content="t('hub.officialTip')">
              <BadgeCheck :size="15" class="official" :aria-label="t('hub.official')" />
            </a-tooltip>
          </div>
          <HubPackageAction :pkg="pkg" class="action" @install="emit('install')" @update="emit('update')" @open="emit('open')" />
        </div>

        <p class="description" :title="pkg.description">{{ pkg.description || t('pluginAdmin.noDescription') }}</p>

        <div class="meta text-desc">
          <span v-if="pkg.latestVersion" class="mono">{{ versionLabel(pkg.latestVersion) }}</span>
          <a-tooltip :content="t('hubCatalog.downloads', { n: pkg.downloads }, pkg.downloads)">
            <span class="meta-item"><Download :size="12" />{{ compactCount(pkg.downloads, locale) }}</span>
          </a-tooltip>
          <span v-if="pkg.tags.length" class="tags">
            <a-tag v-for="tag in pkg.tags.slice(0, MAX_TAGS)" :key="tag" size="small">{{ tag }}</a-tag>
            <a-tag v-if="pkg.tags.length > MAX_TAGS" size="small">+{{ pkg.tags.length - MAX_TAGS }}</a-tag>
          </span>
        </div>
      </div>
    </div>
  </a-card>
</template>

<style scoped>
.hub-card {
  height: 100%;
  border-color: var(--line-border);
  border-radius: 8px;
  cursor: pointer;
  transition: border-color 0.15s;
}
.hub-card:hover {
  border-color: var(--line-border-strong);
}
.hub-card:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
.hub-card :deep(.arco-card-body) {
  height: 100%;
  padding: 20px;
}
.layout {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  height: 100%;
}
.main {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  height: 100%;
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 28px;
}
.action {
  visibility: hidden;
  opacity: 0;
  transition: opacity 0.15s, visibility 0.15s;
}
.hub-card:hover .action,
.hub-card:focus-within .action {
  visibility: visible;
  opacity: 1;
}
/* Touch screens have no hover, the button stays shown. */
@media (hover: none) {
  .action {
    visibility: visible;
    opacity: 1;
  }
}
.name-row {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}
.name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-title);
}
.official {
  flex: none;
  color: var(--color-primary);
}
.description {
  display: -webkit-box;
  min-height: calc(13px * 1.6 * 2);
  margin: 6px 0 0;
  overflow: hidden;
  color: var(--text-caption);
  font-size: 13px;
  line-height: 1.6;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
}
.meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 12px;
  margin-top: auto;
  padding-top: 12px;
  font-size: 12px;
}
.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}
.tags {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 4px;
}
.mono {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
</style>
