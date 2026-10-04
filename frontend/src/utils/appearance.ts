import {
  IconApps, IconArrowRise, IconBarChart, IconBulb, IconCalendar, IconCamera,
  IconCheck, IconCheckSquare, IconClockCircle, IconCloud, IconCode, IconCommand,
  IconCommon, IconCompass, IconDashboard, IconEmail, IconHeart, IconHome,
  IconLayers, IconNav, IconNotification, IconOrderedList, IconPalette, IconPen,
  IconPublic, IconStorage, IconTag, IconThunderbolt, IconTrophy,
  IconUnorderedList, IconVoice,
} from '@arco-design/web-vue/es/icon/index.js'
import {
  Aperture, Barcode, ChartLine, ChartPie, Coffee, Factory, Gamepad2, Laptop,
  MonitorSmartphone, Orbit, Plane, Target,
} from '@lucide/vue'
import type { Component } from 'vue'

export interface Appearance {
  icon: string
  color: string
}

export const APPEARANCE_COLORS = {
  orange: 'linear-gradient(135deg, #ffd475 0%, #ff9eb0 100%)',
  coral: 'linear-gradient(135deg, #ffb580 0%, #fc7faa 100%)',
  pink: 'linear-gradient(135deg, #fa8dc3 0%, #dc71e9 100%)',
  purple: 'linear-gradient(135deg, #a994ff 0%, #cc86ee 100%)',
  indigo: 'linear-gradient(135deg, #9ca7ff 0%, #9b7ff3 100%)',
  blue: 'linear-gradient(135deg, #7aabff 0%, #8d83ff 100%)',
  teal: 'linear-gradient(135deg, #62d0b2 0%, #63c4e2 100%)',
  green: 'linear-gradient(135deg, #6dce9e 0%, #a6c567 100%)',
} as const

export const APPEARANCE_ICONS: Record<string, Component> = {
  table: IconNav,
  blocks: IconCommand,
  list: IconUnorderedList,
  'square-check': IconCheckSquare,
  'list-ordered': IconOrderedList,
  zap: IconThunderbolt,
  layers: IconLayers,
  cloud: IconCloud,
  gamepad: Gamepad2,
  house: IconHome,
  laptop: Laptop,
  tag: IconTag,
  calendar: IconCalendar,
  check: IconCheck,
  clock: IconClockCircle,
  box: IconCommon,
  code: IconCode,
  'chart-pie': ChartPie,
  gauge: IconDashboard,
  target: Target,
  'chart-line': ChartLine,
  'trending-up': IconArrowRise,
  'chart-column': IconBarChart,
  barcode: Barcode,
  globe: IconPublic,
  aperture: Aperture,
  server: IconStorage,
  'monitor-smartphone': MonitorSmartphone,
  camera: IconCamera,
  mic: IconVoice,
  plane: Plane,
  orbit: Orbit,
  lightbulb: IconBulb,
  coffee: Coffee,
  mail: IconEmail,
  compass: IconCompass,
  factory: Factory,
  heart: IconHeart,
  bell: IconNotification,
  palette: IconPalette,
  trophy: IconTrophy,
  'layout-grid': IconApps,
  'pen-tool': IconPen,
}

export function appearanceIcon(icon?: string, fallback: 'project' | 'table' = 'table'): Component {
  return icon && Object.hasOwn(APPEARANCE_ICONS, icon)
    ? APPEARANCE_ICONS[icon]!
    : fallback === 'project' ? IconCommand : IconNav
}

export function appearanceBackground(color?: string): string | undefined {
  return color && Object.hasOwn(APPEARANCE_COLORS, color)
    ? APPEARANCE_COLORS[color as keyof typeof APPEARANCE_COLORS]
    : undefined
}

export function appearanceSelection(icon?: string, color?: string): Appearance {
  return {
    icon: icon && Object.hasOwn(APPEARANCE_ICONS, icon) ? icon : '',
    color: appearanceBackground(color) ? color! : '',
  }
}
