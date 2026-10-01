import {
  Globe,
  LayoutDashboard,
  LockKeyhole,
  MonitorSmartphone,
  Palette,
  ShieldCheck,
  Table2,
  UserRound,
  UsersRound,
} from '@lucide/vue'
import type { Component } from 'vue'

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

export const SETTINGS_NAV: NavGroup[] = [
  {
    title: '个人',
    items: [
      { name: 'settings-profile', label: '个人信息', icon: UserRound },
      { name: 'settings-preferences', label: '偏好设置', icon: Palette },
    ],
  },
  {
    title: '安全',
    items: [
      { name: 'settings-security', label: '账号与安全', icon: ShieldCheck },
      { name: 'settings-devices', label: '登录设备', icon: MonitorSmartphone },
    ],
  },
]

export const ADMIN_NAV: NavGroup[] = [
  {
    title: '概览',
    items: [{ name: 'admin-overview', label: '概览', icon: LayoutDashboard }],
  },
  {
    title: '组织管理',
    items: [{ name: 'admin-users', label: '成员管理', icon: UsersRound }],
  },
  {
    title: '数据管理',
    items: [{ name: 'admin-projects', label: '多维表格', icon: Table2 }],
  },
  {
    title: '系统设置',
    items: [
      { name: 'admin-security', label: '安全与注册', icon: LockKeyhole },
      { name: 'admin-site', label: '站点信息', icon: Globe },
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
