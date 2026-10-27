<script setup lang="ts">
import { localText, type LocalText } from '@/utils/localText'
import { Message, type FileItem } from '@arco-design/web-vue'
import { ArrowRight, CodeXml, ExternalLink, FileCode2, Link2, Upload, X } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminShortcutsApi, type AdminFieldShortcut } from '@/api/shortcut'
import SHORTCUT_STARTER from '@/assets/templates/field-shortcut.js?raw'
import HubFetchState from '@/components/admin/HubFetchState.vue'
import HubReleaseCard from '@/components/admin/HubReleaseCard.vue'
import WorkflowPluginIcon from '@/components/admin/WorkflowPluginIcon.vue'
import CodeEditor from '@/components/common/CodeEditor.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import type { FieldType } from '@/types/bitable'
import { attachmentSize } from '@/utils/attachments'
import { fieldTypeInfo } from '@/utils/fieldTypes'
import { fieldLabel, saveBody, shortcutFields } from '@/utils/shortcut'
import { LANGUAGE_NAMES, languageOf, versionLabel, type PluginLanguage } from '@/utils/workflowPlugin'

const props = defineProps<{
  upgrade?: AdminFieldShortcut | null
  /** Downloads the latest release of the upgraded shortcut from Sayrud Hub instead of asking for a source. */
  fromHub?: boolean
  /** Installs the latest release of this Sayrud Hub package, picked in its catalog. */
  hubID?: string
  /** Name of the Sayrud Hub package, shown while its release is downloaded. */
  hubName?: string
}>()
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ installed: [shortcut: AdminFieldShortcut] }>()
const { t, locale } = useI18n()
const text = (value: LocalText | undefined) => localText(value, locale.value)

const MAX_SOURCE_SIZE = 200 * 1024
const step = ref(1)
const hubMode = ref(false)
const source = ref<'file' | 'url' | 'code'>('file')
const file = ref<{ name: string; size: number; text: string } | null>(null)
const url = ref('')
const code = ref(SHORTCUT_STARTER)
const language = ref<PluginLanguage>('javascript')
const error = ref('')
const reading = ref(false)
const inspecting = ref(false)
const installing = ref(false)
const inspection = ref<AdminFieldShortcut | null>(null)
const inspectionFields = computed(() => (inspection.value ? shortcutFields(inspection.value) : []))

const busy = computed(() => reading.value || inspecting.value || installing.value)
const hubPackage = computed(() => props.hubID || (props.fromHub ? props.upgrade?.id : undefined))
const canInspect = computed(() => {
  if (source.value === 'file') return !!file.value
  if (source.value === 'url') return /^https?:\/\/\S+$/i.test(url.value.trim())

  return code.value.trim() !== ''
})

let requestId = 0
watch(
  visible,
  (open) => {
    requestId++
    step.value = 1
    source.value = 'file'
    file.value = null
    url.value = ''
    error.value = ''
    inspection.value = null
    reading.value = false
    inspecting.value = false
    installing.value = false

    hubMode.value = open && !!hubPackage.value
    if (open) {
      code.value = props.upgrade?.code ?? SHORTCUT_STARTER
      language.value = props.upgrade?.language ?? 'javascript'
      if (hubMode.value) void inspectHub()
    }
  },
  { immediate: true },
)

watch([source, url, code, language], () => (error.value = ''))

async function onFile(_: FileItem[], item: FileItem) {
  const f = item.file
  if (!f || busy.value) return

  if (!/\.m?[jt]s$/i.test(f.name)) {
    Message.warning(t('shortcutAdmin.invalidFileType'))
    return
  }

  if (f.size > MAX_SOURCE_SIZE) {
    Message.warning(t('pluginAdmin.fileTooLarge'))
    return
  }

  const request = requestId
  reading.value = true
  error.value = ''

  try {
    const text = await f.text()
    if (request === requestId) file.value = { name: f.name, size: f.size, text }
  } catch (e) {
    if (request === requestId) error.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (request === requestId) reading.value = false
  }
}

async function inspect() {
  if (!canInspect.value || busy.value) return

  const request = requestId
  inspecting.value = true
  error.value = ''

  try {
    const result = await adminShortcutsApi.inspect(
      source.value === 'url'
        ? { url: url.value.trim() }
        : source.value === 'file'
          ? { source: file.value!.text, language: languageOf(file.value!.name) }
          : { source: code.value, language: language.value },
    )
    if (request !== requestId) return

    inspection.value = result
    step.value = 2
  } catch (e) {
    if (request === requestId) error.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (request === requestId) inspecting.value = false
  }
}

async function inspectHub() {
  const hubID = hubPackage.value
  if (!hubID || busy.value) return

  const request = requestId
  inspecting.value = true
  error.value = ''

  try {
    const result = await adminShortcutsApi.inspect({ hubID })
    if (request !== requestId) return

    inspection.value = result
    step.value = 2
  } catch (e) {
    if (request === requestId) error.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (request === requestId) inspecting.value = false
  }
}

function chooseSource() {
  hubMode.value = false
  error.value = ''
}

async function install() {
  if (!inspection.value || busy.value) return

  installing.value = true

  try {
    const i = inspection.value
    const upgrade = props.upgrade
    const shortcut = upgrade
      ? await adminShortcutsApi.update(upgrade.uid, saveBody(i, { enabled: upgrade.enabled, ...(i.signature && { signature: i.signature }) }))
      : await adminShortcutsApi.install(i.code, i.language, i.signature)
    Message.success(t(upgrade ? 'pluginAdmin.upgradeSuccess' : 'pluginAdmin.installSuccess', { name: text(shortcut.name) }))
    emit('installed', shortcut)
    visible.value = false
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    installing.value = false
  }
}
</script>

<template>
  <a-modal
    v-model:visible="visible"
    width="min(680px, calc(100vw - 32px))"
    :mask-closable="false"
    :closable="!busy"
    :esc-to-close="!busy"
    unmount-on-close
    :title="upgrade ? t('pluginAdmin.upgradeTitle', { name: text(upgrade.name) }) : t('shortcutAdmin.installTitle')"
  >
    <HubFetchState
      v-if="step === 1 && hubMode"
      :loading="inspecting"
      :error="error"
      :name="upgrade ? text(upgrade.name) : (hubName ?? '')"
      @retry="inspectHub"
      @choose-source="chooseSource"
    />

    <div v-else-if="step === 1" class="source-step">
      <a-radio-group v-model="source" type="button" size="large" :disabled="busy" class="source-options">
        <a-radio value="file"><span class="inline"><Upload :size="16" />{{ t('pluginAdmin.sourceFile') }}</span></a-radio>
        <a-radio value="url"><span class="inline"><Link2 :size="16" />{{ t('pluginAdmin.sourceURL') }}</span></a-radio>
        <a-radio value="code"><span class="inline"><CodeXml :size="16" />{{ t('pluginAdmin.sourceCode') }}</span></a-radio>
      </a-radio-group>

      <div v-if="source === 'file'" class="source-body">
        <div v-if="file" class="file-card">
          <FileCode2 :size="24" class="file-icon" />
          <div class="file-text">
            <div class="file-name ellipsis">{{ file.name }}</div>
            <div class="text-desc">{{ LANGUAGE_NAMES[languageOf(file.name)] }} · {{ attachmentSize(file.size) }}</div>
          </div>
          <a-button type="text" shape="circle" size="small" :disabled="busy" :aria-label="t('common.delete')" @click="file = null">
            <template #icon><X :size="15" /></template>
          </a-button>
        </div>
        <a-upload v-else draggable :auto-upload="false" :show-file-list="false" accept=".js,.mjs,.ts,.mts" :disabled="busy" @change="onFile">
          <template #upload-button>
            <a-spin :loading="reading">
              <div class="dropzone">
                <span class="dropzone-icon"><Upload :size="22" /></span>
                <div class="file-name">{{ t('shortcutAdmin.dropFile') }}</div>
                <div class="text-desc">{{ t('shortcutAdmin.dropFileHint', { size: attachmentSize(MAX_SOURCE_SIZE) }) }}</div>
              </div>
            </a-spin>
          </template>
        </a-upload>
      </div>

      <div v-else-if="source === 'url'" class="source-body">
        <a-input v-model="url" size="large" allow-clear :disabled="busy" :placeholder="t('shortcutAdmin.urlPlaceholder')" @press-enter="inspect">
          <template #prefix><Link2 :size="16" /></template>
        </a-input>
        <p class="text-desc hint">{{ t('pluginAdmin.urlHint') }}</p>
      </div>

      <div v-else class="source-body">
        <div class="code-toolbar">
          <a-radio-group v-model="language" type="button" size="small" :disabled="busy">
            <a-radio value="javascript">{{ LANGUAGE_NAMES.javascript }}</a-radio>
            <a-radio value="typescript">{{ LANGUAGE_NAMES.typescript }}</a-radio>
          </a-radio-group>
          <a-button type="text" size="small" :disabled="busy" @click="code = SHORTCUT_STARTER">{{ t('pluginAdmin.resetTemplate') }}</a-button>
        </div>
        <CodeEditor :key="language" v-model="code" :language="language" height="340px" :aria-label="t('pluginAdmin.sourceCode')" @submit="inspect" />
      </div>

      <a-alert v-if="error" type="error" class="error">{{ error }}</a-alert>
    </div>

    <div v-else-if="inspection" class="review-step">
      <div class="review-head">
        <WorkflowPluginIcon :icon="inspection.icon || (inspection.permissions.ai ? 'sparkles' : 'zap')" :color="inspection.color || 'var(--color-primary)'" :size="44" />
        <div class="review-title">
          <div class="review-name">
            <span>{{ text(inspection.name) }}</span>
            <a-tag size="small">{{ versionLabel(inspection.version) }}</a-tag>
            <a-tag size="small">{{ LANGUAGE_NAMES[inspection.language] }}</a-tag>
            <a-tag v-if="upgrade && upgrade.version !== inspection.version" size="small" color="arcoblue">
              {{ t('pluginAdmin.upgradeFrom', { from: upgrade.version, to: inspection.version }) }}
            </a-tag>
          </div>
          <div v-if="text(inspection.description)" class="text-desc">{{ text(inspection.description) }}</div>
        </div>
        <a-link v-if="inspection.hub" :href="inspection.hub.url" target="_blank" rel="noopener noreferrer" class="hub-link">
          {{ t('hub.viewOnHub') }}<ExternalLink :size="12" />
        </a-link>
      </div>

      <HubReleaseCard v-if="inspection.hub" :hub="inspection.hub" />

      <a-descriptions :column="2" layout="vertical" class="review-details">
        <a-descriptions-item v-if="inspection.homepage" :label="t('pluginAdmin.homepage')">
          <a-link :href="inspection.homepage" target="_blank" rel="noopener noreferrer">{{ inspection.homepage }}</a-link>
        </a-descriptions-item>
        <a-descriptions-item :label="t('shortcutAdmin.resultType')">
          <span class="inline">
            <FieldTypeIcon :type="inspection.resultType as FieldType" />
            {{ fieldTypeInfo(inspection.resultType as FieldType).label }}
          </span>
        </a-descriptions-item>
        <a-descriptions-item :label="t('shortcutAdmin.timeout')">{{ t('pluginAdmin.timeoutValue', { n: inspection.timeout }) }}</a-descriptions-item>
        <a-descriptions-item :label="t('shortcutAdmin.aiEnabled')">
          {{ t(inspection.permissions.ai ? 'shortcutAdmin.aiUsed' : 'shortcutAdmin.aiNotUsed') }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('shortcutAdmin.formItems')">
          <div v-if="inspectionFields.length" class="tags">
            <a-tag v-for="item in inspectionFields" :key="item.key" size="small">{{ fieldLabel(item, locale) }}<span v-if="item.required" class="required">*</span></a-tag>
          </div>
          <span v-else class="text-desc">{{ t('pluginAdmin.noInputs') }}</span>
        </a-descriptions-item>
        <a-descriptions-item :label="t('shortcutAdmin.domains')" :span="2">
          <div v-if="inspection.permissions.fetch.length" class="tags">
            <span v-if="inspection.permissions.fetch.includes('*')" class="text-desc">{{ t('pluginAdmin.networkAny') }}</span>
            <a-tag v-for="host in inspection.permissions.fetch.filter((h) => h !== '*')" :key="host" size="small">{{ host }}</a-tag>
          </div>
          <span v-else class="text-desc">{{ t('pluginAdmin.networkNone') }}</span>
        </a-descriptions-item>
        <a-descriptions-item v-if="inspection.credentials.length" :label="t('shortcutAdmin.credentials')" :span="2">
          <div class="tags">
            <a-tag v-for="credential in inspection.credentials" :key="credential.key" size="small">{{ text(credential.label) }} · {{ credential.type }}</a-tag>
          </div>
          <p class="text-desc hint">{{ t('shortcutAdmin.credentialsHint') }}</p>
        </a-descriptions-item>
      </a-descriptions>

      <a-alert v-if="!inspection.hub?.official" type="warning">{{ t('shortcutAdmin.trustWarning') }}</a-alert>
    </div>

    <template #footer>
      <a-space>
        <template v-if="step === 1 && hubMode">
          <a-button :disabled="busy" @click="visible = false">{{ t('common.cancel') }}</a-button>
        </template>
        <template v-else-if="step === 1">
          <a-button :disabled="busy" @click="visible = false">{{ t('common.cancel') }}</a-button>
          <a-button type="primary" :disabled="!canInspect || reading" :loading="inspecting" @click="inspect">
            <span class="inline">{{ t('pluginAdmin.next') }}<ArrowRight v-if="!inspecting" :size="15" /></span>
          </a-button>
        </template>
        <template v-else>
          <a-button v-if="hubMode" :disabled="installing" @click="visible = false">{{ t('common.cancel') }}</a-button>
          <a-button v-else :disabled="installing" @click="step = 1">{{ t('pluginAdmin.back') }}</a-button>
          <a-button type="primary" :loading="installing" @click="install">{{ t(upgrade ? 'pluginAdmin.confirmUpgrade' : 'pluginAdmin.confirmInstall') }}</a-button>
        </template>
      </a-space>
    </template>
  </a-modal>
</template>

<style scoped>
.source-options {
  display: flex;
}
.source-options :deep(.arco-radio-button) {
  flex: 1;
  text-align: center;
}
.inline {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.source-body {
  margin-top: 16px;
}
.code-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.source-body :deep(.arco-upload),
.source-body :deep(.arco-upload-wrapper),
.source-body :deep(.arco-spin) {
  width: 100%;
}
.dropzone {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 36px 16px;
  border: 1px dashed var(--line-border-strong);
  border-radius: 8px;
  background: var(--bg-base);
  text-align: center;
  cursor: pointer;
  transition: border-color 0.15s, background-color 0.15s;
}
.dropzone:hover,
:deep(.arco-upload-drag-active) .dropzone {
  border-color: var(--color-primary);
  background: var(--bg-primary-soft);
}
.dropzone-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  margin-bottom: 4px;
  border-radius: 50%;
  background: var(--bg-primary-soft);
  color: var(--color-primary);
}
.file-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  background: var(--bg-base);
}
.file-icon {
  flex: none;
  color: var(--color-primary);
}
.file-text,
.review-title {
  flex: 1;
  min-width: 0;
}
.file-name,
.review-name {
  font-weight: 500;
  color: var(--text-title);
}
.hint {
  margin: 8px 0 0;
  font-size: 12px;
}
.error {
  margin-top: 16px;
}
.review-head {
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  overflow-wrap: anywhere;
}
.hub-link {
  align-self: start;
  gap: 4px;
  font-size: 13px;
  white-space: nowrap;
}
.review-name {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 4px;
  font-size: 17px;
}
.review-details {
  margin: 20px 0 8px;
}
.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.tags :deep(.arco-tag) {
  max-width: 100%;
  height: auto;
  white-space: normal;
  overflow-wrap: anywhere;
}
.required {
  margin-left: 3px;
  color: var(--color-danger);
}
@media (max-width: 480px) {
  .review-head {
    grid-template-columns: 44px minmax(0, 1fr);
  }
  .hub-link {
    grid-column: 2;
    justify-self: end;
  }
  .source-options {
    flex-direction: column;
  }
}
</style>
