import { Message } from '@arco-design/web-vue'
import { computed, onMounted, reactive, ref } from 'vue'

import { adminApi, type SystemSettings } from '@/api/admin'
import { t } from '@/i18n'
import { useSiteStore } from '@/stores/site'

/** Shared by the admin settings pages: loads the settings, edits the draft and refreshes the site information after saving. */
export function useSystemSettings() {
  const site = useSiteStore()
  const original = ref<SystemSettings | null>(null)
  const draft = reactive<SystemSettings>({
    siteName: '',
    allowSignUp: true,
    passwordMinLength: 8,
    sessionTTLDays: 30,
    externalURL: '',
    allowPasswordSignIn: true,
    loginNotice: '',
  })
  const loading = ref(true)
  const saving = ref(false)

  const dirty = computed(
    () => !!original.value && (Object.keys(draft) as (keyof SystemSettings)[]).some((k) => draft[k] !== original.value![k]),
  )

  function reset() {
    if (original.value) Object.assign(draft, original.value)
  }

  /** Resolves false if saving fails, the error has been shown. */
  async function save(): Promise<boolean> {
    saving.value = true
    try {
      original.value = await adminApi.saveSettings({
        ...draft,
        siteName: draft.siteName.trim(),
        externalURL: draft.externalURL.trim(),
        loginNotice: draft.loginNotice.trim(),
      })
      reset()
      await site.refresh()
      Message.success(t('common.saved'))
      return true
    } catch (e) {
      Message.error(e instanceof Error ? e.message : String(e))
      return false
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
