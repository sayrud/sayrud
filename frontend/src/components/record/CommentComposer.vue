<script setup lang="ts">
import { Send, X } from '@lucide/vue'
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import UserAvatar from '@/components/common/UserAvatar.vue'
import { trackMentions, type ChosenMember, type EditorMention } from '@/utils/comments'
import { directory } from '@/utils/directory'

const text = defineModel<string>('text', { required: true })
const mentions = defineModel<EditorMention[]>('mentions', { required: true })
const props = defineProps<{
  /** Name of the author replied to, or empty for a root comment. */
  replyToName?: string
  editing?: boolean
  sending?: boolean
  /** Characters allowed in a comment, the same as collab.MaxCommentLength of the backend. */
  maxLength: number
}>()
const emit = defineEmits<{ submit: []; cancel: [] }>()
const { t } = useI18n()

const mentionRef = ref<{ focus?: () => void; $el?: HTMLElement }>()
const keyword = ref('')

/** The members matching the text after the at sign, the Bots included. */
const candidates = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  return directory.value.members.filter((u) => !k || u.userName.toLowerCase().includes(k) || u.email.toLowerCase().includes(k)).slice(0, 20)
})
/** The option values are the user IDs to tell apart the members of the same name, onInput replaces the inserted ID with the name. */
const options = computed(() => candidates.value.map((u) => ({ value: String(u.id), label: u.userName })))
const length = computed(() => [...text.value].length)
const canSend = computed(() => !!text.value.trim() && length.value <= props.maxLength && !props.sending)

/** The member chosen from the candidates, whose name is inserted by the next input. */
let chosen: ChosenMember | undefined

function onSelect(value: unknown) {
  const user = directory.value.members.find((u) => String(u.id) === value)
  chosen = user && { id: user.id, name: user.userName, inserted: String(value) }
}

function onInput(next: string) {
  // The textarea still holds the previous text when a candidate is chosen, and the edited one when typing; the text after the cursor is untouched in both.
  const textarea = mentionRef.value?.$el?.querySelector('textarea')
  const after = textarea ? textarea.value.length - textarea.selectionEnd : undefined
  const edited = trackMentions(text.value, next, mentions.value, chosen, after)
  chosen = undefined
  mentions.value = edited.mentions
  text.value = edited.text
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
    e.preventDefault()
    if (canSend.value) emit('submit')
  }
  if (e.key === 'Escape' && (props.replyToName || props.editing)) {
    e.stopPropagation()
    emit('cancel')
  }
}

/** Focuses the input with the cursor at the end, e.g. after choosing to reply. */
function focus() {
  void nextTick(() => {
    const textarea = mentionRef.value?.$el?.querySelector('textarea')
    if (!textarea) return
    textarea.focus()
    textarea.setSelectionRange(textarea.value.length, textarea.value.length)
  })
}
defineExpose({ focus })
</script>

<template>
  <div class="composer">
    <div v-if="replyToName || editing" class="context">
      <span class="ellipsis">{{ editing ? t('comments.editing') : t('comments.replyTo', { name: replyToName }) }}</span>
      <a-button type="text" size="mini" class="btn-muted" :aria-label="t('common.cancel')" @click="emit('cancel')">
        <template #icon><X :size="13" /></template>
      </a-button>
    </div>
    <a-mention
      ref="mentionRef"
      :model-value="text"
      type="textarea"
      :data="options"
      :auto-size="{ minRows: 2, maxRows: 8 }"
      :placeholder="t('comments.placeholder')"
      class="input"
      @search="(text: string) => (keyword = text)"
      @select="onSelect"
      @update:model-value="onInput"
      @keydown="onKeydown"
    >
      <template #option="{ data }">
        <span class="option">
          <template v-for="u in candidates.filter((c) => String(c.id) === data.value)" :key="u.id">
            <UserAvatar :name="u.userName" :color="u.color" :avatar-url="u.avatarUrl" :bot="!!u.isBot" :size="20" />
            <span class="ellipsis">{{ u.userName }}</span>
            <span class="email ellipsis">{{ u.isBot ? t('member.bot') : u.email }}</span>
          </template>
        </span>
      </template>
    </a-mention>
    <div class="footer">
      <span class="hint" :class="{ over: length > maxLength }">
        {{ length > maxLength ? t('comments.tooLong', { n: maxLength }) : t('comments.sendHint') }}
      </span>
      <a-button type="primary" size="small" :disabled="!canSend" :loading="sending" @click="emit('submit')">
        <template #icon><Send :size="13" /></template>
        {{ editing ? t('common.save') : t('comments.send') }}
      </a-button>
    </div>
  </div>
</template>

<style scoped>
.composer {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 16px 14px;
  border-top: 1px solid var(--color-border-2);
  background: var(--color-bg-2);
}
.context {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 2px 4px 2px 10px;
  border-radius: 6px;
  font-size: 12px;
  color: var(--color-text-2);
  background: var(--color-fill-2);
}
.input :deep(textarea) {
  font-size: 13.5px;
}
.option {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.email {
  font-size: 12px;
  color: var(--color-text-3);
}
.footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.hint {
  font-size: 12px;
  color: var(--color-text-3);
}
.hint.over {
  color: rgb(var(--danger-6));
}
</style>
