<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminApi } from '@/api/admin'
import UserAvatar from '@/components/common/UserAvatar.vue'

const { t } = useI18n()

export interface PickerUser {
  id: number
  userName: string
  email: string
  color: string
  avatarUrl?: string
}

const props = defineProps<{
  modelValue?: number
  /** Users that can not be picked, e.g. the deleted user or the current owner. */
  excludeIds?: number[]
  /** Initial options, to show the default value. */
  initial?: PickerUser[]
  placeholder?: string
}>()
const emit = defineEmits<{ 'update:modelValue': [value: number | undefined] }>()

const options = ref<PickerUser[]>(props.initial ?? [])
const loading = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
let seq = 0

async function search(keyword: string) {
  const current = ++seq
  loading.value = true
  try {
    const { users } = await adminApi.users({ keyword, status: 'active', pageSize: 20 })
    if (current !== seq) return
    const exclude = new Set(props.excludeIds ?? [])
    const selected = options.value.find((u) => u.id === props.modelValue)
    const found = users.filter((u) => !exclude.has(u.id))
    options.value = selected && !found.some((u) => u.id === selected.id) ? [selected, ...found] : found
  } finally {
    if (current === seq) loading.value = false
  }
}

function onSearch(keyword: string) {
  clearTimeout(timer)
  timer = setTimeout(() => search(keyword), 300)
}

watch(
  () => props.initial,
  (v) => {
    if (v?.length) options.value = [...v]
  },
)
onBeforeUnmount(() => clearTimeout(timer))
search('')
</script>

<template>
  <a-select
    :model-value="modelValue"
    :placeholder="placeholder ?? t('admin.userPicker.placeholder')"
    :loading="loading"
    :filter-option="false"
    allow-search
    @search="onSearch"
    @update:model-value="(v: unknown) => emit('update:modelValue', v as number | undefined)"
  >
    <a-option v-for="u in options" :key="u.id" :value="u.id" :label="`${u.userName}（${u.email}）`">
      <div class="picker-option">
        <UserAvatar :name="u.userName" :color="u.color" :avatar-url="u.avatarUrl" :size="22" />
        <span class="ellipsis">{{ u.userName }}</span>
        <span class="text-desc ellipsis">{{ u.email }}</span>
      </div>
    </a-option>
  </a-select>
</template>

<style scoped>
.picker-option {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
</style>
