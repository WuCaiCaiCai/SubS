import { useState } from 'react'
import { AlertOctagon, ArrowLeft, ShieldAlert, Trash2, TriangleAlert } from 'lucide-react'
import { api, ApiError, type SystemInfo } from '../lib/api'
import { toast, useAsync } from '../lib/hooks'
import { ErrorNote, SectionLabel, SkeletonRows, Spinner } from '../components/ui'

export function DestroyPage({ onNavigate }: { onNavigate: (path: string) => void }) {
  const system = useAsync(() => api.get<SystemInfo & { latestBackupSha?: string }>('/api/system'))
  const [phrase, setPhrase] = useState('')
  const [sha, setSha] = useState('')
  const [ack, setAck] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [stage, setStage] = useState<'idle' | 'final'>('idle')

  const instanceName = system.data?.instanceName ?? ''
  const latestSha = system.data?.latestBackupSha ?? ''

  const run = async (dryRun: boolean) => {
    setError('')
    if (phrase.trim() !== instanceName) {
      setError(`确认短语与实例名不一致，请输入「${instanceName}」`)
      return
    }
    if (!ack) {
      setError('请先勾选确认知晓不可恢复')
      return
    }
    if (!dryRun && !sha.trim()) {
      setError('请填写已下载备份的 SHA-256')
      return
    }
    setBusy(true)
    try {
      await api.post('/api/destroy', {
        confirmPhrase: phrase.trim(),
        backupSha256: sha.trim(),
        dryRun,
      })
      if (dryRun) {
        toast('info', '预检查通过：可以执行销毁')
        setStage('final')
      } else {
        toast('success', '实例已销毁')
        window.setTimeout(() => {
          window.location.href = '/'
        }, 800)
      }
    } catch (err) {
      if (err instanceof ApiError) {
        const map: Record<string, string> = {
          confirm_mismatch: '确认短语与实例名不一致',
          backup_required: '缺少备份 SHA-256，请先创建并下载备份',
          backup_not_found: '未找到匹配的备份记录',
          backup_not_downloaded: '该备份尚未下载，请先下载',
        }
        setError(map[err.code] ?? err.message)
      } else {
        setError('请求失败')
      }
    } finally {
      setBusy(false)
    }
  }

  if (system.loading) return <SkeletonRows rows={3} />

  return (
    <div className="mx-auto max-w-xl space-y-5">
      <button
        type="button"
        onClick={() => onNavigate('/')}
        className="inline-flex items-center gap-1 text-[0.75rem] text-fg-muted hover:text-fg-soft"
      >
        <ArrowLeft size={12} /> 返回总览
      </button>

      <div className="rounded-lg border border-danger/30 bg-red-500/[0.04] p-4">
        <div className="flex items-center gap-2">
          <AlertOctagon size={16} className="text-danger" />
          <h1 className="text-[0.875rem] font-semibold text-danger">销毁实例</h1>
        </div>
        <p className="mt-2 text-[0.7812rem] leading-relaxed text-danger/80">
          此操作会删除数据库、主密钥与全部备份文件，且
          <b> 不可恢复 </b>。为防止误操作，需要先导出一份备份，并提交该备份的 SHA-256 进行校验。
        </p>
        <ul className="mt-2.5 space-y-1 text-[0.75rem] text-danger/70">
          <li>· 将清空：机场订阅、VPS、节点、模板、编排</li>
          <li>· 将清空：管理员账号、会话、审计记录</li>
          <li>· 将删除：master.key 与 backups/ 目录</li>
          <li>· 订阅链接会立即永久失效</li>
        </ul>
      </div>

      <section className="card space-y-3.5 p-4">
        <div className="flex items-center gap-2">
          <ShieldAlert size={14} className="text-fg-muted" />
          <h2 className="text-[0.8125rem] font-semibold">确认</h2>
        </div>

        <label className="block space-y-1.5">
          <span className="text-[0.75rem] font-medium text-fg-soft">
            输入实例名 <code className="mono text-accent">{instanceName}</code>
          </span>
          <input
            className="input mono"
            value={phrase}
            onChange={(e) => setPhrase(e.target.value)}
            placeholder={instanceName}
            autoComplete="off"
          />
        </label>

        <label className="block space-y-1.5">
          <div className="flex items-center justify-between gap-2">
            <span className="text-[0.75rem] font-medium text-fg-soft">已下载备份的 SHA-256</span>
            {latestSha ? (
              <button
                type="button"
                onClick={() => setSha(latestSha)}
                className="text-[0.6875rem] text-accent hover:underline"
              >
                填入最近已下载备份
              </button>
            ) : (
              <button
                type="button"
                onClick={() => onNavigate('/backup')}
                className="text-[0.6875rem] text-warning hover:underline"
              >
                去备份页创建并下载 →
              </button>
            )}
          </div>
          <input
            className="input mono text-[0.7188rem]"
            value={sha}
            onChange={(e) => setSha(e.target.value)}
            placeholder="64 位十六进制"
            autoComplete="off"
          />
          <span className="block text-[0.6875rem] text-fg-muted">
            在「备份」页创建并下载后会显示该值。
          </span>
        </label>

        <label className="flex items-start gap-2 text-[0.7812rem] text-fg-soft">
          <input
            type="checkbox"
            checked={ack}
            onChange={(e) => setAck(e.target.checked)}
            className="mt-0.5 h-4 w-4"
          />
          我已知晓该操作不可恢复，并已保存好离线备份。
        </label>

        {error && <ErrorNote>{error}</ErrorNote>}

        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            className="btn btn-ghost"
            onClick={() => run(true)}
            disabled={busy}
          >
            {busy ? <Spinner /> : null} 预检查
          </button>
          <button
            type="button"
            className="btn btn-danger"
            onClick={() => run(false)}
            disabled={busy}
          >
            {busy ? <Spinner /> : <Trash2 size={14} />} 永久销毁实例
          </button>
        </div>

        {stage === 'final' && (
          <p className="flex items-start gap-1.5 rounded border border-line bg-surface/60 px-2.5 py-2 text-[0.7188rem] text-fg-muted">
            <TriangleAlert size={13} className="mt-0.5 shrink-0 text-warning" />
            预检查已通过。点击「永久销毁实例」后将立即执行，无法回滚。
          </p>
        )}
      </section>

      <p className="text-center text-[0.6875rem] text-fg-subtle">
        也可以在服务器上执行 <code className="mono">subs destroy --data-dir /var/lib/subs</code>
      </p>
    </div>
  )
}

export { SectionLabel }
