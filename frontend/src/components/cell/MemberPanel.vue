<script setup lang="ts">
import { Check } from '@lucide/vue'
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import UserAvatar from '@/components/common/UserAvatar.vue'
import { useBaseStore } from '@/stores/base'
import type { CellValue, SLField, UserBrief } from '@/types/bitable'
import { directory, memberIds, userOf } from '@/utils/directory'
import MemberTag from './MemberTag.vue'

const { t } = useI18n()

const props = defineProps<{
  field: SLField
  value: CellValue
  initialQuery?: string
}>()
const emit = defineEmits<{ change: [value: CellValue]; close: [] }>()

const store = useBaseStore()
const multiple = computed(() => !!(props.field.metadata as { multiple?: boolean }).multiple)
const query = ref(props.initialQuery ?? '')
const active = ref(0)
const input = ref<HTMLInputElement>()
const listEl = ref<HTMLElement>()

const selected = computed(() => memberIds(props.value))
/** The members who left stay in the list while selected, so they can be removed. */
const candidates = computed<UserBrief[]>(() => {
  const members = directory.value.members
  const left = selected.value.filter((id) => !members.some((u) => u.id === id)).map((id) => userOf(id)?.user).filter(Boolean) as UserBrief[]
  return [...members, ...left]
})
const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return candidates.value
  return candidates.value.filter((u) => u.userName.toLowerCase().includes(q) || (!u.isBot && u.email.toLowerCase().includes(q)))
})

watch(query, () => (active.value = 0))

function toggle(id: number) {
  if (multiple.value) {
    const next = selected.value.includes(id) ? selected.value.filter((x) => x !== id) : [...selected.value, id]
    emit('change', next.length ? next : null)
    query.value = ''
  } else {
    emit('change', selected.value[0] === id ? null : [id])
    emit('close')
  }
}

function onKey(e: KeyboardEvent) {
  if (e.isComposing) return
  const n = Math.max(1, filtered.value.length)
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    active.value = (active.value + 1) % n
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    active.value = (active.value - 1 + n) % n
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const user = filtered.value[active.value]
    if (user) toggle(user.id)
  } else if (e.key === 'Backspace' && !query.value && multiple.value && selected.value.length) {
    const next = selected.value.slice(0, -1)
    emit('change', next.length ? next : null)
  } else if (e.key === 'Escape') {
    e.preventDefault()
    e.stopPropagation()
    emit('close')
  }
  nextTick(() => listEl.value?.querySelector('.active')?.scrollIntoView({ block: 'nearest' }))
}

onMounted(() => {
  input.value?.focus()
  void store.loadMembers()
})
</script>

<template>
  <div class="member-panel" @keydown.stop>
    <div class="search-box">
      <MemberTag v-for="id in multiple ? selected : []" :key="id" :user-id="id" closable @close="toggle(id)" />
      <input ref="input" v-model="query" :placeholder="t('member.search')" @keydown="onKey" />
    </div>
    <div ref="listEl" class="list">
      <div
        v-for="(u, i) in filtered"
        :key="u.id"
        class="item"
        :class="{ active: i === active, left: userOf(u.id)?.left }"
        @mouseenter="active = i"
        @click="toggle(u.id)"
      >
        <UserAvatar :name="u.userName" :color="u.color" :avatar-url="u.avatarUrl" :bot="!!u.isBot" :size="24" class="item-avatar" />
        <span class="name">{{ u.userName }}</span>
        <span v-if="userOf(u.id)?.left" class="desc">{{ t('member.left') }}</span>
        <span v-else-if="u.isBot" class="desc">{{ u.ownerName ? t('member.botOf', { name: u.ownerName }) : t('member.bot') }}</span>
        <Check v-if="selected.includes(u.id)" :size="16" class="check" />
      </div>
      <a-empty v-if="!filtered.length" class="empty" :description="t('member.empty')" />
    </div>
  </div>
</template>

<style scoped>
.member-panel {
  width: 100%;
  min-width: 260px;
}
.search-box {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--line-divider);
}
.search-box input {
  flex: 1;
  min-width: 80px;
  height: 24px;
  border: none;
  outline: none;
  font-size: 14px;
  color: var(--color-text-1);
  background: transparent;
}
.search-box input::placeholder {
  color: var(--color-text-4);
}
.list {
  max-height: 320px;
  overflow: auto;
  padding: 4px;
}
.item {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  padding: 0 8px;
  border-radius: 6px;
  cursor: pointer;
}
.item.active {
  background: var(--color-fill-2);
}
.item.left .item-avatar {
  filter: grayscale(1);
  opacity: 0.5;
}
.name {
  flex: none;
  max-width: 50%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text-1);
}
.item.left .name {
  color: var(--color-text-3);
}
.desc {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  color: var(--color-text-3);
}
.check {
  flex: none;
  margin-left: auto;
  color: var(--color-primary);
}
.empty {
  padding: 12px 0;
}
</style>
