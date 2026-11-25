import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Maximize2, Minimize2 } from 'lucide-react'
import { highlight, type Lang } from '../lib/highlight'

/**
 * 纯前端代码编辑器：透明 textarea 叠在着色层上，行号列独立同步滚动，
 * 支持 Tab 缩进与全屏。模板内容敏感（可能含密码），所以不引第三方编辑器依赖。
 */
export function CodeEditor({
  value,
  onChange,
  language = 'yaml',
  ariaLabel = '代码编辑器',
  height = 520,
  footer,
  readOnly = false,
}: {
  value: string
  onChange: (next: string) => void
  language?: Lang
  ariaLabel?: string
  height?: number
  footer?: React.ReactNode
  readOnly?: boolean
}) {
  const taRef = useRef<HTMLTextAreaElement | null>(null)
  const preRef = useRef<HTMLPreElement | null>(null)
  const gutterRef = useRef<HTMLDivElement | null>(null)
  const [fullscreen, setFullscreen] = useState(false)
  const [caret, setCaret] = useState({ line: 1, col: 1 })

  const html = useMemo(() => highlight(value, language), [value, language])
  const lineCount = useMemo(() => value.split('\n').length, [value])

  const syncScroll = useCallback(() => {
    const ta = taRef.current
    if (!ta) return
    if (preRef.current) {
      preRef.current.scrollTop = ta.scrollTop
      preRef.current.scrollLeft = ta.scrollLeft
    }
    if (gutterRef.current) gutterRef.current.scrollTop = ta.scrollTop
  }, [])

  const updateCaret = useCallback(() => {
    const ta = taRef.current
    if (!ta) return
    const upto = ta.value.slice(0, ta.selectionStart)
    const lines = upto.split('\n')
    setCaret({ line: lines.length, col: lines[lines.length - 1].length + 1 })
  }, [])

  // 内容变化后保持滚动位置，并把光标行号同步出来。
  useLayoutEffect(() => {
    syncScroll()
  }, [html, syncScroll])

  useEffect(() => {
    if (!fullscreen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setFullscreen(false)
    }
    document.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prev
    }
  }, [fullscreen])

  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Tab') {
      e.preventDefault()
      const ta = e.currentTarget
      const start = ta.selectionStart
      const end = ta.selectionEnd
      const next = value.slice(0, start) + '  ' + value.slice(end)
      onChange(next)
      requestAnimationFrame(() => {
        ta.selectionStart = ta.selectionEnd = start + 2
      })
    }
  }

  const lineNumbers = useMemo(() => {
    const rows: number[] = []
    for (let i = 1; i <= lineCount; i++) rows.push(i)
    return rows
  }, [lineCount])

  return (
    <div
      className={`flex flex-col overflow-hidden border border-line bg-canvas ${
        fullscreen ? 'fixed inset-0 z-[70] h-dvh w-screen rounded-none' : 'rounded-xl'
      }`}
    >
      <div className="flex items-center gap-2 border-b border-line bg-surface px-3 py-1.5">
        <span className="mono text-[0.6875rem] uppercase tracking-[0.12em] text-fg-subtle">
          {language === 'yaml' ? 'YAML' : language === 'json' ? 'JSON' : 'CONF'}
        </span>
        <span className="mono text-[0.6875rem] text-fg-subtle">
          {lineCount} 行 · 第 {caret.line} 行第 {caret.col} 列
        </span>
        {footer && <span className="ml-auto text-[0.6875rem] text-fg-subtle">{footer}</span>}
        <button
          type="button"
          className={`${footer ? '' : 'ml-auto'} flex h-8 items-center gap-1.5 rounded-md border border-line px-2 text-[0.6875rem] text-fg-muted transition-colors hover:border-line-strong hover:bg-raised hover:text-fg`}
          onClick={() => setFullscreen((v) => !v)}
          aria-pressed={fullscreen}
        >
          {fullscreen ? <Minimize2 size={13} /> : <Maximize2 size={13} />}
          {fullscreen ? '退出全屏' : '全屏'}
        </button>
      </div>

      <div className="flex min-h-0 flex-1 overflow-hidden">
        <div
          ref={gutterRef}
          aria-hidden
          className="mono w-11 shrink-0 overflow-hidden border-r border-line bg-surface/40 py-3 pr-2 text-right text-[0.7188rem] leading-[1.7] text-fg-subtle select-none"
        >
          {lineNumbers.map((n) => (
            <div key={n}>{n}</div>
          ))}
        </div>

        <div className="relative min-w-0 flex-1">
          <pre
            ref={preRef}
            aria-hidden
            className="mono pointer-events-none absolute inset-0 m-0 overflow-hidden px-3 py-3 text-[0.7188rem] leading-[1.7] whitespace-pre text-fg-soft"
          >
            <code dangerouslySetInnerHTML={{ __html: html }} />
          </pre>
          <textarea
            ref={taRef}
            readOnly={readOnly}
            value={value}
            onChange={(e) => {
              onChange(e.target.value)
              updateCaret()
            }}
            onScroll={syncScroll}
            onKeyDown={onKeyDown}
            onKeyUp={updateCaret}
            onClick={updateCaret}
            spellCheck={false}
            autoCapitalize="off"
            autoCorrect="off"
            wrap="off"
            aria-label={ariaLabel}
            className={`mono absolute inset-0 h-full w-full resize-none overflow-auto bg-transparent px-3 py-3 text-[0.7188rem] leading-[1.7] whitespace-pre text-transparent caret-accent outline-none selection:bg-accent/25 ${
              readOnly ? 'cursor-text' : ''
            }`}
            style={fullscreen ? undefined : { height }}
          />
        </div>
      </div>
    </div>
  )
}
