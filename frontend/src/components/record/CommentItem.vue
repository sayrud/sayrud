<script setup lang="ts">
import { Pencil, Reply, Trash2, Workflow } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { RecordComment } from '@/api/comments'
import MarkdownView from '@/components/common/MarkdownView.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'
import { userName, userOf, workflowName } from '@/utils/directory'

const props = defineProps<{
  comment: RecordComment
  /** Indents the reply, and names the reply replied to if it is not the root. */
  reply?: boolean
  replyToName?: string
  canReply: boolean
  canEdit: boolean
  canDelete: boolean
}>()
const emit = defineEmits<{ reply: []; edit: []; delete: [] }>()
const { t } = useI18n()

const workflowId = computed(() => props.comment.author.workflowId ?? 0)
const author = computed(() => (workflowId.value ? undefined : userOf(props.comment.author.userId ?? 0)))
const name = computed(() => {
  if (workflowId.value) return workflowName(workflowId.value) || t('member.deletedWorkflow')
  return author.value?.user.userName || t('member.unknown')
})
const time = computed(() => {
  const d = dayjs(props.comment.createdAt)
  return d.format(d.isSame(dayjs(), 'year') ? 'MM/DD HH:mm' : 'YYYY/MM/DD HH:mm')
})
const mentionName = (id: number) => userName(id)
</script>

<template>
  <div class="comment" :class="{ reply }">
    <span class="avatar">
      <span v-if="workflowId" class="workflow-avatar" :class="{ small: reply }"><Workflow :size="reply ? 12 : 14" /></span>
      <UserAvatar
        v-else
        :name="name"
        :color="author?.user.color ?? 'var(--color-fill-4)'"
        :avatar-url="author?.user.avatarUrl"
        :bot="!!author?.user.isBot"
        :size="reply ? 22 : 28"
      />
    </span>
    <div class="main">
      <div class="head">
        <span class="name ellipsis" :title="name">{{ name }}</span>
        <a-tag v-if="workflowId" size="small" color="purple" :bordered="false">{{ t('comments.workflow') }}</a-tag>
        <span class="time">{{ time }}</span>
        <a-tooltip v-if="comment.editedAt" :content="dayjs(comment.editedAt).format('YYYY/MM/DD HH:mm')" mini>
          <span class="time">{{ t('comments.edited') }}</span>
        </a-tooltip>
        <span class="spacer" />
        <div v-if="!comment.deleted" class="actions">
          <a-tooltip v-if="canReply" :content="t('comments.reply')" mini>
            <a-button type="text" size="mini" class="btn-muted" :aria-label="t('comments.reply')" @click="emit('reply')">
              <template #icon><Reply :size="14" /></template>
            </a-button>
          </a-tooltip>
          <a-tooltip v-if="canEdit" :content="t('common.edit')" mini>
            <a-button type="text" size="mini" class="btn-muted" :aria-label="t('common.edit')" @click="emit('edit')">
              <template #icon><Pencil :size="14" /></template>
            </a-button>
          </a-tooltip>
          <a-popconfirm v-if="canDelete" :content="t('comments.deleteConfirm')" type="warning" position="br" :ok-button-props="{ status: 'danger' }" @ok="emit('delete')">
            <a-button type="text" size="mini" class="btn-muted" :aria-label="t('common.delete')">
              <template #icon><Trash2 :size="14" /></template>
            </a-button>
          </a-popconfirm>
        </div>
      </div>
      <div v-if="comment.deleted" class="deleted">{{ t('comments.deleted') }}</div>
      <template v-else>
        <div v-if="replyToName" class="reply-to">{{ t('comments.replyTo', { name: replyToName }) }}</div>
        <MarkdownView class="body" :source="comment.body" :mention-name="mentionName" compact />
      </template>
    </div>
  </div>
</template>

<style scoped>
.comment {
  display: flex;
  gap: 10px;
  padding: 8px 8px 8px 0;
  border-radius: 8px;
}
.comment.reply {
  gap: 8px;
  padding: 6px 8px 6px 0;
}
.avatar {
  flex: none;
  display: inline-flex;
  padding-top: 2px;
}
.workflow-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  color: rgb(var(--purple-6));
  background: rgb(var(--purple-1));
}
.workflow-avatar.small {
  width: 22px;
  height: 22px;
}
.main {
  flex: 1;
  min-width: 0;
}
.head {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 24px;
}
.name {
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-1);
}
.time {
  flex: none;
  font-size: 12px;
  color: var(--color-text-3);
}
.spacer {
  flex: 1;
}
.actions {
  display: flex;
  opacity: 0;
  transition: opacity 0.15s;
}
.comment:hover .actions,
.actions:focus-within {
  opacity: 1;
}
.reply-to {
  font-size: 12px;
  color: var(--color-text-3);
}
.body {
  font-size: 13.5px;
}
.deleted {
  font-size: 13px;
  font-style: italic;
  color: var(--color-text-3);
}
</style>
