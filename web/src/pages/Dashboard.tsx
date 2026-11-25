import {
  ArrowUpRight,
  Boxes,
  CheckCircle2,
  DatabaseBackup,
  HardDrive,
  Layers3,
  Plane,
  Plus,
  RefreshCw,
  Server,
  ShieldCheck,
  TriangleAlert,
} from 'lucide-react'
import { api, type Airport, type Profile, type SystemInfo, type VpsServer } from '../lib/api'
import { formatBytes, relativeTime, useAsync } from '../lib/hooks'
import { Badge, CopyButton, EmptyState, SectionLabel, SkeletonRows, Spinner } from '../components/ui'

export function DashboardPage({ onNavigate }: { onNavigate: (path: string) => void }) {
  const system = useAsync(() => api.get<SystemInfo>('/api/system'))
  const airports = useAsync(() => api.get<Airport[]>('/api/airports'))
  const servers = useAsync(() => api.get<VpsServer[]>('/api/servers'))
  const profiles = useAsync(() => api.get<Profile[]>('/api/profiles'))

  const loading = system.loading || airports.loading || servers.loading || profiles.loading
  const s = system.data
  const airportList = airports.data ?? []
  const serverList = servers.data ?? []
  const profileList = profiles.data ?? []
  const airportNodes = airportList.reduce((sum, a) => sum + a.nodeCount, 0)
  const serverNodes = serverList.reduce((sum, v) => sum + v.nodeCount, 0)
  const airportError = airportList.find((a) => a.lastError)

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <SectionLabel>workspace / overview</SectionLabel>
          <h1 className="mt-1 text-page font-semibold tracking-tight">运行概览</h1>
        </div>
        <div className="flex items-center gap-2 text-[0.7188rem] text-fg-muted">
          <CheckCircle2 size={13} className="text-success" />
          服务正常
          <span className="text-fg-subtle">·</span>
          版本 <code className="text-fg-muted">{s?.version || 'dev'}</code>
        </div>
      </header>

      {loading ? (
        <SkeletonRows rows={3} />
      ) : (
        <>
          <div className="grid gap-3 sm:grid-cols-2">
            <SourceCard
              kind="airport"
              count={airportList.length}
              nodes={airportNodes}
              status={
                airportError
                  ? { tone: 'red', text: airportError.lastError }
                  : airportList.length > 0
                    ? { tone: 'neutral', text: `最近更新 ${relativeTime(latest(airportList.map((a) => a.lastOkAt)))}` }
                    : undefined
              }
              empty="尚未添加机场订阅"
              actionLabel="添加机场"
              onAction={() => onNavigate('/sources')}
              onOpen={() => onNavigate('/sources')}
            />
            <SourceCard
              kind="server"
              count={serverList.length}
              nodes={serverNodes}
              status={
                serverList.length > 0
                  ? { tone: 'neutral', text: `${serverNodes} 个自建节点` }
                  : undefined
              }
              empty="尚未添加自建 VPS"
              actionLabel="添加 VPS"
              onAction={() => onNavigate('/sources')}
              onOpen={() => onNavigate('/sources')}
            />
          </div>

          {(s?.newNodes ?? 0) > 0 && (
            <button
              type="button"
              onClick={() => onNavigate('/nodes?isNew=1')}
              className="flex w-full items-center gap-2.5 rounded-lg border border-warning/30 bg-warning/5 px-3.5 py-2.5 text-left text-[0.7812rem] text-warning transition-colors hover:border-warning/50"
            >
              <TriangleAlert size={14} className="shrink-0" />
              <span>
                有 <b className="mono">{s?.newNodes}</b> 个新节点待确认
                {(s?.missingNodes ?? 0) > 0 && ` · ${s?.missingNodes} 个节点已从上游消失`}
              </span>
              <ArrowUpRight size={14} className="ml-auto shrink-0" />
            </button>
          )}

          <section className="card overflow-hidden">
            <div className="flex items-center justify-between border-b border-line px-4 py-3">
              <div className="flex items-center gap-2">
                <ShieldCheck size={14} className="text-accent" />
                <h2 className="text-[0.8125rem] font-semibold">订阅编排</h2>
                <span className="mono text-[0.6875rem] text-fg-muted">{profileList.length}</span>
              </div>
              <button
                type="button"
                onClick={() => onNavigate('/profiles')}
                className="flex items-center gap-1 text-[0.75rem] text-accent hover:text-accent-strong"
              >
                管理 <ArrowUpRight size={12} />
              </button>
            </div>

            {profileList.length === 0 ? (
              <div className="p-4">
                <EmptyState
                  icon={<ShieldCheck size={18} />}
                  title="还没有订阅编排"
                  description="从机场或自建节点里挑选节点，套用模板后生成客户端订阅链接。"
                  action={
                    <button type="button" className="btn btn-primary" onClick={() => onNavigate('/profiles')}>
                      <Plus size={14} /> 新建编排
                    </button>
                  }
                />
              </div>
            ) : (
              <ul className="divide-y divide-line/70">
                {profileList.map((p) => (
                  <li key={p.id}>
                    <button
                      type="button"
                      onClick={() => onNavigate(`/profiles/${p.id}`)}
                      className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-raised/30"
                    >
                      <span
                        className={`h-1.5 w-1.5 shrink-0 rounded-full ${
                          p.enabled ? 'bg-success' : 'bg-line-strong'
                        }`}
                        aria-hidden
                      />
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-[0.8125rem] text-fg">{p.name}</div>
                        <div className="mono mt-0.5 flex flex-wrap items-center gap-x-2 text-[0.6875rem] text-fg-muted">
                          <span>{p.nodeCount} 节点</span>
                          <span className="text-fg-subtle">·</span>
                          <span>{p.templateName || '无模板'}</span>
                          <span className="text-fg-subtle">·</span>
                          <span>拉取 {relativeTime(p.lastFetchAt)}</span>
                        </div>
                      </div>
                      {p.enabled ? (
                        <Badge tone="emerald">启用</Badge>
                      ) : (
                        <Badge tone="neutral">停用</Badge>
                      )}
                      {p.url && <CopyButton value={p.url} label="复制订阅链接" />}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <div className="grid gap-3 sm:grid-cols-2">
            <div className="card p-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <HardDrive size={14} className="text-fg-muted" />
                  <h2 className="text-[0.8125rem] font-semibold">实例状态</h2>
                </div>
                <button
                  type="button"
                  onClick={() => onNavigate('/settings')}
                  aria-label="打开设置"
                  className="text-fg-muted hover:text-fg-soft"
                >
                  <ArrowUpRight size={14} />
                </button>
              </div>
              <dl className="mt-3 space-y-2 text-[0.75rem]">
                <Row label="实例名" value={<code>{s?.instanceName || 'my-subs'}</code>} />
                <Row
                  label="监听"
                  value={
                    <code>
                      {s?.listenHost || '127.0.0.1'}:{s?.listenPort ?? 8787}
                    </code>
                  }
                />
                <Row label="数据库" value={<code>{formatBytes(s?.diskUsedBytes ?? 0)}</code>} />
                <Row
                  label="节点总数"
                  value={<code>{s?.nodes ?? 0}</code>}
                />
              </dl>
            </div>

            <div className="card p-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <DatabaseBackup size={14} className="text-fg-muted" />
                  <h2 className="text-[0.8125rem] font-semibold">备份</h2>
                </div>
                <button
                  type="button"
                  onClick={() => onNavigate('/backup')}
                  aria-label="打开备份"
                  className="text-fg-muted hover:text-fg-soft"
                >
                  <ArrowUpRight size={14} />
                </button>
              </div>
              <p className="mt-3 text-[0.7812rem] text-fg-muted">
                上次备份：<span className="mono">{relativeTime(s?.lastBackupAt)}</span>
              </p>
              {needsBackup(s?.lastBackupAt) && (
                <p className="mt-2 flex items-start gap-1.5 rounded border border-warning/30 bg-warning/5 px-2.5 py-2 text-[0.75rem] text-warning">
                  <TriangleAlert size={13} className="mt-0.5 shrink-0" />
                  已超过 30 天未备份，建议导出一份加密备份。
                </p>
              )}
              <button
                type="button"
                className="btn btn-ghost mt-3 w-full"
                onClick={() => onNavigate('/backup')}
              >
                <DatabaseBackup size={14} /> 创建备份
              </button>
            </div>
          </div>
        </>
      )}

      <p className="flex items-center justify-center gap-1.5 pt-2 text-[0.6875rem] text-fg-subtle">
        <ShieldCheck size={11} /> 凭据仅存储在此实例 · 无遥测
      </p>
    </div>
  )
}

function latest(values: string[]): string | undefined {
  const times = values
    .map((v) => (v ? new Date(v).getTime() : 0))
    .filter((n) => n > 0)
  if (!times.length) return undefined
  return new Date(Math.max(...times)).toISOString()
}

function needsBackup(iso?: string): boolean {
  if (!iso) return true
  return Date.now() - new Date(iso).getTime() > 30 * 24 * 3600 * 1000
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <dt className="text-fg-muted">{label}</dt>
      <dd className="mono truncate text-fg-soft">{value}</dd>
    </div>
  )
}

function SourceCard({
  kind,
  count,
  nodes,
  status,
  empty,
  actionLabel,
  onAction,
  onOpen,
}: {
  kind: 'airport' | 'server'
  count: number
  nodes: number
  status?: { tone: 'neutral' | 'red'; text: string }
  empty: string
  actionLabel: string
  onAction: () => void
  onOpen: () => void
}) {
  const isAirport = kind === 'airport'
  return (
    <article className="card relative overflow-hidden p-4">
      <span
        aria-hidden
        className="absolute inset-y-0 left-0 w-[3px]"
        style={{ background: isAirport ? 'var(--airport-accent)' : 'var(--server-accent)' }}
      />
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-2">
          <span
            className={`flex h-7 w-7 items-center justify-center rounded border ${
              isAirport
                ? 'border-info/25 bg-info/5 text-info'
                : 'border-warning/25 bg-warning/5 text-warning'
            }`}
          >
            {isAirport ? <Plane size={14} /> : <Server size={14} />}
          </span>
          <SectionLabel>{isAirport ? 'airports' : 'self-hosted'}</SectionLabel>
        </div>
        <button
          type="button"
          onClick={onOpen}
          aria-label={isAirport ? '管理机场' : '管理 VPS'}
          className="text-fg-muted transition-colors hover:text-fg-soft"
        >
          <ArrowUpRight size={14} />
        </button>
      </div>

      <div className="mt-3 flex items-baseline gap-2">
        <span className="mono text-2xl font-semibold tracking-tight text-fg">{count}</span>
        <span className="text-[0.75rem] text-fg-muted">{isAirport ? '机场订阅' : '自建 VPS'}</span>
        {count > 0 && (
          <span className="mono ml-auto text-[0.6875rem] text-fg-muted">
            <Boxes size={11} className="mr-1 inline" />
            {nodes} 节点
          </span>
        )}
      </div>

      <div className="mt-3 border-t border-line/70 pt-2.5">
        {status ? (
          <p
            className={`truncate text-[0.7188rem] ${
              status.tone === 'red' ? 'text-danger' : 'text-fg-muted'
            }`}
            title={status.text}
          >
            {status.tone === 'red' && <TriangleAlert size={11} className="mr-1 inline" />}
            {status.text}
          </p>
        ) : (
          <p className="text-[0.7188rem] text-fg-subtle">{empty}</p>
        )}
        <button
          type="button"
          onClick={onAction}
          className="mt-2 inline-flex items-center gap-1 text-[0.75rem] text-accent hover:text-accent-strong"
        >
          <Plus size={12} /> {actionLabel}
        </button>
      </div>
    </article>
  )
}

export { RefreshCw, Spinner, Layers3 }
