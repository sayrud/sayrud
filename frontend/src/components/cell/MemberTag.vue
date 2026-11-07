<script setup lang="ts">
import { Workflow } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import UserAvatar from '@/components/common/UserAvatar.vue'
import { userOf, workflowName } from '@/utils/directory'

const props = withDefaults(
  defineProps<{
    /** User shown, the workflow is shown instead if workflowId is set. */
    userId?: number
    workflowId?: number
    closable?: boolean
    /** Compact size for the inline texts. */
    small?: boolean
  }>(),
  { userId: 0, workflowId: 0, closable: false, small: false },
)
defineEmits<{ close: [] }>()
const { t } = useI18n()

const item = computed(() => (props.workflowId ? undefined : userOf(props.userId)))
const name = computed(() => {
  if (props.workflowId) return workflowName(props.workflowId) || t('member.deletedWorkflow')
  return item.value?.user.userName || t('member.unknown')
})
const left = computed(() => !props.workflowId && (!item.value || item.value.left))
/** Only the members who left or no longer exist explain themselves on hover. */
const tip = computed(() => {
  if (props.workflowId) return ''
  if (!item.value) return t('member.unknownTip')
  return item.value.left ? t('member.leftTip', { name: name.value }) : ''
})
</script>

<template>
  <a-tooltip :content="tip" :disabled="!tip" mini position="top" :popup-container="'body'">
    <a-tag class="member-tag" :class="{ left, small }" :bordered="false" :closable="closable" @close="$emit('close')">
      <template #icon>
        <span class="member-avatar">
          <span v-if="workflowId" class="workflow-icon"><Workflow :size="small ? 10 : 12" /></span>
          <UserAvatar
            v-else
            :name="name"
            :color="item?.user.color ?? 'var(--color-fill-4)'"
            :avatar-url="item?.user.avatarUrl"
            :bot="!!item?.user.isBot"
            :size="small ? 16 : 20"
          />
        </span>
      </template>
      {{ name }}
    </a-tag>
  </a-tooltip>
</template>

<style scoped>
/* Plain avatar and name like the other cell texts, the capsule only appears on hover. */
.member-tag.arco-tag {
  flex: none;
  max-width: 100%;
  height: 24px;
  padding: 0 6px 0 2px;
  border-radius: 12px;
  font-size: 14px;
  font-weight: 400;
  color: var(--color-text-1);
  background: transparent;
  cursor: default;
  transition: background-color 0.15s;
}
.member-tag.arco-tag:hover {
  background: var(--color-fill-2);
}
.member-tag :deep(.arco-tag-icon) {
  display: inline-flex;
  margin-right: 6px;
}
.member-tag :deep(.arco-tag-content) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.member-tag :deep(.arco-tag-close-btn) {
  margin-left: 4px;
  color: var(--color-text-3);
}
.member-avatar {
  display: inline-flex;
}
.member-tag.small.arco-tag {
  height: 20px;
  font-size: 12px;
}
.member-tag.small .workflow-icon {
  width: 16px;
  height: 16px;
}
.member-tag.left.arco-tag {
  color: var(--color-text-3);
}
.member-tag.left .member-avatar {
  filter: grayscale(1);
  opacity: 0.5;
}
.workflow-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  color: rgb(var(--purple-6));
  background: rgb(var(--purple-1));
}
</style>
