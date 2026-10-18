<script setup lang="ts">
import { Message, Modal, type TableColumnData } from '@arco-design/web-vue'
import { CircleArrowUp, CircleCheck, CircleX, Copy, Download, FlaskConical, Globe, KeyRound, Play, Sparkles, Timer, Trash2, WandSparkles } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'

import { ApiError } from '@/api/client'
import { adminApi, type AdminAISettings } from '@/api/admin'
import {
  adminShortcutsApi,
  type AdminFieldShortcut,
  type SaveFieldShortcut,
  type ShortcutFormItem,
  type TestFieldShortcutResp,
} from '@/api/shortcut'
import FieldShortcutInstallModal from '@/components/admin/FieldShortcutInstallModal.vue'
import WorkflowPluginIcon from '@/components/admin/WorkflowPluginIcon.vue'
import CodeEditor from '@/components/common/CodeEditor.vue'
import StateIllustration from '@/components/common/StateIllustration.vue'
import SaveBar from '@/components/console/SaveBar.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import type { FieldType } from '@/types/bitable'
import { fieldTypeInfo } from '@/utils/fieldTypes'
import { formComponentKey, formItemIssueKey, formItemIssueName, loadFormItems, validateFormItems, type FormComponent } from '@/utils/shortcutFormItems'
import { sampleShortcutParams } from '@/utils/shortcut'
import { LANGUAGE_NAMES, pluginFileName, versionLabel, type PluginLanguage } from '@/utils/workflowPlugin'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

type CredentialType = 'bearer' | 'header' | 'query'

interface CredentialDraft {
  key: string
  type: CredentialType
  name: string
  value: string
  hasValue: boolean
}

interface Draft {
  name: string
  version: string
  description: string
  resultType: FieldType
  code: string
  language: PluginLanguage
  formItems: string
  domains: string[]
  credentials: CredentialDraft[]
  timeoutSeconds: number
  enabled: boolean
  aiEnabled: boolean
}

function toDraft(s: AdminFieldShortcut | null): Draft {
  return {
    name: s?.name ?? '',
    version: s?.version ?? '1.0.0',
    description: s?.description ?? '',
    resultType: (s?.resultType ?? 'text') as FieldType,
    code: s?.code ?? '',
    language: s?.language ?? 'javascript',
    formItems: JSON.stringify(s?.formItems ?? [], null, 2),
    domains: [...(s?.domains ?? [])],
    credentials: (s?.credentials ?? []).map((c) => ({ key: c.key, type: c.type as CredentialType, name: c.name ?? '', value: '', hasValue: c.hasValue })),
    timeoutSeconds: s?.timeoutSeconds ?? 30,
    enabled: s?.enabled ?? true,
    aiEnabled: s?.aiEnabled ?? false,
  }
}

const uid = computed(() => (typeof route.params.shortcutUID === 'string' ? route.params.shortcutUID : ''))
const shortcut = ref<AdminFieldShortcut | null>(null)
const loading = ref(false)
const notFound = ref(false)
const saving = ref(false)
const installVisible = ref(false)

const draft = reactive<Draft>(toDraft(null))
const baseline = ref(JSON.stringify(draft))
const dirty = computed(() => JSON.stringify(draft) !== baseline.value)
const sourceDirty = computed(() => !!shortcut.value && (draft.code !== shortcut.value.code || draft.language !== shortcut.value.language))
const syncing = ref(false)
const syncError = ref('')
let syncedCode = draft.code
let syncedLanguage = draft.language

const params = ref(sampleShortcutParams(JSON.parse(draft.formItems) as ShortcutFormItem[]))
const testing = ref(false)
const testResult = ref<TestFieldShortcutResp | null>(null)

const aiSettings = ref<AdminAISettings | null>(null)
const aiLoading = ref(true)
const aiError = ref(false)
const aiReady = computed(() => !!aiSettings.value?.enabled && !!aiSettings.value.baseURL.trim() && !!aiSettings.value.model.trim())
const aiStatus = computed(() => {
  if (aiLoading.value) return t('shortcutAdmin.aiLoading')
  if (aiError.value) return t('shortcutAdmin.aiLoadFailed')
  if (!aiReady.value) return t('shortcutAdmin.aiUnavailable')

  return t('shortcutAdmin.aiReady', { model: aiSettings.value!.model })
})

function reset(s: AdminFieldShortcut | null) {
  Object.assign(draft, toDraft(s))
  syncedCode = draft.code
  syncedLanguage = draft.language
  syncError.value = ''
  baseline.value = JSON.stringify(draft)
  params.value = sampleShortcutParams(JSON.parse(draft.formItems) as ShortcutFormItem[])
  testResult.value = null
}

async function load() {
  notFound.value = false
  if (!uid.value) {
    shortcut.value = null
    reset(null)
    return
  }
  if (shortcut.value?.uid === uid.value) return
  loading.value = true
  try {
    shortcut.value = await adminShortcutsApi.get(uid.value)
    reset(shortcut.value)
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) notFound.value = true
    else Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}
watch(uid, load, { immediate: true })

const TABS = ['overview', 'network', 'code'] as const
type Tab = (typeof TABS)[number]
const availableTabs = computed(() => TABS.filter((value) => value !== 'network' || draft.credentials.length > 0))

// Keep the active tab in the query so it survives a refresh.
const tab = computed<Tab>({
  get: () => (availableTabs.value.includes(route.query.tab as Tab) ? (route.query.tab as Tab) : 'overview'),
  set: (value) => { void switchTab(value) },
})

async function syncFromCode(): Promise<boolean> {
  if (draft.code === syncedCode && draft.language === syncedLanguage) {
    syncError.value = ''
    return true
  }

  const source = draft.code
  const language = draft.language
  try {
    syncing.value = true
    const result = await adminShortcutsApi.inspect({ source, language })
    if (draft.code !== source || draft.language !== language) {
      syncError.value = t('shortcutAdmin.manifestSourceChanged')
      return false
    }

    const credentials = result.credentials.map((c) => {
      const previous = draft.credentials.find((entry) => entry.key === c.key)

      return { key: c.key, type: c.type as CredentialType, name: c.name ?? '', value: previous?.value ?? '', hasValue: previous?.hasValue ?? false }
    })
    Object.assign(draft, {
      name: result.name,
      version: result.version,
      description: result.description,
      resultType: result.resultType,
      formItems: JSON.stringify(result.formItems, null, 2),
      domains: [...result.domains],
      credentials,
      timeoutSeconds: result.timeoutSeconds,
      aiEnabled: result.aiEnabled,
    })
    syncedCode = source
    syncedLanguage = language
    syncError.value = ''
    return true
  } catch (e) {
    syncError.value = e instanceof Error && e.message !== 'syntax' && e.message !== 'manifest'
      ? e.message : t('shortcutAdmin.manifestSyncError')
    return false
  } finally {
    syncing.value = false
  }
}

async function switchTab(value: Tab) {
  if (syncing.value || value === tab.value) return
  if (tab.value === 'code' && !await syncFromCode()) return
  if (!availableTabs.value.includes(value)) value = 'overview'

  await router.replace({ query: { ...route.query, tab: value === 'overview' ? undefined : value } })
}

async function prepareBody(): Promise<SaveFieldShortcut | null> {
  if (!await syncFromCode()) return null
  return body()
}

async function copySource() {
  try {
    await navigator.clipboard.writeText(draft.code)
    Message.success(t('pluginAdmin.copied'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  }
}

function downloadSource() {
  const href = URL.createObjectURL(new Blob([draft.code], { type: 'text/javascript;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = href
  link.download = pluginFileName(uid.value || 'field-shortcut', draft.language)
  link.click()
  URL.revokeObjectURL(href)
}

const formatTime = (time?: string) => time ? dayjs(time).format('YYYY-MM-DD HH:mm') : '—'

/** Form-item definitions for the overview and test parameters. */
const parsedFormItems = computed<ShortcutFormItem[] | null>(() => {
  const loaded = loadFormItems(draft.formItems)
  if (!loaded.ok) return null
  return JSON.parse(draft.formItems) as ShortcutFormItem[]
})

const formItemColumns = computed<TableColumnData[]>(() => [
  { title: t('shortcutAdmin.itemLabel'), dataIndex: 'label' },
  { title: t('shortcutAdmin.itemKey'), slotName: 'key', width: 160 },
  { title: t('shortcutAdmin.itemComponent'), slotName: 'component', width: 140 },
  { title: t('shortcutAdmin.itemRequired'), slotName: 'required', width: 80, align: 'center' },
])
const credentialColumns = computed<TableColumnData[]>(() => [
  { title: t('shortcutAdmin.credentialKey'), slotName: 'key' },
  { title: t('shortcutAdmin.credentialType'), slotName: 'type', width: 120 },
  { title: t('shortcutAdmin.credentialName'), slotName: 'name' },
])

function credentialTypeLabel(type: string) {
  return type === 'bearer' ? 'Bearer' : t(type === 'header' ? 'shortcutAdmin.credentialHeader' : 'shortcutAdmin.credentialQuery')
}

const paramsInvalid = computed(() => {
  try {
    const p = JSON.parse(params.value || '{}') as unknown
    return typeof p !== 'object' || p === null || Array.isArray(p)
  } catch {
    return true
  }
})

/** Invalid definitions are corrected in the source. */
function body(): SaveFieldShortcut | null {
  const loaded = loadFormItems(draft.formItems)
  if (!loaded.ok) {
    Message.warning(t('shortcutAdmin.invalidFormItems'))
    tab.value = 'code'
    return null
  }
  const issue = validateFormItems(loaded.items)[0]
  if (issue) {
    Message.warning(t(formItemIssueKey(issue), { key: formItemIssueName(loaded.items, issue) }))
    tab.value = 'code'
    return null
  }
  if (loaded.items.some((item) => item.component === 'field_options') && draft.resultType !== 'single_select' && draft.resultType !== 'multi_select') {
    Message.warning(t('shortcutAdmin.fieldOptionsTypeRequired'))
    tab.value = 'code'
    return null
  }
  return {
    name: draft.name,
    version: draft.version,
    description: draft.description,
    resultType: draft.resultType as SaveFieldShortcut['resultType'],
    code: draft.code,
    language: draft.language,
    formItems: JSON.parse(draft.formItems) as ShortcutFormItem[],
    domains: draft.domains,
    credentials: draft.credentials.map((c) => ({ key: c.key, type: c.type, name: c.type === 'bearer' ? undefined : c.name, value: c.value || undefined })),
    timeoutSeconds: draft.timeoutSeconds,
    enabled: draft.enabled,
    aiEnabled: draft.aiEnabled,
  }
}

function fillSampleParams() {
  if (parsedFormItems.value) params.value = sampleShortcutParams(parsedFormItems.value)
  else Message.warning(t('shortcutAdmin.invalidFormItems'))
}

async function test() {
  if (testing.value || saving.value || syncing.value) return

  testing.value = true
  try {
    const b = await prepareBody()
    if (!b) return
    if (paramsInvalid.value) {
      Message.warning(t('shortcutAdmin.invalidParams'))
      return
    }

    testResult.value = await adminShortcutsApi.test(b, JSON.parse(params.value || '{}') as Record<string, unknown>, shortcut.value?.uid)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    testing.value = false
  }
}

async function save() {
  if (!shortcut.value || saving.value || testing.value || syncing.value) return

  saving.value = true
  try {
    const b = await prepareBody()
    if (!b) return

    const saved = await adminShortcutsApi.update(shortcut.value.uid, b)
    shortcut.value = saved
    // Use the server response as the baseline, clear credential values, and preserve the test result.
    const result = testResult.value
    const currentParams = params.value
    reset(saved)
    testResult.value = result
    params.value = currentParams
    Message.success(t('common.saved'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

function discard() {
  if (syncing.value) return
  reset(shortcut.value)
}

function onUpgraded(saved: AdminFieldShortcut) {
  shortcut.value = saved
  reset(saved)
}

function remove() {
  const s = shortcut.value
  if (!s) return
  Modal.confirm({
    title: t('shortcutAdmin.uninstallTitle', { name: s.name }),
    content: s.usedBy ? t('shortcutAdmin.deleteInUse', { n: s.usedBy }, s.usedBy) : t('shortcutAdmin.deleteContent'),
    okButtonProps: { status: 'danger' },
    okText: t('pluginAdmin.uninstall'),
    onBeforeOk: async () => {
      try {
        await adminShortcutsApi.delete(s.uid)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      baseline.value = JSON.stringify(draft)
      Message.success(t('pluginAdmin.uninstalled'))
      router.replace({ name: 'admin-field-shortcuts' })
      return true
    },
  })
}

onBeforeRouteLeave(() => {
  if (!dirty.value) return true
  return new Promise<boolean>((resolve) => {
    Modal.confirm({
      title: t('shortcutAdmin.leaveTitle'),
      content: t('shortcutAdmin.leaveContent'),
      okText: t('shortcutAdmin.leave'),
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
    if (!loading.value && !notFound.value) save()
  }
}

onMounted(async () => {
  window.addEventListener('beforeunload', onBeforeUnload)
  window.addEventListener('keydown', onKeydown)

  try {
    aiSettings.value = await adminApi.aiSettings()
  } catch {
    aiError.value = true
  } finally {
    aiLoading.value = false
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', onBeforeUnload)
  window.removeEventListener('keydown', onKeydown)
})

const format = (v: unknown) => (v === undefined ? 'undefined' : JSON.stringify(v, null, 2))
</script>

<template>
  <div class="shortcut-editor">
    <a-breadcrumb class="breadcrumb">
      <a-breadcrumb-item>
        <RouterLink :to="{ name: 'admin-field-shortcuts' }">{{ t('shortcutAdmin.title') }}</RouterLink>
      </a-breadcrumb-item>
      <a-breadcrumb-item>{{ shortcut?.name ?? uid }}</a-breadcrumb-item>
    </a-breadcrumb>

    <a-result v-if="notFound" :status="null" title="404" :subtitle="t('shortcutAdmin.notFound')">
      <template #icon><StateIllustration name="not-found" /></template>
      <template #extra>
        <a-button type="primary" @click="router.push({ name: 'admin-field-shortcuts' })">{{ t('shortcutAdmin.backToList') }}</a-button>
      </template>
    </a-result>

    <a-spin v-else :loading="loading || syncing" class="spin">
      <header v-if="shortcut" class="hero">
        <WorkflowPluginIcon :icon="draft.aiEnabled ? 'sparkles' : 'zap'" color="var(--color-primary)" :size="56" :muted="!draft.enabled" />
        <div class="hero-text">
          <div class="hero-name">
            <span class="ellipsis">{{ draft.name }}</span>
            <a-tag size="small" class="mono">{{ versionLabel(draft.version) }}</a-tag>
          </div>
          <div class="text-desc hero-desc">{{ draft.description || t('pluginAdmin.noDescription') }}</div>
        </div>
        <a-space wrap :size="8" class="hero-actions">
          <a-tooltip :content="t('shortcutAdmin.enabledDescription')">
            <div class="status">
              <span :class="draft.enabled ? 'status-on' : 'text-desc'">{{ draft.enabled ? t('shortcutAdmin.enabledTag') : t('shortcutAdmin.disabledTag') }}</span>
              <a-switch v-model="draft.enabled" :aria-label="t('shortcutAdmin.enabled')" />
            </div>
          </a-tooltip>
          <a-button type="primary" @click="tab = 'code'">
            <template #icon><FlaskConical :size="16" /></template>
            {{ t('shortcutAdmin.test') }}
          </a-button>
          <a-tooltip :content="dirty ? t('shortcutAdmin.upgradeSaveHint') : t('pluginAdmin.upgrade')">
            <a-button :disabled="dirty || saving || syncing || testing" @click="installVisible = true">
              <template #icon><CircleArrowUp :size="16" /></template>
              {{ t('pluginAdmin.upgrade') }}
            </a-button>
          </a-tooltip>
          <a-button status="danger" @click="remove">
            <template #icon><Trash2 :size="16" /></template>
            {{ t('pluginAdmin.uninstall') }}
          </a-button>
        </a-space>
      </header>

      <a-alert v-if="syncError" type="error" class="sync-error">{{ syncError }}</a-alert>

      <div class="workspace" :class="{ single: tab !== 'code' }">
        <div class="main">
          <SettingsSection flush>
            <a-tabs v-model:active-key="tab" :animation="false" class="tabs">
              <a-tab-pane key="overview" :title="t('pluginAdmin.overview')">
                <div class="pane">
                  <a-row :gutter="[32, 32]" class="overview-grid">
                    <a-col :xs="24" :lg="12">
                      <section aria-labelledby="shortcut-basic-info">
                        <a-typography-title id="shortcut-basic-info" :heading="6" class="section-heading">{{ t('pluginAdmin.basicInfo') }}</a-typography-title>
                        <a-descriptions :column="1" size="small" :label-style="{ width: '100px' }" :value-style="{ overflowWrap: 'anywhere' }">
                          <a-descriptions-item :label="t('shortcutAdmin.identifier')"><a-typography-text code>{{ uid }}</a-typography-text></a-descriptions-item>
                          <a-descriptions-item :label="t('shortcutAdmin.resultType')">
                            <span class="type-option"><FieldTypeIcon :type="draft.resultType" /> {{ fieldTypeInfo(draft.resultType).label }}</span>
                          </a-descriptions-item>
                          <a-descriptions-item :label="t('pluginAdmin.language')">{{ LANGUAGE_NAMES[draft.language] }}</a-descriptions-item>
                          <a-descriptions-item :label="t('pluginAdmin.installedAt')">{{ formatTime(shortcut?.createdAt) }}</a-descriptions-item>
                          <a-descriptions-item :label="t('pluginAdmin.updatedAt')">{{ formatTime(shortcut?.updatedAt) }}</a-descriptions-item>
                        </a-descriptions>
                      </section>
                    </a-col>
                    <a-col :xs="24" :lg="12">
                      <section aria-labelledby="shortcut-permissions">
                        <a-typography-title id="shortcut-permissions" :heading="6" class="section-heading">{{ t('pluginAdmin.permissions') }}</a-typography-title>
                        <a-row :gutter="[20, 24]" class="permission-grid">
                          <a-col :xs="24" :sm="12">
                            <a-space align="start" :size="12" class="permission-item">
                              <a-avatar :size="28" shape="square" class="permission-icon" :class="draft.domains.length ? 'info' : 'ok'"><Globe :size="15" /></a-avatar>
                              <div class="permission-body">
                                <a-typography-text bold>{{ t('pluginAdmin.network') }}</a-typography-text>
                                <a-typography-text type="secondary" class="permission-description">{{ t(draft.domains.length ? 'pluginAdmin.networkDomains' : 'pluginAdmin.networkNone') }}</a-typography-text>
                                <a-space v-if="draft.domains.length" wrap :size="4" class="permission-tags"><a-tag v-for="domain in draft.domains" :key="domain" size="small" class="mono">{{ domain }}</a-tag></a-space>
                              </div>
                            </a-space>
                          </a-col>
                          <a-col :xs="24" :sm="12">
                            <a-space align="start" :size="12" class="permission-item">
                              <a-avatar :size="28" shape="square" class="permission-icon" :class="draft.aiEnabled ? 'info' : 'neutral'"><Sparkles :size="15" /></a-avatar>
                              <div class="permission-body">
                                <a-typography-text bold>{{ t('shortcutAdmin.aiEnabled') }}</a-typography-text>
                                <a-typography-text type="secondary" class="permission-description">{{ t(draft.aiEnabled ? 'shortcutAdmin.aiAllowed' : 'shortcutAdmin.aiDenied') }}</a-typography-text>
                              </div>
                            </a-space>
                          </a-col>
                          <a-col :xs="24" :sm="12">
                            <a-space align="start" :size="12" class="permission-item">
                              <a-avatar :size="28" shape="square" class="permission-icon" :class="draft.credentials.length ? 'info' : 'neutral'"><KeyRound :size="15" /></a-avatar>
                              <div class="permission-body">
                                <a-typography-text bold>{{ t('shortcutAdmin.credentials') }}</a-typography-text>
                                <a-typography-text type="secondary" class="permission-description">{{ t(draft.credentials.length ? 'shortcutAdmin.credentialPermissionHint' : 'shortcutAdmin.noCredentialPermission') }}</a-typography-text>
                                <a-space v-if="draft.credentials.length" wrap :size="4" class="permission-tags"><a-tag v-for="credential in draft.credentials" :key="credential.key" size="small" class="mono">{{ credential.key }}</a-tag></a-space>
                              </div>
                            </a-space>
                          </a-col>
                          <a-col :xs="24" :sm="12">
                            <a-space align="start" :size="12" class="permission-item">
                              <a-avatar :size="28" shape="square" class="permission-icon info"><Timer :size="15" /></a-avatar>
                              <div class="permission-body">
                                <a-typography-text bold>{{ t('shortcutAdmin.timeout') }}</a-typography-text>
                                <a-typography-text type="secondary" class="permission-description">{{ t('pluginAdmin.timeoutValue', { n: draft.timeoutSeconds }) }}</a-typography-text>
                              </div>
                            </a-space>
                          </a-col>
                        </a-row>
                        <a-alert v-if="draft.aiEnabled" :type="aiLoading ? 'info' : aiReady ? 'success' : 'warning'" class="ai-status">
                          <a-space wrap>
                            <span>{{ aiStatus }}</span>
                            <RouterLink v-slot="{ href, navigate }" :to="{ name: 'admin-ai' }" custom>
                              <a-link :href="href" @click="navigate">{{ t('shortcutAdmin.aiSettings') }}</a-link>
                            </RouterLink>
                          </a-space>
                        </a-alert>
                      </section>
                    </a-col>
                  </a-row>

                  <a-divider class="overview-divider" />

                  <a-row :gutter="[32, 32]" class="overview-grid">
                    <a-col :xs="24" :xl="draft.credentials.length ? 12 : 24">
                      <section aria-labelledby="shortcut-form-items">
                        <a-space :size="8" align="baseline" class="section-heading">
                          <a-typography-title id="shortcut-form-items" :heading="6">{{ t('shortcutAdmin.formItems') }}</a-typography-title>
                          <a-typography-text type="secondary">{{ parsedFormItems?.length ?? 0 }}</a-typography-text>
                        </a-space>
                        <a-table :columns="formItemColumns" :data="parsedFormItems ?? []" :pagination="false" :bordered="false" :expandable="{}" :scroll="{ x: 560 }" size="small" row-key="key" class="definition-table">
                          <template #key="{ record }"><a-typography-text code>{{ record.key }}</a-typography-text></template>
                          <template #component="{ record }">{{ t(formComponentKey(record.component as FormComponent)) }}</template>
                          <template #required="{ record }"><CircleCheck v-if="record.required" :size="15" class="required-icon" /><span v-else class="text-desc">—</span></template>
                          <template #expand-row="{ record }">
                            <a-descriptions :column="1" size="small" class="item-details" :label-style="{ width: '100px' }" :value-style="{ overflowWrap: 'anywhere' }">
                              <a-descriptions-item :label="t('shortcutAdmin.itemPlaceholder')">{{ record.placeholder || '—' }}</a-descriptions-item>
                              <a-descriptions-item v-if="record.component === 'field_select'" :label="t('shortcutAdmin.fieldTypes')">
                                <a-space v-if="record.fieldTypes?.length" wrap :size="4"><a-tag v-for="type in record.fieldTypes" :key="type" size="small">{{ fieldTypeInfo(type).label }}</a-tag></a-space>
                                <span v-else>{{ t('shortcutAdmin.fieldTypesEmpty') }}</span>
                              </a-descriptions-item>
                              <a-descriptions-item v-if="record.component === 'select'" :label="t('shortcutAdmin.options')">
                                <a-space wrap :size="4"><a-tag v-for="option in record.options" :key="option.value" size="small">{{ option.label }} · {{ option.value }}</a-tag></a-space>
                              </a-descriptions-item>
                              <a-descriptions-item :label="t('shortcutAdmin.defaultValue')">{{ record.default || '—' }}</a-descriptions-item>
                            </a-descriptions>
                          </template>
                          <template #empty><a-empty :description="t('shortcutAdmin.noFormItems')" /></template>
                        </a-table>
                      </section>
                    </a-col>
                    <a-col v-if="draft.credentials.length" :xs="24" :xl="12">
                      <section aria-labelledby="shortcut-credentials">
                        <a-typography-title id="shortcut-credentials" :heading="6" class="section-heading">{{ t('shortcutAdmin.credentials') }}</a-typography-title>
                        <a-table :columns="credentialColumns" :data="draft.credentials" :pagination="false" :bordered="false" :scroll="{ x: 480 }" size="small" row-key="key" class="definition-table">
                          <template #key="{ record }"><a-typography-text code>{{ record.key }}</a-typography-text></template>
                          <template #type="{ record }">{{ credentialTypeLabel(record.type) }}</template>
                          <template #name="{ record }"><span class="mono">{{ record.type === 'bearer' ? 'Authorization' : record.name }}</span></template>
                        </a-table>
                      </section>
                    </a-col>
                  </a-row>
                </div>
              </a-tab-pane>

              <a-tab-pane v-if="draft.credentials.length" key="network" :title="t('shortcutAdmin.credentials')">
                <a-form :model="draft" layout="vertical" class="pane credential-values">
                  <a-form-item v-for="c in draft.credentials" :key="c.key" :label="c.key" :extra="`${credentialTypeLabel(c.type)} · ${c.type === 'bearer' ? 'Authorization' : c.name}`">
                    <a-input-password v-model="c.value" :aria-label="c.key" :placeholder="c.hasValue ? t('shortcutAdmin.credentialKeep') : t('shortcutAdmin.credentialValue')" />
                  </a-form-item>
                </a-form>
              </a-tab-pane>

              <a-tab-pane key="code">
                <template #title>
                  <span class="tab-title">
                    {{ t('shortcutAdmin.code') }}
                    <span v-if="sourceDirty" class="tab-dot" />
                  </span>
                </template>
                <div class="pane">
                  <div class="source-toolbar">
                    <a-radio-group v-model="draft.language" type="button" size="small" :aria-label="t('pluginAdmin.language')">
                      <a-radio value="javascript">{{ LANGUAGE_NAMES.javascript }}</a-radio>
                      <a-radio value="typescript">{{ LANGUAGE_NAMES.typescript }}</a-radio>
                    </a-radio-group>
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
                  <p class="text-desc source-hint">{{ t('shortcutAdmin.sourceEditHint') }}</p>
                  <CodeEditor
                    :key="draft.language"
                    v-model="draft.code"
                    :language="draft.language"
                    height="clamp(420px, calc(100vh - 420px), 860px)"
                    :aria-label="t('shortcutAdmin.code')"
                    :error="!!syncError"
                    @submit="test"
                  />
                </div>
              </a-tab-pane>
            </a-tabs>
          </SettingsSection>
        </div>

        <aside v-if="tab === 'code'" class="aside">
          <SettingsSection :title="t('shortcutAdmin.test')">
            <div class="test-panel">
              <div class="panel-label">
                <span>{{ t('shortcutAdmin.params') }}</span>
                <a-button size="mini" type="text" @click="fillSampleParams">
                  <template #icon><WandSparkles :size="13" /></template>
                  {{ t('shortcutAdmin.sampleParams') }}
                </a-button>
              </div>
              <CodeEditor
                v-model="params"
                language="json"
                min-height="120px"
                max-height="260px"
                :error="paramsInvalid"
                :aria-label="t('shortcutAdmin.params')"
                @submit="test"
              />

              <a-button type="primary" long :loading="testing" @click="test">
                <template #icon><Play :size="14" /></template>
                {{ t('shortcutAdmin.run') }}
              </a-button>

              <div v-if="testResult" class="result">
                <div class="result-status" :class="testResult.error ? 'failed' : 'success'">
                  <component :is="testResult.error ? CircleX : CircleCheck" :size="16" />
                  <span>{{ testResult.error ? t('shortcutAdmin.testFailed') : t('shortcutAdmin.testSuccess') }}</span>
                  <span class="result-duration">{{ t('shortcutAdmin.duration', { ms: testResult.durationMs }) }}</span>
                </div>
                <pre v-if="testResult.error" class="output error-output">{{ testResult.error }}</pre>
                <template v-else>
                  <div class="panel-label">{{ t('shortcutAdmin.returned') }}</div>
                  <CodeEditor :model-value="format(testResult.value)" language="javascript" readonly min-height="40px" max-height="220px" />
                  <template v-if="testResult.cellValue !== undefined">
                    <div class="panel-label">{{ t('shortcutAdmin.cellValue') }}</div>
                    <CodeEditor :model-value="format(testResult.cellValue)" language="javascript" readonly min-height="40px" max-height="220px" />
                  </template>
                </template>
                <template v-if="testResult.logs.length">
                  <div class="panel-label">{{ t('shortcutAdmin.logs') }}</div>
                  <pre class="output">{{ testResult.logs.join('\n') }}</pre>
                </template>
              </div>
              <div v-else class="result-empty">
                <FlaskConical :size="22" />
                <span>{{ t('shortcutAdmin.testEmpty') }}</span>
              </div>
            </div>
          </SettingsSection>
        </aside>
      </div>

      <SaveBar v-if="dirty || tab === 'code'" :dirty="dirty" :saving="saving || syncing" @save="save" @reset="discard" />
    </a-spin>

    <FieldShortcutInstallModal v-if="shortcut" v-model:visible="installVisible" :upgrade="shortcut" @installed="onUpgraded" />
  </div>
</template>

<style scoped>
.breadcrumb {
  margin-bottom: 12px;
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
.sync-error {
  margin-bottom: 16px;
}
.source-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.source-hint {
  margin: 0 0 12px;
  font-size: 12.5px;
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
  overflow-wrap: anywhere;
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
.main > * + * {
  margin-top: 16px;
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
.permission-grid :deep(.arco-col) {
  min-width: 0;
}
.permission-item {
  width: 100%;
}
.permission-item :deep(.arco-space-item:last-child) {
  flex: 1;
  min-width: 0;
}
.permission-icon {
  flex: none;
  border-radius: 8px;
}
.permission-icon.info {
  color: var(--color-primary);
  background: var(--bg-primary-soft);
}
.permission-icon.ok {
  color: var(--color-success);
  background: rgba(var(--success-6), 0.12);
}
.permission-icon.neutral {
  color: var(--text-caption);
  background: var(--bg-base);
}
.permission-body {
  min-width: 0;
  padding-top: 3px;
  overflow-wrap: anywhere;
}
.permission-body :deep(.arco-typography) {
  font-size: 13px;
  line-height: 1.6;
}
.permission-description {
  display: block;
  margin-top: 2px;
}
.permission-tags {
  margin-top: 6px;
}
.permission-tags :deep(.arco-tag) {
  height: auto;
  max-width: 100%;
  white-space: normal;
}
.ai-status {
  margin-top: 20px;
}
.definition-table :deep(.arco-table-th) {
  background: var(--bg-base);
}
.required-icon {
  color: var(--color-success);
}
.mono {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
.item-details {
  padding: 8px 16px;
}
.credential-values :deep(.arco-form-item) {
  max-width: 560px;
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
  background: var(--color-danger);
}
.type-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.test-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  /* Matches the left pane's 20px top padding; the card body already has 8px. */
  padding: 12px 0 16px;
}
.panel-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 28px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-title);
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
.result-duration {
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
@media (max-width: 1180px) {
  .workspace {
    grid-template-columns: minmax(0, 1fr);
  }
  .aside {
    position: static;
  }
}
@media (max-width: 768px) {
  .hero-text {
    min-width: 0;
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
  .source-toolbar {
    flex-wrap: wrap;
  }
}
</style>
