// 主题（浅色 / 深色 / 跟随系统）。状态放在模块级，任何组件都能读，
// 并且写回 localStorage 让下次打开不闪。真正的首屏着色在 index.html 的
// 内联脚本里完成，这里只负责切换与监听系统变化。

export type ThemeMode = 'system' | 'light' | 'dark'
export type ResolvedTheme = 'light' | 'dark'

const STORAGE_KEY = 'subs-theme'
const listeners = new Set<() => void>()

function systemTheme(): ResolvedTheme {
  return window.matchMedia?.('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

function readStored(): ThemeMode {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v === 'light' || v === 'dark' || v === 'system') return v
  } catch {
    /* 隐私模式下 localStorage 可能不可用 */
  }
  return 'system'
}

export function resolveTheme(mode: ThemeMode): ResolvedTheme {
  return mode === 'system' ? systemTheme() : mode
}

export function applyTheme(mode: ThemeMode) {
  const resolved = resolveTheme(mode)
  const root = document.documentElement
  root.dataset.theme = resolved
  root.style.colorScheme = resolved
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', resolved === 'light' ? '#f6f6f7' : '#121214')
  listeners.forEach((fn) => fn())
}

export function setThemeMode(mode: ThemeMode) {
  try {
    localStorage.setItem(STORAGE_KEY, mode)
  } catch {
    /* ignore */
  }
  applyTheme(mode)
}

/** cycle 在「跟随系统 → 浅色 → 深色」之间轮换。 */
export function cycleTheme(): ThemeMode {
  const order: ThemeMode[] = ['system', 'light', 'dark']
  const next = order[(order.indexOf(readStored()) + 1) % order.length]
  setThemeMode(next)
  return next
}

export function currentThemeMode(): ThemeMode {
  return readStored()
}

/** 订阅主题变化（含系统配色变化）。返回取消订阅函数。 */
export function subscribeTheme(fn: () => void): () => void {
  listeners.add(fn)
  const onSystem = () => {
    if (readStored() === 'system') applyTheme('system')
  }
  window.matchMedia?.('(prefers-color-scheme: light)').addEventListener?.('change', onSystem)
  return () => {
    listeners.delete(fn)
    window.matchMedia?.('(prefers-color-scheme: light)').removeEventListener?.('change', onSystem)
  }
}

if (typeof document !== 'undefined') {
  // 客户端启动时确保属性与存储一致（index.html 已提前设置过，这里兜底）。
  applyTheme(readStored())
}
