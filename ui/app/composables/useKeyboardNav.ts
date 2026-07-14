// Google-Reader-style keyboard navigation for the triage list.
// j/k move, m toggle read, s toggle star, o/Enter open reader, v open original,
// Shift+A mark all visible read.
export interface KeyboardHandlers {
  next: () => void
  prev: () => void
  toggleRead: () => void
  toggleStar: () => void
  open: () => void
  openOriginal: () => void
  markAllVisible: () => void
}

function isTyping(): boolean {
  const el = document.activeElement
  if (!el) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || (el as HTMLElement).isContentEditable
}

export function useKeyboardNav(handlers: KeyboardHandlers) {
  function onKey(e: KeyboardEvent) {
    if (isTyping() || e.metaKey || e.ctrlKey || e.altKey) return

    const actions: Record<string, () => void> = {
      j: handlers.next,
      k: handlers.prev,
      m: handlers.toggleRead,
      s: handlers.toggleStar,
      o: handlers.open,
      Enter: handlers.open,
      v: handlers.openOriginal,
    }
    if (e.shiftKey && e.key === 'A') {
      e.preventDefault()
      handlers.markAllVisible()
      return
    }
    const action = actions[e.key]
    if (action) {
      e.preventDefault()
      action()
    }
  }

  onMounted(() => window.addEventListener('keydown', onKey))
  onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
}
