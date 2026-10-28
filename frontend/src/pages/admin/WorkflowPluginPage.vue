<script setup lang="ts">
import { Message, Modal, type TableColumnData } from '@arco-design/web-vue'
import { CircleArrowUp, CircleCheck, CircleX, Copy, Download, FlaskConical, Info, Play, Trash2, WandSparkles, Zap } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'

import { ApiError } from '@/api/client'
import { workflowPluginsApi, type TestWorkflowPluginResp, type WorkflowPlugin } from '@/api/workflowPlugin'
import HubBadge from '@/components/admin/HubBadge.vue'
import HubOriginInfo from '@/components/admin/HubOriginInfo.vue'
import HubPackageAlert from '@/components/admin/HubPackageAlert.vue'
import HubPackageStatus from '@/components/admin/HubPackageStatus.vue'
import WorkflowPluginIcon from '@/components/admin/WorkflowPluginIcon.vue'
import WorkflowPluginInstallModal from '@/components/admin/WorkflowPluginInstallModal.vue'
import WorkflowPluginPermissions from '@/components/admin/WorkflowPluginPermissions.vue'
import CodeEditor from '@/components/common/CodeEditor.vue'
import StateIllustration from '@/components/common/StateIllustration.vue'
import SaveBar from '@/components/console/SaveBar.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useHubUpdates } from '@/composables/useHubUpdates'
import type { FieldWidget, FlowKind, ModuleCategory, ModuleDef } from '@/types/workflow'
import { dslText } from '@/utils/modules'
import { LANGUAGE_NAMES, pluginFileName, sampleConfig, versionLabel } from '@/utils/workflowPlugin'

/** Details of an installed node plugin, with its editable source and a test runner, or of a built-in node. */
const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const CATEGORY_LABELS: Record<ModuleCategory, () => string> = {
  data: () => t('workflow.picker.data'),
  logic: () => t('workflow.picker.logic'),
  integration: () => t('workflow.picker.integration'),
}
const FLOW_LABELS: Record<FlowKind, () => string> = {
  sync: () => t('pluginAdmin.flowSync'),
  async: () => t('pluginAdmin.flowAsync'),
  background: () => t('pluginAdmin.flowBackground'),
}
const WIDGET_LABELS: Partial<Record<FieldWidget, () => string>> = {
  value: () => t('pluginAdmin.widgetValue'),
  number: () => t('pluginAdmin.widgetNumber'),
  select: () => t('pluginAdmin.widgetSelect'),
  credential: () => t('pluginAdmin.widgetCredential'),
  headers: () => t('pluginAdmin.widgetHeaders'),
  body: () => t('pluginAdmin.widgetBody'),
}

const pluginID = computed(() => (typeof route.params.pluginID === 'string' ? route.params.pluginID : ''))
const builtinType = computed(() => (typeof route.params.type === 'string' ? route.params.type : ''))

const plugin = ref<WorkflowPlugin | null>(null)
const builtin = ref<ModuleDef | null>(null)
const loading = ref(false)
const notFound = ref(false)
const toggling = ref(false)

const source = ref('')
const saving = ref(false)
const dirty = computed(() => !!plugin.value && source.value !== (plugin.value.source ?? ''))

const module = computed(() => plugin.value?.module ?? builtin.value ?? null)

const config = ref('{}')
const testing = ref(false)
const result = ref<TestWorkflowPluginResp | null>(null)

const installVisible = ref(false)
const installHubVersion = ref<string>()
const hub = useHubUpdates()

function openUpgrade(version?: string) {
  installHubVersion.value = version
  installVisible.value = true
}

// Updating replaces the source, so the unsaved changes are not lost silently.
function updateFromHub() {
  if (dirty.value) Message.warning(t('hub.saveFirst'))
  else if (plugin.value) openUpgrade(hub.find('workflow_plugin', plugin.value.id, plugin.value.hub)?.latest?.version)
}

/** Shortcut of running the test from the editors. */
const RUN_SHORTCUT = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘ ↵' : 'Ctrl ↵'

function fillSample() {
  if (module.value) config.value = JSON.stringify(sampleConfig(module.value, plugin.value?.credential), null, 2)
}

function reset(p: WorkflowPlugin | null) {
  plugin.value = p
  source.value = p?.source ?? ''
  if (p?.hub) hub.ensure()
}

async function load() {
  notFound.value = false
  result.value = null
  loading.value = true
  try {
    if (pluginID.value) {
      builtin.value = null
      reset(await workflowPluginsApi.get(pluginID.value))
    } else {
      reset(null)
      const { builtins } = await workflowPluginsApi.list()
      builtin.value = builtins.find((m) => m.type === builtinType.value) ?? null
      notFound.value = !builtin.value
    }
    fillSample()
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) notFound.value = true
    else Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}
watch([pluginID, builtinType], load, { immediate: true })

const TABS = ['overview', 'source'] as const
type Tab = (typeof TABS)[number]

// Keep the active tab in the query so it survives a refresh.
const tab = computed<Tab>({
  get: () => (plugin.value && TABS.includes(route.query.tab as Tab) ? (route.query.tab as Tab) : 'overview'),
  set: (value) => router.replace({ query: { ...route.query, tab: value === 'overview' ? undefined : value } }),
})

const inputs = computed(() => module.value?.sections.flatMap((s) => s.fields).filter((f) => f.key) ?? [])

const inputColumns = computed<TableColumnData[]>(() => [
  { title: t('pluginAdmin.fieldName'), slotName: 'label' },
  { title: t('pluginAdmin.fieldKey'), slotName: 'key', width: 180 },
  { title: t('pluginAdmin.fieldWidget'), slotName: 'widget', width: 120 },
  { title: t('pluginAdmin.fieldRequired'), slotName: 'required', width: 90, align: 'center' },
])
const outputColumns = computed<TableColumnData[]>(() => [
  { title: t('pluginAdmin.fieldName'), slotName: 'label' },
  { title: t('pluginAdmin.fieldKey'), slotName: 'key', width: 180 },
  { title: t('pluginAdmin.outputKind'), dataIndex: 'kind', width: 120 },
])

const formatTime = (time: string) => dayjs(time).format('YYYY-MM-DD HH:mm')

async function toggle(enabled: boolean) {
  const p = plugin.value
  if (!p || toggling.value) return
  toggling.value = true
  try {
    const saved = await workflowPluginsApi.setEnabled(p.id, enabled)
    plugin.value = { ...saved, source: p.source }
    Message.success(enabled ? t('pluginAdmin.enabledMessage') : t('pluginAdmin.disabledMessage'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    toggling.value = false
  }
}

async function save() {
  const p = plugin.value
  if (!p || !dirty.value || saving.value) return
  saving.value = true
  try {
    // The source is checked first, so a changed id is not installed as another node.
    const inspection = await workflowPluginsApi.inspect({ source: source.value, language: p.language })
    if (inspection.plugin.id !== p.id) {
      Message.warning(t('pluginAdmin.idChanged', { id: p.id }))
      return
    }
    const saved = await workflowPluginsApi.install(source.value, p.language)
    reset({ ...saved, source: source.value })
    if (p.hub && !saved.hub) void hub.check()
    Message.success(t('common.saved'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

function discard() {
  source.value = plugin.value?.source ?? ''
}

function onUpgraded(saved: WorkflowPlugin) {
  if (saved.hub || plugin.value?.hub) void hub.check()
  if (saved.id !== plugin.value?.id) {
    router.push({ name: 'admin-workflow-plugin', params: { pluginID: saved.id } })
    return
  }
  void load()
}

function uninstall() {
  const p = plugin.value
  if (!p) return
  Modal.confirm({
    title: t('pluginAdmin.uninstallTitle', { name: p.module.label }),
    content: t('pluginAdmin.uninstallContent'),
    okButtonProps: { status: 'danger' },
    okText: t('pluginAdmin.uninstall'),
    onBeforeOk: async () => {
      try {
        await workflowPluginsApi.uninstall(p.id)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      if (p.hub) void hub.check()
      Message.success(t('pluginAdmin.uninstalled'))
      source.value = p.source ?? ''
      router.replace({ name: 'admin-workflow-plugins' })
      return true
    },
  })
}

async function copySource() {
  await navigator.clipboard.writeText(source.value)
  Message.success(t('pluginAdmin.copied'))
}

function downloadSource() {
  const p = plugin.value
  if (!p) return
  const href = URL.createObjectURL(new Blob([source.value], { type: 'text/javascript;charset=utf-8' }))
  const a = document.createElement('a')
  a.href = href
  a.download = pluginFileName(p.id, p.language)
  a.click()
  URL.revokeObjectURL(href)
}

const configInvalid = computed(() => {
  try {
    const v = JSON.parse(config.value || '{}') as unknown
    return typeof v !== 'object' || v === null || Array.isArray(v)
  } catch {
    return true
  }
})

async function runTest() {
  const p = plugin.value
  if (!p || testing.value) return
  if (configInvalid.value) {
    Message.warning(t('pluginAdmin.invalidConfig'))
    return
  }
  testing.value = true
  try {
    const body = { config: JSON.parse(config.value || '{}') as Record<string, unknown> }
    result.value = await workflowPluginsApi.test(dirty.value ? { ...body, source: source.value, language: p.language } : { ...body, pluginID: p.id })
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    testing.value = false
  }
}

onBeforeRouteLeave(() => {
  if (!dirty.value) return true
  return new Promise<boolean>((resolve) => {
    Modal.confirm({
      title: t('pluginAdmin.leaveTitle'),
      content: t('pluginAdmin.leaveContent'),
      okText: t('pluginAdmin.leave'),
      okButtonProps: { status: 'danger' },
      onOk: () => resolve(true),
      onCancel: () => resolve(false),
    })
  })
})

function onBeforeUnload(e: BeforeUnloadEvent) {
  if (dirty.value) e.preventDefault()
}

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === 's') {
    e.preventDefault()
    void save()
  }
}

onMounted(() => {
  window.addEventListener('beforeunload', onBeforeUnload)
  window.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', onBeforeUnload)
  window.removeEventListener('keydown', onKeydown)
})

const format = (v: unknown) => (v === undefined ? 'undefined' : JSON.stringify(v, null, 2))
</script>

<template>
  <div class="plugin-page">
    <a-breadcrumb class="breadcrumb">
      <a-breadcrumb-item>
        <RouterLink :to="{ name: 'admin-workflow-plugins' }">{{ t('pluginAdmin.title') }}</RouterLink>
      </a-breadcrumb-item>
      <a-breadcrumb-item>{{ module ? dslText(module.label) : t('pluginAdmin.title') }}</a-breadcrumb-item>
    </a-breadcrumb>

    <a-result v-if="notFound" :status="null" title="404" :subtitle="t('pluginAdmin.notFound')">
      <template #icon><StateIllustration name="not-found" /></template>
      <template #extra>
        <a-button type="primary" @click="router.push({ name: 'admin-workflow-plugins' })">{{ t('pluginAdmin.backToList') }}</a-button>
      </template>
    </a-result>

    <a-spin v-else :loading="loading" class="spin">
      <template v-if="module">
        <header class="hero">
          <WorkflowPluginIcon :icon="module.icon" :color="module.color" :size="56" :muted="!!plugin && !plugin.enabled" />
          <div class="hero-text">
            <div class="hero-name">
              <span class="ellipsis">{{ dslText(module.label) }}</span>
              <HubBadge v-if="plugin" :hub="plugin.hub" link :size="18" />
              <a-tag v-if="plugin" size="small" class="mono">{{ versionLabel(plugin.version) }}</a-tag>
              <a-tag v-else size="small" color="arcoblue">{{ t('pluginAdmin.builtinTag') }}</a-tag>
              <HubPackageStatus v-if="plugin" kind="workflow_plugin" :id="plugin.id" :hub="plugin.hub" class="hero-status" @update="updateFromHub" />
            </div>
            <div class="text-desc hero-desc">{{ dslText(module.description) || t('pluginAdmin.noDescription') }}</div>
          </div>
          <a-space v-if="plugin" wrap :size="8" class="hero-actions">
            <div class="status">
              <span :class="plugin.enabled ? 'status-on' : 'text-desc'">{{ plugin.enabled ? t('pluginAdmin.enabledTag') : t('pluginAdmin.disabledTag') }}</span>
              <a-switch :model-value="plugin.enabled" :loading="toggling" :aria-label="t('pluginAdmin.enabledTag')" @change="(v) => toggle(!!v)" />
            </div>
            <a-button type="primary" @click="tab = 'source'">
              <template #icon><FlaskConical :size="16" /></template>
              {{ t('pluginAdmin.test') }}
            </a-button>
            <a-button @click="openUpgrade()">
              <template #icon><CircleArrowUp :size="16" /></template>
              {{ t('pluginAdmin.upgrade') }}
            </a-button>
            <a-button status="danger" @click="uninstall">
              <template #icon><Trash2 :size="16" /></template>
              {{ t('pluginAdmin.uninstall') }}
            </a-button>
          </a-space>
        </header>

        <HubPackageAlert
          v-if="plugin"
          kind="workflow_plugin"
          :id="plugin.id"
          :hub="plugin.hub"
          :disabled="dirty"
          :disabled-reason="t('hub.saveFirst')"
          @update="updateFromHub"
        />

        <div class="workspace" :class="{ single: !plugin || tab === 'overview' }">
          <div class="main">
            <SettingsSection flush>
              <a-tabs v-model:active-key="tab" :animation="false" class="tabs">
                <a-tab-pane key="overview" :title="t('pluginAdmin.overview')">
                  <div class="pane">
                    <a-row :gutter="[32, 32]" class="overview-grid">
                      <a-col :xs="24" :lg="plugin ? 12 : 24">
                        <section aria-labelledby="node-basic-info">
                          <a-typography-title id="node-basic-info" :heading="6" class="section-heading">{{ t('pluginAdmin.basicInfo') }}</a-typography-title>
                          <a-descriptions
                            :column="plugin ? 1 : { xs: 1, md: 2 }"
                            size="small"
                            :label-style="{ width: '100px' }"
                            :value-style="{ overflowWrap: 'anywhere' }"
                            class="info"
                          >
                            <a-descriptions-item :label="t('pluginAdmin.category')">
                              <a-tag size="small" color="arcoblue">{{ CATEGORY_LABELS[module.category]() }}</a-tag>
                            </a-descriptions-item>
                            <template v-if="plugin">
                              <a-descriptions-item :label="t('pluginAdmin.author')">{{ module.plugin?.author || '—' }}</a-descriptions-item>
                              <a-descriptions-item :label="t('pluginAdmin.language')">{{ LANGUAGE_NAMES[plugin.language] }}</a-descriptions-item>
                              <a-descriptions-item v-if="plugin.hub" :label="t('hub.origin')">
                                <HubOriginInfo :hub="plugin.hub" />
                              </a-descriptions-item>
                              <a-descriptions-item v-if="module.plugin?.homepage" :label="t('pluginAdmin.homepage')">
                                <a-link :href="module.plugin.homepage" target="_blank" rel="noopener noreferrer" class="break">
                                  {{ module.plugin.homepage }}
                                </a-link>
                              </a-descriptions-item>
                              <a-descriptions-item :label="t('pluginAdmin.installedAt')">{{ formatTime(plugin.createdAt) }}</a-descriptions-item>
                              <a-descriptions-item :label="t('pluginAdmin.updatedAt')">{{ formatTime(plugin.updatedAt) }}</a-descriptions-item>
                              <a-descriptions-item v-if="plugin.sourceURL" :label="t('pluginAdmin.sourceURLLabel')">
                                <a-typography-paragraph :ellipsis="{ rows: 2, showTooltip: true }" class="source-url mono">
                                  {{ plugin.sourceURL }}
                                </a-typography-paragraph>
                              </a-descriptions-item>
                            </template>
                            <a-descriptions-item v-else :label="t('pluginAdmin.flows')">
                              <a-space wrap :size="4">
                                <a-tag v-for="f in module.flows" :key="f" size="small">{{ FLOW_LABELS[f]() }}</a-tag>
                              </a-space>
                            </a-descriptions-item>
                          </a-descriptions>
                        </section>
                      </a-col>

                      <a-col v-if="plugin" :xs="24" :lg="12">
                        <section aria-labelledby="node-permissions" class="overview-permissions">
                          <a-typography-title id="node-permissions" :heading="6" class="section-heading">{{ t('pluginAdmin.permissions') }}</a-typography-title>
                          <WorkflowPluginPermissions :module="module" :domains="plugin.domains" :timeout="plugin.timeout" :credential="plugin.credential" />
                        </section>
                      </a-col>
                    </a-row>

                    <a-divider class="overview-divider" />

                    <a-row :gutter="[32, 32]" class="overview-grid">
                      <a-col :xs="24" :xl="12">
                        <section aria-labelledby="node-inputs">
                          <a-space :size="8" align="baseline" class="section-heading">
                            <a-typography-title id="node-inputs" :heading="6">{{ t('pluginAdmin.inputs') }}</a-typography-title>
                            <a-typography-text type="secondary">{{ inputs.length }}</a-typography-text>
                          </a-space>
                          <a-table
                            :columns="inputColumns"
                            :data="inputs"
                            :pagination="false"
                            :scroll="{ x: 520 }"
                            :bordered="false"
                            size="small"
                            row-key="key"
                            class="io-table"
                          >
                            <template #label="{ record }">{{ dslText(record.label) || record.key }}</template>
                            <template #key="{ record }"><a-typography-text code>{{ record.key }}</a-typography-text></template>
                            <template #widget="{ record }">{{ WIDGET_LABELS[record.widget as FieldWidget]?.() ?? record.widget }}</template>
                            <template #required="{ record }">
                              <CircleCheck v-if="record.required" :size="15" class="required-icon" />
                              <span v-else class="text-desc">—</span>
                            </template>
                            <template #empty><a-empty :description="t('pluginAdmin.noInputs')" /></template>
                          </a-table>
                        </section>
                      </a-col>

                      <a-col :xs="24" :xl="12">
                        <section aria-labelledby="node-outputs">
                          <a-space :size="8" align="baseline" class="section-heading">
                            <a-typography-title id="node-outputs" :heading="6">{{ t('pluginAdmin.outputs') }}</a-typography-title>
                            <a-typography-text type="secondary">{{ module.outputs.length }}</a-typography-text>
                          </a-space>
                          <a-table
                            :columns="outputColumns"
                            :data="module.outputs"
                            :pagination="false"
                            :scroll="{ x: 460 }"
                            :bordered="false"
                            size="small"
                            row-key="key"
                            class="io-table"
                          >
                            <template #label="{ record }">{{ dslText(record.label) }}</template>
                            <template #key="{ record }"><a-typography-text code>{{ record.key }}</a-typography-text></template>
                            <template #empty><a-empty :description="t('pluginAdmin.noOutputs')" /></template>
                          </a-table>
                        </section>
                      </a-col>
                    </a-row>
                  </div>
                </a-tab-pane>

                <a-tab-pane v-if="plugin" key="source">
                  <template #title>
                    <span class="tab-title">
                      {{ t('pluginAdmin.source') }}
                      <span v-if="dirty" class="tab-dot" />
                    </span>
                  </template>
                  <div class="pane">
                    <div class="source-toolbar">
                      <span class="text-desc source-hint">{{ plugin.hub ? t('hub.editDetaches') : t('pluginAdmin.sourceEditHint') }}</span>
                      <a-space :size="4">
                        <a-button size="small" @click="copySource">
                          <template #icon><Copy :size="14" /></template>
                          {{ t('pluginAdmin.copy') }}
                        </a-button>
                        <a-button size="small" @click="downloadSource">
                          <template #icon><Download :size="14" /></template>
                          {{ t('pluginAdmin.downloadSource') }}
                        </a-button>
                      </a-space>
                    </div>
                    <CodeEditor
                      :key="plugin.checksum"
                      v-model="source"
                      :language="plugin.language"
                      height="clamp(420px, calc(100vh - 420px), 860px)"
                      :aria-label="t('pluginAdmin.source')"
                      @submit="runTest"
                    />
                  </div>
                </a-tab-pane>
              </a-tabs>
            </SettingsSection>
          </div>

          <aside v-if="plugin && tab === 'source'" class="aside">
            <SettingsSection :title="t('pluginAdmin.test')">
              <template #extra>
                <a-tooltip :content="RUN_SHORTCUT" position="tl">
                  <a-button type="primary" size="small" :loading="testing" class="run" @click="runTest">
                    <template #icon><Play :size="13" fill="currentColor" /></template>
                    {{ t('pluginAdmin.run') }}
                  </a-button>
                </a-tooltip>
              </template>
              <div class="test-panel">
                <div v-if="module.action || dirty" class="notes">
                  <div v-if="module.action" class="note warn">
                    <Zap :size="13" />
                    <span>{{ t('pluginAdmin.testActionWarning') }}</span>
                  </div>
                  <div v-if="dirty" class="note info">
                    <Info :size="13" />
                    <span>{{ t('pluginAdmin.testUnsaved') }}</span>
                  </div>
                </div>
                <div class="panel-row">
                  <span class="label">{{ t('pluginAdmin.testConfig') }}</span>
                  <a-button size="mini" type="text" @click="fillSample">
                    <template #icon><WandSparkles :size="13" /></template>
                    {{ t('pluginAdmin.sampleConfig') }}
                  </a-button>
                </div>
                <CodeEditor
                  v-model="config"
                  language="json"
                  min-height="140px"
                  max-height="280px"
                  :error="configInvalid"
                  :aria-label="t('pluginAdmin.testConfig')"
                  @submit="runTest"
                />
                <div class="text-desc test-hint">{{ t('pluginAdmin.testHint') }}</div>

                <div v-if="result" class="result">
                  <div class="result-status" :class="result.error ? 'failed' : 'success'">
                    <component :is="result.error ? CircleX : CircleCheck" :size="16" />
                    <span>{{ result.error ? t('pluginAdmin.testFailed') : t('pluginAdmin.testSuccess') }}</span>
                    <span class="result-meta">
                      {{ t('pluginAdmin.fetches', { n: result.fetches }, result.fetches) }} · {{ t('pluginAdmin.duration', { ms: result.durationMs }) }}
                    </span>
                  </div>
                  <pre v-if="result.error" class="output error-output">{{ result.error }}</pre>
                  <template v-else>
                    <div class="label">{{ t('pluginAdmin.returned') }}</div>
                    <CodeEditor :model-value="format(result.value)" language="json" readonly min-height="40px" max-height="240px" />
                  </template>
                  <template v-if="result.logs.length">
                    <div class="label">{{ t('pluginAdmin.logs') }}</div>
                    <pre class="output">{{ result.logs.join('\n') }}</pre>
                  </template>
                </div>
                <div v-else class="result-empty">
                  <FlaskConical :size="22" />
                  <span>{{ t('pluginAdmin.testEmpty') }}</span>
                </div>
              </div>
            </SettingsSection>
          </aside>
        </div>

        <SaveBar v-if="plugin && (tab === 'source' || dirty)" :dirty="dirty" :saving="saving" @save="save" @reset="discard" />
      </template>
    </a-spin>

    <WorkflowPluginInstallModal v-if="plugin" v-model:visible="installVisible" :upgrade="plugin" :from-hub="!!installHubVersion" :hub-version="installHubVersion" @installed="onUpgraded" />
  </div>
</template>

<style scoped>
.breadcrumb {
  margin-bottom: 16px;
}
.breadcrumb a {
  color: var(--text-caption);
}
.breadcrumb a:hover {
  color: var(--color-primary);
}
.spin {
  display: block;
  min-height: 240px;
}
.hero {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
}
.hero-text {
  flex: 1;
  min-width: 240px;
}
.hero-name {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  font-size: 20px;
  font-weight: 600;
  line-height: 28px;
  color: var(--text-title);
}
.hero-desc {
  margin-top: 4px;
  line-height: 1.6;
}
.hero-status {
  font-weight: 400;
  line-height: normal;
}
.status {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-right: 8px;
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
}
.status-on {
  color: var(--color-success);
}
.mono {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
.break {
  overflow-wrap: anywhere;
}
.workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 400px;
  gap: 16px;
  align-items: start;
}
.workspace.single {
  grid-template-columns: minmax(0, 1fr);
}
.main {
  min-width: 0;
}
.aside {
  position: sticky;
  top: 0;
  min-width: 0;
  /* Allow for the 56px header and leave room for the save bar below. */
  max-height: calc(100vh - 56px - 96px);
  overflow-y: auto;
  border-radius: 8px;
}
.aside :deep(.arco-card-header) {
  box-sizing: border-box;
  height: 50px !important;
  padding: 0 24px !important;
}
.tabs :deep(.arco-tabs-nav) {
  height: 50px;
  padding: 0 24px;
}
.tabs :deep(.arco-tabs-nav::before) {
  background-color: var(--line-border);
}
.tabs :deep(.arco-tabs-tab) {
  height: 50px;
  margin: 0 28px 0 0;
  padding: 0;
  font-size: 14px;
  box-sizing: border-box;
}
.tabs :deep(.arco-tabs-tab-title) {
  padding: 0;
}
.tabs :deep(.arco-tabs-tab-title::before) {
  display: none;
}
.tabs :deep(.arco-tabs-tab-active) {
  font-weight: 500;
}
.tabs :deep(.arco-tabs-content) {
  padding: 0;
}
.pane {
  padding: 20px 24px 24px;
}
.tab-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.tab-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-warning);
}
.overview-grid :deep(.arco-col) {
  min-width: 0;
}

.section-heading {
  margin: 0 0 16px;
}

.section-heading :deep(.arco-typography) {
  margin: 0;
}

.overview-divider {
  margin: 28px 0;
}

.source-url.arco-typography {
  margin: 0;
}

.overview-permissions :deep(.permissions) {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px 20px;
}

.overview-permissions :deep(.perm-warning) {
  grid-column: 1 / -1;
}

.io-table :deep(.arco-table-th) {
  background: var(--bg-base);
}

.required-icon {
  color: var(--color-success);
}
.source-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.source-hint {
  font-size: 12.5px;
}
.test-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 0 16px;
}
.notes {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.note {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  padding: 7px 10px;
  border-radius: 6px;
  font-size: 12.5px;
  line-height: 18px;
}
.note svg {
  flex: none;
  margin-top: 2.5px;
}
.note.warn {
  background: rgba(var(--warning-6), 0.1);
  color: var(--color-warning);
}
.note.info {
  background: var(--bg-primary-soft);
  color: var(--color-primary);
}
.run {
  padding: 0 12px;
  border-radius: 6px;
}
.panel-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 28px;
}
.label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-title);
}
.test-hint {
  font-size: 12px;
}
.result {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
  padding-top: 16px;
  border-top: 1px solid var(--line-border);
}
.result-status {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
}
.result-status.success {
  color: var(--color-success);
}
.result-status.failed {
  color: var(--color-danger);
}
.result-meta {
  margin-left: auto;
  font-size: 12px;
  font-weight: 400;
  color: var(--text-caption);
}
.output {
  margin: 0;
  padding: 10px 12px;
  max-height: 240px;
  overflow: auto;
  border: 1px solid var(--line-border);
  border-radius: 6px;
  background: var(--bg-base);
  color: var(--text-title);
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
.error-output {
  border-color: var(--color-danger);
  color: var(--color-danger);
}
.result-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  padding: 28px 16px;
  border: 1px dashed var(--line-border-strong);
  border-radius: 6px;
  color: var(--text-placeholder);
  font-size: 12px;
  text-align: center;
}
@media (max-width: 768px) {
  .hero-text {
    min-width: 0;
    flex-basis: 200px;
  }

  .hero-actions {
    width: 100%;
  }

  .hero-actions :deep(.arco-space-item:first-child) {
    flex-basis: 100%;
  }

  .pane {
    padding: 16px;
  }

  .overview-permissions :deep(.permissions) {
    grid-template-columns: minmax(0, 1fr);
    gap: 16px;
  }
}

@media (max-width: 1180px) {
  .workspace {
    grid-template-columns: minmax(0, 1fr);
  }
  .aside {
    position: static;
    max-height: none;
  }
}
</style>
