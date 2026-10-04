<script setup lang="ts">
import { IconRobot } from '@arco-design/web-vue/es/icon'
import { KeyRound, Network, Plug, ShieldCheck } from '@lucide/vue'
import { computed, type Component } from 'vue'

import deepseek from '@/assets/providers/deepseek-color.svg'
import gemini from '@/assets/providers/gemini-color.svg'
import gitea from '@/assets/providers/gitea.svg'
import githubDark from '@/assets/providers/github-dark.svg'
import github from '@/assets/providers/github.svg'
import gitlab from '@/assets/providers/gitlab.svg'
import gogs from '@/assets/providers/gogs.svg'
import google from '@/assets/providers/google.svg'
import groq from '@/assets/providers/groq.svg'
import keycloak from '@/assets/providers/keycloak.svg'
import kimi from '@/assets/providers/kimi.svg'
import lmstudio from '@/assets/providers/lmstudio.svg'
import microsoft from '@/assets/providers/microsoft.svg'
import minimax from '@/assets/providers/minimax-color.svg'
import ollama from '@/assets/providers/ollama.svg'
import openai from '@/assets/providers/openai.svg'
import openrouter from '@/assets/providers/openrouter.svg'
import qwen from '@/assets/providers/qwen-color.svg'
import siliconflow from '@/assets/providers/siliconcloud-color.svg'
import tokenhub from '@/assets/providers/tokenhub.svg'
import windows from '@/assets/providers/windows.svg'
import zhipu from '@/assets/providers/zhipu-color.svg'
import { useThemeStore } from '@/stores/theme'

const props = withDefaults(defineProps<{ icon: string; size?: number }>(), { size: 18 })

const themeStore = useThemeStore()

const BRANDS: Record<string, { light: string; dark?: string; mask?: boolean }> = {
  github: { light: github, dark: githubDark },
  gitlab: { light: gitlab },
  gogs: { light: gogs },
  gitea: { light: gitea },
  google: { light: google },
  microsoft: { light: microsoft },
  windows: { light: windows },
  keycloak: { light: keycloak },
  openai: { light: openai, mask: true },
  deepseek: { light: deepseek },
  kimi: { light: kimi, mask: true },
  zhipu: { light: zhipu },
  qwen: { light: qwen },
  minimax: { light: minimax },
  siliconflow: { light: siliconflow },
  tokenhub: { light: tokenhub },
  openrouter: { light: openrouter, mask: true },
  gemini: { light: gemini },
  groq: { light: groq, mask: true },
  ollama: { light: ollama, mask: true },
  lmstudio: { light: lmstudio, mask: true },
}

const PROTOCOLS: Record<string, Component> = { oidc: KeyRound, saml: ShieldCheck, ldap: Network }

const brandSrc = computed(() => {
  const brand = BRANDS[props.icon]
  if (!brand) return ''
  return themeStore.theme === 'dark' && brand.dark ? brand.dark : brand.light
})
</script>

<template>
  <span
    v-if="brandSrc && BRANDS[icon]?.mask"
    class="provider-icon provider-icon-mask"
    :style="{ width: `${size}px`, height: `${size}px`, '--provider-icon-mask': `url(${JSON.stringify(brandSrc)})` }"
    aria-hidden="true"
  />
  <img v-else-if="brandSrc" :src="brandSrc" :width="size" :height="size" alt="" class="provider-icon" />
  <IconRobot v-else-if="icon === 'custom'" :size="size" class="provider-icon" aria-hidden="true" />
  <component :is="PROTOCOLS[icon] ?? Plug" v-else :size="size" class="provider-icon" aria-hidden="true" />
</template>

<style scoped>
.provider-icon {
  flex: none;
  object-fit: contain;
}

.provider-icon-mask {
  display: inline-block;
  background-color: currentColor;
  -webkit-mask: var(--provider-icon-mask) center / contain no-repeat;
  mask: var(--provider-icon-mask) center / contain no-repeat;
}
</style>
