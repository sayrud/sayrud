import { defineStore } from 'pinia'
import { ref } from 'vue'

import { siteApi, type SiteInfo } from '@/api/site'

const DEFAULT_INFO: SiteInfo = {
  siteName: 'Sayrud',
  allowSignUp: true,
  passwordMinLength: 8,
  allowPasswordSignIn: true,
  loginNotice: '',
  providers: [],
}

export const useSiteStore = defineStore('site', () => {
  const info = ref<SiteInfo>({ ...DEFAULT_INFO })
  let loading: Promise<SiteInfo> | null = null
  let loaded = false

  /** Requests only once, the defaults are kept if it fails. */
  function ensureLoaded(): Promise<SiteInfo> {
    if (loaded) return Promise.resolve(info.value)
    loading ??= refresh().finally(() => (loading = null))
    return loading
  }

  async function refresh(): Promise<SiteInfo> {
    try {
      info.value = await siteApi.get()
      loaded = true
    } catch {
      // The site information only affects the titles and hints, keep the defaults if it fails.
    }
    return info.value
  }

  /** Builds the page title as "page · site name", or the site name alone. */
  function title(page?: string): string {
    return page ? `${page} · ${info.value.siteName}` : info.value.siteName
  }

  return { info, ensureLoaded, refresh, title }
})
