<script setup lang="ts">
import dayjs from 'dayjs'
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

import { membersApi } from '@/api/bitable'
import UserAvatar from '@/components/common/UserAvatar.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { SelectOption, SLField } from '@/types/bitable'
import type { WorkflowValue } from '@/types/workflow'
import { directory, setMembers } from '@/utils/directory'
import { isDateField, isUserField } from '@/utils/fieldTypes'

const props = defineProps<{ field: SLField; op: string; disabled?: boolean }>()
const model = defineModel<WorkflowValue | undefined>()
const { t } = useI18n()
const editor = useWorkflowEditorStore()
let mounted = true
onBeforeUnmount(() => { mounted = false })

/** in and nin compare against multiple values, and so do the writes of the multiple member fields. */
const multiple = computed(() => props.op === 'in' || props.op === 'nin' || (!props.op && props.field.type === 'member' && !!(props.field.metadata as { multiple?: boolean }).multiple))

// The members are written and compared by their emails, which the server resolves.
onMounted(async () => {
  if (!isUserField(props.field) || directory.value.members.length || !editor.projectUID) return
  const projectUID = editor.projectUID
  try {
    const list = await membersApi.list(projectUID)
    if (mounted && editor.projectUID === projectUID) setMembers(list.map((m) => m.user))
  } catch {
    // The emails can still be entered as texts.
  }
})
const value = computed(() => (model.value?.kind === 'const' ? model.value.const : undefined))
const options = computed(() => ((props.field.metadata as { options?: SelectOption[] }).options ?? []).map((o) => o.name))

function set(v: unknown) {
  model.value = v === undefined || v === '' ? undefined : { kind: 'const', const: v }
}

const list = computed(() => (Array.isArray(value.value) ? value.value.map(String) : value.value === undefined ? [] : [String(value.value)]))
function setList(items: (string | number)[]) {
  set(items.length ? items.map((x) => (props.field.type === 'number' ? Number(x) : String(x))) : undefined)
}

const dateValue = computed(() => (typeof value.value === 'string' && value.value ? value.value : undefined))
function setDate(v: unknown) {
  set(typeof v === 'string' && v ? dayjs(v).format() : undefined)
}
</script>

<template>
  <a-select
    v-if="field.type === 'single_select' || field.type === 'multi_select'"
    :model-value="multiple ? list : (value as string | undefined)"
    :multiple="multiple"
    :disabled="disabled"
    :placeholder="t('workflow.record.selectOption')"
    allow-clear
    @change="(v) => (multiple ? setList(v as string[]) : set(v))"
  >
    <a-option v-for="name in options" :key="name" :value="name">{{ name }}</a-option>
  </a-select>
  <a-select
    v-else-if="isUserField(field)"
    :model-value="multiple ? list : (value as string | undefined)"
    :multiple="multiple"
    :disabled="disabled"
    :placeholder="t('member.placeholder')"
    allow-clear
    allow-search
    allow-create
    @change="(v) => (multiple ? setList(v as string[]) : set(v))"
  >
    <a-option v-for="u in directory.members" :key="u.id" :value="u.email" :label="u.userName">
      <span class="user-option">
        <UserAvatar :name="u.userName" :color="u.color" :avatar-url="u.avatarUrl" :bot="!!u.isBot" :size="18" />
        <span>{{ u.userName }}</span>
      </span>
    </a-option>
  </a-select>
  <a-input-tag
    v-else-if="multiple"
    :model-value="list"
    :disabled="disabled"
    :placeholder="t('workflow.record.enterValues')"
    allow-clear
    unique-value
    @change="(v) => setList(v as string[])"
  />
  <a-input-number v-else-if="field.type === 'number'" :model-value="typeof value === 'number' ? value : undefined" :disabled="disabled" @change="(v) => set(v ?? undefined)" />
  <a-date-picker
    v-else-if="isDateField(field)"
    :model-value="dateValue"
    show-time
    format="YYYY-MM-DD HH:mm:ss"
    :time-picker-props="{ format: 'HH:mm:ss' }"
    :disabled="disabled"
    style="width: 100%"
    @change="setDate"
  />
  <a-radio-group v-else-if="field.type === 'checkbox'" type="button" size="small" :model-value="value === true" :disabled="disabled" @change="(v) => set(v === true)">
    <a-radio :value="true">{{ t('workflow.record.checked') }}</a-radio>
    <a-radio :value="false">{{ t('workflow.record.unchecked') }}</a-radio>
  </a-radio-group>
  <a-input v-else :model-value="typeof value === 'string' ? value : ''" :disabled="disabled" :placeholder="t('workflow.value.constPlaceholder')" allow-clear @input="set" @clear="set(undefined)" />
</template>


<style scoped>
.user-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
</style>
