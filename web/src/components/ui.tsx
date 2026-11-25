// Shared presentational primitives. Every visual token comes from style.css /
// Tailwind theme so themes stay swappable.
import { useState, type ReactNode } from 'react'
import { AlertTriangle, Check, Copy, Loader2, X } from 'lucide-react'
import { copyText, dismissToast, toast as _toast, useToasts } from '../lib/hooks'

export function Badge({
  children,
  tone = 'neutral',
  className = '',
}: {
  children: ReactNode
  tone?: 'neutral' | 'cyan' | 'emerald' | 'amber' | 'red' | 'sky' | 'orange'
  className?: string
}) {
  const tones: Record<string, string> = {
    neutral: 'border-zinc-700 text-zinc-400',
    cyan: 'border-cyan-400/40 text-cyan-300 bg-cyan-400/5',
    emerald: 'border-emerald-400/40 text-emerald-300 bg-emerald-400/5',
    amber: 'border-amber-400/40 text-amber-300 bg-amber-400/5',
    red: 'border-red-400/40 text-red-300 bg-red-400/5',
    sky: 'border-sky-400/40 text-sky-300 bg-sky-400/5',
    orange: 'border-orange-400/40 text-orange-300 bg-orange-400/5',
  }
  return <span className={`badge ${tones[tone]} ${className}`}>{children}</span>
}

const protocolTone: Record<string, 'cyan' | 'sky' | 'orange' | 'emerald' | 'neutral'> = {
  hy2: 'cyan',
  vless: 'sky',
  tuic: 'emerald',
  anytls: 'orange',
  mieru: 'neutral',
}

export function ProtocolBadge({ protocol }: { protocol: string }) {
  return (
    <Badge tone={protocolTone[protocol] ?? 'neutral'} className="shrink-0">
      {protocol.toUpperCase()}
    </Badge>
  )
}

export function SourceDot({ type }: { type: 'airport' | 'server' }) {
  return (
    <span
      aria-hidden
      className={`h-1.5 w-1.5 shrink-0 rounded-full ${
        type === 'airport' ? 'bg-[var(--airport-accent)]' : 'bg-[var(--server-accent)]'
      }`}
    />
  )
}

export function SectionLabel({ children }: { children: ReactNode }) {
  return <div className="label-xs">{children}</div>
}

export function Spinner({ className = '' }: { className?: string }) {
  return <Loader2 size={14} className={`animate-spin ${className}`} aria-hidden />
}

export function CopyButton({
  value,
  label = '复制',
  className = '',
}: {
  value: string
  label?: string
  className?: string
}) {
  const [done, setDone] = useState(false)
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      className={`inline-flex h-8 w-8 items-center justify-center rounded border border-zinc-800 text-zinc-400 transition-colors hover:border-zinc-700 hover:text-zinc-200 ${className}`}
      onClick={async () => {
        const ok = await copyText(value)
        _toast(ok ? 'success' : 'error', ok ? '已复制到剪贴板' : '复制失败，请手动选择')
        if (ok) {
          setDone(true)
          window.setTimeout(() => setDone(false), 1500)
        }
      }}
    >
      {done ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
    </button>
  )
}

export function EmptyState({
  icon,
  title,
  description,
  action,
}: {
  icon: ReactNode
  title: string
  description: string
  action?: ReactNode
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-zinc-800 px-6 py-14 text-center">
      <div className="flex h-10 w-10 items-center justify-center rounded-md border border-zinc-800 bg-zinc-900 text-zinc-500">
        {icon}
      </div>
      <div className="space-y-1">
        <p className="text-[13px] font-medium text-zinc-200">{title}</p>
        <p className="max-w-sm text-[12.5px] leading-relaxed text-zinc-500">{description}</p>
      </div>
      {action}
    </div>
  )
}

export function ErrorNote({ children }: { children: ReactNode }) {
  return (
    <p role="alert" className="flex items-start gap-1.5 text-[12.5px] text-red-300">
      <AlertTriangle size={13} className="mt-0.5 shrink-0" aria-hidden />
      <span>{children}</span>
    </p>
  )
}

export function SkeletonRows({ rows = 4 }: { rows?: number }) {
  return (
    <div className="space-y-2" aria-hidden>
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="h-14 animate-pulse rounded-md border border-zinc-800/60 bg-zinc-900/40" />
      ))}
    </div>
  )
}

export function Modal({
  open,
  title,
  description,
  onClose,
  children,
  footer,
  wide,
}: {
  open: boolean
  title: string
  description?: string
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  wide?: boolean
}) {
  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-black/60 p-0 backdrop-blur-sm md:items-center md:p-6">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={`flex max-h-[92dvh] w-full flex-col overflow-hidden rounded-t-xl border border-zinc-800 bg-zinc-900 shadow-2xl md:rounded-lg ${
          wide ? 'md:max-w-3xl' : 'md:max-w-md'
        }`}
      >
        <div className="flex items-start justify-between gap-4 border-b border-zinc-800 px-4 py-3">
          <div>
            <h2 className="text-[14px] font-semibold text-zinc-100">{title}</h2>
            {description && <p className="mt-0.5 text-[12px] text-zinc-500">{description}</p>}
          </div>
          <button
            type="button"
            aria-label="关闭"
            onClick={onClose}
            className="-mr-1 flex h-8 w-8 shrink-0 items-center justify-center rounded text-zinc-500 hover:bg-zinc-800 hover:text-zinc-200"
          >
            <X size={16} />
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">{children}</div>
        {footer && <div className="flex justify-end gap-2 border-t border-zinc-800 px-4 py-3">{footer}</div>}
      </div>
    </div>
  )
}

export function Toaster() {
  const list = useToasts()
  return (
    <div
      aria-live="polite"
      className="pointer-events-none fixed inset-x-3 top-3 z-[60] flex flex-col items-center gap-2 md:inset-x-auto md:right-4 md:items-end"
    >
      {list.map((t) => (
        <div
          key={t.id}
          className={`pointer-events-auto flex w-full max-w-sm items-start gap-2 rounded-md border bg-zinc-900/95 px-3 py-2.5 text-[12.5px] shadow-lg backdrop-blur ${
            t.kind === 'success'
              ? 'border-emerald-500/40 text-emerald-100'
              : t.kind === 'error'
                ? 'border-red-500/40 text-red-100'
                : t.kind === 'warning'
                  ? 'border-amber-500/40 text-amber-100'
                  : 'border-zinc-700 text-zinc-100'
          }`}
        >
          <span className="flex-1">{t.message}</span>
          <button
            type="button"
            aria-label="关闭提示"
            onClick={() => dismissToast(t.id)}
            className="text-zinc-500 hover:text-zinc-200"
          >
            <X size={13} />
          </button>
        </div>
      ))}
    </div>
  )
}
