<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { ref, watch } from 'vue'

import { adminApi, type AdminProject } from '@/api/admin'
import UserPicker from './UserPicker.vue'

const props = defineProps<{ visible: boolean; project: AdminProject | null }>()
const emit = defineEmits<{ 'update:visible': [value: boolean]; transferred: [] }>()

const userID = ref<number>()

watch(
  () => props.visible,
  (v) => {
    if (v) userID.value = undefined
  },
)

async function submit() {
  if (!props.project) return false
  if (!userID.value) {
    Message.warning('请选择新所有者')
    return false
  }
  try {
    await adminApi.transferProject(props.project.uid, userID.value)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    return false
  }
  Message.success('所有者已转移')
  emit('transferred')
  return true
}
</script>

<template>
  <a-modal
    :visible="visible"
    title="转移所有者"
    :width="480"
    title-align="start"
    ok-text="转移"
    :on-before-ok="submit"
    @update:visible="emit('update:visible', $event)"
  >
    <p class="hint">
      将「{{ project?.name }}」转移给其他成员。
      <template v-if="project?.owner">原所有者 {{ project.owner.userName }} 将成为「可管理」协作者。</template>
    </p>
    <a-form :model="{}" layout="vertical">
      <a-form-item label="新所有者" required>
        <UserPicker
          v-if="visible"
          v-model="userID"
          :exclude-ids="project?.owner ? [project.owner.id] : []"
          placeholder="搜索成员名称或邮箱"
        />
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<style scoped>
.hint {
  margin: 0 0 16px;
  line-height: 22px;
  color: var(--text-caption);
}
</style>
