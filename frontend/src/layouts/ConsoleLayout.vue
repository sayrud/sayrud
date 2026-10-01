<script setup lang="ts">
import { House, Menu } from '@lucide/vue'
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import logoDark from '@/assets/logo-dark.svg'
import logo from '@/assets/logo.svg'
import UserMenu from '@/components/common/UserMenu.vue'
import ConsoleNav from '@/components/console/ConsoleNav.vue'
import { useSiteStore } from '@/stores/site'
import { useThemeStore } from '@/stores/theme'
import { navLabel, type NavGroup } from './consoleNav'

const props = defineProps<{ title: string; nav: NavGroup[] }>()

const route = useRoute()
const router = useRouter()
const site = useSiteStore()
const themeStore = useThemeStore()
const drawerVisible = ref(false)

watch(
  [() => route.name, () => site.info.siteName],
  () => (document.title = site.title(navLabel(props.nav, route.name) ?? props.title)),
  { immediate: true },
)
site.ensureLoaded()
</script>

<template>
  <a-layout class="console">
    <a-layout-header class="console-header">
      <RouterLink to="/" class="brand">
        <img :src="themeStore.theme === 'dark' ? logoDark : logo" :alt="site.info.siteName" />
      </RouterLink>
      <a-divider direction="vertical" class="header-divider" />
      <span class="console-title">{{ title }}</span>
      <div class="header-right">
        <a-button class="menu-btn" type="text" shape="square" aria-label="打开菜单" @click="drawerVisible = true">
          <template #icon><Menu :size="18" /></template>
        </a-button>
        <a-button type="text" class="home-btn" @click="router.push('/')">
          <template #icon><House :size="15" /></template>
          <span class="home-text">返回首页</span>
        </a-button>
        <UserMenu />
      </div>
    </a-layout-header>

    <a-layout class="console-body">
      <a-layout-sider class="console-sider" :width="232">
        <ConsoleNav :nav="nav" />
      </a-layout-sider>
      <a-layout-content class="console-main">
        <div class="console-content">
          <RouterView />
        </div>
      </a-layout-content>
    </a-layout>

    <a-drawer
      v-model:visible="drawerVisible"
      placement="left"
      :width="260"
      :footer="false"
      :title="title"
      unmount-on-close
      class="console-drawer"
    >
      <ConsoleNav :nav="nav" @navigate="drawerVisible = false" />
    </a-drawer>
  </a-layout>
</template>

<style scoped>
.console {
  height: 100%;
  background: var(--bg-base);
}
.console-header {
  display: flex;
  flex: none;
  align-items: center;
  gap: 12px;
  height: 56px;
  padding: 0 24px;
  background: var(--bg-body);
  border-bottom: 1px solid var(--line-border);
}
.menu-btn {
  display: none;
}
/* The logo and the avatar keep the same size and position as the home page header. */
.brand {
  display: flex;
  align-items: center;
}
.brand img {
  height: 32px;
  width: auto;
}
.header-divider {
  height: 16px;
  margin: 0;
  border-color: var(--line-border-strong);
}
.console-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-title);
  white-space: nowrap;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-left: auto;
}
.home-btn.arco-btn,
.menu-btn.arco-btn {
  color: var(--text-caption);
}
.home-btn.arco-btn:hover,
.menu-btn.arco-btn:hover {
  color: var(--text-title);
}
.console-body {
  flex: 1;
  min-height: 0;
}
.console-sider {
  overflow-y: auto;
  background: var(--bg-body);
  border-right: 1px solid var(--line-border);
  box-shadow: none;
}
.console-main {
  min-width: 0;
  overflow-y: auto;
}
.console-content {
  max-width: 1080px;
  margin: 0 auto;
  padding: 24px 32px 48px;
}
@media (max-width: 768px) {
  .console-header {
    gap: 8px;
    padding: 0 16px;
  }
  .menu-btn.arco-btn {
    display: inline-flex;
  }
  .console-sider {
    display: none;
  }
  .console-content {
    padding: 16px 16px 40px;
  }
  .home-text {
    display: none;
  }
}
</style>

<style>
.console-drawer .arco-drawer-body {
  padding: 0;
}
</style>
