import { useRef, useState } from 'react'
import { Check, DatabaseBackup, Download, FileArchive, ShieldAlert, Upload } from 'lucide-react'
import { api, ApiError, type BackupResult } from '../lib/api'
import { formatBytes, relativeTime, toast, useAsync } from '../lib/hooks'
import { Badge, CopyButton, ErrorNote, SectionLabel, SkeletonRows, Spinner } from '../components/ui'

export function BackupPage() {
  const system = useAsync(() => api.get<{ lastBackupAt: string; instanceName: string }>('/api/system'))
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<BackupResult | null>(null)
  const [downloaded, setDownloaded] = useState(false)

  const [restoreFile, setRestoreFile] = useState<File | null>(null)
  const [restorePassword, setRestorePassword] = useState('')
  const [restoring, setRestoring] = useState(false)
  const [restoreError, setRestoreError] = useState('')
  const fileInput = useRef<HTMLInputElement>(null)

  const create = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (password.length < 12) {
      setError('备份密码至少 12 位')
      return
    }
    if (password !== confirm) {
      setError('两次输入的密码不一致')
      return
    }
    setBusy(true)
    try {
      const res = await api.post<BackupResult>('/api/backup', { password })
      setResult(res)
      setDownloaded(false)
      toast('success', '备份已生成，请立即下载')
      system.reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '生成备份失败')
    } finally {
      setBusy(false)
    }
  }

  const download = () => {
    if (!result) return
    const a = document.createElement('a')
    a.href = `/api/backup/${result.downloadId}`
    a.download = `subs-backup-${result.sha256.slice(0, 12)}.sbox`
    document.body.appendChild(a)
    a.click()
    a.remove()
    setDownloaded(true)
    toast('success', '已开始下载，请核对 SHA-256')
  }

  const restore = async (e: React.FormEvent) => {
    e.preventDefault()
    setRestoreError('')
    if (!restoreFile) {
      setRestoreError('请选择 .sbox 备份文件')
      return
    }
    if (restorePassword.length < 12) {
      setRestoreError('备份密码至少 12 位')
      return
    }
    setRestoring(true)
    try {
      const form = new FormData()
      form.append('file', restoreFile)
      form.append('password', restorePassword)
      const res = await fetch('/api/restore', { method: 'POST', body: form, credentials: 'same-origin', headers: { 'X-CSRF-Token': csrf() } })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new ApiError(res.status, body.code ?? 'restore_failed', body.message ?? '恢复失败')
      }
      toast('success', '恢复完成；如需启用 Caddy，请 SSH 配置 Caddy，正在重新登录…')
      window.setTimeout(() => {
        window.location.href = '/'
      }, 1200)
    } catch (err) {
      setRestoreError(err instanceof ApiError ? err.message : '恢复失败')
    } finally {
      setRestoring(false)
    }
  }

  return (
    <div className="space-y-5">
      <header>
        <SectionLabel>workspace / backup</SectionLabel>
        <h1 className="mt-1 text-page font-semibold tracking-tight">备份与恢复</h1>
        <p className="mt-1 text-[0.7812rem] text-fg-muted">
          导出的 <code className="mono">.sbox</code> 包含数据库、全部凭据及已配置的 Caddy 反代文件，使用 age 加密；不包含 Caddy 证书/私钥，密码一旦丢失无法找回。
        </p>
      </header>

      <div className="rounded-md border border-line bg-surface/40 px-3 py-2.5 text-[0.75rem] text-fg-muted">
        {system.loading ? (
          '读取备份状态…'
        ) : (
          <span className="flex items-center gap-2">
            <DatabaseBackup size={13} className="text-fg-muted" />
            上次备份：<span className="mono text-fg-soft">{relativeTime(system.data?.lastBackupAt)}</span>
            {!system.data?.lastBackupAt && <Badge tone="amber">从未备份</Badge>}
          </span>
        )}
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <form onSubmit={create} className="card space-y-3.5 p-4">
          <div className="flex items-center gap-2">
            <Download size={14} className="text-accent" />
            <h2 className="text-[0.8125rem] font-semibold">创建备份</h2>
          </div>

          <label className="block space-y-1.5">
            <span className="text-[0.75rem] font-medium text-fg-soft">加密密码</span>
            <input
              className="input"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
            />
            <span className="block text-[0.6875rem] text-fg-muted">至少 12 位；与登录密码无关，请单独保存。</span>
          </label>

          <label className="block space-y-1.5">
            <span className="text-[0.75rem] font-medium text-fg-soft">确认密码</span>
            <input
              className="input"
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              autoComplete="new-password"
            />
          </label>

          {error && <ErrorNote>{error}</ErrorNote>}

          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? <Spinner /> : <DatabaseBackup size={14} />} 生成备份
          </button>

          {result && (
            <div className="rounded-md border border-success/25 bg-success/5 p-3">
              <div className="flex items-center gap-1.5 text-[0.75rem] text-success">
                <Check size={13} /> 备份已生成 · {formatBytes(result.sizeBytes)}
              </div>
              <div className="mono mt-2 flex items-center gap-1.5">
                <span className="truncate text-[0.6562rem] text-fg-muted" title={result.sha256}>
                  {result.sha256}
                </span>
                <CopyButton value={result.sha256} label="复制 SHA-256" />
              </div>
              <button type="button" className="btn btn-primary mt-2.5 w-full" onClick={download}>
                <Download size={13} /> 下载 .sbox 文件
              </button>
              {!downloaded && (
                <p className="mt-1.5 text-[0.6875rem] text-warning/80">下载完成前，销毁实例会被拒绝。</p>
              )}
            </div>
          )}
        </form>

        <form onSubmit={restore} className="card space-y-3.5 p-4">
          <div className="flex items-center gap-2">
            <Upload size={14} className="text-warning" />
            <h2 className="text-[0.8125rem] font-semibold">从备份恢复</h2>
          </div>

          <div className="rounded-md border border-warning/30 bg-warning/5 px-2.5 py-2 text-[0.7188rem] text-warning">
            <span className="flex items-start gap-1.5">
              <ShieldAlert size={13} className="mt-0.5 shrink-0" />
              恢复会覆盖当前数据库并清空登录会话，操作前请先导出一份当前备份。
            </span>
          </div>

          <div>
            <span className="text-[0.75rem] font-medium text-fg-soft">备份文件</span>
            <button
              type="button"
              onClick={() => fileInput.current?.click()}
              className="mt-1.5 flex w-full items-center gap-2 rounded border border-dashed border-line-strong px-3 py-3 text-left text-[0.75rem] text-fg-muted hover:border-line-strong hover:text-fg"
            >
              <FileArchive size={15} className="shrink-0" />
              <span className="min-w-0 flex-1 truncate">
                {restoreFile ? restoreFile.name : '选择 .sbox 文件'}
              </span>
              {restoreFile && <span className="mono text-[0.6875rem] text-fg-muted">{formatBytes(restoreFile.size)}</span>}
            </button>
            <input
              ref={fileInput}
              type="file"
              accept=".sbox,application/octet-stream"
              className="hidden"
              onChange={(e) => setRestoreFile(e.target.files?.[0] ?? null)}
            />
          </div>

          <label className="block space-y-1.5">
            <span className="text-[0.75rem] font-medium text-fg-soft">备份密码</span>
            <input
              className="input"
              type="password"
              value={restorePassword}
              onChange={(e) => setRestorePassword(e.target.value)}
              autoComplete="off"
            />
          </label>

          {restoreError && <ErrorNote>{restoreError}</ErrorNote>}

          <button type="submit" className="btn btn-danger" disabled={restoring}>
            {restoring ? <Spinner /> : <Upload size={14} />} 恢复并覆盖
          </button>
        </form>
      </div>

      <section className="card p-4">
        <SectionLabel>命令行备份</SectionLabel>
        <p className="mt-1 text-[0.75rem] text-fg-muted">在服务器上无需登录即可导出/恢复：</p>
        <pre className="mono mt-2 overflow-x-auto rounded border border-line bg-canvas p-2.5 text-[0.7188rem] leading-relaxed text-fg-soft">
          {`subs backup  --data-dir /var/lib/subs --out ./backup.sbox\nsubs restore --data-dir /var/lib/subs --in ./backup.sbox`}
        </pre>
      </section>
    </div>
  )
}

function csrf(): string {
  const m = document.cookie.match(/(?:^|;\s*)subs_csrf=([^;]+)/)
  return m ? decodeURIComponent(m[1]) : ''
}

export { SkeletonRows }
