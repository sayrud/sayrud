<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminApi, type AdminProject } from '@/api/admin'
import UserPicker from './UserPicker.vue'

const { t } = useI18n()

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
    Message.warning(t('admin.transferOwner.required'))
    return false
  }
  try {
    await adminApi.transferProject(props.project.uid, userID.value)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    return false
  }
  Message.success(t('admin.transferOwner.transferred'))
  emit('transferred')
  return true
}
</script>

<template>
  <a-modal
    :visible="visible"
    :title="t('admin.transferOwner.title')"
    :width="480"
    title-align="start"
    :ok-text="t('admin.projects.transfer')"
    :on-before-ok="submit"
    @update:visible="emit('update:visible', $event)"
  >
    <p class="hint">
      {{ t('admin.transferOwner.hint', { name: project?.name ?? '' }) }}
      <template v-if="project?.owner">{{ t('admin.transferOwner.previousOwner', { name: project.owner.userName }) }}</template>
    </p>
    <a-form :model="{}" layout="vertical">
      <a-form-item :label="t('admin.transferOwner.newOwner')" required>
        <UserPicker
          v-if="visible"
          v-model="userID"
          :exclude-ids="project?.owner ? [project.owner.id] : []"
          :placeholder="t('admin.userPicker.placeholder')"
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
