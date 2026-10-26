<script setup lang="ts">
import { Message, type FileItem } from '@arco-design/web-vue'
import { ArrowRight, CodeXml, FileCode2, Link2, Upload, X } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { workflowPluginsApi, type PluginInspection, type PluginLanguage, type WorkflowPlugin } from '@/api/workflowPlugin'
import PLUGIN_STARTER from '@/assets/templates/workflow-plugin.js?raw'
import WorkflowPluginIcon from '@/components/admin/WorkflowPluginIcon.vue'
import WorkflowPluginPermissions from '@/components/admin/WorkflowPluginPermissions.vue'
import CodeEditor from '@/components/common/CodeEditor.vue'
import { attachmentSize } from '@/utils/attachments'
import { dslText } from '@/utils/modules'
import { LANGUAGE_NAMES, MAX_PLUGIN_SOURCE, languageOf, versionLabel } from '@/utils/workflowPlugin'

/** Installs a node plugin from a file, a URL or code written in place, after reviewing what it can do. */
const props = defineProps<{
  /** The installed plugin to upgrade; the URL it was downloaded from is filled in. */
  upgrade?: WorkflowPlugin | null
  /** Opens with the code editor and the starter template. */
  startWithCode?: boolean
}>()
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ installed: [plugin: WorkflowPlugin] }>()
const { t } = useI18n()

type Source = 'file' | 'url' | 'code'

const step = ref<1 | 2>(1)
const source = ref<Source>('file')
const file = ref<{ name: string; size: number; text: string } | null>(null)
const url = ref('')
const code = ref(PLUGIN_STARTER)
const language = ref<PluginLanguage>('javascript')
const error = ref('')
const inspecting = ref(false)
const installing = ref(false)
const inspection = ref<PluginInspection | null>(null)
const busy = computed(() => inspecting.value || installing.value)

watch(
  visible,
  (open) => {
    if (!open) return
    step.value = 1
    error.value = ''
    inspection.value = null
    file.value = null
    url.value = props.upgrade?.sourceURL ?? ''
    source.value = props.startWithCode ? 'code' : props.upgrade?.sourceURL ? 'url' : 'file'
  },
  { immediate: true },
)
watch([source, url, code, language], () => (error.value = ''))

const title = computed(() => (props.upgrade ? t('pluginAdmin.upgradeTitle', { name: props.upgrade.module.label }) : t('pluginAdmin.installTitle')))

async function onFile(_: FileItem[], item: FileItem) {
  const f = item.file
  if (!f) return
  if (f.size > MAX_PLUGIN_SOURCE) {
    Message.warning(t('pluginAdmin.fileTooLarge'))
    return
  }
  file.value = { name: f.name, size: f.size, text: await f.text() }
  error.value = ''
}

function resetTemplate() {
  code.value = PLUGIN_STARTER
  language.value = 'javascript'
}

const canInspect = computed(() => {
  if (source.value === 'file') return !!file.value
  if (source.value === 'url') return /^https?:\/\/\S+$/i.test(url.value.trim())
  return code.value.trim() !== ''
})

async function inspect() {
  if (!canInspect.value || inspecting.value) return
  inspecting.value = true
  error.value = ''
  try {
    inspection.value = await workflowPluginsApi.inspect(
      source.value === 'url'
        ? { url: url.value.trim() }
        : source.value === 'file'
          ? { source: file.value!.text, language: languageOf(file.value!.name) }
          : { source: code.value, language: language.value },
    )
    step.value = 2
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    inspecting.value = false
  }
}

async function install() {
  const i = inspection.value
  if (!i || installing.value) return
  installing.value = true
  try {
    const plugin = await workflowPluginsApi.install(i.source, i.language, source.value === 'url' ? url.value.trim() : undefined)
    Message.success(i.installed ? t('pluginAdmin.upgradeSuccess', { name: plugin.module.label }) : t('pluginAdmin.installSuccess', { name: plugin.module.label }))
    emit('installed', plugin)
    visible.value = false
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    installing.value = false
  }
}

const plugin = computed(() => inspection.value?.plugin)
const installed = computed(() => inspection.value?.installed)
const replacesOther = computed(() => !!props.upgrade && !!plugin.value && plugin.value.id !== props.upgrade.id)
const inputs = computed(() => plugin.value?.module.sections.flatMap((s) => s.fields).filter((f) => f.key) ?? [])
</script>

<template>
  <a-modal
    v-model:visible="visible"
    width="min(680px, calc(100vw - 32px))"
    :mask-closable="false"
    :closable="!busy"
    :esc-to-close="!busy"
    unmount-on-close
    :title="title"
  >
    <div v-if="step === 1" class="source-step">
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
            <div class="dropzone">
              <span class="dropzone-icon"><Upload :size="22" /></span>
              <div class="file-name">{{ t('pluginAdmin.dropFile') }}</div>
              <div class="text-desc">{{ t('pluginAdmin.dropFileHint', { size: attachmentSize(MAX_PLUGIN_SOURCE) }) }}</div>
            </div>
          </template>
        </a-upload>
      </div>

      <div v-else-if="source === 'url'" class="source-body">
        <a-input v-model="url" size="large" allow-clear :disabled="busy" :placeholder="t('pluginAdmin.urlPlaceholder')" @press-enter="inspect">
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
          <a-button type="text" size="small" :disabled="busy" @click="resetTemplate">{{ t('pluginAdmin.resetTemplate') }}</a-button>
        </div>
        <CodeEditor :key="language" v-model="code" :language="language" height="340px" :aria-label="t('pluginAdmin.sourceCode')" @submit="inspect" />
      </div>

      <a-alert v-if="error" type="error" class="error">{{ error }}</a-alert>
    </div>

    <div v-else-if="plugin" class="review-step">
      <div class="review-head">
        <WorkflowPluginIcon :icon="plugin.module.icon" :color="plugin.module.color" :size="44" />
        <div class="review-title">
          <div class="review-name">
            <span>{{ plugin.module.label }}</span>
            <a-tag size="small" class="mono">{{ versionLabel(plugin.version) }}</a-tag>
            <a-tag v-if="!installed" size="small" color="green">{{ t('pluginAdmin.newInstall') }}</a-tag>
            <a-tag v-else-if="installed.version !== plugin.version" size="small" color="arcoblue">
              {{ t('pluginAdmin.upgradeFrom', { from: installed.version, to: plugin.version }) }}
            </a-tag>
            <a-tag v-else size="small">{{ t('pluginAdmin.reinstall') }}</a-tag>
          </div>
          <div v-if="plugin.module.description" class="text-desc">{{ plugin.module.description }}</div>
          <div v-if="plugin.module.plugin?.author || plugin.module.plugin?.homepage" class="review-meta text-desc">
            <span v-if="plugin.module.plugin?.author">{{ plugin.module.plugin.author }}</span>
            <template v-if="plugin.module.plugin?.homepage">
              <span v-if="plugin.module.plugin?.author" class="dot">·</span><a-link :href="plugin.module.plugin.homepage" target="_blank" rel="noopener noreferrer">{{ t('pluginAdmin.homepage') }}</a-link>
            </template>
          </div>
        </div>
      </div>

      <a-alert v-if="replacesOther" type="warning" class="error">{{ t('pluginAdmin.replacesOther', { id: plugin.id }) }}</a-alert>

      <a-descriptions :column="2" layout="vertical" class="review-details">
        <a-descriptions-item :label="t('pluginAdmin.inputs')">
          <ul v-if="inputs.length" class="io-list">
            <li v-for="f in inputs" :key="f.key">
              <span class="ellipsis">{{ dslText(f.label) || f.key }}</span>
              <span v-if="f.required" class="required">*</span>
              <code class="io-key">{{ f.key }}</code>
            </li>
          </ul>
          <div v-else class="text-desc">{{ t('pluginAdmin.noInputs') }}</div>
        </a-descriptions-item>
        <a-descriptions-item :label="t('pluginAdmin.outputs')">
          <ul v-if="plugin.module.outputs.length" class="io-list">
            <li v-for="o in plugin.module.outputs" :key="o.key">
              <span class="ellipsis">{{ dslText(o.label) }}</span>
              <code class="io-key">{{ o.key }}: {{ o.kind }}</code>
            </li>
          </ul>
          <div v-else class="text-desc">{{ t('pluginAdmin.noOutputs') }}</div>
        </a-descriptions-item>
        <a-descriptions-item :label="t('pluginAdmin.permissions')" :span="2">
          <WorkflowPluginPermissions :module="plugin.module" :domains="plugin.domains" :timeout="plugin.timeout" :credential="plugin.credential" />
        </a-descriptions-item>
      </a-descriptions>

      <a-alert type="warning">{{ t('pluginAdmin.trustWarning') }}</a-alert>
    </div>

    <template #footer>
      <a-space>
        <template v-if="step === 1">
          <a-button :disabled="busy" @click="visible = false">{{ t('common.cancel') }}</a-button>
          <a-button type="primary" :disabled="!canInspect" :loading="inspecting" @click="inspect">
            <span class="inline">{{ t('pluginAdmin.next') }}<ArrowRight v-if="!inspecting" :size="15" /></span>
          </a-button>
        </template>
        <template v-else>
          <a-button :disabled="installing" @click="step = 1">{{ t('pluginAdmin.back') }}</a-button>
          <a-button type="primary" :loading="installing" @click="install">
            {{ !installed ? t('pluginAdmin.confirmInstall') : installed.version === plugin?.version ? t('pluginAdmin.reinstall') : t('pluginAdmin.confirmUpgrade') }}
          </a-button>
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
.source-body :deep(.arco-upload-wrapper) {
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
.mono {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
.error {
  margin-top: 16px;
}
.review-head {
  display: flex;
  align-items: center;
  gap: 12px;
  overflow-wrap: anywhere;
}
.review-name {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 4px;
  font-size: 17px;
}
.review-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  margin-top: 4px;
  font-size: 12px;
}
.dot {
  margin: 0 6px;
}
.review-details {
  margin: 20px 0 8px;
}
.io-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: 13px;
}
.io-list li {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  color: var(--text-title);
}
.required {
  margin-left: 3px;
  color: var(--color-danger);
}
.io-key {
  max-width: 100%;
  margin-left: auto;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--fill-hover);
  color: var(--text-caption);
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 11.5px;
  white-space: normal;
  overflow-wrap: anywhere;
}
@media (max-width: 480px) {
  .source-options {
    flex-direction: column;
  }
}
</style>
