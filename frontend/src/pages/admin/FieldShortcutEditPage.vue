<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { CircleCheck, CircleX, FlaskConical, Play, Plus, Trash2, WandSparkles } from '@lucide/vue'
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
import CodeEditor from '@/components/common/CodeEditor.vue'
import ShortcutFormItemsEditor from '@/components/admin/ShortcutFormItemsEditor.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SaveBar from '@/components/console/SaveBar.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import type { FieldType } from '@/types/bitable'
import { fieldTypeInfo } from '@/utils/fieldTypes'
import { formItemIssueKey, formItemIssueName, loadFormItems, validateFormItems } from '@/utils/shortcutFormItems'
import { sampleShortcutParams, shortcutStarter, SHORTCUT_HOST_TYPES } from '@/utils/shortcut'

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
  description: string
  resultType: FieldType
  code: string
  formItems: string
  domains: string[]
  credentials: CredentialDraft[]
  timeoutSeconds: number
  enabled: boolean
  aiEnabled: boolean
}

function toDraft(s: AdminFieldShortcut | null): Draft {
  const starter = shortcutStarter(t('shortcutAdmin.templateText'))

  return {
    name: s?.name ?? '',
    description: s?.description ?? '',
    resultType: (s?.resultType ?? starter.resultType) as FieldType,
    code: s?.code ?? starter.code,
    formItems: JSON.stringify(s?.formItems ?? starter.formItems, null, 2),
    domains: [...(s?.domains ?? [])],
    credentials: (s?.credentials ?? []).map((c) => ({ key: c.key, type: c.type as CredentialType, name: c.name ?? '', value: '', hasValue: c.hasValue })),
    timeoutSeconds: s?.timeoutSeconds ?? starter.timeoutSeconds,
    enabled: s?.enabled ?? true,
    aiEnabled: s?.aiEnabled ?? starter.aiEnabled,
  }
}

const uid = computed(() => (typeof route.params.shortcutUID === 'string' ? route.params.shortcutUID : ''))
const shortcut = ref<AdminFieldShortcut | null>(null)
const loading = ref(false)
const notFound = ref(false)
const saving = ref(false)

const draft = reactive<Draft>(toDraft(null))
const baseline = ref(JSON.stringify(draft))
const dirty = computed(() => JSON.stringify(draft) !== baseline.value)

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
  // Reuse the saved shortcut when creation navigates to its edit route.
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

const TABS = ['basic', 'formItems', 'code', 'network'] as const
type Tab = (typeof TABS)[number]

// Keep the active tab in the query so it survives a refresh.
const tab = computed<Tab>({
  get: () => (TABS.includes(route.query.tab as Tab) ? (route.query.tab as Tab) : 'basic'),
  set: (value) => router.replace({ query: { ...route.query, tab: value === 'basic' ? undefined : value } }),
})

const isNew = computed(() => !uid.value)
const title = computed(() => {
  if (shortcut.value) return t('shortcutAdmin.editTitle', { name: shortcut.value.name })
  return isNew.value ? t('shortcutAdmin.createTitle') : ''
})

/** Parsed form-item array when the JSON can be loaded into cards, otherwise null. */
const parsedFormItems = computed<ShortcutFormItem[] | null>(() => {
  const loaded = loadFormItems(draft.formItems)
  if (!loaded.ok) return null
  return JSON.parse(draft.formItems) as ShortcutFormItem[]
})

const formItemsInvalid = computed(() => {
  const loaded = loadFormItems(draft.formItems)
  return !loaded.ok || validateFormItems(loaded.items).length > 0
})

const paramsInvalid = computed(() => {
  try {
    const p = JSON.parse(params.value || '{}') as unknown
    return typeof p !== 'object' || p === null || Array.isArray(p)
  } catch {
    return true
  }
})

/** Returns the request body. Warns and stays on the form-items tab when the JSON cannot be loaded or is invalid. */
function body(): SaveFieldShortcut | null {
  const loaded = loadFormItems(draft.formItems)
  if (!loaded.ok) {
    Message.warning(t('shortcutAdmin.invalidFormItems'))
    tab.value = 'formItems'
    return null
  }
  const issue = validateFormItems(loaded.items)[0]
  if (issue) {
    Message.warning(t(formItemIssueKey(issue), { key: formItemIssueName(loaded.items, issue) }))
    tab.value = 'formItems'
    return null
  }
  return {
    name: draft.name,
    description: draft.description,
    resultType: draft.resultType as SaveFieldShortcut['resultType'],
    code: draft.code,
    formItems: JSON.parse(draft.formItems) as ShortcutFormItem[],
    domains: draft.domains,
    credentials: draft.credentials.map((c) => ({ key: c.key, type: c.type, name: c.type === 'bearer' ? undefined : c.name, value: c.value || undefined })),
    timeoutSeconds: draft.timeoutSeconds,
    enabled: draft.enabled,
    aiEnabled: draft.aiEnabled,
  }
}

function addCredential() {
  draft.credentials.push({ key: `key${draft.credentials.length + 1}`, type: 'bearer', name: '', value: '', hasValue: false })
}

function fillSampleParams() {
  if (parsedFormItems.value) params.value = sampleShortcutParams(parsedFormItems.value)
  else Message.warning(t('shortcutAdmin.invalidFormItems'))
}

async function test() {
  if (testing.value) return
  const b = body()
  if (!b) return
  if (paramsInvalid.value) {
    Message.warning(t('shortcutAdmin.invalidParams'))
    return
  }
  testing.value = true
  try {
    testResult.value = await adminShortcutsApi.test(b, JSON.parse(params.value || '{}') as Record<string, unknown>, shortcut.value?.uid)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    testing.value = false
  }
}

async function save() {
  if (saving.value) return
  const b = body()
  if (!b) return
  saving.value = true
  try {
    const created = !shortcut.value
    const saved = shortcut.value ? await adminShortcutsApi.update(shortcut.value.uid, b) : await adminShortcutsApi.create(b)
    shortcut.value = saved
    // Use the server response as the baseline, clear credential values, and preserve the test result.
    const result = testResult.value
    const currentParams = params.value
    reset(saved)
    testResult.value = result
    params.value = currentParams
    Message.success(t('common.saved'))
    if (created) router.replace({ name: 'admin-field-shortcut-edit', params: { shortcutUID: saved.uid }, query: route.query })
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

function discard() {
  reset(shortcut.value)
}

function remove() {
  const s = shortcut.value
  if (!s) return
  Modal.confirm({
    title: t('shortcutAdmin.deleteTitle', { name: s.name }),
    content: s.usedBy ? t('shortcutAdmin.deleteInUse', { n: s.usedBy }, s.usedBy) : t('shortcutAdmin.deleteContent'),
    okButtonProps: { status: 'danger' },
    okText: t('common.delete'),
    onBeforeOk: async () => {
      try {
        await adminShortcutsApi.delete(s.uid)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      baseline.value = JSON.stringify(draft)
      Message.success(t('common.deleted'))
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
      <a-breadcrumb-item>{{ shortcut?.name ?? (isNew ? t('shortcutAdmin.createTitle') : uid) }}</a-breadcrumb-item>
    </a-breadcrumb>

    <a-result v-if="notFound" status="404" :subtitle="t('shortcutAdmin.notFound')">
      <template #extra>
        <a-button type="primary" @click="router.push({ name: 'admin-field-shortcuts' })">{{ t('shortcutAdmin.backToList') }}</a-button>
      </template>
    </a-result>

    <a-spin v-else :loading="loading" class="spin">
      <PageHeader :title="title" :description="isNew ? t('shortcutAdmin.createDescription') : undefined" class="editor-header">
        <template #extra>
          <a-tooltip :content="t('shortcutAdmin.enabledDescription')">
            <div class="shortcut-status">
              <span>{{ draft.enabled ? t('shortcutAdmin.enabledTag') : t('shortcutAdmin.disabledTag') }}</span>
              <a-switch v-model="draft.enabled" :aria-label="t('shortcutAdmin.enabled')" />
            </div>
          </a-tooltip>
          <a-button v-if="shortcut" status="danger" @click="remove">
            <template #icon><Trash2 :size="16" /></template>
            {{ t('shortcutAdmin.dangerTitle') }}
          </a-button>
        </template>
      </PageHeader>

      <div class="workspace">
        <div class="main">
          <div class="tabs-card">
            <a-tabs v-model:active-key="tab" :animation="false" class="tabs">
              <a-tab-pane key="basic" :title="t('shortcutAdmin.basic')">
                <a-form :model="draft" layout="vertical" class="pane">
                  <div class="grid">
                    <a-form-item :label="t('shortcutAdmin.name')" required>
                      <a-input v-model="draft.name" :max-length="64" show-word-limit :placeholder="t('shortcutAdmin.namePlaceholder')" />
                    </a-form-item>
                    <a-form-item :label="t('shortcutAdmin.resultType')" :extra="t('shortcutAdmin.resultTypeDescription')">
                      <a-select v-model="draft.resultType">
                        <a-option v-for="rt in SHORTCUT_HOST_TYPES" :key="rt" :value="rt">
                          <span class="type-option"><FieldTypeIcon :type="rt" /> {{ fieldTypeInfo(rt).label }}</span>
                        </a-option>
                      </a-select>
                    </a-form-item>
                  </div>
                  <a-form-item :label="t('shortcutAdmin.description')">
                    <a-textarea
                      v-model="draft.description"
                      :max-length="500"
                      show-word-limit
                      :auto-size="{ minRows: 3, maxRows: 6 }"
                      :placeholder="t('shortcutAdmin.descriptionPlaceholder')"
                    />
                  </a-form-item>
                  <div class="grid">
                    <a-form-item :label="t('shortcutAdmin.timeout')" :extra="t('shortcutAdmin.timeoutDescription')">
                      <a-input-number v-model="draft.timeoutSeconds" :min="1" :max="300" class="timeout">
                        <template #suffix>{{ t('shortcutAdmin.seconds') }}</template>
                      </a-input-number>
                    </a-form-item>
                    <a-form-item :label="t('shortcutAdmin.aiEnabled')" :extra="t('shortcutAdmin.aiEnabledDescription')">
                      <a-switch v-model="draft.aiEnabled" />
                    </a-form-item>
                  </div>
                  <a-alert v-if="draft.aiEnabled" :type="aiLoading ? 'info' : aiReady ? 'success' : 'warning'">
                    <a-space wrap>
                      <span>{{ aiStatus }}</span>
                      <RouterLink v-slot="{ href, navigate }" :to="{ name: 'admin-ai' }" custom>
                        <a-link :href="href" @click="navigate">{{ t('shortcutAdmin.aiSettings') }}</a-link>
                      </RouterLink>
                    </a-space>
                  </a-alert>
                </a-form>
              </a-tab-pane>

              <a-tab-pane key="formItems">
                <template #title>
                  <span class="tab-title">
                    {{ t('shortcutAdmin.formItems') }}
                    <span v-if="formItemsInvalid" class="tab-dot" />
                  </span>
                </template>
                <div class="pane">
                  <ShortcutFormItemsEditor :key="baseline" v-model="draft.formItems" />
                </div>
              </a-tab-pane>

              <a-tab-pane key="code" :title="t('shortcutAdmin.code')">
                <div class="pane">
                  <CodeEditor
                    v-model="draft.code"
                    language="javascript"
                    height="clamp(420px, calc(100vh - 380px), 860px)"
                    :aria-label="t('shortcutAdmin.code')"
                    @submit="test"
                  />
                </div>
              </a-tab-pane>

              <a-tab-pane key="network" :title="t('shortcutAdmin.network')">
                <a-form :model="draft" layout="vertical" class="pane">
                  <a-form-item :label="t('shortcutAdmin.domains')" :extra="t('shortcutAdmin.domainsDescription')">
                    <a-input-tag v-model="draft.domains" :placeholder="t('shortcutAdmin.domainsPlaceholder')" allow-clear unique-value />
                  </a-form-item>
                  <a-form-item :label="t('shortcutAdmin.credentials')">
                    <div class="credentials">
                      <div v-if="draft.credentials.length" class="credential-table">
                        <div class="credential-row credential-head">
                          <span>{{ t('shortcutAdmin.credentialKey') }}</span>
                          <span>{{ t('shortcutAdmin.credentialType') }}</span>
                          <span>{{ t('shortcutAdmin.credentialName') }}</span>
                          <span>{{ t('shortcutAdmin.credentialValue') }}</span>
                          <span />
                        </div>
                        <div v-for="(c, i) in draft.credentials" :key="i" class="credential-row">
                          <a-input v-model="c.key" :placeholder="t('shortcutAdmin.credentialKey')" class="mono-input" />
                          <a-select v-model="c.type">
                            <a-option value="bearer">Bearer</a-option>
                            <a-option value="header">{{ t('shortcutAdmin.credentialHeader') }}</a-option>
                            <a-option value="query">{{ t('shortcutAdmin.credentialQuery') }}</a-option>
                          </a-select>
                          <a-input v-if="c.type !== 'bearer'" v-model="c.name" :placeholder="t('shortcutAdmin.credentialName')" class="mono-input" />
                          <span v-else class="text-desc credential-na">Authorization</span>
                          <a-input-password
                            v-model="c.value"
                            :placeholder="c.hasValue ? t('shortcutAdmin.credentialKeep') : t('shortcutAdmin.credentialValue')"
                          />
                          <a-button type="text" status="danger" shape="square" :aria-label="t('common.delete')" @click="draft.credentials.splice(i, 1)">
                            <template #icon><Trash2 :size="15" /></template>
                          </a-button>
                        </div>
                      </div>
                      <div v-else class="credential-empty text-desc">{{ t('shortcutAdmin.noCredentials') }}</div>
                      <div>
                        <a-button size="small" @click="addCredential">
                          <template #icon><Plus :size="14" /></template>
                          {{ t('shortcutAdmin.addCredential') }}
                        </a-button>
                      </div>
                    </div>
                  </a-form-item>
                </a-form>
              </a-tab-pane>
            </a-tabs>
          </div>
        </div>

        <aside class="aside">
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

      <SaveBar :dirty="dirty || isNew" :saving="saving" @save="save" @reset="discard" />
    </a-spin>
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
}

.editor-header > :deep(.arco-space) {
  flex-wrap: wrap;
  max-width: 100%;
}

.shortcut-status {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--text-title);
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
}

.mono-input :deep(input) {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12.5px;
}
.workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 400px;
  gap: 16px;
  align-items: start;
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
.tabs-card {
  overflow: hidden;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  background: var(--bg-body);
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
  background: var(--color-danger);
}
.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 24px;
}

.type-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.timeout {
  width: 180px;
  max-width: 100%;
}

.credentials {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}
.credential-table {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.credential-row {
  display: grid;
  grid-template-columns: 140px 128px 160px minmax(0, 1fr) 32px;
  gap: 8px;
  align-items: center;
}
.credential-head {
  font-size: 12px;
  color: var(--text-caption);
}
.credential-na {
  padding: 0 12px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12.5px;
}
.credential-empty {
  padding: 14px 16px;
  border: 1px dashed var(--line-border-strong);
  border-radius: 6px;
  text-align: center;
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
  .grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .credential-row {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }
  .credential-row:not(.credential-head) + .credential-row {
    padding-top: 8px;
    border-top: 1px solid var(--line-border);
  }
  .credential-head {
    display: none;
  }
}
</style>
