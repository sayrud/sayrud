<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { Check, Copy } from '@lucide/vue'
import { watch } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

/** The plaintext of a token shown once, with the warning that it can not be seen again. */
const props = defineProps<{ secret: string; warning?: string }>()
const copied = defineModel<boolean>('copied', { default: false })

watch(
  () => props.secret,
  () => (copied.value = false),
)

function copy() {
  navigator.clipboard?.writeText(props.secret).then(
    () => {
      copied.value = true
      Message.success(t('accessToken.copied'))
    },
    () => Message.error(t('accessToken.copyFailed')),
  )
}
</script>

<template>
  <div class="secret">
    <a-alert type="warning" :title="t('accessToken.secretTitle')">{{ warning ?? t('accessToken.secretWarning') }}</a-alert>
    <div class="secret-row">
      <a-input :model-value="secret" readonly class="secret-input" @focus="(e: FocusEvent) => (e.target as HTMLInputElement).select()" />
      <a-button type="primary" :status="copied ? 'success' : 'normal'" @click="copy">
        <template #icon>
          <Check v-if="copied" :size="14" />
          <Copy v-else :size="14" />
        </template>
        {{ copied ? t('accessToken.copied') : t('accessToken.copy') }}
      </a-button>
    </div>
  </div>
</template>

<style scoped>
.secret {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.secret-row {
  display: flex;
  gap: 8px;
}
.secret-input {
  flex: 1;
}
.secret-input :deep(input) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
</style>
