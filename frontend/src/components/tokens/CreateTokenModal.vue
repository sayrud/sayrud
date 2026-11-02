<script setup lang="ts">
import { Message, type FieldRule, type FormInstance } from '@arco-design/web-vue'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { AccessTokenApi, CreatedAccessToken } from '@/api/account'
import { useSecretGuard } from '@/composables/useSecretGuard'
import { newTokenAccess, tokenAccessBody, type TokenProjectOption } from '@/utils/accessToken'
import TokenAccessFields from './TokenAccessFields.vue'
import TokenSecret from './TokenSecret.vue'

const { t } = useI18n()

const props = defineProps<{
  visible: boolean
  api: AccessTokenApi
  searchProjects: (keyword: string) => Promise<TokenProjectOption[]>
}>()
const emit = defineEmits<{ 'update:visible': [value: boolean]; created: [token: CreatedAccessToken] }>()

const formRef = ref<FormInstance>()
const model = reactive({ name: '', ...newTokenAccess() })
const saving = ref(false)
const created = ref<CreatedAccessToken | null>(null)
const secret = computed(() => created.value?.secret)
const { copied, requestClose } = useSecretGuard(secret, () => emit('update:visible', false))

const nameRules = computed<FieldRule[]>(() => [{ required: true, message: t('accessToken.nameRequired') }])

watch(
  () => props.visible,
  (v) => {
    if (!v) return
    Object.assign(model, { name: '', ...newTokenAccess(), customDate: undefined })
    created.value = null
    formRef.value?.clearValidate()
  },
)

async function submit() {
  if (await formRef.value?.validate()) return
  saving.value = true
  try {
    created.value = await props.api.create({ name: model.name.trim(), ...tokenAccessBody(model) })
    emit('created', created.value)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <a-modal
    :visible="visible"
    :title="created ? t('accessToken.created') : t('accessToken.createTitle')"
    :width="560"
    title-align="start"
    :mask-closable="!created"
    :esc-to-close="!created"
    unmount-on-close
    @cancel="requestClose"
  >
    <a-form v-if="!created" ref="formRef" :model="model" layout="vertical">
      <a-form-item field="name" :label="t('accessToken.name')" :rules="nameRules">
        <a-input v-model="model.name" :placeholder="t('accessToken.namePlaceholder')" :max-length="64" show-word-limit @press-enter="submit" />
      </a-form-item>
      <TokenAccessFields v-model="model" :search-projects="searchProjects" @restricted-change="formRef?.clearValidate('projects')" />
    </a-form>
    <TokenSecret v-else v-model:copied="copied" :secret="created.secret" />

    <template #footer>
      <template v-if="!created">
        <a-button @click="emit('update:visible', false)">{{ t('common.cancel') }}</a-button>
        <a-button type="primary" :loading="saving" @click="submit">{{ t('accessToken.create') }}</a-button>
      </template>
      <a-button v-else type="primary" @click="requestClose">{{ t('accessToken.savedClose') }}</a-button>
    </template>
  </a-modal>
</template>
