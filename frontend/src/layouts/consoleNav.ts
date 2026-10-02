import {
  Globe,
  LayoutDashboard,
  LockKeyhole,
  LogIn,
  MonitorSmartphone,
  Palette,
  ShieldCheck,
  Table2,
  UserRound,
  UsersRound,
} from '@lucide/vue'
import type { Component } from 'vue'

import { t } from '@/i18n'

export interface NavItem {
  /** Route name. */
  name: string
  label: string
  icon: Component
}

export interface NavGroup {
  title: string
  items: NavItem[]
}

function navItem(name: string, icon: Component, label: () => string): NavItem {
  return {
    name,
    icon,
    get label() {
      return label()
    },
  }
}

export const SETTINGS_NAV: NavGroup[] = [
  {
    get title() {
      return t('consoleNav.settingsPersonal')
    },
    items: [
      navItem('settings-profile', UserRound, () => t('consoleNav.profile')),
      navItem('settings-preferences', Palette, () => t('consoleNav.preferences')),
    ],
  },
  {
    get title() {
      return t('consoleNav.settingsSecurity')
    },
    items: [
      navItem('settings-security', ShieldCheck, () => t('consoleNav.security')),
      navItem('settings-devices', MonitorSmartphone, () => t('consoleNav.devices')),
    ],
  },
]

export const ADMIN_NAV: NavGroup[] = [
  {
    get title() {
      return t('consoleNav.overview')
    },
    items: [navItem('admin-overview', LayoutDashboard, () => t('consoleNav.overview'))],
  },
  {
    get title() {
      return t('consoleNav.organization')
    },
    items: [navItem('admin-users', UsersRound, () => t('consoleNav.users'))],
  },
  {
    get title() {
      return t('consoleNav.data')
    },
    items: [navItem('admin-projects', Table2, () => t('consoleNav.projects'))],
  },
  {
    get title() {
      return t('consoleNav.system')
    },
    items: [
      navItem('admin-site', Globe, () => t('consoleNav.site')),
      navItem('admin-security', LockKeyhole, () => t('consoleNav.adminSecurity')),
      navItem('admin-sign-in-methods', LogIn, () => t('sso.admin.title')),
    ],
  },
]

/** Finds the label of the menu item by route name, for the page title. */
export function navLabel(nav: NavGroup[], name: unknown): string | undefined {
  for (const g of nav) {
    const item = g.items.find((i) => i.name === name)
    if (item) return item.label
  }
  return undefined
}
