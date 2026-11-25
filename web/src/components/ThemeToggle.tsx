import { useEffect, useState } from 'react'
import { Monitor, Moon, Sun } from 'lucide-react'
import {
  currentThemeMode,
  cycleTheme,
  setThemeMode,
  subscribeTheme,
  type ThemeMode,
} from '../lib/theme'

const ICON = { system: Monitor, light: Sun, dark: Moon } as const
const LABEL: Record<ThemeMode, string> = { system: '跟随系统', light: '浅色', dark: '深色' }

/** 单按钮循环：跟随系统 → 浅色 → 深色。空间紧张的地方用它。 */
export function ThemeToggle({ compact = false }: { compact?: boolean }) {
  const [mode, setMode] = useState<ThemeMode>(() => currentThemeMode())
  // 系统配色变化时只关心是否需要重绘图标，模式本身不变。
  useEffect(() => subscribeTheme(() => setMode(currentThemeMode())), [])

  const Icon = ICON[mode]
  return (
    <button
      type="button"
      title={`主题：${LABEL[mode]}（点击切换）`}
      aria-label={`主题：${LABEL[mode]}，点击切换`}
      onClick={() => {
        setMode(cycleTheme())
      }}
      className={`flex items-center justify-center gap-1.5 rounded-md border border-line text-fg-muted transition-colors hover:border-line-strong hover:bg-raised hover:text-fg ${
        compact ? 'h-9 w-9' : 'h-9 px-2.5'
      }`}
    >
      <Icon size={15} />
      {!compact && <span className="text-[0.7188rem]">{LABEL[mode]}</span>}
    </button>
  )
}

/** 三选一的分段控件，放在设置页里。 */
export function ThemePicker() {
  const [mode, setMode] = useState<ThemeMode>(() => currentThemeMode())
  useEffect(() => subscribeTheme(() => setMode(currentThemeMode())), [])

  return (
    <div className="inline-flex rounded-lg border border-line p-0.5" role="group" aria-label="界面主题">
      {(Object.keys(LABEL) as ThemeMode[]).map((m) => {
        const Icon = ICON[m]
        const active = mode === m
        return (
          <button
            key={m}
            type="button"
            aria-pressed={active}
            onClick={() => {
              setThemeMode(m)
            }}
            className={`flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-[0.7188rem] transition-colors ${
              active ? 'bg-raised text-fg' : 'text-fg-muted hover:text-fg-soft'
            }`}
          >
            <Icon size={13} />
            {LABEL[m]}
          </button>
        )
      })}
    </div>
  )
}
