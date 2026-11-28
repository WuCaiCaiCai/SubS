import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  TouchSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
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
  Pencil,
  Plane,
  Plus,
  Search,
  Server,
  ShieldCheck,
  Sparkles,
  Trash2,
  X,
} from 'lucide-react'
import {
  api,
  ApiError,
  type NamingRule,
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
  const [dirty, setDirty] = useState(false)
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
      setDirty(false)
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
      setDirty(false)
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
          {dirty ? (
            <span className="text-[0.75rem] text-warning">有未保存更改</span>
          ) : saved ? (
            <span className="flex items-center gap-1 text-[0.75rem] text-success">
              <Check size={13} /> 已保存
            </span>
          ) : null}
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
          onChange={(next) => {
            setItems(next)
            setDirty(true)
          }}
          onNext={async () => {
            const ok = await saveItems(items)
            if (ok) setStep(1)
          }}
        />
      )}

      {step === 1 && (
        <NodeOrderStep
          profileId={profileId}
          initialRule={p.namingRule}
          nodes={profileNodes}
          items={items}
          overrides={overrides}
          duplicates={duplicates}
          onOverrides={(next) => {
            setOverrides(next)
            setDirty(true)
          }}
          onReorder={(next) => {
            setItems(next)
            void saveItems(next)
          }}
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
    if (covered(n)) {
      // 整源全选状态下单独取消某条：把该整源拆成其余已启用节点
      const gk = key(n)
      const g = groups.find((x) => x.key === gk)
      if (!g) return
      const replacement: ProfileItem[] = g.nodes
        .filter((x) => x.enabled && !x.missing && x.id !== n.id)
        .map((x, idx) => ({
          kind: 'node',
          refId: x.id,
          sourceType: '',
          nameOverride: '',
          sortOrder: items.length + idx,
          label: x.rawName,
          protocol: x.protocol,
          nodeCount: 1,
        }))
      const next: ProfileItem[] = []
      for (const it of items) {
        if (it.kind === 'source' && `${it.sourceType}:${it.refId}` === gk) {
          next.push(...replacement)
        } else {
          next.push(it)
        }
      }
      onChange(next)
      return
    }
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
                            disabled={n.missing || !n.enabled}
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

const SEGMENT_META: { key: string; label: string; sample: (pad: number, upper: boolean) => string }[] = [
  { key: 'flag', label: '国旗', sample: () => '🇭🇰' },
  { key: 'index', label: '序号', sample: (pad) => String(1).padStart(pad, '0') },
  { key: 'region', label: '地区', sample: () => 'HK' },
  { key: 'protocol', label: '协议', sample: (_pad, upper) => (upper ? 'HY2' : 'hy2') },
  { key: 'source', label: '来源', sample: () => '主力机场' },
  { key: 'rawName', label: '原名', sample: () => '香港专线 01' },
]

const NAMING_PRESETS: { label: string; segments: string[]; separator: string }[] = [
  { label: '国旗-地区-序号-协议', segments: ['flag', 'region', 'index', 'protocol'], separator: '-' },
  { label: '序号-地区-协议', segments: ['index', 'region', 'protocol'], separator: '-' },
  { label: '国旗-来源-原名', segments: ['flag', 'source', 'rawName'], separator: '-' },
  { label: '保持原名', segments: ['rawName'], separator: '-' },
]

const SEPARATOR_CHOICES = [
  { value: '-', label: '-' },
  { value: '_', label: '_' },
  { value: ' ', label: '空格' },
  { value: '|', label: '|' },
  { value: '/', label: '/' },
]

function NodeOrderStep({
  profileId,
  initialRule,
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
  initialRule?: NamingRule
  items: ProfileItem[]
  nodes: ProfileNode[]
  overrides: Record<number, string>
  duplicates: string[]
  onOverrides: (v: Record<number, string>) => void
  onReorder: (items: ProfileItem[]) => void
  onSave: () => void
  onRuleSaved: () => void
}) {
  const [rule, setRule] = useState<NamingRule>(() => ({
    segments: initialRule?.segments?.length ? initialRule.segments : ['index', 'region', 'protocol', 'rawName'],
    separator: initialRule?.separator ?? '-',
    zeroPad: initialRule?.zeroPad || 2,
    upperProtocol: initialRule?.upperProtocol ?? true,
  }))
  const [savingRule, setSavingRule] = useState(false)
  const [renamingId, setRenamingId] = useState<number | null>(null)

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 200, tolerance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  useEffect(() => {
    if (!initialRule) return
    setRule({
      segments: initialRule.segments?.length ? initialRule.segments : ['index', 'region', 'protocol', 'rawName'],
      separator: initialRule.separator ?? '-',
      zeroPad: initialRule.zeroPad || 2,
      upperProtocol: initialRule.upperProtocol ?? true,
    })
  }, [initialRule])

  const previewExample = useMemo(() => {
    const parts = rule.segments
      .map((seg) => SEGMENT_META.find((m) => m.key === seg)?.sample(rule.zeroPad, rule.upperProtocol))
      .filter(Boolean)
    return parts.join(rule.separator) || '（请至少保留一个片段）'
  }, [rule])

  const moveSegment = (idx: number, dir: -1 | 1) => {
    const target = idx + dir
    if (target < 0 || target >= rule.segments.length) return
    const next = [...rule.segments]
    ;[next[idx], next[target]] = [next[target], next[idx]]
    setRule({ ...rule, segments: next })
  }

  const removeSegment = (seg: string) => {
    if (rule.segments.length <= 1) return
    setRule({ ...rule, segments: rule.segments.filter((s) => s !== seg) })
  }

  const addSegment = (seg: string) => {
    if (rule.segments.includes(seg)) return
    setRule({ ...rule, segments: [...rule.segments, seg] })
  }

  interface OrderGroup {
    id: string
    sourceKey: string
    sourceType: 'airport' | 'server'
    sourceId: number
    label: string
    isWholeSource: boolean
    nodes: (ProfileNode & { seq: number })[]
  }

  // 按来源分组聚合，保证同一机场/VPS 的节点始终在一起不散开。
  const orderGroups = useMemo(() => {
    const byId = new Map(nodes.map((n) => [n.nodeId, n]))
    const bySource = new Map<string, ProfileNode[]>()
    for (const n of nodes) {
      const k = `${n.sourceType}:${n.sourceId}`
      const list = bySource.get(k) ?? []
      list.push(n)
      bySource.set(k, list)
    }
    const groupsMap = new Map<string, OrderGroup>()
    const order: string[] = []
    let seqCount = 0

    const ensureGroup = (sourceType: 'airport' | 'server', sourceId: number, label: string, isWhole: boolean) => {
      const sk = `${sourceType}:${sourceId}`
      let g = groupsMap.get(sk)
      if (!g) {
        g = {
          id: `group:${sk}`,
          sourceKey: sk,
          sourceType,
          sourceId,
          label,
          isWholeSource: isWhole,
          nodes: [],
        }
        groupsMap.set(sk, g)
        order.push(sk)
      } else if (isWhole) {
        g.isWholeSource = true
      }
      return g
    }

    for (const it of items) {
      if (it.kind === 'source') {
        const sk = `${it.sourceType}:${it.refId}`
        const g = ensureGroup(it.sourceType as 'airport' | 'server', it.refId, it.label, true)
        for (const n of bySource.get(sk) ?? []) {
          if (!g.nodes.some((x) => x.nodeId === n.nodeId)) {
            seqCount += 1
            g.nodes.push({ ...n, seq: seqCount })
          }
        }
      } else {
        const n = byId.get(it.refId)
        if (!n) continue
        const g = ensureGroup(n.sourceType, n.sourceId, n.sourceName, false)
        if (!g.nodes.some((x) => x.nodeId === n.nodeId)) {
          seqCount += 1
          g.nodes.push({ ...n, seq: seqCount })
        }
      }
    }
    return order.map((sk) => groupsMap.get(sk)!)
  }, [items, nodes])

  const buildItemsFromGroups = useCallback(
    (groups: OrderGroup[]): ProfileItem[] => {
      const out: ProfileItem[] = []
      for (const g of groups) {
        if (g.isWholeSource) {
          out.push({
            kind: 'source',
            refId: g.sourceId,
            sourceType: g.sourceType,
            nameOverride: '',
            sortOrder: out.length,
            label: g.label,
            protocol: '',
            nodeCount: g.nodes.length,
          })
        } else {
          for (const n of g.nodes) {
            out.push({
              kind: 'node',
              refId: n.nodeId,
              sourceType: '',
              nameOverride: overrides[n.nodeId] ?? '',
              sortOrder: out.length,
              label: n.rawName,
              protocol: n.protocol,
              nodeCount: 1,
            })
          }
        }
      }
      return out
    },
    [overrides],
  )

  const moveGroup = (groupIdx: number, dir: -1 | 1) => {
    const target = groupIdx + dir
    if (target < 0 || target >= orderGroups.length) return
    onReorder(buildItemsFromGroups(arrayMove(orderGroups, groupIdx, target)))
  }

  const moveNodeInGroup = (groupIdx: number, fromNodeIdx: number, toNodeIdx: number) => {
    if (fromNodeIdx === toNodeIdx) return
    const g = orderGroups[groupIdx]
    if (!g) return
    const nextGroups = orderGroups.map((grp, idx) =>
      idx === groupIdx
        ? { ...grp, isWholeSource: false, nodes: arrayMove(grp.nodes, fromNodeIdx, toNodeIdx) }
        : grp,
    )
    onReorder(buildItemsFromGroups(nextGroups))
  }

  const handleGroupDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    const fromIdx = orderGroups.findIndex((g) => g.id === active.id)
    const toIdx = orderGroups.findIndex((g) => g.id === over.id)
    if (fromIdx >= 0 && toIdx >= 0) {
      onReorder(buildItemsFromGroups(arrayMove(orderGroups, fromIdx, toIdx)))
    }
  }

  const applyRule = async () => {
    if (rule.segments.length === 0) {
      toast('error', '请至少保留一个命名片段')
      return
    }
    setSavingRule(true)
    try {
      await api.put(`/api/profiles/${profileId}`, { namingRule: rule })
      toast('success', '命名规则已应用')
      onRuleSaved()
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '保存命名规则失败')
    } finally {
      setSavingRule(false)
    }
  }

  return (
    <section className="grid gap-4 lg:grid-cols-[1fr_320px]">
      <div className="space-y-3">
        {orderGroups.length === 0 ? (
          <EmptyState icon={<ListOrdered size={18} />} title="还没有节点" description="回到第 1 步挑选节点。" />
        ) : (
          <>
            <p className="text-[0.6875rem] text-fg-subtle">
              可拖动来源卡片调整机场/VPS 顺序，或在卡片内拖动/长按节点调整组内顺序（松手自动保存）。
            </p>
            <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleGroupDragEnd}>
              <SortableContext items={orderGroups.map((g) => g.id)} strategy={verticalListSortingStrategy}>
                <div className="space-y-3">
                  {orderGroups.map((group, groupIdx) => (
                    <SortableGroupCard
                      key={group.id}
                      group={group}
                      sensors={sensors}
                      canMoveUp={groupIdx > 0}
                      canMoveDown={groupIdx < orderGroups.length - 1}
                      renamingId={renamingId}
                      overrides={overrides}
                      duplicates={duplicates}
                      onMoveUp={() => moveGroup(groupIdx, -1)}
                      onMoveDown={() => moveGroup(groupIdx, 1)}
                      onReorderNodes={(fromIdx, toIdx) => moveNodeInGroup(groupIdx, fromIdx, toIdx)}
                      onToggleRename={(nodeId) => setRenamingId(renamingId === nodeId ? null : nodeId)}
                      onChangeOverride={(nodeId, val) => onOverrides({ ...overrides, [nodeId]: val })}
                      onClearOverride={(nodeId) => {
                        const next = { ...overrides }
                        delete next[nodeId]
                        onOverrides(next)
                      }}
                    />
                  ))}
                </div>
              </SortableContext>
            </DndContext>
          </>
        )}
        <div className="flex items-center justify-end">
          <button type="button" className="btn btn-primary" onClick={onSave}>
            保存并继续 <ArrowRight size={14} />
          </button>
        </div>
      </div>

      <aside className="space-y-3">
        <div className="card space-y-3.5 p-3.5">
          <div className="flex items-center justify-between">
            <SectionLabel>命名规则</SectionLabel>
            <span className="text-[0.6875rem] text-fg-subtle">点击积木调整顺序</span>
          </div>

          <div className="space-y-1.5">
            <span className="block text-[0.7188rem] text-fg-muted">常用预设</span>
            <div className="flex flex-wrap gap-1.5">
              {NAMING_PRESETS.map((preset) => {
                const active =
                  rule.segments.join(',') === preset.segments.join(',') && rule.separator === preset.separator
                return (
                  <button
                    key={preset.label}
                    type="button"
                    onClick={() => setRule({ ...rule, segments: [...preset.segments], separator: preset.separator })}
                    className={`rounded-md border px-2 py-1 text-[0.6875rem] transition-colors ${
                      active
                        ? 'border-accent/50 bg-accent/15 text-accent-strong'
                        : 'border-line bg-canvas text-fg-muted hover:border-line-strong hover:text-fg'
                    }`}
                  >
                    {preset.label}
                  </button>
                )
              })}
            </div>
          </div>

          <div className="space-y-1.5">
            <span className="block text-[0.7188rem] text-fg-muted">当前组合顺序</span>
            <div className="flex flex-wrap gap-1.5">
              {rule.segments.map((seg, idx) => {
                const meta = SEGMENT_META.find((m) => m.key === seg)
                return (
                  <div
                    key={seg}
                    className="inline-flex items-center gap-1 rounded-md border border-accent/35 bg-accent/10 px-2 py-1 text-[0.7188rem] text-fg"
                  >
                    <button
                      type="button"
                      disabled={idx === 0}
                      onClick={() => moveSegment(idx, -1)}
                      className="text-fg-muted hover:text-fg disabled:opacity-25"
                      title="左移"
                    >
                      ←
                    </button>
                    <span className="font-medium">{meta?.label ?? seg}</span>
                    <button
                      type="button"
                      disabled={idx === rule.segments.length - 1}
                      onClick={() => moveSegment(idx, 1)}
                      className="text-fg-muted hover:text-fg disabled:opacity-25"
                      title="右移"
                    >
                      →
                    </button>
                    {rule.segments.length > 1 && (
                      <button
                        type="button"
                        onClick={() => removeSegment(seg)}
                        className="ml-0.5 text-fg-subtle hover:text-danger"
                        title="移除"
                      >
                        <X size={11} />
                      </button>
                    )}
                  </div>
                )
              })}
            </div>
            {SEGMENT_META.some((m) => !rule.segments.includes(m.key)) && (
              <div className="flex flex-wrap items-center gap-1 pt-1">
                <span className="text-[0.6562rem] text-fg-subtle">添加：</span>
                {SEGMENT_META.filter((m) => !rule.segments.includes(m.key)).map((m) => (
                  <button
                    key={m.key}
                    type="button"
                    onClick={() => addSegment(m.key)}
                    className="inline-flex items-center gap-0.5 rounded border border-dashed border-line-strong px-1.5 py-0.5 text-[0.6875rem] text-fg-muted hover:border-accent hover:text-accent"
                  >
                    <Plus size={10} /> {m.label}
                  </button>
                ))}
              </div>
            )}
          </div>

          <div className="space-y-1.5">
            <span className="block text-[0.7188rem] text-fg-muted">分隔符</span>
            <div className="flex flex-wrap items-center gap-1">
              {SEPARATOR_CHOICES.map((ch) => (
                <button
                  key={ch.label}
                  type="button"
                  onClick={() => setRule({ ...rule, separator: ch.value })}
                  className={`h-7 min-w-8 rounded border px-2 font-mono text-[0.6875rem] transition-colors ${
                    rule.separator === ch.value
                      ? 'border-accent/50 bg-accent/15 text-accent-strong'
                      : 'border-line bg-canvas text-fg-muted hover:border-line-strong'
                  }`}
                >
                  {ch.label}
                </button>
              ))}
              <input
                className="input mono h-7 w-12 px-1.5 text-center text-[0.7188rem]"
                value={rule.separator}
                maxLength={4}
                onChange={(e) => setRule({ ...rule, separator: e.target.value })}
                aria-label="自定义分隔符"
              />
            </div>
          </div>

          <div className="flex items-center justify-between gap-2">
            <span className="text-[0.7188rem] text-fg-muted">序号位数</span>
            <div className="flex gap-1">
              {[
                { v: 1, label: '1' },
                { v: 2, label: '01' },
                { v: 3, label: '001' },
              ].map((opt) => (
                <button
                  key={opt.v}
                  type="button"
                  onClick={() => setRule({ ...rule, zeroPad: opt.v })}
                  className={`h-7 rounded border px-2 font-mono text-[0.6875rem] transition-colors ${
                    rule.zeroPad === opt.v
                      ? 'border-accent/50 bg-accent/15 text-accent-strong'
                      : 'border-line bg-canvas text-fg-muted hover:border-line-strong'
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>

          <label className="flex items-center justify-between text-[0.7188rem] text-fg-soft">
            <span>协议名大写</span>
            <input
              type="checkbox"
              checked={rule.upperProtocol}
              onChange={(e) => setRule({ ...rule, upperProtocol: e.target.checked })}
              className="h-4 w-4"
            />
          </label>

          <div className="rounded-md border border-line bg-canvas px-2.5 py-2">
            <div className="text-[0.6562rem] text-fg-subtle">实时效果示例</div>
            <div className="mono mt-0.5 truncate text-[0.75rem] text-accent-strong">{previewExample}</div>
          </div>

          <button type="button" className="btn btn-primary w-full" onClick={applyRule} disabled={savingRule}>
            {savingRule ? <Spinner /> : null} 应用到当前编排
          </button>
        </div>
      </aside>
    </section>
  )
}

function SortableGroupCard({
  group,
  sensors,
  canMoveUp,
  canMoveDown,
  renamingId,
  overrides,
  duplicates,
  onMoveUp,
  onMoveDown,
  onReorderNodes,
  onToggleRename,
  onChangeOverride,
  onClearOverride,
}: {
  group: {
    id: string
    sourceKey: string
    sourceType: 'airport' | 'server'
    sourceId: number
    label: string
    isWholeSource: boolean
    nodes: (ProfileNode & { seq: number })[]
  }
  sensors: ReturnType<typeof useSensors>
  canMoveUp: boolean
  canMoveDown: boolean
  renamingId: number | null
  overrides: Record<number, string>
  duplicates: string[]
  onMoveUp: () => void
  onMoveDown: () => void
  onReorderNodes: (fromIdx: number, toIdx: number) => void
  onToggleRename: (nodeId: number) => void
  onChangeOverride: (nodeId: number, val: string) => void
  onClearOverride: (nodeId: number) => void
}) {
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({
    id: group.id,
  })
  const [expanded, setExpanded] = useState(true)
  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  const handleNodeDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    const fromIdx = group.nodes.findIndex((n) => `n:${n.nodeId}` === active.id)
    const toIdx = group.nodes.findIndex((n) => `n:${n.nodeId}` === over.id)
    if (fromIdx >= 0 && toIdx >= 0) {
      onReorderNodes(fromIdx, toIdx)
    }
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`overflow-hidden rounded-lg border border-line bg-canvas ${
        isDragging ? 'relative z-20 shadow-lg ring-1 ring-accent/60' : ''
      }`}
    >
      <div className="flex items-center gap-2 bg-surface/60 px-2.5 py-2">
        <button
          ref={setActivatorNodeRef}
          type="button"
          {...attributes}
          {...listeners}
          aria-label={`拖动来源 ${group.label}`}
          title="拖动调整来源顺序"
          className="flex h-7 w-6 shrink-0 touch-none cursor-grab items-center justify-center rounded text-fg-subtle hover:bg-raised hover:text-fg active:cursor-grabbing"
        >
          <GripVertical size={14} />
        </button>
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
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
              group.sourceType === 'airport' ? 'bg-[var(--airport-accent)]' : 'bg-[var(--server-accent)]'
            }`}
          />
          <span className="truncate text-strong font-medium text-fg">{group.label}</span>
          <span className="mono shrink-0 text-meta text-fg-subtle">
            {group.sourceType === 'airport' ? '机场' : 'VPS'} · {group.nodes.length}
          </span>
          {group.isWholeSource && <Badge tone="cyan">整源</Badge>}
        </button>
        <div className="flex shrink-0 items-center gap-0.5">
          <button
            type="button"
            aria-label={`上移来源 ${group.label}`}
            disabled={!canMoveUp}
            onClick={onMoveUp}
            className="flex h-7 w-7 items-center justify-center rounded text-fg-subtle hover:bg-raised hover:text-fg disabled:opacity-30"
          >
            <ChevronUp size={13} />
          </button>
          <button
            type="button"
            aria-label={`下移来源 ${group.label}`}
            disabled={!canMoveDown}
            onClick={onMoveDown}
            className="flex h-7 w-7 items-center justify-center rounded text-fg-subtle hover:bg-raised hover:text-fg disabled:opacity-30"
          >
            <ChevronDown size={13} />
          </button>
        </div>
      </div>

      {expanded && (
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleNodeDragEnd}>
          <SortableContext items={group.nodes.map((n) => `n:${n.nodeId}`)} strategy={verticalListSortingStrategy}>
            <ul className="divide-y divide-line/60 border-t border-line/70">
              {group.nodes.map((n, nodeIdx) => {
                const isRenaming = renamingId === n.nodeId
                const customName = overrides[n.nodeId] ?? ''
                return (
                  <SortableNodeRow
                    key={n.nodeId}
                    node={n}
                    isRenaming={isRenaming}
                    customName={customName}
                    isDuplicate={duplicates.includes(n.outputName) && !customName}
                    canMoveUp={nodeIdx > 0}
                    canMoveDown={nodeIdx < group.nodes.length - 1}
                    onMoveUp={() => onReorderNodes(nodeIdx, nodeIdx - 1)}
                    onMoveDown={() => onReorderNodes(nodeIdx, nodeIdx + 1)}
                    onToggleRename={() => onToggleRename(n.nodeId)}
                    onChangeOverride={(val) => onChangeOverride(n.nodeId, val)}
                    onClearOverride={() => onClearOverride(n.nodeId)}
                  />
                )
              })}
            </ul>
          </SortableContext>
        </DndContext>
      )}
    </div>
  )
}

function SortableNodeRow({
  node,
  isRenaming,
  customName,
  isDuplicate,
  canMoveUp,
  canMoveDown,
  onMoveUp,
  onMoveDown,
  onToggleRename,
  onChangeOverride,
  onClearOverride,
}: {
  node: ProfileNode & { seq: number }
  isRenaming: boolean
  customName: string
  isDuplicate: boolean
  canMoveUp: boolean
  canMoveDown: boolean
  onMoveUp: () => void
  onMoveDown: () => void
  onToggleRename: () => void
  onChangeOverride: (val: string) => void
  onClearOverride: () => void
}) {
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({
    id: `n:${node.nodeId}`,
  })
  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  return (
    <li
      ref={setNodeRef}
      style={style}
      className={`px-2.5 py-2 pl-6 ${isDragging ? 'relative z-10 bg-raised shadow-md ring-1 ring-accent/60' : ''}`}
    >
      <div className="flex items-center gap-2">
        <button
          ref={setActivatorNodeRef}
          type="button"
          {...attributes}
          {...listeners}
          aria-label="拖动排序"
          title="拖动调整组内顺序"
          className="flex h-7 w-6 shrink-0 touch-none cursor-grab items-center justify-center rounded text-fg-subtle hover:bg-raised hover:text-fg active:cursor-grabbing"
        >
          <GripVertical size={13} />
        </button>
        <span className="mono w-6 shrink-0 text-center text-meta text-fg-subtle">
          {String(node.seq).padStart(2, '0')}
        </span>
        <div className="min-w-0 flex-1">
          <div className="mono truncate text-[0.75rem] text-fg-soft">{node.rawName}</div>
          <div className="mt-0.5 flex items-center gap-1.5">
            <span className="mono truncate text-[0.75rem] text-accent-strong/90">
              {customName || node.outputName}
            </span>
            {customName && <Badge tone="cyan">自定义</Badge>}
            {isDuplicate && <Badge tone="amber">重名</Badge>}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-0.5">
          <button
            type="button"
            aria-label="自定义节点名称"
            title="自定义节点名称"
            onClick={onToggleRename}
            className={`flex h-7 w-7 items-center justify-center rounded transition-colors ${
              isRenaming || customName ? 'bg-accent/15 text-accent' : 'text-fg-subtle hover:bg-raised hover:text-fg'
            }`}
          >
            <Pencil size={12} />
          </button>
          <button
            type="button"
            aria-label="上移"
            disabled={!canMoveUp}
            onClick={onMoveUp}
            className="flex h-7 w-7 items-center justify-center rounded text-fg-subtle hover:bg-raised hover:text-fg disabled:opacity-30"
          >
            <ChevronUp size={13} />
          </button>
          <button
            type="button"
            aria-label="下移"
            disabled={!canMoveDown}
            onClick={onMoveDown}
            className="flex h-7 w-7 items-center justify-center rounded text-fg-subtle hover:bg-raised hover:text-fg disabled:opacity-30"
          >
            <ChevronDown size={13} />
          </button>
        </div>
      </div>
      {isRenaming && (
        <div className="mt-2 flex items-center gap-1.5 pl-8">
          <input
            className="input mono h-8 flex-1 text-[0.75rem]"
            placeholder={node.outputName ?? '留空恢复自动命名'}
            value={customName}
            onChange={(e) => onChangeOverride(e.target.value)}
            autoFocus
          />
          {customName && (
            <button
              type="button"
              onClick={onClearOverride}
              className="btn btn-ghost h-8 px-2 text-[0.6875rem]"
              title="恢复自动命名"
            >
              恢复默认
            </button>
          )}
          <button type="button" onClick={onToggleRename} className="btn btn-ghost h-8 px-2 text-[0.6875rem]">
            完成
          </button>
        </div>
      )}
    </li>
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
  const [slug, setSlug] = useState(profile.slug)
  const [target, setTarget] = useState<'mihomo' | 'sing-box' | 'loon'>('mihomo')
  const [preview, setPreview] = useState<{ content: string; skipped: string[] } | null>(null)
  const [busy, setBusy] = useState(false)
  const [rotating, setRotating] = useState(false)

  useEffect(() => {
    setTemplateId(profile.templateId)
    setEnabled(profile.enabled)
    setSlug(profile.slug)
  }, [profile])

  const save = async () => {
    setBusy(true)
    try {
      const cleaned = slug.trim().toLowerCase()
      if (cleaned && !/^[a-z0-9](-?[a-z0-9])*$/.test(cleaned)) {
        toast('error', '订阅名仅支持小写字母、数字与连字符')
        return
      }
      await api.put(`/api/profiles/${profile.id}`, {
        name: profile.name,
        note: profile.note,
        slug: cleaned,
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

          <label className="block space-y-1.5">
            <span className="text-[0.75rem] font-medium text-fg-soft">订阅名（URL 路径段）</span>
            <div className="flex items-center gap-1.5">
              <span className="mono shrink-0 text-[0.7188rem] text-fg-subtle">/s/</span>
              <input
                className="input mono"
                value={slug}
                onChange={(e) => setSlug(e.target.value.toLowerCase())}
                placeholder="my-family"
                maxLength={32}
                autoComplete="off"
                spellCheck={false}
              />
              <span className="mono shrink-0 text-[0.7188rem] text-fg-subtle">/&lt;token&gt;</span>
            </div>
            <span className="block text-[0.6875rem] text-fg-muted">
              可选。小写字母、数字、连字符；留空则使用 <code className="mono">/s/&lt;token&gt;</code> 旧式链接。保存后订阅链接变为该形式。
            </span>
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
          slug={profile.slug}
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
