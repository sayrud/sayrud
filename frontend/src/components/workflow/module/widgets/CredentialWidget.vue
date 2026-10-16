<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { Pencil, Plus } from '@lucide/vue'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import CredentialDialog from '@/components/workflow/module/CredentialDialog.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { Credential, FieldDef } from '@/types/workflow'
import { credentialTypeDef } from '@/utils/modules'
import { findStep } from '@/utils/workflow'

/** Selects a project credential of a type, which can be created or edited in place; credentials are shared in the project. */
const props = defineProps<{ stepId: string; field: FieldDef }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const config = computed(() => findStep(store.spec!, props.stepId)!.config)
const def = computed(() => credentialTypeDef(props.field.credentialType))
const loading = ref(false)

const options = computed(() => (store.credentials ?? []).filter((c) => c.type === props.field.credentialType))
const selected = computed(() => (config.value[props.field.key!] as string | undefined) || undefined)
const current = computed(() => options.value.find((c) => c.uid === selected.value) ?? null)
const missing = computed(() => !!selected.value && store.credentials !== null && !current.value)

function summary(c: Credential) {
  return (def.value?.summary ?? []).map((k) => c.data[k]).filter((v) => v !== undefined && v !== '').join(' · ')
}

function select(uid: string | undefined) {
  config.value[props.field.key!] = uid ?? ''
}

const dialogOpen = ref(false)
const editing = ref<Credential | null>(null)

function open(c: Credential | null) {
  editing.value = c
  dialogOpen.value = true
}

onMounted(async () => {
  loading.value = true
  try {
    await store.loadCredentials()
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="credential">
    <div class="row">
      <a-select :model-value="selected" :loading="loading" :placeholder="t('workflow.credential.select')" allow-clear class="select" @change="(v) => select(v as string | undefined)" @clear="select(undefined)">
        <a-option v-for="c in options" :key="c.uid" :value="c.uid" :label="c.name">
          <div class="option">
            <span class="ellipsis">{{ c.name }}</span>
            <span class="option-summary ellipsis">{{ summary(c) }}</span>
          </div>
        </a-option>
        <template #empty>
          <div class="empty">{{ t('workflow.credential.empty') }}</div>
        </template>
      </a-select>
      <a-tooltip v-if="store.canManage && current" :content="t('workflow.credential.edit')" mini>
        <a-button :aria-label="t('workflow.credential.edit')" @click="open(current)"><template #icon><Pencil :size="14" /></template></a-button>
      </a-tooltip>
    </div>
    <div v-if="current && summary(current)" class="hint">{{ summary(current) }}</div>
    <div v-if="missing" class="missing">{{ t('workflow.credential.missing') }}</div>
    <a-button v-if="store.canManage" type="text" size="small" class="create" @click="open(null)">
      <template #icon><Plus :size="13" /></template>
      {{ t('workflow.credential.create') }}
    </a-button>
    <CredentialDialog
      v-if="store.canManage && field.credentialType"
      v-model:visible="dialogOpen"
      :type="field.credentialType"
      :credential="editing"
      @saved="(c) => select(c.uid)"
      @deleted="(uid) => uid === selected && select(undefined)"
    />
  </div>
</template>

<style scoped>
.credential {
  width: 100%;
}
.row {
  display: flex;
  gap: 6px;
}
.select {
  flex: 1;
  min-width: 0;
}
.option {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}
.option-summary {
  font-size: 12px;
  color: var(--text-placeholder);
}
.empty {
  padding: 10px 12px;
  font-size: 12px;
  color: var(--text-placeholder);
}
.hint {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-placeholder);
}
.missing {
  margin-top: 4px;
  font-size: 12px;
  color: var(--color-danger);
}
.create {
  margin-top: 4px;
  padding-left: 0;
}
</style>
