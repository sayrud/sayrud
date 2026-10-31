<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { Copy, Info, KeyRound, Plus, Trash2 } from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, h, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { apiEndpoint, workflowsApi } from '@/api/workflow'
import ErrorResponseEditor from '@/components/workflow/ErrorResponseEditor.vue'
import PanelSection from '@/components/workflow/PanelSection.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { WorkflowKey } from '@/types/workflow'
import { HTTP_METHODS, METHOD_COLORS } from '@/utils/workflow'

dayjs.extend(relativeTime)

const { t } = useI18n()
const store = useWorkflowEditorStore()
const api = computed(() => store.spec!.trigger.api!)
const endpoint = computed(() => apiEndpoint(store.projectUID, api.value.path || '/'))

const keys = ref<WorkflowKey[]>([])
const keysLoading = ref(false)
const creating = ref(false)
const keyName = ref('')
const createVisible = ref(false)

function copy(text: string) {
  navigator.clipboard?.writeText(text).then(
    () => Message.success(t('workflow.request.copied')),
    () => undefined,
  )
}

async function loadKeys() {
  if (!store.workflow || !store.canManage) return
  keysLoading.value = true
  try {
    keys.value = await workflowsApi.keys(store.projectUID, store.workflow.uid)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    keysLoading.value = false
  }
}

async function createKey() {
  const name = keyName.value.trim()
  if (!name || !store.workflow) return false
  creating.value = true
  try {
    const { key, secret } = await workflowsApi.createKey(store.projectUID, store.workflow.uid, name)
    keys.value.push(key)
    keyName.value = ''
    Modal.success({
      title: t('workflow.request.keySecretTitle'),
      content: () =>
        h('div', [
          h('div', { style: { marginBottom: '10px', color: 'var(--text-caption)' } }, t('workflow.request.keySecretContent')),
          h('code', { style: { display: 'block', padding: '10px 12px', borderRadius: '6px', background: 'var(--bg-base)', fontFamily: 'SF Mono, Menlo, Consolas, monospace', fontSize: '12.5px', wordBreak: 'break-all', userSelect: 'all' } }, secret),
        ]),
      okText: t('workflow.request.copyAndClose'),
      onOk: () => copy(secret),
      width: 520,
    })
    return true
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    return false
  } finally {
    creating.value = false
  }
}

function removeKey(key: WorkflowKey) {
  Modal.warning({
    title: t('workflow.request.deleteKeyTitle', { name: key.name }),
    content: t('workflow.request.deleteKeyContent'),
    hideCancel: false,
    okText: t('common.delete'),
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      await workflowsApi.deleteKey(store.projectUID, store.workflow!.uid, key.id)
      keys.value = keys.value.filter((k) => k.id !== key.id)
    },
  })
}

function onPathBlur() {
  const p = api.value.path.trim()
  api.value.path = p && !p.startsWith('/') ? `/${p}` : p
}

watch(() => api.value.auth.type, (type) => {
  if (type === 'api_key' && !keys.value.length) void loadKeys()
})
onMounted(() => {
  if (api.value.auth.type === 'api_key') void loadKeys()
})
</script>

<template>
  <PanelSection :title="t('workflow.request.entry')">
    <a-form-item :label="t('workflow.request.method')">
      <a-radio-group v-model="api.method" type="button" class="method-group">
        <a-radio v-for="m in HTTP_METHODS" :key="m" :value="m">
          <span class="method" :class="METHOD_COLORS[m]">{{ m }}</span>
        </a-radio>
      </a-radio-group>
    </a-form-item>
    <a-form-item :label="t('workflow.request.path')" :extra="t('workflow.request.pathHelp')">
      <a-input v-model="api.path" class="mono" :max-length="200" @blur="onPathBlur" />
    </a-form-item>
    <a-form-item :label="t('workflow.request.endpoint')">
      <div class="endpoint">
        <code class="endpoint-url">{{ endpoint }}</code>
        <a-tooltip :content="t('workflow.request.copy')" mini>
          <a-button type="text" size="mini" class="btn-muted" @click="copy(endpoint)"><template #icon><Copy :size="13" /></template></a-button>
        </a-tooltip>
      </div>
    </a-form-item>
  </PanelSection>

  <PanelSection :title="t('workflow.request.security')">
    <div class="switch-row">
      <a-typography-text class="workflow-field-label">{{ t('workflow.request.authKey') }}</a-typography-text>
      <a-tooltip :content="t('workflow.request.authKeyDesc')" mini>
        <Info :size="14" class="switch-info" />
      </a-tooltip>
      <a-switch :model-value="api.auth.type === 'api_key'" @change="(on) => (api.auth.type = on ? 'api_key' : 'none')" />
    </div>
    <div v-if="api.auth.type === 'api_key' && store.canManage" class="keys">
      <div class="keys-head">
        <span class="keys-title">{{ t('workflow.request.keys') }}</span>
        <a-button size="mini" @click="createVisible = true">
          <template #icon><Plus :size="12" /></template>
          {{ t('workflow.request.createKey') }}
        </a-button>
      </div>
      <a-spin :loading="keysLoading" class="keys-spin">
        <div v-for="key in keys" :key="key.id" class="key-row">
          <KeyRound :size="14" class="key-icon" />
          <div class="key-text">
            <div class="key-name ellipsis">{{ key.name }}</div>
            <div class="key-meta">
              <code>{{ key.prefix }}…</code>
              <span>{{ key.lastUsedAt ? t('workflow.request.keyLastUsed', { time: dayjs(key.lastUsedAt).fromNow() }) : t('workflow.request.keyNeverUsed') }}</span>
            </div>
          </div>
          <a-button type="text" size="mini" class="btn-muted" :title="t('common.delete')" :aria-label="t('common.delete')" @click="removeKey(key)"><template #icon><Trash2 :size="13" /></template></a-button>
        </div>
        <div v-if="!keys.length && !keysLoading" class="keys-empty">{{ t('workflow.request.noKeys') }}</div>
      </a-spin>
    </div>

    <!-- eslint-disable-next-line i18n-audit/source -- Keep the API response identical across interface languages. -->
    <ErrorResponseEditor v-if="api.auth.type === 'api_key'" v-model="api.auth.errorResponse" :status="401" code="unauthorized" message="a valid API key is required" />

    <div class="switch-row">
      <a-typography-text class="workflow-field-label">{{ t('workflow.request.rateLimit') }}</a-typography-text>
      <a-tooltip :content="t('workflow.request.rateLimitDesc')" mini>
        <Info :size="14" class="switch-info" />
      </a-tooltip>
      <a-switch v-model="api.rateLimit.enabled" />
    </div>
    <div v-if="api.rateLimit.enabled" class="rate-inputs">
      <a-input-number v-model="api.rateLimit.window" :min="1" :max="86400">
        <template #prefix>{{ t('workflow.request.window') }}</template>
        <template #suffix>{{ t('workflow.request.seconds') }}</template>
      </a-input-number>
      <a-input-number v-model="api.rateLimit.limit" :min="1" :max="100000">
        <template #prefix>{{ t('workflow.request.limit') }}</template>
        <template #suffix>{{ t('workflow.request.times') }}</template>
      </a-input-number>
    </div>

    <!-- eslint-disable-next-line i18n-audit/source -- Keep the API response identical across interface languages. -->
    <ErrorResponseEditor v-if="api.rateLimit.enabled" v-model="api.rateLimit.errorResponse" :status="429" code="rate_limited" message="too many requests" retry-after />
  </PanelSection>

  <a-modal v-model:visible="createVisible" :title="t('workflow.request.createKey')" :width="420" :ok-loading="creating" :on-before-ok="createKey">
    <a-input v-model="keyName" :placeholder="t('workflow.request.keyNamePlaceholder')" :max-length="64" @press-enter="createKey().then((ok) => ok && (createVisible = false))" />
  </a-modal>
</template>

<style scoped>
.method-group {
  display: flex;
  width: 100%;
}
.method-group :deep(.arco-radio-button) {
  flex: 1;
  text-align: center;
}
.method {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  font-weight: 600;
}
.mono :deep(input) {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
}
.endpoint {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 6px 6px 6px 10px;
  border-radius: 6px;
  background: var(--bg-base);
}
.endpoint-url {
  flex: 1;
  min-width: 0;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  color: var(--text-title);
  overflow-wrap: anywhere;
}
.keys {
  margin: 2px 0 10px;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--bg-base);
}
.keys-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}
.keys-title {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text-caption);
}
.keys-spin {
  display: block;
}
.key-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
}
.key-row + .key-row {
  border-top: 1px solid var(--line-border);
}
.key-icon {
  color: var(--text-caption);
  flex: none;
}
.key-text {
  flex: 1;
  min-width: 0;
}
.key-name {
  font-size: 13px;
  color: var(--text-title);
}
.key-meta {
  display: flex;
  gap: 8px;
  font-size: 12px;
  color: var(--text-placeholder);
}
.key-meta code {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
.keys-empty {
  padding: 8px 0 4px;
  font-size: 12px;
  color: var(--text-placeholder);
}
.switch-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 40px;
}
.switch-info {
  color: var(--text-placeholder);
  cursor: help;
  flex: none;
}
.switch-row :deep(.arco-switch) {
  margin-left: auto;
}
.rate-inputs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin: 2px 0 10px;
}
</style>
