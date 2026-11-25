import { useEffect, useMemo, useState } from 'react'
import {
  AlertTriangle,
  MoreHorizontal,
  Pencil,
  Plane,
  Plus,
  RefreshCw,
  Server,
  Trash2,
} from 'lucide-react'
import { api, ApiError, type Airport, type RefreshStats, type VpsServer } from '../lib/api'
import { relativeTime, toast, useAsync, useConfirm } from '../lib/hooks'
import {
  Badge,
  EmptyState,
  ErrorNote,
  Modal,
  SectionLabel,
  SkeletonRows,
  Spinner,
} from '../components/ui'

type Tab = 'airport' | 'server'

export function SourcesPage({ onNavigate }: { onNavigate: (path: string) => void }) {
  const [tab, setTab] = useState<Tab>('airport')
  const airports = useAsync(() => api.get<Airport[]>('/api/airports'))
  const servers = useAsync(() => api.get<VpsServer[]>('/api/servers'))
  const { request, confirm, close } = useConfirm()

  const [airportModal, setAirportModal] = useState<{ mode: 'create' } | { mode: 'edit'; airport: Airport } | null>(null)
  const [serverModal, setServerModal] = useState<{ mode: 'create' } | { mode: 'edit'; server: VpsServer } | null>(null)
  const [refreshing, setRefreshing] = useState<number | null>(null)
  const [refreshResult, setRefreshResult] = useState<{ airport: Airport; stats: RefreshStats } | null>(null)

  const refresh = async (airport: Airport) => {
    setRefreshing(airport.id)
    try {
      const stats = await api.post<RefreshStats>(`/api/airports/${airport.id}/refresh`)
      setRefreshResult({ airport, stats })
      airports.reload()
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '刷新失败')
    } finally {
      setRefreshing(null)
    }
  }

  const removeAirport = async (airport: Airport) => {
    const ok = await confirm({
      title: `删除机场「${airport.name}」？`,
      description: `将同时删除其 ${airport.nodeCount} 个节点。若节点已被编排引用，需要确认强制删除。`,
      confirmLabel: '删除',
      danger: true,
    })
    if (!ok) return
    try {
      await api.del(`/api/airports/${airport.id}`)
      toast('success', '机场已删除')
      airports.reload()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        const force = await confirm({
          title: '节点正在被编排引用',
          description: '强制删除会同时把这些节点从所有编排中移除。是否继续？',
          confirmLabel: '强制删除',
          danger: true,
        })
        if (!force) return
        try {
          await api.del(`/api/airports/${airport.id}?force=1`)
          toast('success', '机场及引用已删除')
          airports.reload()
        } catch (e2) {
          toast('error', e2 instanceof ApiError ? e2.message : '删除失败')
        }
        return
      }
      toast('error', err instanceof ApiError ? err.message : '删除失败')
    }
  }

  const removeServer = async (server: VpsServer) => {
    const ok = await confirm({
      title: `删除 VPS「${server.name}」？`,
      description: `将同时删除其 ${server.nodeCount} 个节点。`,
      confirmLabel: '删除',
      danger: true,
    })
    if (!ok) return
    try {
      await api.del(`/api/servers/${server.id}`)
      toast('success', 'VPS 已删除')
      servers.reload()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        const force = await confirm({
          title: '节点正在被编排引用',
          description: '强制删除会同时把这些节点从所有编排中移除。是否继续？',
          confirmLabel: '强制删除',
          danger: true,
        })
        if (!force) return
        try {
          await api.del(`/api/servers/${server.id}?force=1`)
          toast('success', 'VPS 及引用已删除')
          servers.reload()
        } catch (e2) {
          toast('error', e2 instanceof ApiError ? e2.message : '删除失败')
        }
        return
      }
      toast('error', err instanceof ApiError ? err.message : '删除失败')
    }
  }

  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <SectionLabel>workspace / sources</SectionLabel>
          <h1 className="mt-1 text-page font-semibold tracking-tight">来源管理</h1>
          <p className="mt-1 text-[0.7812rem] text-fg-muted">
            机场订阅与自建 VPS 是两类平级来源，节点都会汇入节点库。
          </p>
        </div>
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => (tab === 'airport' ? setAirportModal({ mode: 'create' }) : setServerModal({ mode: 'create' }))}
        >
          <Plus size={14} /> {tab === 'airport' ? '添加机场' : '添加 VPS'}
        </button>
      </header>

      <div role="tablist" aria-label="来源类型" className="flex gap-1 rounded-lg border border-line bg-surface/50 p-1">
        <TabButton
          active={tab === 'airport'}
          onClick={() => setTab('airport')}
          icon={<Plane size={14} />}
          label="机场"
          count={airports.data?.length ?? 0}
          accent="var(--airport-accent)"
        />
        <TabButton
          active={tab === 'server'}
          onClick={() => setTab('server')}
          icon={<Server size={14} />}
          label="VPS"
          count={servers.data?.length ?? 0}
          accent="var(--server-accent)"
        />
      </div>

      {tab === 'airport' && (
        <section aria-label="机场列表">
          {airports.loading ? (
            <SkeletonRows />
          ) : airports.error ? (
            <ErrorNote>{airports.error.message}</ErrorNote>
          ) : (airports.data?.length ?? 0) === 0 ? (
            <EmptyState
              icon={<Plane size={18} />}
              title="还没有机场订阅"
              description="粘贴机场提供的订阅链接，SubS 会拉取并解析其中的节点。链接含凭据，将加密存储。"
              action={
                <button type="button" className="btn btn-primary" onClick={() => setAirportModal({ mode: 'create' })}>
                  <Plus size={14} /> 添加第一个机场
                </button>
              }
            />
          ) : (
            <ul className="space-y-2">
              {airports.data!.map((a) => (
                <li key={a.id} className="card group relative overflow-hidden">
                  <span
                    aria-hidden
                    className="absolute inset-y-0 left-0 w-[3px]"
                    style={{ background: 'var(--airport-accent)' }}
                  />
                  <div className="flex flex-wrap items-center gap-3 px-4 py-3 pl-4">
                    <Plane size={15} className="shrink-0 text-info" aria-hidden />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate text-[0.8125rem] font-medium text-fg">{a.name}</span>
                        {!a.enabled && <Badge tone="neutral">停用</Badge>}
                      </div>
                      <div className="mono mt-0.5 flex flex-wrap items-center gap-x-2 text-[0.6875rem] text-fg-muted">
                        <span>{a.nodeCount} 节点</span>
                        <span className="text-fg-subtle">·</span>
                        <span title={a.lastOkAt || ''}>更新 {relativeTime(a.lastOkAt)}</span>
                        {a.lastError && (
                          <>
                            <span className="text-fg-subtle">·</span>
                            <span className="truncate text-danger" title={a.lastError}>
                              {a.lastError}
                            </span>
                          </>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-1.5">
                      <button
                        type="button"
                        className="btn btn-ghost h-9 px-2.5"
                        onClick={() => refresh(a)}
                        disabled={refreshing === a.id}
                      >
                        {refreshing === a.id ? <Spinner /> : <RefreshCw size={13} />}
                        <span className="hidden sm:inline">刷新</span>
                      </button>
                      <IconButton label="编辑" onClick={() => setAirportModal({ mode: 'edit', airport: a })}>
                        <Pencil size={14} />
                      </IconButton>
                      <IconButton label="删除" danger onClick={() => removeAirport(a)}>
                        <Trash2 size={14} />
                      </IconButton>
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>
      )}

      {tab === 'server' && (
        <section aria-label="VPS 列表">
          {servers.loading ? (
            <SkeletonRows />
          ) : servers.error ? (
            <ErrorNote>{servers.error.message}</ErrorNote>
          ) : (servers.data?.length ?? 0) === 0 ? (
            <EmptyState
              icon={<Server size={18} />}
              title="还没有自建 VPS"
              description="先创建 VPS 分组，再导入该服务器上的节点链接（hy2 / vless / tuic / anytls / mieru）。"
              action={
                <button type="button" className="btn btn-primary" onClick={() => setServerModal({ mode: 'create' })}>
                  <Plus size={14} /> 添加第一个 VPS
                </button>
              }
            />
          ) : (
            <ul className="space-y-2">
              {servers.data!.map((v) => (
                <li key={v.id} className="card relative overflow-hidden">
                  <span
                    aria-hidden
                    className="absolute inset-y-0 left-0 w-[3px]"
                    style={{ background: 'var(--server-accent)' }}
                  />
                  <div className="flex flex-wrap items-center gap-3 px-4 py-3">
                    <Server size={15} className="shrink-0 text-warning" aria-hidden />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-[0.8125rem] font-medium text-fg">{v.name}</div>
                      <div className="mono mt-0.5 flex flex-wrap items-center gap-x-2 text-[0.6875rem] text-fg-muted">
                        {v.address && <span className="truncate">{v.address}</span>}
                        {v.address && <span className="text-fg-subtle">·</span>}
                        <span>{v.nodeCount} 节点</span>
                        {v.note && (
                          <>
                            <span className="text-fg-subtle">·</span>
                            <span className="truncate">{v.note}</span>
                          </>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-1.5">
                      <button
                        type="button"
                        className="btn btn-ghost h-9 px-2.5"
                        onClick={() => onNavigate(`/nodes?sourceType=server&sourceId=${v.id}&import=1`)}
                      >
                        <Plus size={13} />
                        <span className="hidden sm:inline">添加节点</span>
                      </button>
                      <IconButton label="编辑" onClick={() => setServerModal({ mode: 'edit', server: v })}>
                        <Pencil size={14} />
                      </IconButton>
                      <IconButton label="删除" danger onClick={() => removeServer(v)}>
                        <Trash2 size={14} />
                      </IconButton>
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>
      )}

      <AirportForm
        state={airportModal}
        onClose={() => setAirportModal(null)}
        onSaved={() => {
          setAirportModal(null)
          airports.reload()
        }}
      />
      <ServerForm
        state={serverModal}
        onClose={() => setServerModal(null)}
        onSaved={() => {
          setServerModal(null)
          servers.reload()
        }}
      />

      <RefreshDialog result={refreshResult} onClose={() => setRefreshResult(null)} onGoNodes={() => onNavigate('/nodes')} />

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
        <p className="text-[0.7812rem] text-fg-muted">此操作不可撤销，请确认后继续。</p>
      </Modal>
    </div>
  )
}

function TabButton({
  active,
  onClick,
  icon,
  label,
  count,
  accent,
}: {
  active: boolean
  onClick: () => void
  icon: React.ReactNode
  label: string
  count: number
  accent: string
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={`flex h-9 flex-1 items-center justify-center gap-2 rounded-md text-[0.7812rem] transition-colors ${
        active ? 'bg-raised text-fg' : 'text-fg-muted hover:text-fg-soft'
      }`}
    >
      <span style={{ color: active ? accent : undefined }}>{icon}</span>
      {label}
      <span className="mono text-[0.6875rem] text-fg-muted">{count}</span>
    </button>
  )
}

function IconButton({
  label,
  onClick,
  children,
  danger,
}: {
  label: string
  onClick: () => void
  children: React.ReactNode
  danger?: boolean
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      className={`flex h-9 w-9 items-center justify-center rounded border border-transparent transition-colors hover:border-line-strong hover:bg-raised/60 ${
        danger ? 'text-fg-muted hover:text-danger' : 'text-fg-muted hover:text-fg'
      }`}
    >
      {children}
    </button>
  )
}

function AirportForm({
  state,
  onClose,
  onSaved,
}: {
  state: { mode: 'create' } | { mode: 'edit'; airport: Airport } | null
  onClose: () => void
  onSaved: () => void
}) {
  const editing = state?.mode === 'edit' ? state.airport : undefined
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [note, setNote] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!state) return
    setName(editing?.name ?? '')
    setUrl(editing?.url ?? '')
    setNote(editing?.note ?? '')
    setEnabled(editing?.enabled ?? true)
    setError('')
  }, [state, editing])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      if (editing) {
        await api.put(`/api/airports/${editing.id}`, { name, url, note, enabled })
        toast('success', '机场已更新')
      } else {
        const created = await api.post<Airport>('/api/airports', { name, url, note })
        toast('success', '机场已创建，正在拉取节点…')
        try {
          const stats = await api.post<RefreshStats>(`/api/airports/${created.id}/refresh`)
          toast('success', `拉取完成：+${stats.added} 新增 · ${stats.matched} 匹配 · ${stats.missing} 缺失`)
        } catch (err) {
          toast('warning', err instanceof ApiError ? `已保存，但拉取失败：${err.message}` : '已保存，但拉取失败')
        }
      }
      onSaved()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={!!state}
      title={editing ? '编辑机场' : '添加机场'}
      description="订阅链接通常包含你的账号凭据，保存前会用实例主密钥加密。"
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            取消
          </button>
          <button type="submit" form="airport-form" className="btn btn-primary" disabled={busy}>
            {busy && <Spinner />}
            {editing ? '保存' : '创建并拉取'}
          </button>
        </>
      }
    >
      <form id="airport-form" onSubmit={submit} className="space-y-3.5">
        <label className="block space-y-1.5">
          <span className="text-[0.75rem] font-medium text-fg-soft">名称</span>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} required placeholder="例如：主力机场" />
        </label>
        <label className="block space-y-1.5">
          <span className="text-[0.75rem] font-medium text-fg-soft">订阅链接</span>
          <textarea
            className="input mono h-20 resize-y py-2 break-all"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            required
            placeholder="https://example.com/api/v1/client/subscribe?token=…"
          />
          <span className="block text-[0.7188rem] text-fg-muted">
            {editing?.url?.includes('***') ? '留空或保留遮蔽值表示不修改链接' : '地址含凭据，将加密存储'}
          </span>
        </label>
        <label className="block space-y-1.5">
          <span className="text-[0.75rem] font-medium text-fg-soft">备注（可选）</span>
          <input className="input" value={note} onChange={(e) => setNote(e.target.value)} />
        </label>
        {editing && (
          <label className="flex items-center gap-2 text-[0.7812rem] text-fg-soft">
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} className="h-4 w-4" />
            启用（停用后该来源节点不参与订阅输出）
          </label>
        )}
        {error && <ErrorNote>{error}</ErrorNote>}
      </form>
    </Modal>
  )
}

function ServerForm({
  state,
  onClose,
  onSaved,
}: {
  state: { mode: 'create' } | { mode: 'edit'; server: VpsServer } | null
  onClose: () => void
  onSaved: () => void
}) {
  const editing = state?.mode === 'edit' ? state.server : undefined
  const [name, setName] = useState('')
  const [address, setAddress] = useState('')
  const [note, setNote] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!state) return
    setName(editing?.name ?? '')
    setAddress(editing?.address ?? '')
    setNote(editing?.note ?? '')
    setError('')
  }, [state, editing])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      if (editing) {
        await api.put(`/api/servers/${editing.id}`, { name, address, note })
        toast('success', 'VPS 已更新')
      } else {
        await api.post('/api/servers', { name, address, note })
        toast('success', 'VPS 已创建')
      }
      onSaved()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={!!state}
      title={editing ? '编辑 VPS' : '添加 VPS'}
      description="VPS 是一个节点分组，用来归属你自建服务器上的节点。"
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            取消
          </button>
          <button type="submit" form="server-form" className="btn btn-primary" disabled={busy}>
            {busy && <Spinner />}
            保存
          </button>
        </>
      }
    >
      <form id="server-form" onSubmit={submit} className="space-y-3.5">
        <label className="block space-y-1.5">
          <span className="text-[0.75rem] font-medium text-fg-soft">分组名称</span>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} required placeholder="例如：东京 BGP" />
        </label>
        <label className="block space-y-1.5">
          <span className="text-[0.75rem] font-medium text-fg-soft">服务器地址（可选）</span>
          <input className="input mono" value={address} onChange={(e) => setAddress(e.target.value)} placeholder="jp.example.com" />
        </label>
        <label className="block space-y-1.5">
          <span className="text-[0.75rem] font-medium text-fg-soft">备注（可选）</span>
          <input className="input" value={note} onChange={(e) => setNote(e.target.value)} />
        </label>
        {error && <ErrorNote>{error}</ErrorNote>}
      </form>
    </Modal>
  )
}

function RefreshDialog({
  result,
  onClose,
  onGoNodes,
}: {
  result: { airport: Airport; stats: RefreshStats } | null
  onClose: () => void
  onGoNodes: () => void
}) {
  const errors = result?.stats.errors ?? []
  const summary = useMemo(() => {
    if (!result) return []
    const { stats } = result
    return [
      { label: '新增', value: stats.added, tone: 'amber' as const },
      { label: '匹配', value: stats.matched, tone: 'emerald' as const },
      { label: '缺失', value: stats.missing, tone: 'neutral' as const },
      { label: '忽略', value: stats.ignored, tone: 'neutral' as const },
    ]
  }, [result])

  return (
    <Modal
      open={!!result}
      title="刷新结果"
      description={result ? `${result.airport.name} · 已完成拉取` : undefined}
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            关闭
          </button>
          <button type="button" className="btn btn-primary" onClick={onGoNodes}>
            查看节点
          </button>
        </>
      }
    >
      <div className="grid grid-cols-4 gap-2">
        {summary.map((s) => (
          <div key={s.label} className="rounded-md border border-line bg-canvas/60 px-2 py-2.5 text-center">
            <div className="mono text-lg font-semibold text-fg">{s.value}</div>
            <div className="text-[0.6875rem] text-fg-muted">{s.label}</div>
          </div>
        ))}
      </div>
      {errors.length > 0 && (
        <div className="mt-3 space-y-1.5">
          <div className="flex items-center gap-1.5 text-[0.75rem] text-warning">
            <AlertTriangle size={12} /> {errors.length} 行解析失败（已跳过）
          </div>
          <ul className="mono max-h-40 space-y-1 overflow-y-auto rounded border border-line bg-canvas/60 p-2 text-[0.7188rem] text-fg-muted">
            {errors.slice(0, 50).map((e, i) => (
              <li key={i}>
                第 {e.line} 行：{e.reason}
              </li>
            ))}
          </ul>
        </div>
      )}
    </Modal>
  )
}

export { MoreHorizontal }
