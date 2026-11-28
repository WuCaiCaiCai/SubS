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
    neutral: 'border-line-strong text-fg-muted',
    cyan: 'border-accent/40 text-accent bg-accent/5',
    emerald: 'border-success/40 text-success bg-success/5',
    amber: 'border-warning/40 text-warning bg-warning/5',
    red: 'border-danger/40 text-danger bg-danger/5',
    sky: 'border-info/40 text-info bg-info/5',
    orange: 'border-warning/40 text-warning bg-warning/5',
  }
  return <span className={`badge ${tones[tone]} ${className}`}>{children}</span>
}

const protocolTone: Record<string, 'cyan' | 'sky' | 'orange' | 'emerald' | 'neutral'> = {
  hy2: 'cyan',
  vless: 'sky',
  tuic: 'emerald',
  anytls: 'orange',
  mieru: 'neutral',
  ss: 'neutral',
  vmess: 'neutral',
  trojan: 'neutral',
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

/** 开关：受控组件，disabled 时视觉与交互同步关闭。 */
export function Switch({
  checked,
  onChange,
  disabled = false,
  label,
}: {
  checked: boolean
  onChange: (next: boolean) => void
  disabled?: boolean
  label?: string
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => !disabled && onChange(!checked)}
      className={`relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors ${
        checked ? 'bg-accent' : 'bg-line'
      } ${disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer hover:opacity-90'}`}
    >
      <span
        className={`pointer-events-none absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white shadow-sm transition-transform ${
          checked ? 'translate-x-5' : 'translate-x-0'
        }`}
      />
    </button>
  )
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
      className={`inline-flex h-8 w-8 items-center justify-center rounded border border-line text-fg-muted transition-colors hover:border-line-strong hover:text-fg ${className}`}
      onClick={async () => {
        const ok = await copyText(value)
        _toast(ok ? 'success' : 'error', ok ? '已复制到剪贴板' : '复制失败，请手动选择')
        if (ok) {
          setDone(true)
          window.setTimeout(() => setDone(false), 1500)
        }
      }}
    >
      {done ? <Check size={14} className="text-success" /> : <Copy size={14} />}
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
    <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-line px-6 py-14 text-center">
      <div className="flex h-10 w-10 items-center justify-center rounded-md border border-line bg-surface text-fg-muted">
        {icon}
      </div>
      <div className="space-y-1">
        <p className="text-[0.8125rem] font-medium text-fg">{title}</p>
        <p className="max-w-sm text-[0.7812rem] leading-relaxed text-fg-muted">{description}</p>
      </div>
      {action}
    </div>
  )
}

export function ErrorNote({ children }: { children: ReactNode }) {
  return (
    <p role="alert" className="flex items-start gap-1.5 text-[0.7812rem] text-danger">
      <AlertTriangle size={13} className="mt-0.5 shrink-0" aria-hidden />
      <span>{children}</span>
    </p>
  )
}

export function SkeletonRows({ rows = 4 }: { rows?: number }) {
  return (
    <div className="space-y-2" aria-hidden>
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="h-14 animate-pulse rounded-md border border-line/60 bg-surface/40" />
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
        className={`flex max-h-[92dvh] w-full flex-col overflow-hidden rounded-t-xl border border-line bg-surface shadow-2xl md:rounded-lg ${
          wide ? 'md:max-w-3xl' : 'md:max-w-md'
        }`}
      >
        <div className="flex items-start justify-between gap-4 border-b border-line px-4 py-3">
          <div>
            <h2 className="text-[0.875rem] font-semibold text-fg">{title}</h2>
            {description && <p className="mt-0.5 text-[0.75rem] text-fg-muted">{description}</p>}
          </div>
          <button
            type="button"
            aria-label="关闭"
            onClick={onClose}
            className="-mr-1 flex h-8 w-8 shrink-0 items-center justify-center rounded text-fg-muted hover:bg-raised hover:text-fg"
          >
            <X size={16} />
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">{children}</div>
        {footer && <div className="flex justify-end gap-2 border-t border-line px-4 py-3">{footer}</div>}
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
          className={`pointer-events-auto flex w-full max-w-sm items-start gap-2 rounded-md border bg-surface/95 px-3 py-2.5 text-[0.7812rem] shadow-lg backdrop-blur ${
            t.kind === 'success'
              ? 'border-success/40 text-success'
              : t.kind === 'error'
                ? 'border-danger/40 text-danger'
                : t.kind === 'warning'
                  ? 'border-warning/40 text-warning'
                  : 'border-line-strong text-fg'
          }`}
        >
          <span className="flex-1">{t.message}</span>
          <button
            type="button"
            aria-label="关闭提示"
            onClick={() => dismissToast(t.id)}
            className="text-fg-muted hover:text-fg"
          >
            <X size={13} />
          </button>
        </div>
      ))}
    </div>
  )
}
