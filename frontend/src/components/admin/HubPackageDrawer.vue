<script setup lang="ts">
import { BadgeCheck, ExternalLink, House, RotateCw } from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { hubApi, type HubKind, type HubPackage, type HubPackageDetail } from '@/api/hub'
import HubPackageAction from '@/components/admin/HubPackageAction.vue'
import HubPackageIcon from '@/components/admin/HubPackageIcon.vue'
import MarkdownView from '@/components/common/MarkdownView.vue'
import StateIllustration from '@/components/common/StateIllustration.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import type { FieldType } from '@/types/bitable'
import { attachmentSize } from '@/utils/attachments'
import { fieldTypeInfo } from '@/utils/fieldTypes'
import { compactCount, hubAction } from '@/utils/hub'
import { LANGUAGE_NAMES, versionLabel } from '@/utils/workflowPlugin'

dayjs.extend(relativeTime)

/** Details of a package of the Sayrud Hub catalog: its readme, what its latest release asks for and what is new in it. */
const props = defineProps<{
  kind: HubKind
  /** Package to show, its listed summary is shown while the details load. */
  summary: HubPackage | null
  categoryLabel?: (category: string) => string
}>()
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ install: [pkg: HubPackage]; update: [pkg: HubPackage]; open: [pkg: HubPackage] }>()
const { t, locale } = useI18n()

const detail = ref<HubPackageDetail | null>(null)
const loading = ref(false)
const error = ref('')
const tab = ref('readme')
let requestId = 0

async function load() {
  const id = props.summary?.id
  if (!id) return
  const request = ++requestId
  loading.value = true
  error.value = ''
  try {
    const result = await hubApi.package(props.kind, id)
    if (request === requestId) detail.value = result
  } catch (e) {
    if (request === requestId) error.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (request === requestId) loading.value = false
  }
}

watch(
  () => [visible.value, props.summary?.id] as const,
  ([open, id], previous) => {
    if (!open || !id) return
    if (previous && previous[0] && previous[1] === id) return
    if (detail.value?.id !== id) detail.value = null
    tab.value = 'readme'
    void load()
  },
  { immediate: true },
)

// The listed summary tells the installation after an install, until the details are loaded again.
const pkg = computed<HubPackage | null>(() => {
  const d = detail.value
  if (d && d.id === props.summary?.id) return { ...d, installed: props.summary.installed }
  return props.summary
})
const latest = computed(() => (detail.value?.id === props.summary?.id ? detail.value?.latest : undefined))
const action = computed(() => (pkg.value ? hubAction(pkg.value, latest.value) : 'unavailable'))
const byline = computed(() => [pkg.value?.author, pkg.value?.publisher && t('hubCatalog.publishedBy', { name: pkg.value.publisher })].filter(Boolean).join(' · '))
const anyHost = computed(() => !!latest.value?.permissions.fetch.includes('*'))
const hosts = computed(() => latest.value?.permissions.fetch.filter((h) => h !== '*') ?? [])
const formatTime = (time?: string) => (time ? dayjs(time).format('YYYY-MM-DD HH:mm') : '—')

defineExpose({ reload: load })
</script>

<template>
  <a-drawer
    v-model:visible="visible"
    width="min(720px, 100vw)"
    :footer="false"
    unmount-on-close
    :title="t('hubCatalog.details')"
    class="hub-drawer"
  >
    <div v-if="pkg" class="body">
      <header class="head">
        <HubPackageIcon :pkg="pkg" :size="56" />
        <div class="head-text">
          <div class="name-row">
            <span class="name">{{ pkg.name }}</span>
            <a-tag v-if="pkg.official" size="small" color="arcoblue" class="official">
              <template #icon><BadgeCheck :size="12" /></template>
              {{ t('hub.official') }}
            </a-tag>
          </div>
          <div v-if="byline" class="text-desc byline">{{ byline }}</div>
          <div class="text-desc description">{{ pkg.description || t('pluginAdmin.noDescription') }}</div>
        </div>
      </header>

      <div class="actions">
        <HubPackageAction
          :pkg="pkg"
          :latest="latest ?? undefined"
          size="medium"
          @install="emit('install', pkg)"
          @update="emit('update', pkg)"
          @open="emit('open', pkg)"
        />
        <a-button :href="pkg.url" target="_blank" rel="noopener noreferrer">
          <template #icon><ExternalLink :size="14" /></template>
          {{ t('hub.viewOnHub') }}
        </a-button>
        <a-button v-if="pkg.homepage" :href="pkg.homepage" target="_blank" rel="noopener noreferrer nofollow">
          <template #icon><House :size="14" /></template>
          {{ t('pluginAdmin.homepage') }}
        </a-button>
      </div>

      <a-alert v-if="latest?.yanked" type="error" class="alert">
        {{ latest.yankReason ? t('hub.reason', { reason: latest.yankReason }) : t('hub.yankedHint') }}
      </a-alert>
      <a-alert v-if="action === 'incompatible'" type="warning" class="alert">{{ t('hubCatalog.incompatibleHint', { version: latest!.minAppVersion }) }}</a-alert>
      <a-alert v-else-if="action === 'conflict'" type="info" class="alert">{{ t('hubCatalog.conflictHint', { version: versionLabel(pkg.installed!.version) }) }}</a-alert>
      <a-alert v-else-if="action === 'update'" type="info" class="alert">
        {{ t('hubCatalog.updateHint', { from: versionLabel(pkg.installed!.version), to: versionLabel(pkg.latestVersion) }) }}
      </a-alert>

      <div class="stats">
        <div class="stat">
          <div class="stat-label">{{ t('pluginAdmin.version') }}</div>
          <div class="stat-value mono">{{ pkg.latestVersion ? versionLabel(pkg.latestVersion) : '—' }}</div>
        </div>
        <div class="stat">
          <div class="stat-label">{{ t('hubCatalog.downloadsLabel') }}</div>
          <div class="stat-value">{{ compactCount(pkg.downloads, locale) }}</div>
        </div>
        <div class="stat">
          <div class="stat-label">{{ t('hubCatalog.updated') }}</div>
          <div class="stat-value">{{ pkg.releasedAt ? dayjs(pkg.releasedAt).fromNow() : '—' }}</div>
        </div>
        <div class="stat">
          <template v-if="kind === 'workflow_plugin'">
            <div class="stat-label">{{ t('pluginAdmin.category') }}</div>
            <div class="stat-value">{{ (pkg.category && categoryLabel?.(pkg.category)) || '—' }}</div>
          </template>
          <template v-else>
            <div class="stat-label">{{ t('shortcutAdmin.resultType') }}</div>
            <div class="stat-value inline">
              <template v-if="latest?.resultType">
                <FieldTypeIcon :type="latest.resultType as FieldType" />{{ fieldTypeInfo(latest.resultType as FieldType).label }}
              </template>
              <template v-else>—</template>
            </div>
          </template>
        </div>
      </div>

      <div v-if="pkg.tags.length" class="tags">
        <a-tag v-for="tag in pkg.tags" :key="tag" size="small">{{ tag }}</a-tag>
      </div>

      <a-result v-if="error && !detail" status="error" :title="t('hubCatalog.detailsFailed')" :subtitle="error" class="state">
        <template #extra>
          <a-button type="primary" @click="load">
            <template #icon><RotateCw :size="15" /></template>
            {{ t('common.retry') }}
          </a-button>
        </template>
      </a-result>

      <a-skeleton v-else-if="loading && !detail" animation class="state">
        <a-skeleton-line :rows="6" :line-spacing="14" />
      </a-skeleton>

      <a-tabs v-else-if="detail" v-model:active-key="tab" :animation="false" class="tabs">
        <a-tab-pane key="readme" :title="t('hubCatalog.readme')">
          <MarkdownView v-if="detail.readme.trim()" :source="detail.readme" />
          <a-empty v-else :description="t('hubCatalog.noReadme')">
            <template #image><StateIllustration name="no-results" :width="120" /></template>
          </a-empty>
        </a-tab-pane>

        <a-tab-pane key="permissions" :title="t('pluginAdmin.permissions')">
          <a-descriptions v-if="latest" :column="1" size="medium" :label-style="{ width: '140px' }" :value-style="{ overflowWrap: 'anywhere' }">
            <a-descriptions-item :label="t('pluginAdmin.network')">
              <span v-if="anyHost" class="warning-text">{{ t('pluginAdmin.networkAny') }}</span>
              <a-space v-if="hosts.length" wrap :size="4" :class="{ 'after-any': anyHost }">
                <a-tag v-for="host in hosts" :key="host" size="small" class="mono">{{ host }}</a-tag>
              </a-space>
              <span v-if="!anyHost && !hosts.length" class="text-desc">{{ t('pluginAdmin.networkNone') }}</span>
            </a-descriptions-item>
            <a-descriptions-item v-if="kind === 'field_shortcut'" :label="t('shortcutAdmin.aiEnabled')">
              <a-tag size="small" :color="latest.permissions.ai ? 'purple' : undefined">
                {{ latest.permissions.ai ? t('shortcutAdmin.enabledTag') : t('shortcutAdmin.disabledTag') }}
              </a-tag>
            </a-descriptions-item>
            <a-descriptions-item :label="t('shortcutAdmin.credentials')">
              <a-space v-if="latest.permissions.credentials.length" wrap :size="4">
                <a-tag v-for="c in latest.permissions.credentials" :key="c" size="small">{{ c }}</a-tag>
              </a-space>
              <span v-else class="text-desc">{{ t('hubCatalog.noCredentials') }}</span>
            </a-descriptions-item>
            <a-descriptions-item :label="t('pluginAdmin.timeout')">{{ t('pluginAdmin.timeoutValue', { n: latest.timeout }) }}</a-descriptions-item>
            <a-descriptions-item v-if="kind === 'workflow_plugin'" :label="t('hubCatalog.preview')">
              {{ latest.hasPreview ? t('hubCatalog.previewYes') : t('hubCatalog.previewNo') }}
            </a-descriptions-item>
          </a-descriptions>
          <a-empty v-else :description="t('hubCatalog.unavailableHint')" />
          <p class="text-desc hint">{{ t('hubCatalog.permissionsHint') }}</p>
        </a-tab-pane>

        <a-tab-pane key="release" :title="t('hubCatalog.release')">
          <template v-if="latest">
            <a-descriptions :column="{ xs: 1, sm: 2 }" size="medium" layout="vertical" class="release">
              <a-descriptions-item :label="t('pluginAdmin.version')">
                <span class="inline">
                  <span class="mono">{{ versionLabel(latest.version) }}</span>
                  <a-tag v-if="latest.prerelease" size="small" color="orange">{{ t('hubCatalog.prerelease') }}</a-tag>
                </span>
              </a-descriptions-item>
              <a-descriptions-item :label="t('hubCatalog.publishedAt')">{{ formatTime(latest.publishedAt) }}</a-descriptions-item>
              <a-descriptions-item :label="t('hubCatalog.minAppVersion')">
                {{ latest.minAppVersion ? t('hubCatalog.minAppVersionValue', { version: latest.minAppVersion }) : t('hubCatalog.anyVersion') }}
              </a-descriptions-item>
              <a-descriptions-item :label="t('pluginAdmin.language')">{{ LANGUAGE_NAMES[latest.language] }} · {{ attachmentSize(latest.size) }}</a-descriptions-item>
            </a-descriptions>
            <div class="changelog-title">{{ t('hub.changelog', { version: versionLabel(latest.version) }) }}</div>
            <div v-if="latest.changelog.trim()" class="changelog">{{ latest.changelog }}</div>
            <div v-else class="text-desc">{{ t('hubCatalog.noChangelog') }}</div>
          </template>
          <a-empty v-else :description="t('hubCatalog.unavailableHint')" />
        </a-tab-pane>
      </a-tabs>
    </div>
  </a-drawer>
</template>

<style scoped>
.body {
  display: flex;
  flex-direction: column;
}
.head {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}
.head-text {
  flex: 1;
  min-width: 0;
}
.name-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.name {
  font-size: 20px;
  font-weight: 600;
  line-height: 28px;
  color: var(--text-title);
  overflow-wrap: anywhere;
}
.official :deep(.arco-tag-icon) {
  display: inline-flex;
}
.byline {
  margin-top: 2px;
  font-size: 13px;
}
.description {
  margin-top: 6px;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 20px;
}
.alert {
  margin-top: 16px;
}
.stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-top: 20px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  overflow: hidden;
}
.stat {
  min-width: 0;
  padding: 12px 16px;
}
.stat + .stat {
  border-left: 1px solid var(--line-border);
}
.stat-label {
  margin-bottom: 4px;
  font-size: 12px;
  color: var(--text-caption);
}
.stat-value {
  overflow: hidden;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-title);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.inline {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 16px;
}
.state {
  margin-top: 24px;
}
.tabs {
  margin-top: 16px;
}
.tabs :deep(.arco-tabs-content) {
  padding-top: 16px;
}
.warning-text {
  color: var(--color-warning);
}
.after-any {
  margin-top: 6px;
}
.hint {
  margin: 16px 0 0;
  font-size: 12px;
}
.release {
  margin-bottom: 8px;
}
.changelog-title {
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-title);
}
.changelog {
  padding: 12px 16px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  background: var(--bg-base);
  font-size: 13px;
  line-height: 1.7;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.mono {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
@media (max-width: 560px) {
  .stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .stat:nth-child(3) {
    border-left: none;
  }
  .stat:nth-child(n + 3) {
    border-top: 1px solid var(--line-border);
  }
}
</style>
