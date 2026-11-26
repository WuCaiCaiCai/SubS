import { useEffect, useState } from 'react'
import { Download, Save, Settings2, ShieldCheck, Terminal } from 'lucide-react'
import { api, ApiError, type CaddyConfig, type SystemInfo } from '../lib/api'
import { toast, useAsync } from '../lib/hooks'
import { Badge, ErrorNote, SectionLabel, SkeletonRows, Spinner } from '../components/ui'

export function SettingsPage() {
  const system = useAsync(() => api.get<SystemInfo>('/api/system'))
  const [name, setName] = useState('')
  const [externalUrl, setExternalUrl] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [savedAt, setSavedAt] = useState(0)
  const [caddy, setCaddy] = useState<CaddyConfig>({ domain: '', port: 8787 })
  const [caddyBusy, setCaddyBusy] = useState(false)
  const [caddyError, setCaddyError] = useState('')
  const [caddyfile, setCaddyfile] = useState('')

  useEffect(() => {
    const s = system.data
    if (!s) return
    setName(s.instanceName)
    setExternalUrl(s.externalUrl ?? '')
    api.get<CaddyConfig>('/api/caddy').then(setCaddy).catch(() => undefined)
  }, [system.data])

  const saveCaddy = async (e: React.FormEvent) => {
    e.preventDefault()
    setCaddyBusy(true)
    setCaddyError('')
    try {
      const result = await api.put<{ config: CaddyConfig; caddyfile: string }>('/api/caddy', caddy)
      setCaddy(result.config)
      setCaddyfile(result.caddyfile)
      toast('success', 'Caddy 配置已保存')
    } catch (err) {
      setCaddyError(err instanceof ApiError ? err.message : '保存 Caddy 配置失败')
    } finally {
      setCaddyBusy(false)
    }
  }

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

      </div>

      <form onSubmit={saveCaddy} className="card space-y-3.5 p-4">
        <div className="flex items-center gap-2">
          <ShieldCheck size={14} className="text-success" />
          <h2 className="text-[0.8125rem] font-semibold">Caddy HTTPS 反向代理</h2>
          <Badge tone="amber">手动部署</Badge>
        </div>
        <p className="max-w-3xl text-[0.75rem] leading-relaxed text-fg-muted">
          生成仅代理到本机 SubS 的 Caddyfile，并纳入加密备份。保存不会安装、重载 Caddy 或更改监听地址；请先将 SubS 监听改为 127.0.0.1，再由服务器管理员部署配置。Caddy 会自动申请并续期公网证书，域名 DNS 必须指向本机且 80/443 可达。
        </p>
        <div className="grid gap-3 sm:grid-cols-[1fr_10rem]">
          <label className="block space-y-1.5">
            <span className="text-[0.75rem] font-medium text-fg-soft">公网域名</span>
            <input className="input mono" value={caddy.domain} onChange={(e) => setCaddy({ ...caddy, domain: e.target.value })} placeholder="subs.example.com" autoComplete="off" />
          </label>
          <label className="block space-y-1.5">
            <span className="text-[0.75rem] font-medium text-fg-soft">本机上游端口</span>
            <input className="input mono" type="number" min={1} max={65535} value={caddy.port} onChange={(e) => setCaddy({ ...caddy, port: Number(e.target.value) })} />
          </label>
        </div>
        {caddyError && <ErrorNote>{caddyError}</ErrorNote>}
        <div className="flex flex-wrap gap-2">
          <button type="submit" className="btn btn-primary" disabled={caddyBusy}>
            {caddyBusy ? <Spinner /> : <Save size={14} />} 保存配置
          </button>
          {caddy.domain && <a className="btn" href="/api/caddyfile"><Download size={14} /> 下载 Caddyfile</a>}
        </div>
        {caddyfile && <pre className="mono overflow-x-auto rounded border border-line bg-canvas p-3 text-[0.7188rem] leading-relaxed text-fg-soft">{caddyfile}</pre>}
        <p className="text-[0.6875rem] text-fg-muted">跨服务器恢复只迁移域名和代理配置，不包含 Caddy 证书或私钥；在目标机部署后会重新签发。Caddyfile 导出接口需管理员登录。</p>
      </form>

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
