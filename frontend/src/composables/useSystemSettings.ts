import { Message } from '@arco-design/web-vue'
import { computed, onMounted, reactive, ref } from 'vue'

import { adminApi, type SystemSettings } from '@/api/admin'
import { useSiteStore } from '@/stores/site'

/** Shared by the admin settings pages: loads the settings, edits the draft and refreshes the site information after saving. */
export function useSystemSettings() {
  const site = useSiteStore()
  const original = ref<SystemSettings | null>(null)
  const draft = reactive<SystemSettings>({ siteName: '', allowSignUp: true, passwordMinLength: 8, sessionTTLDays: 30 })
  const loading = ref(true)
  const saving = ref(false)

  const dirty = computed(
    () => !!original.value && (Object.keys(draft) as (keyof SystemSettings)[]).some((k) => draft[k] !== original.value![k]),
  )

  function reset() {
    if (original.value) Object.assign(draft, original.value)
  }

  async function save() {
    saving.value = true
    try {
      original.value = await adminApi.saveSettings({ ...draft, siteName: draft.siteName.trim() })
      reset()
      await site.refresh()
      Message.success('已保存')
    } catch (e) {
      Message.error(e instanceof Error ? e.message : String(e))
    } finally {
      saving.value = false
    }
  }

  onMounted(async () => {
    try {
      original.value = await adminApi.settings()
      reset()
    } catch (e) {
      Message.error(e instanceof Error ? e.message : String(e))
    } finally {
      loading.value = false
    }
  })

  return { draft, loading, saving, dirty, save, reset }
}
