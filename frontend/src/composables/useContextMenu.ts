import type { Component } from 'vue'
import { shallowRef } from 'vue'

export interface MenuItem {
  key?: string
  label?: string
  icon?: Component
  danger?: boolean
  disabled?: boolean
  divider?: boolean
  /** 右侧的快捷键提示。 */
  hint?: string
  onClick?: () => void
}

export interface MenuState {
  x: number
  y: number
  items: MenuItem[]
  onClose?: () => void
}

export const menuState = shallowRef<MenuState | null>(null)

/** 在指定位置打开全局菜单；传入鼠标事件时使用事件坐标。 */
export function openMenu(pos: MouseEvent | { x: number; y: number }, items: MenuItem[], onClose?: () => void) {
  if (pos instanceof MouseEvent) {
    pos.preventDefault()
    pos.stopPropagation()
  }
  const x = pos instanceof MouseEvent ? pos.clientX : pos.x
  const y = pos instanceof MouseEvent ? pos.clientY : pos.y
  menuState.value?.onClose?.()
  menuState.value = { x, y, items, onClose }
}

export function closeMenu() {
  menuState.value?.onClose?.()
  menuState.value = null
}
