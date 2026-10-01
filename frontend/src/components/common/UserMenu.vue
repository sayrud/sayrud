<script setup lang="ts">
import { Check, ChevronRight, Languages, LogOut, ShieldCheck, SunMoon, UserRound } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { LOCALE_OPTIONS } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'
import { THEME_OPTIONS } from '@/utils/theme'
import UserAvatar from './UserAvatar.vue'

withDefaults(defineProps<{ size?: number }>(), { size: 28 })

const auth = useAuthStore()
const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const router = useRouter()
const { t } = useI18n()

const themeLabel = computed(() => THEME_OPTIONS.find((o) => o.value === themeStore.mode)?.label)
const localeLabel = computed(() => LOCALE_OPTIONS.find((o) => o.value === localeStore.locale)?.label)

async function signOut() {
  await auth.signOut().catch(() => undefined)
  router.replace({ name: 'login' })
}
</script>

<template>
  <a-dropdown v-if="auth.user" trigger="click" position="br" :popup-max-height="false">
    <UserAvatar :name="auth.user.userName" :color="auth.user.color" :size="size" class="trigger" />
    <template #content>
      <div class="profile">
        <UserAvatar :name="auth.user.userName" :color="auth.user.color" :size="36" />
        <div class="profile-text">
          <div class="profile-name ellipsis">{{ auth.user.userName }}</div>
          <div class="profile-email ellipsis">{{ auth.user.email }}</div>
        </div>
      </div>
      <!-- The menu pops up at the right edge of the page, the submenu opens to the left to stay in the viewport. -->
      <a-dsubmenu trigger="hover" position="lt">
        <template #icon><SunMoon :size="15" /></template>
        {{ t('userMenu.appearance') }}
        <template #suffix>
          <span class="submenu-suffix">
            {{ themeLabel }}
            <ChevronRight :size="14" />
          </span>
        </template>
        <template #content>
          <a-doption v-for="o in THEME_OPTIONS" :key="o.value" @click="themeStore.setMode(o.value)">
            <span class="theme-option">
              {{ o.label }}
              <Check v-if="themeStore.mode === o.value" :size="15" class="theme-check" />
            </span>
          </a-doption>
        </template>
      </a-dsubmenu>
      <a-dsubmenu trigger="hover" position="lt">
        <template #icon><Languages :size="15" /></template>
        {{ t('common.language') }}
        <template #suffix>
          <span class="submenu-suffix">
            {{ localeLabel }}
            <ChevronRight :size="14" />
          </span>
        </template>
        <template #content>
          <a-doption v-for="o in LOCALE_OPTIONS" :key="o.value" @click="localeStore.setLocale(o.value)">
            <span class="theme-option">
              {{ o.label }}
              <Check v-if="localeStore.locale === o.value" :size="15" class="theme-check" />
            </span>
          </a-doption>
        </template>
      </a-dsubmenu>
      <div class="menu-divider" />
      <a-doption @click="router.push({ name: 'settings-profile' })">
        <template #icon><UserRound :size="15" /></template>
        {{ t('userMenu.accountSettings') }}
      </a-doption>
      <a-doption v-if="auth.user.isAdmin" @click="router.push({ name: 'admin-overview' })">
        <template #icon><ShieldCheck :size="15" /></template>
        {{ t('userMenu.adminConsole') }}
      </a-doption>
      <div class="menu-divider" />
      <a-doption @click="signOut">
        <template #icon><LogOut :size="15" /></template>
        {{ t('userMenu.signOut') }}
      </a-doption>
    </template>
  </a-dropdown>
</template>

<style scoped>
.trigger {
  cursor: pointer;
}
.profile {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 240px;
  padding: 8px 12px 12px;
  margin-bottom: 4px;
  border-bottom: 1px solid var(--line-border);
}
.profile-text {
  min-width: 0;
}
.profile-name {
  font-weight: 600;
  line-height: 22px;
}
.profile-email {
  font-size: 12px;
  line-height: 18px;
  color: var(--text-placeholder);
}
.menu-divider {
  height: 1px;
  margin: 4px 0;
  background: var(--line-border);
}
.submenu-suffix {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  color: var(--text-placeholder);
  font-size: 13px;
}
.theme-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  min-width: 120px;
}
.theme-check {
  color: var(--color-primary);
}
</style>
