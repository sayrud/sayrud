<script setup lang="ts">
import { KeyRound, Network, Plug, ShieldCheck } from '@lucide/vue'
import { computed, type Component } from 'vue'

import gitea from '@/assets/providers/gitea.svg'
import githubDark from '@/assets/providers/github-dark.svg'
import github from '@/assets/providers/github.svg'
import gitlab from '@/assets/providers/gitlab.svg'
import gogs from '@/assets/providers/gogs.svg'
import google from '@/assets/providers/google.svg'
import keycloak from '@/assets/providers/keycloak.svg'
import microsoft from '@/assets/providers/microsoft.svg'
import windows from '@/assets/providers/windows.svg'
import { useThemeStore } from '@/stores/theme'

const props = withDefaults(defineProps<{ icon: string; size?: number }>(), { size: 18 })

const themeStore = useThemeStore()

/** 品牌图标为 svgl.app 的官方 SVG（Gitea / Gogs 取自各自官方仓库），协议类用 lucide 图标。 */
const BRANDS: Record<string, { light: string; dark?: string }> = {
  github: { light: github, dark: githubDark },
  gitlab: { light: gitlab },
  gogs: { light: gogs },
  gitea: { light: gitea },
  google: { light: google },
  microsoft: { light: microsoft },
  windows: { light: windows },
  keycloak: { light: keycloak },
}

const PROTOCOLS: Record<string, Component> = { oidc: KeyRound, saml: ShieldCheck, ldap: Network }

const brandSrc = computed(() => {
  const brand = BRANDS[props.icon]
  if (!brand) return ''
  return themeStore.theme === 'dark' && brand.dark ? brand.dark : brand.light
})
</script>

<template>
  <img v-if="brandSrc" :src="brandSrc" :width="size" :height="size" alt="" class="provider-icon" />
  <component :is="PROTOCOLS[icon] ?? Plug" v-else :size="size" class="provider-icon" aria-hidden="true" />
</template>

<style scoped>
.provider-icon {
  flex: none;
  object-fit: contain;
}
</style>
