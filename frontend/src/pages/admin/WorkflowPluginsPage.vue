<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { CircleArrowUp, Ellipsis, Eye, Globe, KeyRound, Plus, Search, Trash2, Workflow, Zap } from '@lucide/vue'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { workflowPluginsApi, type WorkflowPlugin } from '@/api/workflowPlugin'
import WorkflowPluginIcon from '@/components/admin/WorkflowPluginIcon.vue'
import WorkflowPluginInstallModal from '@/components/admin/WorkflowPluginInstallModal.vue'
import StateIllustration from '@/components/common/StateIllustration.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import type { ModuleCategory, ModuleDef } from '@/types/workflow'
import { dslText } from '@/utils/modules'
import { LANGUAGE_NAMES, versionLabel } from '@/utils/workflowPlugin'

const { t } = useI18n()
const router = useRouter()

const CATEGORY_LABELS: Record<ModuleCategory, () => string> = {
  data: () => t('workflow.picker.data'),
  logic: () => t('workflow.picker.logic'),
  integration: () => t('workflow.picker.integration'),
}

const plugins = ref<WorkflowPlugin[]>([])
const builtins = ref<ModuleDef[]>([])
const loading = ref(true)
const toggling = ref('')
const view = ref<'installed' | 'builtin'>('installed')
const keyword = ref('')

const installVisible = ref(false)
const installWithCode = ref(false)
const upgradeTarget = ref<WorkflowPlugin | null>(null)


async function load() {
  loading.value = true
  try {
    const resp = await workflowPluginsApi.list()
    plugins.value = resp.plugins
    builtins.value = resp.builtins
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}
onMounted(load)

function matches(m: ModuleDef, extra = '') {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return true
  return [dslText(m.label), dslText(m.description), m.type, extra].some((s) => s.toLowerCase().includes(k))
}

const shownPlugins = computed(() => plugins.value.filter((p) => matches(p.module, p.module.plugin?.author)))
const shownBuiltins = computed(() => builtins.value.filter((m) => matches(m)))
const enabledCount = computed(() => plugins.value.filter((p) => p.enabled).length)

function openInstall(withCode = false) {
  upgradeTarget.value = null
  installWithCode.value = withCode
  installVisible.value = true
}

function openUpgrade(p: WorkflowPlugin) {
  upgradeTarget.value = p
  installWithCode.value = false
  installVisible.value = true
}

function openPlugin(p: WorkflowPlugin) {
  router.push({ name: 'admin-workflow-plugin', params: { pluginID: p.id } })
}

function openBuiltin(m: ModuleDef) {
  router.push({ name: 'admin-workflow-builtin', params: { type: m.type } })
}

function replace(saved: WorkflowPlugin) {
  const i = plugins.value.findIndex((p) => p.id === saved.id)
  if (i >= 0) plugins.value.splice(i, 1, saved)
  else plugins.value.push(saved)
}

function onInstalled(saved: WorkflowPlugin) {
  view.value = 'installed'
  replace(saved)
}

function onUninstalled(id: string) {
  plugins.value = plugins.value.filter((p) => p.id !== id)
}

async function toggle(p: WorkflowPlugin, enabled: boolean) {
  toggling.value = p.id
  try {
    replace(await workflowPluginsApi.setEnabled(p.id, enabled))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    toggling.value = ''
  }
}

function uninstall(p: WorkflowPlugin) {
  Modal.confirm({
    title: t('pluginAdmin.uninstallTitle', { name: p.module.label }),
    content: p.usedBy ? t('pluginAdmin.uninstallInUse', { n: p.usedBy }, p.usedBy) : t('pluginAdmin.uninstallContent'),
    okButtonProps: { status: 'danger' },
    okText: t('pluginAdmin.uninstall'),
    onBeforeOk: async () => {
      try {
        await workflowPluginsApi.uninstall(p.id)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      onUninstalled(p.id)
      Message.success(t('pluginAdmin.uninstalled'))
      return true
    },
  })
}
</script>

<template>
  <div>
    <PageHeader :title="t('pluginAdmin.title')" :description="t('pluginAdmin.pageDescription')">
      <template #extra>
        <a-button type="primary" @click="openInstall()">
          <template #icon><Plus :size="16" /></template>
          {{ t('pluginAdmin.install') }}
        </a-button>
      </template>
    </PageHeader>

    <div class="toolbar">
      <a-radio-group v-model="view" type="button">
        <a-radio value="installed">
          {{ t('pluginAdmin.installed') }}
          <span class="count">{{ plugins.length }}</span>
        </a-radio>
        <a-radio value="builtin">
          {{ t('pluginAdmin.builtin') }}
          <span class="count">{{ builtins.length }}</span>
        </a-radio>
      </a-radio-group>
      <span v-if="view === 'installed' && plugins.length" class="text-desc summary">
        {{ t('pluginAdmin.enabledSummary', { n: enabledCount, total: plugins.length }) }}
      </span>
      <a-input v-model="keyword" allow-clear :placeholder="t('pluginAdmin.search')" class="search">
        <template #prefix><Search :size="15" /></template>
      </a-input>
    </div>

    <a-spin :loading="loading" class="spin">
      <template v-if="view === 'installed'">
        <div v-if="shownPlugins.length" class="grid">
          <article
            v-for="p in shownPlugins"
            :key="p.id"
            class="card"
            :class="{ disabled: !p.enabled }"
            tabindex="0"
            @click="openPlugin(p)"
            @keydown.enter.self="openPlugin(p)"
          >
            <div class="card-body">
              <div class="card-head">
                <WorkflowPluginIcon :icon="p.module.icon" :color="p.module.color" :muted="!p.enabled" />
                <div class="card-title">
                  <div class="name ellipsis">{{ p.module.label }}</div>
                  <div class="meta ellipsis">
                    {{ versionLabel(p.version) }}<template v-if="p.language === 'typescript'"> · {{ LANGUAGE_NAMES.typescript }}</template
                    ><template v-if="p.module.plugin?.author"> · {{ p.module.plugin.author }}</template>
                  </div>
                </div>
                <span class="card-aside" @click.stop>
                  <a-switch
                    size="small"
                    :model-value="p.enabled"
                    :loading="toggling === p.id"
                    :disabled="!!toggling && toggling !== p.id"
                    :aria-label="t('pluginAdmin.enabledTag')"
                    @change="(v) => toggle(p, !!v)"
                  />
                </span>
              </div>
              <p class="desc">{{ p.module.description || t('pluginAdmin.noDescription') }}</p>
            </div>
            <div class="card-foot">
              <div class="chips">
                <span class="category">{{ CATEGORY_LABELS[p.module.category]() }}</span>
                <a-tooltip v-if="p.module.action" :content="t('pluginAdmin.actionTag')">
                  <span class="chip orange"><Zap :size="12" /></span>
                </a-tooltip>
                <a-tooltip
                  v-if="p.domains.length"
                  :content="p.domains.includes('*') ? t('pluginAdmin.anyHostTag') : t('pluginAdmin.hostsTag', { n: p.domains.length }, p.domains.length)"
                >
                  <span class="chip blue"><Globe :size="12" /></span>
                </a-tooltip>
                <a-tooltip v-if="p.credential" :content="t('pluginAdmin.credentialTag')">
                  <span class="chip purple"><KeyRound :size="12" /></span>
                </a-tooltip>
              </div>
              <a-tooltip :content="p.usedBy ? t('pluginAdmin.usedBy', { n: p.usedBy }, p.usedBy) : t('pluginAdmin.notUsed')">
                <span class="usage" :class="{ active: p.usedBy }"><Workflow :size="13" />{{ p.usedBy }}</span>
              </a-tooltip>
              <span class="more" @click.stop>
                <a-dropdown trigger="click" position="br">
                  <a-button type="text" size="mini" shape="circle" :aria-label="t('common.actions')">
                    <template #icon><Ellipsis :size="15" /></template>
                  </a-button>
                  <template #content>
                    <a-doption @click="openPlugin(p)">
                      <template #icon><Eye :size="15" /></template>
                      {{ t('pluginAdmin.details') }}
                    </a-doption>
                    <a-doption @click="openUpgrade(p)">
                      <template #icon><CircleArrowUp :size="15" /></template>
                      {{ t('pluginAdmin.upgrade') }}
                    </a-doption>
                    <a-doption class="danger-option" @click="uninstall(p)">
                      <template #icon><Trash2 :size="15" /></template>
                      {{ t('pluginAdmin.uninstall') }}
                    </a-doption>
                  </template>
                </a-dropdown>
              </span>
            </div>
          </article>
        </div>

        <div v-else-if="!loading && keyword.trim() && plugins.length" class="empty">
          <StateIllustration name="no-results" :width="140" />
          <div class="text-desc">{{ t('pluginAdmin.noMatch') }}</div>
        </div>

        <div v-else-if="!loading" class="empty empty-card">
          <StateIllustration name="feature-upgrade" :width="160" />
          <div class="empty-title">{{ t('pluginAdmin.empty') }}</div>
          <div class="text-desc empty-desc">{{ t('pluginAdmin.emptyDescription') }}</div>
          <a-space>
            <a-button type="primary" @click="openInstall()">
              <template #icon><Plus :size="16" /></template>
              {{ t('pluginAdmin.install') }}
            </a-button>
            <a-button @click="openInstall(true)">{{ t('pluginAdmin.startFromTemplate') }}</a-button>
          </a-space>
        </div>
      </template>

      <template v-else>
        <div v-if="shownBuiltins.length" class="grid">
          <article v-for="m in shownBuiltins" :key="m.type" class="card" tabindex="0" @click="openBuiltin(m)" @keydown.enter.self="openBuiltin(m)">
            <div class="card-body">
              <div class="card-head">
                <WorkflowPluginIcon :icon="m.icon" :color="m.color" />
                <div class="card-title">
                  <div class="name ellipsis">{{ dslText(m.label) }}</div>
                  <div class="meta ellipsis mono">{{ m.type }}</div>
                </div>
                <span class="card-aside builtin-badge">{{ t('pluginAdmin.builtinTag') }}</span>
              </div>
              <p class="desc">{{ dslText(m.description) }}</p>
            </div>
            <div class="card-foot">
              <div class="chips">
                <span class="category">{{ CATEGORY_LABELS[m.category]() }}</span>
                <a-tooltip v-if="m.action" :content="t('pluginAdmin.actionTag')">
                  <span class="chip orange"><Zap :size="12" /></span>
                </a-tooltip>
              </div>
            </div>
          </article>
        </div>
        <div v-else-if="!loading" class="empty">
          <StateIllustration name="no-results" :width="140" />
          <div class="text-desc">{{ t('pluginAdmin.noMatch') }}</div>
        </div>
      </template>
    </a-spin>

    <WorkflowPluginInstallModal v-model:visible="installVisible" :upgrade="upgradeTarget" :start-with-code="installWithCode" @installed="onInstalled" />
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}
.count {
  margin-left: 6px;
  padding: 0 6px;
  border-radius: 8px;
  background: var(--fill-hover);
  color: var(--text-caption);
  font-size: 12px;
}
.summary {
  font-size: 13px;
}
.search {
  width: 260px;
  margin-left: auto;
}
.spin {
  display: block;
  min-height: 200px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}
.card {
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--line-border);
  border-radius: 12px;
  background: var(--bg-body);
  cursor: pointer;
  transition:
    border-color 0.2s,
    box-shadow 0.2s,
    transform 0.2s;
}
.card:hover {
  border-color: var(--line-border-strong);
  box-shadow: var(--shadow-popover);
  transform: translateY(-1px);
}
.card:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
.card-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
}
.card-head {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.card-title {
  flex: 1;
  min-width: 0;
}
.name {
  font-size: 15px;
  font-weight: 600;
  line-height: 22px;
  color: var(--text-title);
}
.card.disabled .name {
  color: var(--text-caption);
}
.meta {
  font-size: 12px;
  line-height: 18px;
  color: var(--text-placeholder);
}
.mono {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
.card-aside {
  display: inline-flex;
  flex: none;
  align-self: flex-start;
  align-items: center;
  height: 22px;
  cursor: default;
}
.builtin-badge {
  padding: 0 8px;
  border-radius: 11px;
  background: var(--bg-primary-soft);
  color: var(--color-primary);
  font-size: 12px;
  cursor: inherit;
}
.desc {
  display: -webkit-box;
  height: 40px;
  margin: 0;
  overflow: hidden;
  color: var(--text-caption);
  font-size: 13px;
  line-height: 20px;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
}
.card-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 44px;
  padding: 0 10px 0 16px;
  border-top: 1px solid var(--line-border);
  background: var(--bg-base);
}
.chips {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.category {
  overflow: hidden;
  padding: 0 8px;
  border: 1px solid var(--line-border);
  border-radius: 11px;
  background: var(--bg-body);
  color: var(--text-caption);
  font-size: 12px;
  line-height: 20px;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.chip {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 50%;
}
.chip.orange {
  background: rgba(var(--warning-6), 0.14);
  color: var(--color-warning);
}
.chip.blue {
  background: var(--bg-primary-soft);
  color: var(--color-primary);
}
.chip.purple {
  background: rgba(114, 46, 209, 0.12);
  color: rgb(114, 46, 209);
}
.usage {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 4px;
  padding: 0 6px;
  color: var(--text-placeholder);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}
.usage.active {
  color: var(--text-caption);
}
.more {
  display: inline-flex;
  flex: none;
}
.danger-option {
  color: var(--color-danger);
}
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 56px 24px;
  text-align: center;
}
.empty-card {
  border: 1px dashed var(--line-border-strong);
  border-radius: 10px;
  background: var(--bg-body);
}
.empty-title {
  font-size: 15px;
  font-weight: 500;
  color: var(--text-title);
}
.empty-desc {
  max-width: 460px;
  margin-bottom: 12px;
  line-height: 1.6;
}
@media (max-width: 768px) {
  .search {
    width: 100%;
    margin-left: 0;
  }
}
</style>
