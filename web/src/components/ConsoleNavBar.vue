<template>
  <t-head-menu :value="currentTab" v-model:expanded="expendTab" @change="onSelectMenu" @expand="onExpendMenu">
    <template #logo>
      <img height="30" :src="appStore.theme === 'dark' ? logoDark : logo" alt="logo"/>
    </template>
    <t-menu-item value="Dashboard">
      <template #icon>
        <t-icon name="dashboard"/>
      </template>
      仪表盘
    </t-menu-item>
    <t-menu-item value="Projects">
      <template #icon>
        <t-icon name="app"/>
      </template>
      项目
    </t-menu-item>

    <div class="divider" v-if="projectStore.currentProject !== null"></div>

    <t-submenu value="CurrentProject" v-if="projectStore.currentProject !== null">
      <template #title>
        <span>{{ projectStore.currentProject?.name }}</span>
      </template>
      <t-menu-item value="SchemalessTable">数据表</t-menu-item>
      <t-menu-item value="SchemalessRecord">记录</t-menu-item>
      <t-menu-item value="SchemalessApi">接口</t-menu-item>
      <t-menu-item value="ProjectSettings">项目设置</t-menu-item>
    </t-submenu>

    <template #operations>
      <t-button variant="text" shape="square" @click="onSwitchTheme">
        <template #icon>
          <t-icon name="brightness-1"/>
        </template>
      </t-button>

      <t-dropdown :min-column-width="120" trigger="click">
        <template #dropdown>
          <t-dropdown-menu>
            <t-dropdown-item @click="onProfile">
              <user-circle-icon/>
              个人信息
            </t-dropdown-item>
            <t-dropdown-item @click="onLogOut">
              <poweroff-icon/>
              退出登录
            </t-dropdown-item>
          </t-dropdown-menu>
        </template>
        <t-button theme="default" variant="text">
          <template #icon>
            <t-icon name="user-circle"/>
          </template>
          <div>{{ profile.userName }}</div>
          <template #suffix>
            <chevron-down-icon/>
          </template>
        </t-button>
      </t-dropdown>
    </template>
  </t-head-menu>
</template>

<script setup lang="ts">
import {computed, onMounted, ref} from "vue";
import {useAppStore, useProjectStore} from '@/store'
import {useRoute, useRouter} from "vue-router";
import logo from '@/assets/logo.svg'
import logoDark from '@/assets/logo-dark.svg'
import {userProfile, type UserProfileResp} from "@/api/auth.ts";
import {ChevronDownIcon, PoweroffIcon, UserCircleIcon} from 'tdesign-icons-vue-next';

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const projectStore = useProjectStore()
const onSelectMenu = (selectedMenu: string) => {
  router.push({name: selectedMenu})
}

const expendTab = computed(() => {
  return route.name === 'ProjectSettings' || route.name.toString().startsWith('Schemaless') ? ['CurrentProject'] : []
})
const onExpendMenu = () => {
  router.push({name: 'SchemalessTable', params: {uid: projectStore.currentProject?.uid}})
}

const currentTab = computed(() => {
  return route.name
})

const onSwitchTheme = () => {
  if (!appStore.theme || appStore.theme === 'light') {
    document.documentElement.setAttribute('theme-mode', 'dark');
    appStore.setTheme('dark')
  } else {
    document.documentElement.removeAttribute('theme-mode');
    appStore.setTheme('light')
  }
}

const onProfile = () => {
  router.push({name: 'Profile'})
}

const onLogOut = () => {
  appStore.cleanToken()
  router.push({name: 'SignIn'})
}

const profile = ref<UserProfileResp>({} as UserProfileResp)
const getUserProfile = () => {
  userProfile().then(res => {
    profile.value = res
  })
}

onMounted(() => {
  getUserProfile()
})
</script>

<style scoped>
.divider {
  height: 20px;
  width: 2px;
  background-color: var(--td-gray-color-3);
  margin: 0px 14px;
}
</style>
