import { useState } from 'react'
import { ArrowUpRight, Copy, Layers3, Plus, ShieldCheck, Trash2 } from 'lucide-react'
import { api, ApiError, type Profile, type Template } from '../lib/api'
import { copyText, relativeTime, toast, useAsync, useConfirm } from '../lib/hooks'
import {
  Badge,
  CopyButton,
  EmptyState,
  ErrorNote,
  Modal,
  SectionLabel,
  SkeletonRows,
  Spinner,
} from '../components/ui'

export function ProfilesPage({ onNavigate }: { onNavigate: (path: string) => void }) {
  const profiles = useAsync(() => api.get<Profile[]>('/api/profiles'))
  const templates = useAsync(() => api.get<Template[]>('/api/templates'))
  const { request, confirm, close } = useConfirm()
  const [open, setOpen] = useState(false)

  const remove = async (p: Profile) => {
    const ok = await confirm({
      title: `删除编排「${p.name}」？`,
      description: '订阅链接会立即失效，节点本身不受影响。',
      confirmLabel: '删除',
      danger: true,
    })
    if (!ok) return
    try {
      await api.del(`/api/profiles/${p.id}`)
      toast('success', '编排已删除')
      profiles.reload()
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '删除失败')
    }
  }

  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <SectionLabel>workspace / profiles</SectionLabel>
          <h1 className="mt-1 text-base font-semibold tracking-tight">订阅编排</h1>
          <p className="mt-1 text-[12.5px] text-zinc-500">
            每个编排 = 一组节点 + 一套命名规则 + 一个模板，对外暴露一条订阅链接。
          </p>
        </div>
        <button type="button" className="btn btn-primary" onClick={() => setOpen(true)}>
          <Plus size={14} /> 新建编排
        </button>
      </header>

      {profiles.loading ? (
        <SkeletonRows rows={3} />
      ) : profiles.error ? (
        <ErrorNote>{profiles.error.message}</ErrorNote>
      ) : (profiles.data?.length ?? 0) === 0 ? (
        <EmptyState
          icon={<ShieldCheck size={18} />}
          title="还没有订阅编排"
          description="选择节点、调整顺序与命名、套用 Clash 模板，三步生成订阅链接。"
          action={
            <button type="button" className="btn btn-primary" onClick={() => setOpen(true)}>
              <Plus size={14} /> 新建第一个编排
            </button>
          }
        />
      ) : (
        <ul className="grid gap-2.5 sm:grid-cols-2">
          {profiles.data!.map((p) => (
            <li key={p.id} className="card flex flex-col p-4">
              <div className="flex items-start justify-between gap-2">
                <button
                  type="button"
                  onClick={() => onNavigate(`/profiles/${p.id}`)}
                  className="group min-w-0 flex-1 text-left"
                >
                  <div className="flex items-center gap-2">
                    <span className="truncate text-[13.5px] font-medium text-zinc-100 group-hover:text-cyan-200">
                      {p.name}
                    </span>
                    {p.enabled ? <Badge tone="emerald">启用</Badge> : <Badge tone="neutral">停用</Badge>}
                  </div>
                  {p.note && <p className="mt-1 line-clamp-2 text-[12px] text-zinc-500">{p.note}</p>}
                </button>
                <button
                  type="button"
                  aria-label="删除编排"
                  onClick={() => remove(p)}
                  className="flex h-8 w-8 shrink-0 items-center justify-center rounded text-zinc-600 hover:bg-zinc-800 hover:text-red-300"
                >
                  <Trash2 size={13} />
                </button>
              </div>

              <dl className="mono mt-3 grid grid-cols-2 gap-x-3 gap-y-1.5 text-[11.5px]">
                <div className="flex items-center gap-1.5 text-zinc-500">
                  <ShieldCheck size={11} /> 节点
                  <span className="ml-auto text-zinc-300">{p.nodeCount}</span>
                </div>
                <div className="flex items-center gap-1.5 text-zinc-500">
                  <Layers3 size={11} /> 模板
                  <span className="ml-auto truncate text-zinc-300" title={p.templateName}>
                    {p.templateName || '—'}
                  </span>
                </div>
                <div className="col-span-2 flex items-center gap-1.5 text-zinc-500">
                  <Copy size={11} /> 最近拉取
                  <span className="ml-auto text-zinc-300">{relativeTime(p.lastFetchAt)}</span>
                </div>
              </dl>

              <div className="mt-3 flex items-center gap-1.5 border-t border-zinc-800/70 pt-2.5">
                {p.url ? (
                  <>
                    <code className="min-w-0 flex-1 truncate text-[11px] text-zinc-500" title={p.url}>
                      /s/{p.token}
                    </code>
                    <CopyButton value={p.url} label="复制订阅链接" />
                  </>
                ) : (
                  <span className="text-[11.5px] text-amber-300/80">未配置外部地址，无法生成完整链接</span>
                )}
                <button
                  type="button"
                  onClick={() => onNavigate(`/profiles/${p.id}`)}
                  className="flex h-8 items-center gap-1 rounded border border-zinc-700 px-2 text-[12px] text-zinc-300 hover:border-zinc-600 hover:text-zinc-100"
                >
                  编辑 <ArrowUpRight size={12} />
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}

      <CreateProfileDialog
        open={open}
        templates={templates.data ?? []}
        onClose={() => setOpen(false)}
        onCreated={(id) => {
          setOpen(false)
          profiles.reload()
          onNavigate(`/profiles/${id}`)
        }}
      />

      <Modal
        open={!!request}
        title={request?.title ?? ''}
        description={request?.description}
        onClose={() => close(false)}
        footer={
          <>
            <button type="button" className="btn btn-ghost" onClick={() => close(false)}>
              取消
            </button>
            <button
              type="button"
              className={`btn ${request?.danger ? 'btn-danger' : 'btn-primary'}`}
              onClick={() => close(true)}
            >
              {request?.confirmLabel ?? '确认'}
            </button>
          </>
        }
      >
        <p className="text-[12.5px] text-zinc-400">此操作不可撤销。</p>
      </Modal>
    </div>
  )
}

function CreateProfileDialog({
  open,
  templates,
  onClose,
  onCreated,
}: {
  open: boolean
  templates: Template[]
  onClose: () => void
  onCreated: (id: number) => void
}) {
  const [name, setName] = useState('')
  const [note, setNote] = useState('')
  const [templateId, setTemplateId] = useState<number | ''>('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!templateId) {
      setError('请选择一个模板')
      return
    }
    setBusy(true)
    setError('')
    try {
      const p = await api.post<Profile>('/api/profiles', {
        name,
        note,
        templateId,
        namingRule: { segments: ['index', 'region', 'protocol', 'rawName'], separator: '-', zeroPad: 2, upperProtocol: true },
      })
      toast('success', '编排已创建，继续选择节点')
      setName('')
      setNote('')
      onCreated(p.id)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '创建失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={open}
      title="新建编排"
      description="创建后进入三步编辑器：选节点 → 排序命名 → 模板发布。"
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            取消
          </button>
          <button type="submit" form="profile-create-form" className="btn btn-primary" disabled={busy}>
            {busy && <Spinner />} 创建并继续
          </button>
        </>
      }
    >
      <form id="profile-create-form" onSubmit={submit} className="space-y-3.5">
        <label className="block space-y-1.5">
          <span className="text-[12px] font-medium text-zinc-300">编排名称</span>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} required placeholder="例如：日常主力" />
        </label>
        <label className="block space-y-1.5">
          <span className="text-[12px] font-medium text-zinc-300">备注（可选）</span>
          <input className="input" value={note} onChange={(e) => setNote(e.target.value)} />
        </label>
        <label className="block space-y-1.5">
          <span className="text-[12px] font-medium text-zinc-300">模板</span>
          {templates.length === 0 ? (
            <p className="rounded border border-amber-400/30 bg-amber-400/5 px-3 py-2 text-[12px] text-amber-200">
              还没有模板，请先在「模板」页导入一份 Clash 配置。
            </p>
          ) : (
            <select
              className="input"
              value={templateId}
              onChange={(e) => setTemplateId(Number(e.target.value))}
            >
              <option value="">选择模板…</option>
              {templates.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name}
                </option>
              ))}
            </select>
          )}
        </label>
        {error && <ErrorNote>{error}</ErrorNote>}
      </form>
    </Modal>
  )
}

export { copyText }
