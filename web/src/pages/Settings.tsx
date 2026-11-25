import { useEffect, useState } from 'react'
import { Check, Save, Server, Settings2, ShieldCheck, Terminal } from 'lucide-react'
import { api, ApiError, type SystemInfo } from '../lib/api'
import { toast, useAsync } from '../lib/hooks'
import { Badge, ErrorNote, SectionLabel, SkeletonRows, Spinner } from '../components/ui'

export function SettingsPage() {
  const system = useAsync(() => api.get<SystemInfo>('/api/system'))
  const [name, setName] = useState('')
  const [externalUrl, setExternalUrl] = useState('')
  const [host, setHost] = useState('')
  const [port, setPort] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [savedAt, setSavedAt] = useState(0)

  useEffect(() => {
    const s = system.data
    if (!s) return
    setName(s.instanceName)
    setExternalUrl(s.externalUrl ?? '')
    setHost(s.listenHost)
    setPort(String(s.listenPort))
  }, [system.data])

  const saveGeneral = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await api.put('/api/system', { instanceName: name, externalUrl })
      toast('success', '设置已保存')
      setSavedAt(Date.now())
      system.reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  const saveListen = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await api.put('/api/system/listen', { host, port: Number(port) })
      toast('info', '监听设置已保存，重启服务后生效')
      system.reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  if (system.loading) return <SkeletonRows rows={4} />
  if (system.error) return <ErrorNote>{system.error.message}</ErrorNote>

  return (
    <div className="space-y-5">
      <header>
        <SectionLabel>workspace / settings</SectionLabel>
        <h1 className="mt-1 text-page font-semibold tracking-tight">设置</h1>
        <p className="mt-1 text-[0.7812rem] text-fg-muted">实例标识、对外地址与监听参数。</p>
      </header>

      {error && <ErrorNote>{error}</ErrorNote>}

      <div className="grid gap-4 lg:grid-cols-2">
        <form onSubmit={saveGeneral} className="card space-y-3.5 p-4">
          <div className="flex items-center gap-2">
            <Settings2 size={14} className="text-accent" />
            <h2 className="text-[0.8125rem] font-semibold">实例</h2>
            {savedAt > 0 && <Badge tone="emerald">已保存</Badge>}
          </div>

          <label className="block space-y-1.5">
            <span className="text-[0.75rem] font-medium text-fg-soft">实例名</span>
            <input className="input" value={name} onChange={(e) => setName(e.target.value)} required />
            <span className="block text-[0.6875rem] text-fg-muted">销毁实例时需要输入此名称作为确认短语。</span>
          </label>

          <label className="block space-y-1.5">
            <span className="text-[0.75rem] font-medium text-fg-soft">外部地址</span>
            <input
              className="input mono"
              value={externalUrl}
              onChange={(e) => setExternalUrl(e.target.value)}
              placeholder="https://subs.example.com"
            />
            <span className="block text-[0.6875rem] text-fg-muted">
              用于拼接订阅链接；不填则只能复制相对路径 <code className="mono">/s/&lt;token&gt;</code>。
            </span>
          </label>

          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? <Spinner /> : <Save size={14} />} 保存
          </button>
        </form>

        <form onSubmit={saveListen} className="card space-y-3.5 p-4">
          <div className="flex items-center gap-2">
            <Server size={14} className="text-warning" />
            <h2 className="text-[0.8125rem] font-semibold">监听</h2>
          </div>

          <label className="block space-y-1.5">
            <span className="text-[0.75rem] font-medium text-fg-soft">监听地址</span>
            <input className="input mono" value={host} onChange={(e) => setHost(e.target.value)} required />
            <span className="block text-[0.6875rem] text-fg-muted">
              默认仅本机；公网访问建议前置 Nginx/Caddy 反向代理。
            </span>
          </label>

          <label className="block space-y-1.5">
            <span className="text-[0.75rem] font-medium text-fg-soft">端口</span>
            <input
              className="input mono"
              type="number"
              min={1024}
              max={65535}
              value={port}
              onChange={(e) => setPort(e.target.value)}
              required
            />
          </label>

          <button type="submit" className="btn btn-ghost" disabled={busy}>
            {busy ? <Spinner /> : <Check size={14} />} 保存
          </button>
        </form>
      </div>

      <section className="card p-4">
        <div className="flex items-center gap-2">
          <Terminal size={14} className="text-fg-muted" />
          <h2 className="text-[0.8125rem] font-semibold">命令行工具</h2>
        </div>
        <p className="mt-1 text-[0.75rem] text-fg-muted">
          忘记密码或需要清空登录会话时，在服务器上直接执行：
        </p>
        <pre className="mono mt-2 overflow-x-auto rounded border border-line bg-canvas p-2.5 text-[0.7188rem] text-fg-soft">
          {`subs reset-password --data-dir /var/lib/subs\nsubs backup --data-dir /var/lib/subs --out ./backup.sbox`}
        </pre>
      </section>

      <section className="card p-4">
        <div className="flex items-center gap-2">
          <ShieldCheck size={14} className="text-success" />
          <h2 className="text-[0.8125rem] font-semibold">安全</h2>
        </div>
        <dl className="mono mt-2.5 space-y-1.5 text-[0.7188rem]">
          <div className="flex justify-between">
            <dt className="text-fg-muted">凭据加密</dt>
            <dd className="text-fg-soft">AES-256-GCM（master.key）</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-fg-muted">密码哈希</dt>
            <dd className="text-fg-soft">argon2id</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-fg-muted">登录限流</dt>
            <dd className="text-fg-soft">5 次 / 分钟 / IP</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-fg-muted">遥测</dt>
            <dd className="text-fg-soft">无</dd>
          </div>
        </dl>
      </section>
    </div>
  )
}
