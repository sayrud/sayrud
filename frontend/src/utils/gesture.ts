function canScrollX(el: Element, deltaX: number): boolean {
  if (el.scrollWidth <= el.clientWidth) return false
  const { overflowX } = getComputedStyle(el)
  if (overflowX !== 'auto' && overflowX !== 'scroll') return false
  return deltaX < 0 ? el.scrollLeft > 0 : el.scrollLeft + el.clientWidth < el.scrollWidth - 1
}

/**
 * Prevents the two-finger swipe of trackpads from navigating back / forward, which Safari does not stop by overscroll-behavior.
 * A horizontal wheel event is cancelled when no element under the pointer can scroll further in its direction.
 */
export function preventSwipeNavigation() {
  window.addEventListener(
    'wheel',
    (e) => {
      if (e.ctrlKey || Math.abs(e.deltaX) <= Math.abs(e.deltaY)) return
      for (let el = e.target instanceof Element ? e.target : null; el; el = el.parentElement) {
        if (canScrollX(el, e.deltaX)) return
      }
      e.preventDefault()
    },
    { passive: false },
  )
}
