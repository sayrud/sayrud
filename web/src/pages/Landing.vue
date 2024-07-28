<template>
  <t-layout class="layout">
    <t-header>
      <t-head-menu value="" theme="light">
        <template #logo>
          <img height="30" :src="appStore.theme === 'dark' ? logoDark : logo" alt="logo"/>
        </template>
        <t-menu-item href="https://github.red/hello-sayrud/" target="_blank"> 关于</t-menu-item>
        <template #operations>
          <t-button style="margin-right: 10px" variant="text" shape="square" @click="onSwitchTheme">
            <template #icon>
              <t-icon name="brightness-1"/>
            </template>
          </t-button>
          <t-button @click="router.push({name: 'SignIn'})">登录/注册</t-button>
        </template>
      </t-head-menu>
    </t-header>
    <div class="container">
      <h1 class="title">我不想再写 CRUD 了</h1>
      <p class="sub">点点点建表、建字段、操作数据、生成 API...</p>

      <t-space class="cards" :size="40">
        <t-card class="card" :shadow="false" :bordered="false" size="small" v-for="(item, index) in cards"
                v-bind:key="index">
          <template #title>
            <div class="card-header">
              <div class="card-icon">
                <component size="30" :is="item.icon"/>
              </div>
              <div class="card-title">{{ item.title }}</div>
            </div>
          </template>
          <div class="card-content">{{ item.content }}</div>
        </t-card>
      </t-space>

      <t-card class="waitlist" :bordered="false">
        <template #title>
          <h1>加入 Waitlist</h1>
          <span class="sub">Sayrud 还在开发中，仅邀请少量用户参与体验。如果你愿意和我一起共建，欢迎加入 Waitlist！</span>
        </template>
        <t-form ref="form" :data="waitListForm" :colon="true" :label-width="0" @submit="onSubmitWaitlist">
          <t-form-item name="account">
            <t-input clearable placeholder="请输入电子邮箱" size="large">
              <template #prefix-icon>
                <mail-icon/>
              </template>
            </t-input>
          </t-form-item>
          <t-form-item name="">
            <t-textarea placeholder="请输入申请理由" size="large" :autosize="{ minRows: 2, maxRows: 4 }"></t-textarea>
          </t-form-item>

          <t-form-item>
            <t-button theme="primary" type="submit" block size="large">提交</t-button>
          </t-form-item>
        </t-form>
      </t-card>

      <t-footer>Copyright @ 2024 Sayrud. All Rights Reserved</t-footer>
    </div>
  </t-layout>
</template>

<script setup lang="ts">
import logo from '@/assets/logo.svg'
import logoDark from '@/assets/logo-dark.svg'
import {useAppStore} from "@/store";
import {Measurement1Icon, Link1Icon, UserLockedIcon, EarthIcon, MailIcon} from 'tdesign-icons-vue-next';
import {useRouter} from 'vue-router'

const router = useRouter()
const appStore = useAppStore()
if (appStore.theme === 'dark') {
  document.documentElement.setAttribute('theme-mode', 'dark');
}

const cards = [
  {
    title: '图形化操作',
    icon: Measurement1Icon,
    content: '无需编写代码，通过图形化界面操作数据库结构。'
  },
  {
    title: 'API 定义',
    icon: Link1Icon,
    content: '支持生成多种数据类型与筛选条件的 API。'
  },
  {
    title: '权限管理',
    icon: UserLockedIcon,
    content: '支持多种权限与角色管理配置，保障数据安全。'
  },
  {
    title: '自定义域名',
    icon: EarthIcon,
    content: '支持为不同业务绑定自定义域名。'
  }
]

const onSwitchTheme = () => {
  if (!appStore.theme || appStore.theme === 'light') {
    document.documentElement.setAttribute('theme-mode', 'dark');
    appStore.setTheme('dark')
  } else {
    document.documentElement.removeAttribute('theme-mode');
    appStore.setTheme('light')
  }
}

const waitListForm = {}
const onSubmitWaitlist = () => {

}
</script>

<style scoped>
.container {
  display: flex;
  height: calc(100vh - 56px);
  width: 100vw;
  flex-direction: column;
  justify-content: center;
  align-items: center;
}

.title {
  font-size: 3.5em;
  font-weight: 700;
  line-height: 0;
  color: var(--td-brand-color);
}

.sub {
  font-size: 1.5em;
  color: var(--td-text-color-secondary);
}

.cards {
  margin-top: 5em;
}

.card {
  max-width: 240px;
  height: 150px;
  overflow: hidden;
}

.card-header {
  display: flex;
  align-items: center;
  margin-top: 10px;
  margin-left: 10px;
  gap: 12px;
}

.card-title {
  font-size: 1.3em;
  color: var(--td-text-color-primary);
}

.card-content {
  font-size: 1.2em;
  margin-bottom: 15px;
  margin-left: 10px;
  color: var(--td-text-color-secondary);
}

.card-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  background: var(--td-brand-color-1);
  border-radius: 50%;
  color: var(--td-brand-color);
}

.waitlist {
  margin-top: 8em;
}

.waitlist .sub {
  font-size: 1em;
  color: var(--td-text-color-secondary);
}
</style>
