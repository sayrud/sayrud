<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { credentialsApi, type SaveCredentialRequest } from '@/api/workflow'
import ProviderIcon from '@/components/common/ProviderIcon.vue'
import type { Credential, FieldDef, OptionDef } from '@/types/workflow'
import { credentialTypeDef, dslText, isBlank, secretsReady } from '@/utils/modules'

/** Dialog rendered from the credential type DSL that creates a credential, or edits credential if given. */
const visible = defineModel<boolean>('visible', { required: true })
const props = defineProps<{ projectUid: string; type: string; credential: Credential | null }>()
const emit = defineEmits<{ saved: [credential: Credential]; deleted: [uid: string] }>()
const { t } = useI18n()

const def = computed(() => credentialTypeDef(props.type))
const sections = computed(() => def.value?.sections.filter((s) => !s.showIfValue || draft.data[s.showIf!] === s.showIfValue) ?? [])
const fields = computed(() => sections.value.flatMap((s) => s.fields))

const draft = reactive({
  name: '',
  data: {} as Record<string, unknown>,
  secrets: {} as Record<string, string>,
  clearSecrets: [] as string[],
})
const testInput = reactive<Record<string, string>>({})
const testing = ref(false)
const saving = ref(false)
const busy = computed(() => saving.value || testing.value)
const testResult = ref<{ ok: true } | { error: string } | null>(null)

function reset() {
  const c = props.credential
  const data: Record<string, unknown> = {}

  for (const f of def.value?.sections.flatMap((s) => s.fields) ?? []) {
    if (f.secret || !f.key) continue
    data[f.key] = c ? c.data[f.key] : f.default
  }

  Object.assign(draft, { name: c?.name ?? dslText(def.value?.label), data, secrets: {}, clearSecrets: [] })

  for (const k of Object.keys(testInput)) delete testInput[k]

  testResult.value = null
}

watch(() => [visible.value, props.projectUid, props.type, props.credential?.uid], () => {
  if (visible.value) reset()
}, { immediate: true })

const hasSavedSecret = (f: FieldDef) => !!props.credential?.secrets.includes(f.key!) && (props.type !== 'cloud' || props.credential.data.provider === draft.data.provider)
const saved = (f: FieldDef) => hasSavedSecret(f) && !draft.clearSecrets.includes(f.key!)

function toggleClear(f: FieldDef) {
  const i = draft.clearSecrets.indexOf(f.key!)
  if (i >= 0) draft.clearSecrets.splice(i, 1)
  else draft.clearSecrets.push(f.key!)
}

const numberOf = (f: FieldDef) => draft.data[f.key!] as number | undefined
const choiceOf = (f: FieldDef) => draft.data[f.key!] as string | number | boolean | undefined
const textOf = (f: FieldDef) => (draft.data[f.key!] as string | undefined) ?? ''

const optionLabel = (o: OptionDef) => (o.label ? dslText(o.label) : String(o.value))

function selectOption(f: FieldDef, v: unknown) {
  if (props.type === 'cloud' && f.key === 'provider' && v !== draft.data.provider) {
    Object.assign(draft, { data: { provider: v }, secrets: {}, clearSecrets: [] })
    return
  }

  const prev = f.options?.find((o) => o.value === draft.data[f.key!])
  const next = f.options?.find((o) => o.value === v)

  for (const [k, patched] of Object.entries(next?.patch ?? {})) {
    if (isBlank(draft.data[k]) || draft.data[k] === prev?.patch?.[k]) draft.data[k] = patched
  }

  draft.data[f.key!] = v
}

/** Whether a required field is missing; saved secrets count as filled. */
const incomplete = computed(() => !draft.name.trim() || fields.value.some((f) => f.required && (f.secret ? !draft.secrets[f.key!] && !saved(f) : isBlank(draft.data[f.key!]))))

function body(): SaveCredentialRequest {
  const secrets = Object.fromEntries(Object.entries(draft.secrets).filter(([k, v]) => v && fields.value.some((f) => f.secret && f.key === k)))

  return {
    type: props.type,
    name: draft.name.trim(),
    data: Object.fromEntries(Object.entries(draft.data).filter(([k]) => fields.value.some((f) => !f.secret && f.key === k)).map(([k, v]) => [k, typeof v === 'string' ? v.trim() : v])),
    secrets,
    clearSecrets: draft.clearSecrets.filter((k) => !secrets[k]),
  }
}

async function save() {
  if (busy.value || incomplete.value || !def.value) return false
  saving.value = true

  try {
    const c = props.credential
      ? await credentialsApi.update(props.projectUid, props.credential.uid, body())
      : await credentialsApi.create(props.projectUid, body())
    emit('saved', c)
    Message.success(t('common.saved'))
    return true
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    return false
  } finally {
    saving.value = false
  }
}

const testReady = computed(() => !!def.value?.test && def.value.test.fields.every((f) => !f.required || !!testInput[f.key!]?.trim()))

async function test() {
  if (busy.value || incomplete.value || !testReady.value) return
  testing.value = true
  testResult.value = null
  try {
    await credentialsApi.test(props.projectUid, { ...body(), credentialUID: props.credential?.uid, input: { ...testInput } })
    testResult.value = { ok: true }
  } catch (e) {
    testResult.value = { error: e instanceof Error ? e.message : String(e) }
  } finally {
    testing.value = false
  }
}

function remove() {
  const c = props.credential
  if (!c) return
  Modal.warning({
    title: t('workflow.credential.deleteTitle', { name: c.name }),
    content: t('workflow.credential.deleteContent'),
    hideCancel: false,
    okText: t('common.delete'),
    okButtonProps: { status: 'danger' },
    onBeforeOk: async () => {
      try {
        await credentialsApi.remove(props.projectUid, c.uid)
        emit('deleted', c.uid)
        visible.value = false
        Message.success(t('common.deleted'))
        return true
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
    },
  })
}
</script>

<template>
  <a-modal
    v-model:visible="visible"
    :title="credential ? t('workflow.credential.editTitle', { type: dslText(def?.label) }) : t('workflow.credential.createTitle', { type: dslText(def?.label) })"
    :width="560"
    :body-style="{ maxHeight: 'calc(100vh - 160px)', overflowY: 'auto' }"
    :mask-closable="!busy"
    :esc-to-close="!busy"
    :closable="!busy"
    unmount-on-close
  >
    <a-alert v-if="!secretsReady() && fields.some((f) => f.secret)" type="warning" class="alert">{{ t('workflow.credential.secretsMissing') }}</a-alert>
    <a-typography-text v-if="def?.description" type="secondary" class="workflow-field-tip alert">{{ dslText(def.description) }}</a-typography-text>
    <a-form v-if="def" layout="vertical" :model="draft" :disabled="busy" class="form">
      <a-form-item :label="t('workflow.credential.name')" required>
        <a-input v-model="draft.name" :max-length="64" :placeholder="t('workflow.credential.namePlaceholder')" />
      </a-form-item>

      <template v-for="(section, i) in sections" :key="section.showIfValue || i">
        <div v-if="section.title" class="group-title">{{ dslText(section.title) }}</div>
        <a-typography-text v-if="section.description" type="secondary" class="workflow-field-tip alert">{{ dslText(section.description) }}</a-typography-text>
        <div class="grid">
          <a-form-item v-for="f in section.fields" :key="f.key" :label="dslText(f.label)" :extra="dslText(f.help) || undefined" :required="f.required" :class="{ wide: f.widget === 'select' || f.multiline }">
            <div v-if="f.secret && f.multiline" class="multiline-secret">
              <a-textarea
                v-model="draft.secrets[f.key!]"
                :auto-size="{ minRows: 6, maxRows: 12 }"
                :max-length="f.maxLength"
                :placeholder="saved(f) ? t('workflow.credential.secretKeep') : dslText(f.placeholder) || undefined"
                autocomplete="off"
                spellcheck="false"
              />
              <a-link v-if="hasSavedSecret(f) && !draft.secrets[f.key!]" :status="draft.clearSecrets.includes(f.key!) ? 'normal' : 'danger'" @click="toggleClear(f)">
                {{ draft.clearSecrets.includes(f.key!) ? t('workflow.credential.keepSecret') : t('workflow.credential.clearSecret') }}
              </a-link>
            </div>
            <a-input-password
              v-else-if="f.secret"
              v-model="draft.secrets[f.key!]"
              :max-length="f.maxLength"
              autocomplete="new-password"
              :placeholder="saved(f) ? t('workflow.credential.secretKeep') : dslText(f.placeholder) || undefined"
            >
              <template v-if="hasSavedSecret(f) && !draft.secrets[f.key!]" #append>
                <a-link :status="draft.clearSecrets.includes(f.key!) ? 'normal' : 'danger'" @click="toggleClear(f)">
                  {{ draft.clearSecrets.includes(f.key!) ? t('workflow.credential.keepSecret') : t('workflow.credential.clearSecret') }}
                </a-link>
              </template>
            </a-input-password>
            <a-input-number v-else-if="f.widget === 'number'" :model-value="numberOf(f)" :min="f.min" :max="f.max" @change="(v) => (draft.data[f.key!] = v ?? undefined)" />
            <a-radio-group v-else-if="f.widget === 'select' && f.display === 'radio'" :model-value="choiceOf(f)" type="button" @change="(v) => selectOption(f, v)">
              <a-radio v-for="o in f.options" :key="String(o.value)" :value="o.value">{{ optionLabel(o) }}</a-radio>
            </a-radio-group>
            <a-select v-else-if="f.widget === 'select'" :model-value="choiceOf(f) as string" @change="(v) => selectOption(f, v)">
              <template v-if="type === 'cloud' && f.key === 'provider'" #label="{ data }">
                <span class="provider-label"><ProviderIcon :icon="String(data.value)" :size="18" />{{ data.label }}</span>
              </template>
              <a-option v-for="o in f.options" :key="String(o.value)" :value="o.value" :label="optionLabel(o)">
                <span v-if="type === 'cloud' && f.key === 'provider'" class="provider-label"><ProviderIcon :icon="String(o.value)" :size="18" />{{ optionLabel(o) }}</span>
                <template v-else>{{ optionLabel(o) }}</template>
              </a-option>
            </a-select>
            <a-input v-else :model-value="textOf(f)" @update:model-value="(v) => (draft.data[f.key!] = v)" :max-length="f.maxLength" :placeholder="dslText(f.placeholder) || undefined" autocomplete="off" />
          </a-form-item>
        </div>
      </template>

      <template v-if="def.test">
        <div class="group-title">{{ t('workflow.credential.test') }}</div>
        <a-form-item v-for="f in def.test.fields" :key="f.key" :label="dslText(f.label)" :extra="dslText(def.test.description) || undefined">
          <div class="test-row">
            <a-input v-model="testInput[f.key!]" :placeholder="dslText(f.placeholder) || undefined" />
            <a-button :loading="testing" :disabled="incomplete || !testReady" @click="test">{{ dslText(def.test.action) }}</a-button>
          </div>
        </a-form-item>
        <template v-if="testResult">
          <a-alert v-if="'error' in testResult" type="error">{{ testResult.error }}</a-alert>
          <a-alert v-else type="success">{{ dslText(def.test.success) }}</a-alert>
        </template>
      </template>
    </a-form>

    <template #footer>
      <div class="footer">
        <a-button v-if="credential" status="danger" type="text" :disabled="busy" @click="remove">{{ t('common.delete') }}</a-button>
        <span class="spacer" />
        <a-button :disabled="busy" @click="visible = false">{{ t('common.cancel') }}</a-button>
        <a-button type="primary" :loading="saving" :disabled="incomplete || testing || !def" @click="save().then((ok) => ok && (visible = false))">{{ t('common.save') }}</a-button>
      </div>
    </template>
  </a-modal>
</template>

<style scoped>
.alert {
  margin-bottom: 12px;
}
.group-title {
  margin: 4px 0 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-title);
}
.form :deep(.arco-form-item) {
  margin-bottom: 14px;
}
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  column-gap: 12px;
}
.grid .wide {
  grid-column: 1 / -1;
}

.multiline-secret {
  width: 100%;
}

.provider-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.test-row {
  display: flex;
  gap: 8px;
  width: 100%;
}
.footer {
  display: flex;
  align-items: center;
  gap: 8px;
}
.spacer {
  flex: 1;
}
@media (max-width: 480px) {
  .grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
