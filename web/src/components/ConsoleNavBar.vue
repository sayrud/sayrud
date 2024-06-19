<template>
  <t-head-menu :value="route.name" @change="onSelectMenu">
    <template #logo>
      <img height="30" :src="appStore.theme === 'dark' ? logoDark : logo" alt="logo"/>
    </template>
    <t-menu-item value="Dashboard">仪表盘</t-menu-item>
    <t-menu-item value="Projects"> 项目</t-menu-item>

    <template #operations>
      <t-button variant="text" shape="square" @click="onSwitchTheme">
        <template #icon>
          <t-icon name="brightness-1"/>
        </template>
      </t-button>
    </template>
  </t-head-menu>
</template>

<script setup lang="ts">
import {useAppStore} from '@/store'
import {useRoute, useRouter} from "vue-router";
import logo from '@/assets/logo.svg'
import logoDark from '@/assets/logo-dark.svg'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const onSelectMenu = (selectedMenu: string) => {
  router.push({name: selectedMenu})
}

const onSwitchTheme = () => {
  if (!appStore.theme || appStore.theme === 'light') {
    document.documentElement.setAttribute('theme-mode', 'dark');
    appStore.setTheme('dark')
  } else {
    document.documentElement.removeAttribute('theme-mode');
    appStore.setTheme('light')
  }
}
</script>

<style scoped>

</style>
