<script setup lang="ts">
import { BadgeCheck, Download, Tag, UserRound } from '@lucide/vue'
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

const MAX_TAGS = 2
</script>

<template>
  <a-card
    class="hub-card"
    role="button"
    tabindex="0"
    :aria-label="pkg.name"
    @click="emit('details')"
    @keydown.enter.self="emit('details')"
    @keydown.space.self.prevent="emit('details')"
  >
    <div class="head">
      <HubPackageIcon :pkg="pkg" :size="40" />
      <a-card-meta class="identity">
        <template #title>
          <div class="name-row">
            <span class="name ellipsis" :title="pkg.name">{{ pkg.name }}</span>
            <a-tooltip v-if="pkg.official" :content="t('hub.officialTip')">
              <BadgeCheck :size="15" class="official" :aria-label="t('hub.official')" />
            </a-tooltip>
          </div>
        </template>
      </a-card-meta>
      <HubPackageAction :pkg="pkg" size="small" class="actions" @install="emit('install')" @update="emit('update')" @open="emit('open')" />
    </div>

    <a-typography-paragraph class="description" :ellipsis="{ rows: 2 }" :title="pkg.description">
      {{ pkg.description || t('pluginAdmin.noDescription') }}
    </a-typography-paragraph>

    <a-space v-if="pkg.tags.length" class="tags" :size="[6, 6]" wrap>
      <a-tag v-for="tag in pkg.tags.slice(0, MAX_TAGS)" :key="tag" class="keyword-tag" size="small" bordered :title="tag">
        <span class="tag-label ellipsis">#{{ tag }}</span>
      </a-tag>
      <a-tooltip v-if="pkg.tags.length > MAX_TAGS" :content="pkg.tags.slice(MAX_TAGS).join(', ')">
        <a-tag size="small" class="keyword-tag" bordered>+{{ pkg.tags.length - MAX_TAGS }}</a-tag>
      </a-tooltip>
    </a-space>

    <div class="footer">
      <span v-if="pkg.latestVersion" class="version meta-item" :title="versionLabel(pkg.latestVersion)">
        <Tag :size="13" aria-hidden="true" />
        <span class="ellipsis">{{ versionLabel(pkg.latestVersion) }}</span>
      </span>
      <span v-if="pkg.author || pkg.publisher" class="publisher meta-item" :title="pkg.author || pkg.publisher">
        <UserRound :size="13" />
        <span class="ellipsis">{{ pkg.author || pkg.publisher }}</span>
      </span>
      <a-tooltip :content="t('hubCatalog.downloads', { n: pkg.downloads }, pkg.downloads)">
        <span class="downloads meta-item"><Download :size="13" />{{ compactCount(pkg.downloads, locale) }}</span>
      </a-tooltip>
    </div>
  </a-card>
</template>

<style scoped>
.hub-card {
  height: 100%;
  border-color: var(--line-border);
  border-radius: 12px;
  cursor: pointer;
  transition: border-color 0.2s;
}
.hub-card:hover {
  border-color: rgb(var(--primary-3));
}
.hub-card:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
.hub-card :deep(.arco-card-body) {
  display: flex;
  flex-direction: column;
  gap: 10px;
  height: 100%;
  padding: 16px 20px;
}
.head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.identity {
  flex: 1;
  min-width: 100px;
}
.publisher {
  flex: 1;
  min-width: 0;
}
.version {
  flex: none;
  max-width: 120px;
}
.name-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.name {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-title);
}
.official {
  flex: none;
  color: var(--color-primary);
}
.description {
  min-height: calc(13px * 1.5 * 2);
  margin: 0;
  color: var(--text-caption);
  font-size: 13px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.tag-label {
  max-width: 150px;
}
.tags :deep(.keyword-tag) {
  border-color: var(--line-border);
  background: transparent;
  color: var(--text-caption);
}
.footer {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: auto;
  padding-top: 10px;
  border-top: 1px solid var(--line-divider);
  color: var(--text-placeholder);
  font-size: 12px;
}
.actions {
  flex: none;
  align-self: flex-start;
  margin-left: auto;
}
.downloads {
  margin-left: auto;
}
@media (hover: hover) {
  .actions {
    visibility: hidden;
    opacity: 0;
    transition: opacity 0.15s, visibility 0.15s;
  }
  .hub-card:hover .actions,
  .hub-card:focus-within .actions {
    visibility: visible;
    opacity: 1;
  }
}
.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.meta-item svg {
  flex: none;
}
</style>
