<script setup lang="ts">
import { Plus } from '@lucide/vue'
import { nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ShortcutFormItem } from '@/api/shortcut'
import type { SelectOption, SLField } from '@/types/bitable'
import { promptFromDisplay, promptToDisplay, selectableFields } from '@/utils/shortcut'
import FieldTypeIcon from './FieldTypeIcon.vue'
import OptionsEditor from './OptionsEditor.vue'

/** Renders the form items of a shortcut, the values are the inputs of FieldShortcut. */
const props = defineProps<{
  items: ShortcutFormItem[]
  fields: SLField[]
  /** The field being configured, which can not be referenced. */
  fieldUID?: string
}>()

const inputs = defineModel<Record<string, string>>({ required: true })
const options = defineModel<SelectOption[]>('options', { default: () => [] })

const { t } = useI18n()

function set(key: string, value: string | undefined) {
  const next = { ...inputs.value }
  if (value) next[key] = value
  else delete next[key]
  inputs.value = next
}

const referable = () => props.fields.filter((f) => f.uid !== props.fieldUID && f.type !== 'formula')

const prompts = ref<Record<string, HTMLTextAreaElement | null>>({})

function bindPrompt(key: string, el: unknown) {
  prompts.value[key] = (el as { $el?: HTMLElement } | null)?.$el?.querySelector('textarea') ?? null
}

/** Inserts the field reference at the cursor of the prompt. */
async function insertField(item: ShortcutFormItem, field: SLField) {
  const display = promptToDisplay(inputs.value[item.key] ?? '', props.fields)
  const el = prompts.value[item.key]
  const start = el?.selectionStart ?? display.length
  const end = el?.selectionEnd ?? display.length
  const token = `[${field.label}]`
  set(item.key, promptFromDisplay(display.slice(0, start) + token + display.slice(end), props.fields))

  await nextTick()
  el?.focus()
  el?.setSelectionRange(start + token.length, start + token.length)
}
</script>

<template>
  <div class="shortcut-form">
    <div v-for="item in items" :key="item.key" class="row">
      <div class="row-label">
        {{ item.label }}<span v-if="item.required" class="required">*</span>
      </div>

      <OptionsEditor v-if="item.component === 'field_options'" v-model="options" />

      <a-select
        v-else-if="item.component === 'field_select'"
        :model-value="inputs[item.key] || undefined"
        :placeholder="item.placeholder || t('shortcut.pickField')"
        allow-clear
        allow-search
        @update:model-value="(v: unknown) => set(item.key, v as string | undefined)"
      >
        <a-option v-for="f in selectableFields(item, fields, fieldUID)" :key="f.uid" :value="f.uid" :label="f.label">
          <span class="field-option"><FieldTypeIcon :type="f.type" /> {{ f.label }}</span>
        </a-option>
      </a-select>

      <a-select
        v-else-if="item.component === 'select'"
        :model-value="inputs[item.key] || undefined"
        :placeholder="item.placeholder || t('common.selectPlaceholder')"
        :allow-clear="!item.required"
        @update:model-value="(v: unknown) => set(item.key, v as string | undefined)"
      >
        <a-option v-for="o in item.options ?? []" :key="o.value" :value="o.value">{{ o.label }}</a-option>
      </a-select>

      <a-input
        v-else-if="item.component === 'input'"
        :model-value="inputs[item.key] ?? ''"
        :placeholder="item.placeholder || t('common.inputPlaceholder')"
        :max-length="10000"
        @update:model-value="(v: string) => set(item.key, v)"
      />

      <a-textarea
        v-else-if="item.component === 'textarea'"
        :model-value="inputs[item.key] ?? ''"
        :placeholder="item.placeholder"
        :auto-size="{ minRows: 2, maxRows: 6 }"
        :max-length="10000"
        @update:model-value="(v: string) => set(item.key, v)"
      />

      <template v-else-if="item.component === 'prompt'">
        <a-textarea
          :ref="(el: unknown) => bindPrompt(item.key, el)"
          :model-value="promptToDisplay(inputs[item.key] ?? '', fields)"
          :placeholder="item.placeholder"
          :auto-size="{ minRows: 3, maxRows: 8 }"
          :max-length="10000"
          @update:model-value="(v: string) => set(item.key, promptFromDisplay(v, fields))"
        />
        <a-dropdown trigger="click" position="bl">
          <button type="button" class="insert-field"><Plus :size="13" /> {{ t('shortcut.insertField') }}</button>
          <template #content>
            <a-doption v-for="f in referable()" :key="f.uid" @click="insertField(item, f)">
              <span class="field-option"><FieldTypeIcon :type="f.type" /> {{ f.label }}</span>
            </a-doption>
          </template>
        </a-dropdown>
        <div class="hint">{{ t('shortcut.promptHint') }}</div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.row {
  margin-bottom: 12px;
}
.row-label {
  margin-bottom: 6px;
  font-size: 13px;
  color: var(--text-caption);
}
.required {
  margin-left: 2px;
  color: var(--color-danger);
}
.field-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.insert-field {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-top: 6px;
  padding: 2px 6px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--color-primary);
  font-size: 12px;
  cursor: pointer;
}
.insert-field:hover {
  background: var(--fill-hover);
}
.hint {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-placeholder);
}
</style>
