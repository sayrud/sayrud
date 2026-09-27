import type { Component } from 'vue'
import { shallowRef } from 'vue'

export interface MenuItem {
  key?: string
  label?: string
  icon?: Component
  danger?: boolean
  disabled?: boolean
  divider?: boolean
  /** Shortcut hint shown on the right. */
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

/** Opens the global menu at the position, or at the event coordinates if a mouse event is given. */
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
