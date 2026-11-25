import { useEffect, useMemo, useState } from 'react'
import {
  Boxes,
  Check,
  ChevronDown,
  Filter,
  Pencil,
  Plane,
  Plus,
  RotateCcw,
  Search,
  Server,
  Sparkles,
  Trash2,
  X,
} from 'lucide-react'
import {
  api,
  ApiError,
  type Airport,
  type ImportPreview,
  type NodeItem,
  type Protocol,
  type VpsServer,
} from '../lib/api'
import { toast, useAsync, useConfirm } from '../lib/hooks'
import {
  Badge,
  EmptyState,
  ErrorNote,
  Modal,
  ProtocolBadge,
  SectionLabel,
  SkeletonRows,
  Spinner,
} from '../components/ui'

interface Filters {
  sourceType?: 'airport' | 'server'
  sourceId?: number
  protocol?: Protocol
  isNew?: boolean
  missing?: boolean
  q?: string
}

function readFilters(): Filters {
  const p = new URLSearchParams(window.location.search)
  const f: Filters = {}
  const st = p.get('sourceType')
  if (st === 'airport' || st === 'server') f.sourceType = st
  const sid = p.get('sourceId')
  if (sid) f.sourceId = Number(sid)
  const proto = p.get('protocol')
  if (proto) f.protocol = proto as Protocol
  if (p.get('isNew') === '1') f.isNew = true
  if (p.get('missing') === '1') f.missing = true
  const q = p.get('q')
  if (q) f.q = q
  return f
}

export function NodesPage({ onNavigate }: { onNavigate: (path: string) => void }) {
  const [filters, setFilters] = useState<Filters>(readFilters)
  const [search, setSearch] = useState(filters.q ?? '')
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [importOpen, setImportOpen] = useState(new URLSearchParams(window.location.search).get('import') === '1')
  const [editing, setEditing] = useState<NodeItem | null>(null)
  const { request, confirm, close } = useConfirm()

  const airports = useAsync(() => api.get<Airport[]>('/api/airports'))
  const servers = useAsync(() => api.get<VpsServer[]>('/api/servers'))
  const query = useMemo(() => {
    const p = new URLSearchParams()
    if (filters.sourceType) p.set('sourceType', filters.sourceType)
    if (filters.sourceId) p.set('sourceId', String(filters.sourceId))
    if (filters.protocol) p.set('protocol', filters.protocol)
    if (filters.isNew) p.set('isNew', 'true')
    if (filters.missing) p.set('missing', 'true')
    if (filters.q) p.set('q', filters.q)
    const s = p.toString()
    return s ? `?${s}` : ''
  }, [filters])

  const nodes = useAsync(() => api.get<NodeItem[]>(`/api/nodes${query}`), [query])

  const reloadAll = () => {
    nodes.reload()
    airports.reload()
    servers.reload()
    setSelected(new Set())
  }

  const confirmNew = async () => {
    try {
      await api.post('/api/nodes/confirm-new')
      toast('success', '新节点标记已清除')
      reloadAll()
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '操作失败')
    }
  }

  const toggleNode = async (node: NodeItem, enabled: boolean) => {
    try {
      await api.put(`/api/nodes/${node.id}`, {
        rawName: node.rawName,
        config: node.config,
        enabled,
        tags: node.tags,
      })
      nodes.reload()
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '更新失败')
    }
  }

  const restore = async (node: NodeItem) => {
    try {
      await api.put(`/api/nodes/${node.id}`, { rawName: node.rawName, config: node.config, enabled: node.enabled, tags: node.tags })
      toast('info', '节点已标记为可用（仍需上游重新出现）')
    } catch {
      toast('error', '操作失败')
    }
  }

  const removeNode = async (node: NodeItem) => {
    const ok = await confirm({
      title: `删除节点「${node.rawName}」？`,
      description:
        node.inProfiles > 0
          ? `该节点被 ${node.inProfiles} 个编排引用，删除后会从这些编排中移除。`
          : '删除后可从上游刷新重新获取。',
      confirmLabel: '删除',
      danger: true,
    })
    if (!ok) return
    try {
      await api.del(`/api/nodes/${node.id}`)
      toast('success', '节点已删除')
      nodes.reload()
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '删除失败')
    }
  }

  const bulk = async (action: 'enable' | 'disable' | 'delete') => {
    const ids = [...selected]
    if (!ids.length) return
    if (action === 'delete') {
      const ok = await confirm({
        title: `删除 ${ids.length} 个节点？`,
        description: '被编排引用的节点会同时从编排中移除。',
        confirmLabel: '删除',
        danger: true,
      })
      if (!ok) return
    }
    let failed = 0
    for (const id of ids) {
      const node = nodes.data?.find((n) => n.id === id)
      if (!node) continue
      try {
        if (action === 'delete') {
          await api.del(`/api/nodes/${id}`)
        } else {
          await api.put(`/api/nodes/${id}`, {
            rawName: node.rawName,
            config: node.config,
            enabled: action === 'enable',
            tags: node.tags,
          })
        }
      } catch {
        failed++
      }
    }
    toast(failed ? 'warning' : 'success', failed ? `${ids.length - failed} 成功，${failed} 失败` : '批量操作已完成')
    reloadAll()
  }

  const allNodes = nodes.data ?? []
  const sourceOptions = [
    ...(airports.data ?? []).map((a) => ({ type: 'airport' as const, id: a.id, name: a.name, count: a.nodeCount })),
    ...(servers.data ?? []).map((s) => ({ type: 'server' as const, id: s.id, name: s.name, count: s.nodeCount })),
  ]

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <SectionLabel>workspace / nodes</SectionLabel>
          <h1 className="mt-1 text-base font-semibold tracking-tight">节点库</h1>
          <p className="mt-1 text-[12.5px] text-zinc-500">
            机场与自建节点统一存放。新出现的机场节点会标记为待确认。
          </p>
        </div>
        <div className="flex items-center gap-2">
          {(nodes.data ?? []).some((n) => n.isNew) && (
            <button type="button" className="btn btn-ghost" onClick={confirmNew}>
              <Check size={14} /> 确认新节点
            </button>
          )}
          <button type="button" className="btn btn-primary" onClick={() => setImportOpen(true)}>
            <Plus size={14} /> 导入节点
          </button>
        </div>
      </header>

      <div className="grid gap-4 lg:grid-cols-[220px_1fr]">
        <aside className="space-y-3">
          <div className="card p-2.5">
            <div className="mb-1.5 flex items-center justify-between px-1">
              <SectionLabel>来源</SectionLabel>
              {(filters.sourceId || filters.sourceType) && (
                <button
                  type="button"
                  onClick={() => setFilters({ ...filters, sourceType: undefined, sourceId: undefined })}
                  className="text-[11px] text-zinc-500 hover:text-zinc-300"
                >
                  清除
                </button>
              )}
            </div>
            <div className="flex gap-1.5 overflow-x-auto pb-1 lg:flex-col lg:overflow-visible lg:pb-0">
              <button
                type="button"
                onClick={() => setFilters({ ...filters, sourceType: undefined, sourceId: undefined })}
                className={`flex shrink-0 items-center gap-2 rounded px-2 py-1.5 text-[12px] transition-colors lg:w-full ${
                  !filters.sourceType && !filters.sourceId
                    ? 'bg-zinc-800 text-zinc-100'
                    : 'text-zinc-400 hover:bg-zinc-800/50'
                }`}
              >
                <Boxes size={13} /> 全部
                <span className="mono ml-auto text-[11px] text-zinc-500">{airports.data && servers.data ? (airports.data.reduce((s, a) => s + a.nodeCount, 0) + servers.data.reduce((s, v) => s + v.nodeCount, 0)) : ''}</span>
              </button>
              {sourceOptions.map((opt) => {
                const active = filters.sourceId === opt.id && filters.sourceType === opt.type
                return (
                  <button
                    key={`${opt.type}-${opt.id}`}
                    type="button"
                    onClick={() => setFilters({ ...filters, sourceType: opt.type, sourceId: opt.id })}
                    className={`flex shrink-0 items-center gap-2 rounded px-2 py-1.5 text-[12px] transition-colors lg:w-full ${
                      active ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:bg-zinc-800/50'
                    }`}
                  >
                    {opt.type === 'airport' ? (
                      <Plane size={13} className="text-[var(--airport-accent)]" />
                    ) : (
                      <Server size={13} className="text-[var(--server-accent)]" />
                    )}
                    <span className="truncate">{opt.name}</span>
                    <span className="mono ml-auto shrink-0 text-[11px] text-zinc-500">{opt.count}</span>
                  </button>
                )
              })}
            </div>
          </div>

          <div className="card hidden space-y-2 p-2.5 lg:block">
            <SectionLabel>状态</SectionLabel>
            <FilterChip
              active={!!filters.isNew}
              onClick={() => setFilters({ ...filters, isNew: filters.isNew ? undefined : true, missing: undefined })}
            >
              <Sparkles size={12} /> 待确认
            </FilterChip>
            <FilterChip
              active={!!filters.missing}
              onClick={() => setFilters({ ...filters, missing: filters.missing ? undefined : true, isNew: undefined })}
            >
              <RotateCcw size={12} /> 已缺失
            </FilterChip>
          </div>
        </aside>

        <section className="min-w-0 space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative min-w-0 flex-1">
              <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-zinc-500" />
              <input
                className="input pl-8"
                placeholder="搜索节点原始名…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') setFilters({ ...filters, q: search || undefined })
                }}
              />
              {search && (
                <button
                  type="button"
                  aria-label="清除搜索"
                  onClick={() => {
                    setSearch('')
                    setFilters({ ...filters, q: undefined })
                  }}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-300"
                >
                  <X size={13} />
                </button>
              )}
            </div>
            <div className="relative">
              <select
                aria-label="按协议筛选"
                className="input h-11 w-[118px] appearance-none pr-7 md:h-10"
                value={filters.protocol ?? ''}
                onChange={(e) =>
                  setFilters({ ...filters, protocol: (e.target.value || undefined) as Protocol | undefined })
                }
              >
                <option value="">全部协议</option>
                {(['hy2', 'vless', 'tuic', 'anytls', 'mieru'] as Protocol[]).map((p) => (
                  <option key={p} value={p}>
                    {p.toUpperCase()}
                  </option>
                ))}
              </select>
              <ChevronDown size={13} className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-500" />
            </div>
            <button
              type="button"
              onClick={() => setFilters({ ...filters, q: search || undefined })}
              className="btn btn-ghost px-2.5"
            >
              <Filter size={14} />
              <span className="hidden sm:inline">筛选</span>
            </button>
          </div>

          {selected.size > 0 && (
            <div className="flex flex-wrap items-center gap-2 rounded-md border border-cyan-400/30 bg-cyan-400/5 px-3 py-2 text-[12px]">
              <span className="text-cyan-200">已选 {selected.size} 项</span>
              <div className="ml-auto flex items-center gap-1.5">
                <button type="button" className="btn btn-ghost h-8" onClick={() => bulk('enable')}>
                  启用
                </button>
                <button type="button" className="btn btn-ghost h-8" onClick={() => bulk('disable')}>
                  停用
                </button>
                <button type="button" className="btn btn-danger h-8" onClick={() => bulk('delete')}>
                  删除
                </button>
                <button
                  type="button"
                  aria-label="取消选择"
                  className="flex h-8 w-8 items-center justify-center rounded text-cyan-200/70 hover:bg-cyan-400/10"
                  onClick={() => setSelected(new Set())}
                >
                  <X size={14} />
                </button>
              </div>
            </div>
          )}

          {nodes.loading ? (
            <SkeletonRows rows={6} />
          ) : nodes.error ? (
            <ErrorNote>{nodes.error.message}</ErrorNote>
          ) : allNodes.length === 0 ? (
            <EmptyState
              icon={<Boxes size={18} />}
              title="没有匹配的节点"
              description="先添加机场并刷新，或为自建 VPS 导入节点链接。"
              action={
                <button type="button" className="btn btn-primary" onClick={() => setImportOpen(true)}>
                  <Plus size={14} /> 导入节点
                </button>
              }
            />
          ) : (
            <ul className="divide-y divide-zinc-800/70 overflow-hidden rounded-lg border border-zinc-800">
              {allNodes.map((node) => {
                const checked = selected.has(node.id)
                return (
                  <li
                    key={node.id}
                    className={`flex items-start gap-2.5 px-3 py-2.5 transition-colors hover:bg-zinc-900/50 ${
                      checked ? 'bg-cyan-400/[0.04]' : ''
                    } ${!node.enabled ? 'opacity-60' : ''}`}
                  >
                    <input
                      type="checkbox"
                      aria-label={`选择 ${node.rawName}`}
                      checked={checked}
                      onChange={(e) => {
                        const next = new Set(selected)
                        if (e.target.checked) next.add(node.id)
                        else next.delete(node.id)
                        setSelected(next)
                      }}
                      className="mt-1 h-4 w-4 shrink-0"
                    />
                    <span
                      aria-hidden
                      className={`mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full ${
                        node.sourceType === 'airport'
                          ? 'bg-[var(--airport-accent)]'
                          : 'bg-[var(--server-accent)]'
                      }`}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-1.5">
                        <span
                          className={`mono truncate text-[12.5px] ${
                            node.missing ? 'text-zinc-500 line-through' : 'text-zinc-100'
                          }`}
                        >
                          {node.rawName}
                        </span>
                        {node.isNew && (
                          <Badge tone="amber">
                            <Sparkles size={9} /> 新
                          </Badge>
                        )}
                        {node.missing && <Badge tone="neutral">缺失</Badge>}
                        {!node.enabled && <Badge tone="neutral">停用</Badge>}
                      </div>
                      <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1">
                        <ProtocolBadge protocol={node.protocol} />
                        <span className="mono truncate text-[11px] text-zinc-500">
                          {node.host}:{node.portRange || node.port}
                        </span>
                        <span className="text-[10.5px] text-zinc-600">{node.sourceName}</span>
                        {node.inProfiles > 0 && (
                          <span className="text-[10.5px] text-zinc-600">· {node.inProfiles} 编排</span>
                        )}
                      </div>
                    </div>
                    <div className="flex shrink-0 items-center gap-1">
                      <button
                        type="button"
                        role="switch"
                        aria-checked={node.enabled}
                        aria-label={node.enabled ? '停用节点' : '启用节点'}
                        onClick={() => toggleNode(node, !node.enabled)}
                        className={`relative h-5 w-9 rounded-full border transition-colors ${
                          node.enabled
                            ? 'border-cyan-400/40 bg-cyan-400/25'
                            : 'border-zinc-700 bg-zinc-800'
                        }`}
                      >
                        <span
                          className={`absolute top-0.5 h-3.5 w-3.5 rounded-full bg-zinc-200 transition-all ${
                            node.enabled ? 'left-[18px]' : 'left-0.5'
                          }`}
                        />
                      </button>
                      {node.missing && (
                        <button
                          type="button"
                          aria-label="标记为可用"
                          title="标记为可用"
                          onClick={() => restore(node)}
                          className="flex h-8 w-8 items-center justify-center rounded text-zinc-500 hover:bg-zinc-800 hover:text-emerald-300"
                        >
                          <RotateCcw size={13} />
                        </button>
                      )}
                      <button
                        type="button"
                        aria-label="编辑节点"
                        title="编辑节点"
                        onClick={() => setEditing(node)}
                        className="flex h-8 w-8 items-center justify-center rounded text-zinc-500 hover:bg-zinc-800 hover:text-zinc-200"
                      >
                        <Pencil size={13} />
                      </button>
                      <button
                        type="button"
                        aria-label="删除节点"
                        title="删除节点"
                        onClick={() => removeNode(node)}
                        className="flex h-8 w-8 items-center justify-center rounded text-zinc-500 hover:bg-zinc-800 hover:text-red-300"
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>
                  </li>
                )
              })}
            </ul>
          )}
        </section>
      </div>

      <ImportDialog
        open={importOpen}
        defaultSourceId={filters.sourceType === 'server' ? filters.sourceId : undefined}
        servers={servers.data ?? []}
        onClose={() => setImportOpen(false)}
        onDone={() => {
          setImportOpen(false)
          reloadAll()
        }}
        onCreateServer={() => onNavigate('/sources')}
      />

      <Modal
        open={!!editing}
        title="编辑节点"
        onClose={() => setEditing(null)}
        footer={
          <button type="button" className="btn btn-ghost" onClick={() => setEditing(null)}>
            关闭
          </button>
        }
      >
        {editing && (
          <div className="space-y-3">
            <div>
              <SectionLabel>原始名</SectionLabel>
              <p className="mono mt-1 break-all text-[12.5px] text-zinc-200">{editing.rawName}</p>
            </div>
            <div>
              <SectionLabel>指纹</SectionLabel>
              <p className="mono mt-1 break-all text-[11px] text-zinc-500">{editing.fingerprint}</p>
            </div>
            <div>
              <SectionLabel>配置（含凭据，已加密存储）</SectionLabel>
              <pre className="mono mt-1 max-h-64 overflow-auto rounded border border-zinc-800 bg-zinc-950 p-2 text-[11.5px] text-zinc-400">
                {JSON.stringify(editing.config, null, 2)}
              </pre>
            </div>
          </div>
        )}
      </Modal>

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

function FilterChip({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={`flex w-full items-center gap-1.5 rounded px-2 py-1.5 text-[12px] transition-colors ${
        active ? 'bg-amber-400/10 text-amber-200' : 'text-zinc-400 hover:bg-zinc-800/50'
      }`}
    >
      {children}
    </button>
  )
}

function ImportDialog({
  open,
  defaultSourceId,
  servers,
  onClose,
  onDone,
  onCreateServer,
}: {
  open: boolean
  defaultSourceId?: number
  servers: VpsServer[]
  onClose: () => void
  onDone: () => void
  onCreateServer: () => void
}) {
  const [sourceId, setSourceId] = useState<number | ''>(defaultSourceId ?? '')
  const [text, setText] = useState('')
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (open) {
      setSourceId(defaultSourceId ?? (servers[0]?.id || ''))
      setPreview(null)
      setError('')
    }
  }, [open, defaultSourceId, servers])

  const parse = async (commit: boolean) => {
    if (!sourceId) {
      setError('请先选择 VPS 分组')
      return
    }
    if (!text.trim()) {
      setError('请粘贴节点链接')
      return
    }
    setBusy(true)
    setError('')
    try {
      const res = await api.post<ImportPreview>('/api/nodes/import', {
        sourceId,
        text,
        commit,
      })
      setPreview(res)
      if (commit) {
        toast('success', `已导入 ${res.saved} 个节点`)
        onDone()
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '解析失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={open}
      wide
      title="导入自建节点"
      description="支持 hy2 / vless / tuic / anytls / mieru 分享链接，可一次粘贴多行、整段 base64 订阅，或直接贴 mieru 客户端配置 JSON。"
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            取消
          </button>
          <button type="button" className="btn btn-ghost" onClick={() => parse(false)} disabled={busy}>
            {busy && <Spinner />} 解析预览
          </button>
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => parse(true)}
            disabled={busy || !preview || preview.parsed.length === 0}
          >
            确认导入 {preview ? `(${preview.parsed.length})` : ''}
          </button>
        </>
      }
    >
      <div className="space-y-3">
        <label className="block space-y-1.5">
          <span className="text-[12px] font-medium text-zinc-300">归属 VPS</span>
          {servers.length === 0 ? (
            <div className="flex items-center justify-between rounded border border-amber-400/30 bg-amber-400/5 px-3 py-2 text-[12px] text-amber-200">
              还没有 VPS 分组
              <button type="button" onClick={onCreateServer} className="underline">
                去创建
              </button>
            </div>
          ) : (
            <select
              className="input"
              value={sourceId}
              onChange={(e) => setSourceId(Number(e.target.value))}
            >
              <option value="">选择分组…</option>
              {servers.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          )}
        </label>

        <label className="block space-y-1.5">
          <span className="text-[12px] font-medium text-zinc-300">节点链接</span>
          <textarea
            className="input mono h-40 resize-y py-2 text-[12px] leading-relaxed break-all"
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder={'hy2://password@host:443?sni=host#香港 01\nvless://uuid@host:443?security=reality&...#东京 02'}
            spellCheck={false}
          />
        </label>

        {error && <ErrorNote>{error}</ErrorNote>}

        {preview && (
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-2 text-[12px]">
              <Badge tone="emerald">{preview.parsed.length} 可导入</Badge>
              {preview.errors.length > 0 && <Badge tone="red">{preview.errors.length} 解析失败</Badge>}
              {preview.ignored > 0 && <Badge tone="neutral">{preview.ignored} 忽略</Badge>}
            </div>
            <ul className="max-h-56 divide-y divide-zinc-800 overflow-y-auto rounded border border-zinc-800">
              {preview.parsed.map((p, i) => (
                <li key={i} className="flex items-center gap-2 px-2.5 py-1.5">
                  <Check size={12} className="shrink-0 text-emerald-400" />
                  <span className="mono truncate text-[12px] text-zinc-200">{p.rawName}</span>
                  <ProtocolBadge protocol={p.protocol} />
                  <span className="mono ml-auto truncate text-[11px] text-zinc-500">
                    {p.host}:{p.portRange || p.port}
                  </span>
                </li>
              ))}
              {preview.errors.map((e, i) => (
                <li key={`e-${i}`} className="flex items-start gap-2 px-2.5 py-1.5">
                  <X size={12} className="mt-0.5 shrink-0 text-red-400" />
                  <span className="text-[11.5px] text-red-300">
                    第 {e.line} 行：{e.reason}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </Modal>
  )
}
