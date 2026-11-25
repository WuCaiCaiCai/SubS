import { useState } from 'react'
import { Eye, EyeOff, KeyRound, Loader2, LockKeyhole, ShieldCheck } from 'lucide-react'
import { api, ApiError, setCsrfToken, type Me } from '../lib/api'
import { ErrorNote } from '../components/ui'

type Mode = 'login' | 'init'

export function LoginPage({ onAuthed }: { onAuthed: () => void }) {
  const [mode, setMode] = useState<Mode>('login')
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [initToken, setInitToken] = useState('')
  const [show, setShow] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (mode === 'init') {
      if (password.length < 12) {
        setError('密码至少 12 位')
        return
      }
      if (password !== confirm) {
        setError('两次输入的密码不一致')
        return
      }
    }
    setBusy(true)
    try {
      const path = mode === 'init' ? '/api/auth/init' : '/api/auth/login'
      const body =
        mode === 'init'
          ? { username, password, initToken }
          : { username, password }
      const res = await api.post<Me>(path, body)
      if (res.csrfToken) setCsrfToken(res.csrfToken)
      onAuthed()
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === 'init_disabled') {
          setError('服务端未启用初始化（未设置 SUBS_INIT_TOKEN），请改用登录')
          setMode('login')
        } else if (err.code === 'rate_limited') {
          setError('尝试过于频繁，请稍后再试')
        } else if (err.code === 'already_initialized') {
          setError('实例已初始化，请直接登录')
          setMode('login')
        } else {
          setError(err.message)
        }
      } else {
        setError('无法连接服务端')
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-dvh items-center justify-center bg-canvas px-4">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex items-center gap-2.5">
          <div className="flex h-8 w-8 items-center justify-center rounded border border-accent/30 bg-accent/10 font-mono text-[0.875rem] font-semibold text-accent">
            S
          </div>
          <div>
            <div className="mono text-[0.9375rem] font-semibold tracking-tight">SubS</div>
            <div className="text-[0.7188rem] text-fg-muted">自托管订阅管理</div>
          </div>
        </div>

        <div className="card p-5">
          <div className="mb-4 flex items-center gap-2">
            <ShieldCheck size={15} className="text-accent" />
            <h1 className="text-[0.875rem] font-semibold">
              {mode === 'login' ? '登录实例' : '初始化实例'}
            </h1>
          </div>

          <form onSubmit={submit} className="space-y-3.5">
            <Field label="管理员账号">
              <input
                className="input"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="username"
                required
              />
            </Field>

            <Field label="密码">
              <div className="relative">
                <input
                  className="input pr-11"
                  type={show ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete={mode === 'init' ? 'new-password' : 'current-password'}
                  required
                />
                <button
                  type="button"
                  onClick={() => setShow((v) => !v)}
                  aria-label={show ? '隐藏密码' : '显示密码'}
                  className="absolute inset-y-0 right-0 flex w-11 items-center justify-center text-fg-muted hover:text-fg-soft"
                >
                  {show ? <EyeOff size={15} /> : <Eye size={15} />}
                </button>
              </div>
            </Field>

            {mode === 'init' && (
              <>
                <Field label="确认密码">
                  <input
                    className="input"
                    type={show ? 'text' : 'password'}
                    value={confirm}
                    onChange={(e) => setConfirm(e.target.value)}
                    autoComplete="new-password"
                    required
                  />
                </Field>
                <Field label="初始化令牌" hint="服务端启动时的 SUBS_INIT_TOKEN">
                  <input
                    className="input"
                    value={initToken}
                    onChange={(e) => setInitToken(e.target.value)}
                    autoComplete="off"
                    required
                  />
                </Field>
              </>
            )}

            {error && <ErrorNote>{error}</ErrorNote>}

            <button type="submit" className="btn btn-primary w-full" disabled={busy}>
              {busy ? <Loader2 size={14} className="animate-spin" /> : <LockKeyhole size={14} />}
              {mode === 'login' ? '登录' : '创建实例'}
            </button>
          </form>

          <div className="mt-4 border-t border-line pt-3 text-center">
            <button
              type="button"
              onClick={() => {
                setMode(mode === 'login' ? 'init' : 'login')
                setError('')
              }}
              className="inline-flex items-center gap-1.5 text-[0.75rem] text-fg-muted hover:text-fg-soft"
            >
              <KeyRound size={12} />
              {mode === 'login' ? '首次使用？初始化实例' : '已有实例？返回登录'}
            </button>
          </div>
        </div>

        <p className="mt-4 flex items-center justify-center gap-1.5 text-[0.6875rem] text-fg-subtle">
          <LockKeyhole size={11} />
          凭据仅存储在本机，不发送到任何第三方
        </p>
      </div>
    </div>
  )
}

function Field({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children: React.ReactNode
}) {
  return (
    <label className="block space-y-1.5">
      <span className="block text-[0.75rem] font-medium text-fg-soft">{label}</span>
      {children}
      {hint && <span className="block text-[0.7188rem] text-fg-muted">{hint}</span>}
    </label>
  )
}
