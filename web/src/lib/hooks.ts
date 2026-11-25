// Small shared hooks: data fetching with SWR-ish semantics, toasts, clipboard.
import { useCallback, useEffect, useRef, useState } from 'react'

export function useAsync<T>(fn: () => Promise<T>, deps: unknown[] = []) {
  const [data, setData] = useState<T | undefined>()
  const [error, setError] = useState<Error | undefined>()
  const [loading, setLoading] = useState(true)
  const [nonce, setNonce] = useState(0)
  const alive = useRef(true)

  useEffect(() => {
    alive.current = true
    setLoading(true)
    fn()
      .then((v) => {
        if (alive.current) {
          setData(v)
          setError(undefined)
        }
      })
      .catch((e: Error) => {
        if (alive.current) setError(e)
      })
      .finally(() => {
        if (alive.current) setLoading(false)
      })
    return () => {
      alive.current = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, nonce])

  const reload = useCallback(() => setNonce((n) => n + 1), [])
  return { data, error, loading, reload, setData }
}

export type ToastKind = 'success' | 'error' | 'warning' | 'info'
export interface Toast {
  id: number
  kind: ToastKind
  message: string
}

let toastSeq = 0
const listeners = new Set<(toasts: Toast[]) => void>()
let toasts: Toast[] = []

function emit() {
  listeners.forEach((l) => l([...toasts]))
}

export function toast(kind: ToastKind, message: string) {
  const id = ++toastSeq
  toasts = [...toasts, { id, kind, message }]
  emit()
  window.setTimeout(() => dismissToast(id), kind === 'error' ? 6000 : 4000)
}

export function dismissToast(id: number) {
  toasts = toasts.filter((t) => t.id !== id)
  emit()
}

export function useToasts() {
  const [list, setList] = useState<Toast[]>(toasts)
  useEffect(() => {
    listeners.add(setList)
    setList([...toasts])
    return () => {
      listeners.delete(setList)
    }
  }, [])
  return list
}

export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}

export function relativeTime(iso?: string): string {
  if (!iso) return '从未'
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return '从未'
  const diff = Date.now() - then
  const min = Math.floor(diff / 60000)
  if (min < 1) return '刚刚'
  if (min < 60) return `${min} 分钟前`
  const hours = Math.floor(min / 60)
  if (hours < 24) return `${hours} 小时前`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days} 天前`
  return `${Math.floor(days / 30)} 个月前`
}

export function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v < 10 && i > 0 ? 1 : 0)} ${units[i]}`
}

// Confirm dialog state used by destructive actions.
export function useConfirm() {
  const [request, setRequest] = useState<{
    title: string
    description: string
    confirmLabel?: string
    danger?: boolean
    resolve: (ok: boolean) => void
  } | null>(null)

  const confirm = useCallback(
    (opts: { title: string; description: string; confirmLabel?: string; danger?: boolean }) =>
      new Promise<boolean>((resolve) => setRequest({ ...opts, resolve })),
    [],
  )

  const close = useCallback(
    (ok: boolean) => {
      request?.resolve(ok)
      setRequest(null)
    },
    [request],
  )

  return { request, confirm, close }
}
