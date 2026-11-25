import { useEffect, useMemo, useState } from 'react'
import { Check, FileCode2, Loader2, Plus, Save, Trash2, TriangleAlert, X } from 'lucide-react'
import { api, ApiError, type Template } from '../lib/api'
import { relativeTime, toast, useAsync, useConfirm } from '../lib/hooks'
import { Badge, EmptyState, ErrorNote, SectionLabel, SkeletonRows, Spinner } from '../components/ui'
import { CodeEditor } from '../components/CodeEditor'

export function TemplatesPage() {
  const templates = useAsync(() => api.get<Template[]>('/api/templates'))
  const { request, confirm, close } = useConfirm()
  const [activeId, setActiveId] = useState<number | null>(null)
  const [draft, setDraft] = useState<{ name: string; content: string; target: string }>({
    name: '',
    content: '',
    target: 'mihomo',
  })
  const [isNew, setIsNew] = useState(false)
  const [issues, setIssues] = useState<{ line: number; message: string }[] | null>(null)
  const [checking, setChecking] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const list = templates.data ?? []
  const active = useMemo(() => list.find((t) => t.id === activeId), [list, activeId])

  useEffect(() => {
    if (isNew) return
    if (!activeId && list.length > 0) {
      setActiveId(list[0].id)
    }
  }, [list, activeId, isNew])

  useEffect(() => {
    if (isNew) return
    if (active) {
      setDraft({ name: active.name, content: active.content, target: active.target || 'mihomo' })
      setIssues(null)
      setError('')
    }
  }, [active, isNew])

  const startNew = () => {
    setIsNew(true)
    setActiveId(null)
    setDraft({
      name: '新模板',
      target: 'mihomo',
      content:
        'mixed-port: 7890\nallow-lan: false\nmode: rule\nlog-level: info\nexternal-controller: 127.0.0.1:9090\n\nproxies:\n\nproxy-groups:\n  - name: PROXY\n    type: select\n    proxies:\n      - DIRECT\n\nrules:\n  - MATCH,PROXY\n',
    })
    setIssues(null)
  }

  const save = async () => {
    if (!draft.name.trim() || !draft.content.trim()) {
      setError('名称与内容不能为空')
      return
    }
    setSaving(true)
    setError('')
    try {
      if (isNew) {
        const created = await api.post<Template>('/api/templates', {
          name: draft.name,
          target: draft.target,
          content: draft.content,
        })
        toast('success', '模板已创建')
        setIsNew(false)
        templates.reload()
        setActiveId(created.id)
      } else if (active) {
        await api.put(`/api/templates/${active.id}`, {
          name: draft.name,
          target: draft.target,
          content: draft.content,
        })
        toast('success', '模板已保存')
        templates.reload()
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const validate = async () => {
    if (!active) {
      setError('请先保存模板再校验')
      return
    }
    setChecking(true)
    try {
      const res = await api.post<{ ok: boolean; errors: { line: number; message: string }[] }>(
        `/api/templates/${active.id}/validate`,
        { content: draft.content },
      )
      setIssues(res.errors ?? [])
      if (res.ok) {
        toast('success', '模板校验通过')
        templates.reload()
      } else {
        toast('warning', `发现 ${res.errors?.length ?? 0} 个问题`)
      }
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '校验失败')
    } finally {
      setChecking(false)
    }
  }

  const remove = async (t: Template) => {
    const ok = await confirm({
      title: `删除模板「${t.name}」？`,
      description: '引用该模板的编排会失去配置来源，需要重新指定模板。',
      confirmLabel: '删除',
      danger: true,
    })
    if (!ok) return
    try {
      await api.del(`/api/templates/${t.id}`)
      toast('success', '模板已删除')
      setActiveId(null)
      setIsNew(false)
      templates.reload()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        toast('error', '该模板正被编排引用，请先解除引用')
      } else {
        toast('error', err instanceof ApiError ? err.message : '删除失败')
      }
    }
  }

  const lang = draft.target === 'sing-box' ? 'json' : draft.target === 'loon' ? 'ini' : 'yaml'

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <SectionLabel>workspace / templates</SectionLabel>
          <h1 className="mt-1 text-page font-semibold tracking-tight">模板管理</h1>
          <p className="mt-1 text-[0.7812rem] text-fg-muted">
            粘贴完整的 Mihomo 配置，SubS 只替换其中的 proxies 与节点列表，其余原样保留。
          </p>
        </div>
        <button type="button" className="btn btn-primary" onClick={startNew}>
          <Plus size={14} /> 新建模板
        </button>
      </header>

      {templates.loading ? (
        <SkeletonRows rows={3} />
      ) : (
        <div className="grid gap-4 lg:grid-cols-[240px_1fr]">
          <aside className="space-y-1.5">
            {list.length === 0 && !isNew ? (
              <EmptyState
                icon={<FileCode2 size={18} />}
                title="还没有模板"
                description="导入一份 Clash 配置作为模板。"
                action={
                  <button type="button" className="btn btn-primary" onClick={startNew}>
                    <Plus size={14} /> 新建模板
                  </button>
                }
              />
            ) : (
              <ul className="overflow-hidden rounded-lg border border-line">
                {isNew && (
                  <li className="flex items-center gap-2 border-b border-line bg-raised/40 px-3 py-2.5">
                    <span className="h-1.5 w-1.5 rounded-full bg-accent" />
                    <span className="truncate text-[0.7812rem] text-fg">{draft.name || '未命名'}</span>
                    <Badge tone="cyan">草稿</Badge>
                  </li>
                )}
                {list.map((t) => (
                  <li key={t.id} className={isNew ? 'opacity-50' : ''}>
                    <button
                      type="button"
                      onClick={() => {
                        setIsNew(false)
                        setActiveId(t.id)
                      }}
                      className={`flex w-full flex-col gap-0.5 px-3 py-2.5 text-left transition-colors ${
                        activeId === t.id && !isNew ? 'bg-raised/60' : 'hover:bg-surface/60'
                      }`}
                    >
                      <span className="truncate text-[0.7812rem] text-fg">{t.name}</span>
                      <span className="mono flex items-center gap-1.5 text-[0.6562rem] text-fg-muted">
                        {t.target}
                        <span className="text-fg-subtle">·</span>
                        {relativeTime(t.lastCheck)}
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </aside>

          <section className="min-w-0 space-y-3">
            {!active && !isNew ? (
              <div className="card flex h-64 items-center justify-center text-[0.7812rem] text-fg-muted">
                从左侧选择一个模板开始编辑
              </div>
            ) : (
              <>
                <div className="flex flex-wrap items-center gap-2">
                  <input
                    className="input h-10 min-w-0 flex-1 text-[0.8125rem]"
                    value={draft.name}
                    onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                    aria-label="模板名称"
                  />
                  <select
                    className="input h-12 w-32"
                    value={draft.target}
                    onChange={(e) => setDraft({ ...draft, target: e.target.value })}
                    aria-label="目标客户端"
                  >
                    <option value="mihomo">Mihomo</option>
                    <option value="sing-box">sing-box</option>
                    <option value="loon">Loon</option>
                  </select>
                  <button type="button" className="btn btn-ghost" onClick={validate} disabled={checking || !active}>
                    {checking ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />} 校验
                  </button>
                  <button type="button" className="btn btn-primary" onClick={save} disabled={saving}>
                    {saving ? <Spinner /> : <Save size={14} />} 保存
                  </button>
                  {active && (
                    <button
                      type="button"
                      aria-label="删除模板"
                      className="btn btn-ghost px-2.5 text-fg-muted hover:text-danger"
                      onClick={() => remove(active)}
                    >
                      <Trash2 size={14} />
                    </button>
                  )}
                </div>

                {error && <ErrorNote>{error}</ErrorNote>}

                {issues !== null && (
                  <div
                    className={`rounded-md border px-3 py-2 ${
                      issues.length === 0
                        ? 'border-success/30 bg-success/5'
                        : 'border-danger/30 bg-danger/5'
                    }`}
                  >
                    <div className="flex items-center gap-1.5 text-[0.75rem]">
                      {issues.length === 0 ? (
                        <>
                          <Check size={13} className="text-success" />
                          <span className="text-success">校验通过：结构、占位符与节点引用均正常</span>
                        </>
                      ) : (
                        <>
                          <TriangleAlert size={13} className="text-danger" />
                          <span className="text-danger">{issues.length} 个问题</span>
                        </>
                      )}
                    </div>
                    {issues.length > 0 && (
                      <ul className="mt-1.5 space-y-0.5">
                        {issues.map((i, idx) => (
                          <li key={idx} className="mono flex items-start gap-1.5 text-[0.7188rem] text-danger/90">
                            <X size={11} className="mt-0.5 shrink-0" />
                            <span>
                              {i.line > 0 && <span className="text-danger/70">第 {i.line} 行 · </span>}
                              {i.message}
                            </span>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                )}

                <CodeEditor
                  value={draft.content}
                  onChange={(content) => setDraft({ ...draft, content })}
                  language={lang}
                  ariaLabel="模板内容"
                  footer={draft.target}
                />

                <details className="rounded-md border border-line bg-surface/40 px-3 py-2 text-[0.7188rem] text-fg-muted">
                  <summary className="cursor-pointer text-fg-soft">占位符说明</summary>
                  <ul className="mono mt-2 space-y-1 text-[0.6875rem]">
                    <li>
                      <code className="text-accent"># subs:proxies</code> — 节点定义注入点（可选；
                      未标注时自动替换顶层 <code>proxies:</code>）
                    </li>
                    <li>
                      <code className="text-accent"># subs:nodes</code> — 代理组内节点名列表注入点
                    </li>
                    <li>
                      <code className="text-accent"># subs:names</code> — 同上，用于需要全量节点名的组
                    </li>
                    <li className="text-fg-muted">
                      使用 <code>include-all: true</code> 的组无需占位符，会自动包含所有节点。
                    </li>
                  </ul>
                </details>
              </>
            )}
          </section>
        </div>
      )}

      <ConfirmModal request={request} onClose={close} />
    </div>
  )
}

function ConfirmModal({
  request,
  onClose,
}: {
  request: { title: string; description?: string; confirmLabel?: string; danger?: boolean } | null
  onClose: (ok: boolean) => void
}) {
  if (!request) return null
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      role="dialog"
      aria-modal="true"
      onClick={() => onClose(false)}
    >
      <div className="w-full max-w-sm rounded-lg border border-line bg-surface p-4" onClick={(e) => e.stopPropagation()}>
        <h2 className="text-[0.8438rem] font-semibold">{request.title}</h2>
        {request.description && <p className="mt-1.5 text-[0.7812rem] text-fg-muted">{request.description}</p>}
        <div className="mt-4 flex justify-end gap-2">
          <button type="button" className="btn btn-ghost" onClick={() => onClose(false)}>
            取消
          </button>
          <button
            type="button"
            className={`btn ${request.danger ? 'btn-danger' : 'btn-primary'}`}
            onClick={() => onClose(true)}
          >
            {request.confirmLabel ?? '确认'}
          </button>
        </div>
      </div>
    </div>
  )
}
