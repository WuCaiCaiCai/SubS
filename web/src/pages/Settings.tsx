import { useEffect, useState } from 'react'
import { Lock, Save, Settings2, ShieldCheck, Terminal } from 'lucide-react'
import { api, ApiError, type SystemInfo } from '../lib/api'
import { toast, useAsync } from '../lib/hooks'
import { Badge, ErrorNote, SectionLabel, SkeletonRows, Spinner, Switch } from '../components/ui'

export function SettingsPage() {
  const system = useAsync(() => api.get<SystemInfo>('/api/system'))
  const [name, setName] = useState('')
  const [externalUrl, setExternalUrl] = useState('')
  const [httpsOnly, setHttpsOnly] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [savedAt, setSavedAt] = useState(0)

  useEffect(() => {
    const s = system.data
    if (!s) return
    setName(s.instanceName)
    setExternalUrl(s.externalUrl ?? '')
    setHttpsOnly(s.httpsOnly)
  }, [system.data])

  const saveGeneral = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const saved = await api.put<{ instanceName: string; externalUrl: string }>('/api/system', { instanceName: name, externalUrl })
      toast('success', '设置已保存')
      setName(saved.instanceName)
      setExternalUrl(saved.externalUrl ?? '')
      setSavedAt(Date.now())
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  const toggleHttpsOnly = async (next: boolean) => {
    // 开启前确认当前连接已是 HTTPS，否则开关一开就把浏览器自己锁在外面。
    if (next && window.location.protocol !== 'https:') {
      toast('error', '当前连接非 HTTPS，无法开启该开关')
      return
    }
    setBusy(true)
    setError('')
    // 乐观更新：立即折叠开关；失败时回弹。误触开启不会被锁：
    // 服务器只在一个已有 https 连接的后续同步请求完成锁定，页面刷新不再造成影响。
    setHttpsOnly(next)
    try {
      const saved = await api.put<{ httpsOnly: boolean }>('/api/system', { httpsOnly: next })
      setHttpsOnly(saved.httpsOnly)
      toast('success', next ? '已开启仅 HTTPS 访问' : '已关闭仅 HTTPS 访问')
    } catch (err) {
      setHttpsOnly(!next)
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
        <p className="mt-1 text-[0.7812rem] text-fg-muted">实例标识、订阅地址与安全信息。</p>
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

        <section className="card space-y-3.5 p-4">
          <div className="flex items-center gap-2">
            <Lock size={14} className="text-accent" />
            <h2 className="text-[0.8125rem] font-semibold">仅支持 HTTPS 访问</h2>
            {httpsOnly && <Badge tone="emerald">已启用</Badge>}
          </div>
          <p className="text-[0.75rem] leading-relaxed text-fg-muted">
            开启后拒绝所有未加密（明文 HTTP）请求，包括管理面板与订阅链接。反向代理与证书（Caddy、Nginx、宝塔等）由你自行部署，本实例只信任直连 TLS 或代理转发的
            <code className="mono">X-Forwarded-Proto: https</code>。
          </p>
          <div className="flex items-center justify-between gap-3 rounded-lg border border-line bg-canvas p-3">
            <div className="text-[0.75rem]">
              <div className="font-medium text-fg-soft">拒绝明文连接</div>
              <div className="mt-0.5 text-[0.6875rem] text-fg-muted">
                {httpsOnly ? '非 HTTPS 请求返回 426，健康检查 /healthz 除外' : '当前允许 HTTP 与 HTTPS 访问'}
              </div>
            </div>
            <Switch checked={httpsOnly} disabled={busy} onChange={toggleHttpsOnly} />
          </div>
          {window.location.protocol !== 'https:' && (
            <p className="text-[0.6875rem] text-warning">
              当前页面通过 HTTP 访问，需先配置 HTTPS（如 Caddy / Nginx 反代）再开启此开关。
            </p>
          )}
        </section>
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
