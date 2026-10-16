import { defineStore } from 'pinia'
import { ref } from 'vue'

import { workflowsApi } from '@/api/workflow'
import type { WorkflowListItem } from '@/types/workflow'

/** The project's workflow list in the sidebar; kept in sync when the editor changes the name, trigger type, or publish state. */
export const useProjectWorkflowsStore = defineStore('projectWorkflows', () => {
  const projectUID = ref('')
  const list = ref<WorkflowListItem[]>([])
  const loading = ref(false)

  async function load(pid: string) {
    projectUID.value = pid
    loading.value = true
    try {
      const items = await workflowsApi.list(pid)
      if (projectUID.value === pid) list.value = items
    } catch {
      if (projectUID.value === pid) list.value = []
    } finally {
      loading.value = false
    }
  }

  function upsert(item: WorkflowListItem) {
    const i = list.value.findIndex((a) => a.uid === item.uid)
    if (i >= 0) list.value[i] = { ...list.value[i]!, ...item }
    else list.value.unshift(item)
  }

  function patch(uid: string, p: Partial<WorkflowListItem>) {
    const item = list.value.find((a) => a.uid === uid)
    if (item) Object.assign(item, p)
  }

  function remove(uid: string) {
    list.value = list.value.filter((a) => a.uid !== uid)
  }

  function clear() {
    projectUID.value = ''
    list.value = []
  }

  return { projectUID, list, loading, load, upsert, patch, remove, clear }
})
