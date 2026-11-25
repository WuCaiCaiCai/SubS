import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  ArrowLeft,
  ArrowRight,
  Check,
  ChevronDown,
  ChevronRight,
  ChevronUp,
  GripVertical,
  Layers3,
  ListOrdered,
  Minus,
  Plane,
  Search,
  Server,
  ShieldCheck,
  Sparkles,
  Trash2,
} from 'lucide-react'
import {
  api,
  ApiError,
  type NodeItem,
  type Profile,
  type ProfileItem,
  type ProfileNode,
  type Template,
} from '../lib/api'
import { relativeTime, toast, useAsync } from '../lib/hooks'
import {
  Badge,
  CopyButton,
  EmptyState,
  ErrorNote,
  ProtocolBadge,
  SectionLabel,
  SkeletonRows,
  Spinner,
} from '../components/ui'
import { CodeEditor } from '../components/CodeEditor'
import { SubscriptionOutput } from '../components/SubscriptionOutput'

const STEPS = [
  { key: 'select', label: '选择节点', icon: ListOrdered },
  { key: 'order', label: '排序与命名', icon: GripVertical },
  { key: 'publish', label: '模板与发布', icon: ShieldCheck },
] as const

type Step = 0 | 1 | 2

export function ProfileEditorPage({
  profileId,
  onNavigate,
}: {
  profileId: number
  onNavigate: (path: string) => void
}) {
  const profile = useAsync(() => api.get<Profile>(`/api/profiles/${profileId}`))
  const templates = useAsync(() => api.get<Template[]>('/api/templates'))
  const [step, setStep] = useState<Step>(0)
  const [items, setItems] = useState<ProfileItem[]>([])
  const [overrides, setOverrides] = useState<Record<number, string>>({})
  const [profileNodes, setProfileNodes] = useState<ProfileNode[]>([])
  const [duplicates, setDuplicates] = useState<string[]>([])
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

  const loadNaming = useCallback(async () => {
    try {
      const res = await api.get<{ nodes: ProfileNode[]; duplicates: string[] }>(
        `/api/profiles/${profileId}/naming-preview`,
      )
      setProfileNodes(res.nodes)
      setDuplicates(res.duplicates)
    } catch {
      /* ignore */
    }
  }, [profileId])

  const loadSelection = useCallback(async () => {
    try {
      const res = await api.get<ProfileItem[]>(`/api/profiles/${profileId}/items`)
      setItems(res ?? [])
      const nodes = await api.get<{ nodes: ProfileNode[]; duplicates: string[] }>(
        `/api/profiles/${profileId}/naming-preview`,
      )
      setProfileNodes(nodes.nodes ?? [])
      const map: Record<number, string> = {}
      for (const it of res ?? []) {
        if (it.kind === 'node' && it.nameOverride) map[it.refId] = it.nameOverride
      }
      setOverrides(map)
    } catch {
      /* ignore */
    }
  }, [profileId])

  useEffect(() => {
    void loadSelection()
  }, [loadSelection])

  const saveItems = async (list: ProfileItem[], ov?: Record<number, string>) => {
    setError('')
    try {
      const res = await api.put<ProfileNode[]>(`/api/profiles/${profileId}/nodes`, {
        items: list.map((it) => ({
          kind: it.kind,
          refId: it.refId,
          sourceType: it.sourceType,
          nameOverride: it.kind === 'node' ? (ov?.[it.refId] ?? overrides[it.refId] ?? '') : '',
        })),
      })
      setProfileNodes(res)
      setSaved(true)
      window.setTimeout(() => setSaved(false), 1800)
      void loadNaming()
      return true
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '保存失败')
      return false
    }
  }

  if (profile.loading) return <SkeletonRows rows={5} />
  if (profile.error) return <ErrorNote>{profile.error.message}</ErrorNote>
  const p = profile.data!

  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <button
            type="button"
            onClick={() => onNavigate('/profiles')}
            className="mb-1.5 inline-flex items-center gap-1 text-[0.75rem] text-fg-muted hover:text-fg-soft"
          >
            <ArrowLeft size={12} /> 返回编排列表
          </button>
          <SectionLabel>profile / {p.id}</SectionLabel>
          <h1 className="mt-1 truncate text-page font-semibold tracking-tight">{p.name}</h1>
        </div>
        <div className="flex items-center gap-2">
          {saved && (
            <span className="flex items-center gap-1 text-[0.75rem] text-success">
              <Check size={13} /> 已保存
            </span>
          )}
          <ProfileMeta profile={p} />
        </div>
      </header>

      <ol className="flex items-center gap-1 rounded-lg border border-line bg-surface/40 p-1">
        {STEPS.map((s, i) => {
          const active = step === i
          const done = step > i
          return (
            <li key={s.key} className="flex flex-1 items-center">
              <button
                type="button"
                onClick={() => setStep(i as Step)}
                aria-current={active ? 'step' : undefined}
                className={`flex h-9 flex-1 items-center justify-center gap-2 rounded-md text-[0.7812rem] transition-colors ${
                  active ? 'bg-raised text-fg' : 'text-fg-muted hover:text-fg-soft'
                }`}
              >
                <s.icon size={13} className={active ? 'text-accent' : ''} />
                <span className="hidden sm:inline">{s.label}</span>
                <span className="mono text-[0.6562rem] text-fg-subtle">{i + 1}</span>
                {done && <Check size={12} className="text-success" />}
              </button>
              {i < STEPS.length - 1 && <span className="mx-1 h-px w-3 bg-raised sm:w-6" aria-hidden />}
            </li>
          )
        })}
      </ol>

      {error && <ErrorNote>{error}</ErrorNote>}

      {step === 0 && (
        <NodeSelectStep
          items={items}
          onChange={setItems}
          onNext={async () => {
            const ok = await saveItems(items)
            if (ok) setStep(1)
          }}
        />
      )}

      {step === 1 && (
        <NodeOrderStep
          profileId={profileId}
          nodes={profileNodes}
          items={items}
          overrides={overrides}
          duplicates={duplicates}
          onOverrides={setOverrides}
          onReorder={setItems}
          onRuleSaved={() => {
            profile.reload()
            void loadNaming()
          }}
          onSave={async () => {
            const ok = await saveItems(items, overrides)
            if (ok) {
              toast('success', '顺序与命名已保存')
              setStep(2)
            }
          }}
        />
      )}

      {step === 2 && (
        <PublishStep
          profile={p}
          templates={templates.data ?? []}
          onSaved={() => profile.reload()}
          onBack={() => setStep(1)}
        />
      )}
    </div>
  )
}

function ProfileMeta({ profile }: { profile: Profile }) {
  return (
    <div className="flex items-center gap-2">
      <Badge tone={profile.enabled ? 'emerald' : 'neutral'}>{profile.enabled ? '启用中' : '已停用'}</Badge>
      <span className="mono text-[0.6875rem] text-fg-muted">拉取 {relativeTime(profile.lastFetchAt)}</span>
    </div>
  )
}

function NodeSelectStep({
  items,
  onChange,
  onNext,
}: {
  items: ProfileItem[]
  onChange: (items: ProfileItem[]) => void
  onNext: () => void
}) {
  const nodes = useAsync(() => api.get<NodeItem[]>('/api/nodes'))
  const [q, setQ] = useState('')
  const [open, setOpen] = useState<Record<string, boolean>>({})
  const [busy, setBusy] = useState(false)

  const key = (n: { sourceType: string; sourceId: number }) => `${n.sourceType}:${n.sourceId}`
  const sourceItem = (k: string) =>
    items.find((it) => it.kind === 'source' && `${it.sourceType}:${it.refId}` === k)
  const nodeItem = (id: number) => items.find((it) => it.kind === 'node' && it.refId === id)

  const groups = useMemo(() => {
    const k = q.trim().toLowerCase()
    const map = new Map<
      string,
      { key: string; label: string; type: 'airport' | 'server'; id: number; nodes: NodeItem[] }
    >()
    for (const n of nodes.data ?? []) {
      const gk = key(n)
      const hit =
        !k || n.rawName.toLowerCase().includes(k) || n.host.toLowerCase().includes(k) || n.sourceName.toLowerCase().includes(k)
      if (!hit) continue
      const g = map.get(gk) ?? {
        key: gk,
        label: n.sourceName,
        type: n.sourceType,
        id: n.sourceId,
        nodes: [],
      }
      g.nodes.push(n)
      map.set(gk, g)
    }
    return [...map.values()]
  }, [nodes.data, q])

  /** 一个节点是否被整源覆盖。 */
  const covered = (n: NodeItem) => Boolean(sourceItem(key(n)))
  /** 整源的三态：全选 / 部分 / 未选。 */
  const sourceState = (g: { key: string; nodes: NodeItem[] }) => {
    if (sourceItem(g.key)) return 'all' as const
    const picked = g.nodes.filter((n) => nodeItem(n.id)).length
    if (picked === 0) return 'none' as const
    return picked === g.nodes.length ? 'all-nodes' as const : 'some' as const
  }

  const toggleSource = (g: { key: string; label: string; type: 'airport' | 'server'; id: number; nodes: NodeItem[] }) => {
    if (sourceItem(g.key)) {
      // 取消整源：同时丢掉这些节点的单点选择，语义才符合“都不要”
      const ids = new Set(g.nodes.map((n) => n.id))
      onChange(items.filter((it) => !(it.kind === 'source' && `${it.sourceType}:${it.refId}` === g.key) && !(it.kind === 'node' && ids.has(it.refId))))
      return
    }
    // 选中整源时丢掉该来源下已单点选中的节点，避免重复计数与冗余条目
    const ids = new Set(g.nodes.map((n) => n.id))
    onChange([
      ...items.filter(
        (it) =>
          !(it.kind === 'source' && `${it.sourceType}:${it.refId}` === g.key) &&
          !(it.kind === 'node' && ids.has(it.refId)),
      ),
      { kind: 'source', refId: g.id, sourceType: g.type, nameOverride: '', sortOrder: items.length, label: g.label, protocol: '', nodeCount: g.nodes.length },
    ])
  }

  const toggleNode = (n: NodeItem) => {
    if (covered(n)) return
    onChange(
      nodeItem(n.id)
        ? items.filter((it) => !(it.kind === 'node' && it.refId === n.id))
        : [
            ...items,
            {
              kind: 'node',
              refId: n.id,
              sourceType: '',
              nameOverride: '',
              sortOrder: items.length,
              label: n.rawName,
              protocol: n.protocol,
              nodeCount: 1,
            },
          ],
    )
  }

  const selectedCount = useMemo(() => {
    const coveredIds = new Set<number>()
    for (const it of items) {
      if (it.kind !== 'source') continue
      for (const n of nodes.data ?? []) {
        if (key(n) === `${it.sourceType}:${it.refId}`) coveredIds.add(n.id)
      }
    }
    const explicit = items.filter((it) => it.kind === 'node' && !coveredIds.has(it.refId)).length
    return coveredIds.size + explicit
  }, [items, nodes.data])
  const sourceCount = items.filter((it) => it.kind === 'source').length

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-0 flex-1">
          <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-fg-muted" />
          <input
            className="input pl-8"
            placeholder="搜索来源、节点名或地址…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
        </div>
        <div className="flex items-center gap-1.5">
          <button
            type="button"
            className="btn btn-ghost h-10"
            onClick={() => setOpen(Object.fromEntries(groups.map((g) => [g.key, true])))}
          >
            全部展开
          </button>
          <button type="button" className="btn btn-ghost h-10" onClick={() => setOpen({})}>
            收起
          </button>
        </div>
      </div>

      {nodes.loading ? (
        <SkeletonRows rows={6} />
      ) : nodes.error ? (
        <ErrorNote>{nodes.error.message}</ErrorNote>
      ) : groups.length === 0 ? (
        <EmptyState
          icon={<Sparkles size={18} />}
          title={q ? '没有匹配的节点' : '没有可选择的节点'}
          description={q ? '换个关键词，或清空搜索看全部来源。' : '先到「来源」添加机场并刷新，或为自建 VPS 导入节点。'}
        />
      ) : (
        <ul className="divide-y divide-line/70 overflow-hidden rounded-lg border border-line">
          {groups.map((g) => {
            const state = sourceState(g)
            const expanded = open[g.key] ?? Boolean(q.trim())
            const partial = state === 'some'
            const all = state === 'all' || state === 'all-nodes'
            return (
              <li key={g.key}>
                <div className="flex items-center gap-2.5 bg-surface/40 px-3 py-2.5">
                  <button
                    type="button"
                    role="checkbox"
                    aria-checked={partial ? 'mixed' : all}
                    aria-label={`整源选择 ${g.label}`}
                    onClick={() => toggleSource(g)}
                    className={`flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-colors ${
                      all || partial ? 'border-accent bg-accent text-canvas' : 'border-line-strong hover:border-fg-subtle'
                    }`}
                  >
                    {state === 'all' ? <Check size={11} strokeWidth={3} /> : partial ? <Minus size={11} strokeWidth={3} /> : null}
                  </button>
                  <button
                    type="button"
                    onClick={() => setOpen({ ...open, [g.key]: !expanded })}
                    aria-expanded={expanded}
                    className="flex min-w-0 flex-1 items-center gap-2 text-left"
                  >
                    {expanded ? (
                      <ChevronDown size={13} className="shrink-0 text-fg-subtle" />
                    ) : (
                      <ChevronRight size={13} className="shrink-0 text-fg-subtle" />
                    )}
                    <span
                      aria-hidden
                      className={`h-1.5 w-1.5 shrink-0 rounded-full ${
                        g.type === 'airport' ? 'bg-[var(--airport-accent)]' : 'bg-[var(--server-accent)]'
                      }`}
                    />
                    <span className="truncate text-strong font-medium text-fg">{g.label}</span>
                    <span className="mono shrink-0 text-meta text-fg-subtle">
                      {g.type === 'airport' ? '机场' : 'VPS'} · {g.nodes.length}
                    </span>
                    {state === 'all' && <Badge tone="cyan">整源</Badge>}
                  </button>
                  <button
                    type="button"
                    onClick={() => toggleSource(g)}
                    className="shrink-0 text-meta text-accent-strong hover:underline"
                  >
                    {state === 'all' ? '取消整源' : '整源全选'}
                  </button>
                </div>
                {expanded && (
                  <ul className="divide-y divide-line/50">
                    {g.nodes.map((n) => {
                      const checked = Boolean(nodeItem(n.id)) || covered(n)
                      const locked = covered(n)
                      return (
                        <li key={n.id} className="flex items-center gap-2.5 py-2 pl-9 pr-3">
                          <input
                            type="checkbox"
                            aria-label={`选择 ${n.rawName}`}
                            checked={checked}
                            disabled={locked || n.missing || !n.enabled}
                            onChange={() => toggleNode(n)}
                            className="h-4 w-4 shrink-0"
                          />
                          <div className="min-w-0 flex-1">
                            <div className="mono truncate text-[0.8125rem] text-fg-soft">{n.rawName}</div>
                            <div className="mt-0.5 flex items-center gap-2">
                              <ProtocolBadge protocol={n.protocol} />
                              <span className="mono truncate text-meta text-fg-muted">
                                {n.host}:{n.portRange || n.port}
                              </span>
                            </div>
                          </div>
                          {locked && <span className="shrink-0 text-meta text-fg-subtle">随整源</span>}
                          {n.missing && <Badge tone="neutral">缺失</Badge>}
                          {!n.enabled && <Badge tone="neutral">停用</Badge>}
                        </li>
                      )
                    })}
                  </ul>
                )}
              </li>
            )
          })}
        </ul>
      )}

      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-line pt-3">
        <span className="mono text-[0.75rem] text-fg-muted">
          已选 {selectedCount} 个节点{sourceCount > 0 ? ` · ${sourceCount} 个整源` : ''}
        </span>
        <button
          type="button"
          className="btn btn-primary"
          disabled={items.length === 0}
          onClick={async () => {
            setBusy(true)
            await onNext()
            setBusy(false)
          }}
        >
          {busy ? <Spinner /> : null}
          下一步：排序与命名 <ArrowRight size={14} />
        </button>
      </div>
    </section>
  )
}

function NodeOrderStep({
  profileId,
  items,
  nodes,
  overrides,
  duplicates,
  onOverrides,
  onReorder,
  onSave,
  onRuleSaved,
}: {
  profileId: number
  items: ProfileItem[]
  nodes: ProfileNode[]
  overrides: Record<number, string>
  duplicates: string[]
  onOverrides: (v: Record<number, string>) => void
  onReorder: (items: ProfileItem[]) => void
  onSave: () => void
  onRuleSaved: () => void
}) {
  const [rule, setRule] = useState({ segments: 'index,region,protocol,rawName', separator: '-', zeroPad: 2, upperProtocol: true })
  const [savingRule, setSavingRule] = useState(false)

  // 展开成展示行：整源一行块头 + 其下节点，单节点各一行；排序只动 items。
  const rows = useMemo(() => {
    const byId = new Map(nodes.map((n) => [n.nodeId, n]))
    const bySource = new Map<string, ProfileNode[]>()
    for (const n of nodes) {
      const k = `${n.sourceType}:${n.sourceId}`
      const list = bySource.get(k) ?? []
      list.push(n)
      bySource.set(k, list)
    }
    const out: { type: 'item' | 'child'; item: ProfileItem; node?: ProfileNode; index: number }[] = []
    items.forEach((it, index) => {
      if (it.kind === 'node') {
        const n = byId.get(it.refId)
        if (n) out.push({ type: 'item', item: it, node: n, index })
        return
      }
      out.push({ type: 'item', item: it, index })
      for (const n of bySource.get(`${it.sourceType}:${it.refId}`) ?? []) {
        out.push({ type: 'child', item: it, node: n, index })
      }
    })
    return out
  }, [items, nodes])

  const move = (index: number, dir: -1 | 1) => {
    const target = index + dir
    if (target < 0 || target >= items.length) return
    const next = [...items]
    ;[next[index], next[target]] = [next[target], next[index]]
    onReorder(next)
  }

  const applyRule = async () => {
    setSavingRule(true)
    try {
      const segments = rule.segments
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
      await api.put(`/api/profiles/${profileId}`, {
        namingRule: { segments, separator: rule.separator, zeroPad: rule.zeroPad, upperProtocol: rule.upperProtocol },
      })
      toast('success', '命名规则已更新')
      onRuleSaved()
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '保存命名规则失败')
    } finally {
      setSavingRule(false)
    }
  }

  const explicit = items.filter((it) => it.kind === 'node')
  let seq = 0

  return (
    <section className="grid gap-4 lg:grid-cols-[1fr_280px]">
      <div className="space-y-2">
        {items.length === 0 ? (
          <EmptyState icon={<ListOrdered size={18} />} title="还没有节点" description="回到第 1 步挑选节点。" />
        ) : (
          <ul className="divide-y divide-line/70 overflow-hidden rounded-lg border border-line">
            {rows.map((row) => {
              if (row.type === 'child') {
                seq += 1
                const n = row.node!
                return (
                  <li key={`${row.item.kind}-${row.item.refId}-${n.nodeId}`} className="flex items-center gap-2 py-2 pl-11 pr-2.5">
                    <span className="mono w-6 shrink-0 text-center text-meta text-fg-subtle">
                      {String(seq).padStart(2, '0')}
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="mono truncate text-[0.75rem] text-fg-soft">{n.rawName}</div>
                      <div className="mt-0.5 flex items-center gap-1.5">
                        <span className="mono truncate text-[0.75rem] text-accent-strong/90">{n.outputName}</span>
                        {duplicates.includes(n.outputName) && <Badge tone="amber">重名</Badge>}
                      </div>
                    </div>
                  </li>
                )
              }
              const isSource = row.item.kind === 'source'
              if (!isSource) seq += 1
              return (
                <li key={`${row.item.kind}-${row.item.refId}`} className={`flex items-center gap-2 px-2.5 py-2 ${isSource ? 'bg-surface/40' : ''}`}>
                  {isSource ? (
                    <Layers3 size={13} className="ml-1 shrink-0 text-accent" />
                  ) : (
                    <span className="mono w-6 shrink-0 text-center text-meta text-fg-subtle">
                      {String(seq).padStart(2, '0')}
                    </span>
                  )}
                  <div className="min-w-0 flex-1">
                    <div className={`truncate ${isSource ? 'text-strong font-medium text-fg' : 'mono text-[0.75rem] text-fg-soft'}`}>
                      {isSource ? row.item.label : row.node?.rawName}
                    </div>
                    {!isSource && (
                      <div className="mt-0.5 flex items-center gap-1.5">
                        <span className="mono truncate text-[0.75rem] text-accent-strong/90">{row.node?.outputName}</span>
                        {row.node && duplicates.includes(row.node.outputName) && <Badge tone="amber">重名</Badge>}
                      </div>
                    )}
                  </div>
                  {isSource && <Badge tone="cyan">整源 {row.item.nodeCount || ''}</Badge>}
                  <div className="flex shrink-0 items-center gap-0.5">
                    <button
                      type="button"
                      aria-label={isSource ? `整源上移 ${row.item.label}` : '上移'}
                      disabled={row.index === 0}
                      onClick={() => move(row.index, -1)}
                      className="flex h-7 w-7 items-center justify-center rounded text-fg-subtle hover:bg-raised hover:text-fg disabled:opacity-30"
                    >
                      <ChevronUp size={13} />
                    </button>
                    <button
                      type="button"
                      aria-label={isSource ? `整源下移 ${row.item.label}` : '下移'}
                      disabled={row.index === items.length - 1}
                      onClick={() => move(row.index, 1)}
                      className="flex h-7 w-7 items-center justify-center rounded text-fg-subtle hover:bg-raised hover:text-fg disabled:opacity-30"
                    >
                      <ChevronDown size={13} />
                    </button>
                  </div>
                </li>
              )
            })}
          </ul>
        )}
        <div className="flex items-center justify-end">
          <button type="button" className="btn btn-primary" onClick={onSave}>
            保存并继续 <ArrowRight size={14} />
          </button>
        </div>
      </div>

      <aside className="space-y-3">
        <div className="card space-y-3 p-3.5">
          <SectionLabel>命名规则</SectionLabel>
          <label className="block space-y-1">
            <span className="text-[0.7188rem] text-fg-muted">片段顺序</span>
            <input
              className="input mono h-9 text-[0.75rem]"
              value={rule.segments}
              onChange={(e) => setRule({ ...rule, segments: e.target.value })}
            />
            <span className="block text-[0.6562rem] text-fg-subtle">可选：index / region / protocol / rawName</span>
          </label>
          <label className="block space-y-1">
            <span className="text-[0.7188rem] text-fg-muted">分隔符</span>
            <input
              className="input mono h-9 text-[0.75rem]"
              value={rule.separator}
              onChange={(e) => setRule({ ...rule, separator: e.target.value })}
            />
          </label>
          <label className="block space-y-1">
            <span className="text-[0.7188rem] text-fg-muted">序号位数</span>
            <input
              type="number"
              min={1}
              max={4}
              className="input mono h-9 text-[0.75rem]"
              value={rule.zeroPad}
              onChange={(e) => setRule({ ...rule, zeroPad: Number(e.target.value) })}
            />
          </label>
          <label className="flex items-center gap-2 text-[0.7188rem] text-fg-soft">
            <input
              type="checkbox"
              checked={rule.upperProtocol}
              onChange={(e) => setRule({ ...rule, upperProtocol: e.target.checked })}
              className="h-4 w-4"
            />
            协议名大写
          </label>
          <button type="button" className="btn btn-ghost w-full" onClick={applyRule} disabled={savingRule}>
            {savingRule ? <Spinner /> : null} 应用规则
          </button>
        </div>

        <div className="card p-3.5">
          <SectionLabel>单节点改名</SectionLabel>
          <p className="mt-1 text-[0.6875rem] text-fg-muted">
            只对逐个挑选的节点生效；整源里的节点沿用自动命名。
          </p>
          <div className="mt-2 space-y-1.5">
            {explicit.length === 0 && <p className="text-[0.6875rem] text-fg-subtle">当前没有单独挑选的节点。</p>}
            {explicit.slice(0, 8).map((it) => {
              const n = nodes.find((x) => x.nodeId === it.refId)
              return (
                <div key={it.refId} className="space-y-1">
                  <span className="mono block truncate text-[0.6562rem] text-fg-muted">{n?.rawName ?? it.label}</span>
                  <input
                    className="input mono h-8 text-[0.7188rem]"
                    placeholder={n?.outputName ?? ''}
                    value={overrides[it.refId] ?? ''}
                    onChange={(e) => onOverrides({ ...overrides, [it.refId]: e.target.value })}
                  />
                </div>
              )
            })}
          </div>
        </div>
      </aside>
    </section>
  )
}

function PublishStep({
  profile,
  templates,
  onSaved,
  onBack,
}: {
  profile: Profile
  templates: Template[]
  onSaved: () => void
  onBack: () => void
}) {
  const [templateId, setTemplateId] = useState(profile.templateId)
  const [enabled, setEnabled] = useState(profile.enabled)
  const [target, setTarget] = useState<'mihomo' | 'sing-box' | 'loon'>('mihomo')
  const [preview, setPreview] = useState<{ content: string; skipped: string[] } | null>(null)
  const [busy, setBusy] = useState(false)
  const [rotating, setRotating] = useState(false)

  useEffect(() => {
    setTemplateId(profile.templateId)
    setEnabled(profile.enabled)
  }, [profile])

  const save = async () => {
    setBusy(true)
    try {
      await api.put(`/api/profiles/${profile.id}`, {
        name: profile.name,
        note: profile.note,
        templateId,
        enabled,
        namingRule: profile.namingRule,
      })
      toast('success', '编排已保存')
      onSaved()
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  const runPreview = async () => {
    setBusy(true)
    try {
      const res = await api.post<{ target: string; content: string; skipped: string[] }>(
        `/api/profiles/${profile.id}/preview`,
        { target },
      )
      setPreview(res)
      if (res.skipped.length) {
        toast('warning', `${res.skipped.length} 个节点被跳过（该客户端不支持对应协议）`)
      }
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '预览失败')
    } finally {
      setBusy(false)
    }
  }

  const rotate = async () => {
    setRotating(true)
    try {
      await api.post(`/api/profiles/${profile.id}/rotate-token`)
      toast('success', '订阅令牌已轮换，旧链接立即失效')
      onSaved()
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '轮换失败')
    } finally {
      setRotating(false)
    }
  }

  return (
    <section className="grid gap-4 lg:grid-cols-[1fr_320px]">
      <div className="space-y-3">
        <div className="card space-y-3.5 p-4">
          <div className="flex items-center gap-2">
            <Layers3 size={14} className="text-accent" />
            <h2 className="text-[0.8125rem] font-semibold">模板与状态</h2>
          </div>

          <label className="block space-y-1.5">
            <span className="text-[0.75rem] font-medium text-fg-soft">Clash 模板</span>
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
          </label>

          <label className="flex items-center gap-2 text-[0.7812rem] text-fg-soft">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="h-4 w-4"
            />
            启用该编排（停用后订阅链接返回 404）
          </label>

          <div className="flex flex-wrap gap-2">
            <button type="button" className="btn btn-primary" onClick={save} disabled={busy}>
              {busy ? <Spinner /> : null} 保存
            </button>
            <button type="button" className="btn btn-ghost" onClick={onBack}>
              <ArrowLeft size={14} /> 返回排序
            </button>
          </div>
        </div>

        <div className="card p-4">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-[0.8125rem] font-semibold">配置预览</h2>
            <div className="ml-auto flex items-center gap-1 rounded-md border border-line p-0.5">
              {(['mihomo', 'sing-box', 'loon'] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setTarget(t)}
                  aria-pressed={target === t}
                  className={`rounded px-2 py-1 text-[0.7188rem] transition-colors ${
                    target === t ? 'bg-raised text-fg' : 'text-fg-muted hover:text-fg-soft'
                  }`}
                >
                  {t === 'mihomo' ? 'Mihomo' : t === 'sing-box' ? 'sing-box' : 'Loon'}
                </button>
              ))}
            </div>
            <button type="button" className="btn btn-ghost" onClick={runPreview} disabled={busy}>
              {busy ? <Spinner /> : null} 生成预览
            </button>
          </div>

          {preview ? (
            <>
              {preview.skipped.length > 0 && (
                <p className="mt-2 rounded border border-warning/30 bg-warning/5 px-2.5 py-1.5 text-[0.7188rem] text-warning">
                  已跳过 {preview.skipped.length} 个不支持的节点：{preview.skipped.join('、')}
                </p>
              )}
              <div className="mt-3">
                <CodeEditor
                  value={preview.content}
                  onChange={() => {}}
                  readOnly
                  language={target === 'sing-box' ? 'json' : target === 'loon' ? 'ini' : 'yaml'}
                  ariaLabel="配置预览"
                  height={420}
                  footer={`${target} · 只读`}
                />
              </div>
            </>
          ) : (
            <p className="mt-3 text-[0.75rem] text-fg-muted">
              选择目标客户端后生成预览，确认无误再复制订阅链接。
            </p>
          )}
        </div>
      </div>

      <aside className="space-y-3">
        <SubscriptionOutput
          url={profile.url}
          name={profile.name}
          onRotate={rotate}
          rotating={rotating}
        />

        <div className="card p-4">
          <SectionLabel>客户端</SectionLabel>
          <ul className="mt-2 space-y-1.5 text-[0.7188rem] text-fg-muted">
            <li className="flex items-center gap-1.5">
              <Plane size={11} className="text-fg-subtle" /> Mihomo：Clash Verge / FlClash
            </li>
            <li className="flex items-center gap-1.5">
              <Server size={11} className="text-fg-subtle" /> sing-box：SFA / SFI
            </li>
            <li className="flex items-center gap-1.5">
              <ShieldCheck size={11} className="text-fg-subtle" /> Loon（iOS）
            </li>
          </ul>
          <p className="mt-2 text-[0.6875rem] text-fg-subtle">mieru 协议仅支持 Mihomo。</p>
        </div>
      </aside>

    </section>
  )
}

export { Trash2, CopyButton, Plane, Server }
