import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  ArrowLeft,
  ArrowRight,
  Check,
  ChevronDown,
  ChevronUp,
  Copy,
  GripVertical,
  KeyRound,
  Layers3,
  ListOrdered,
  Plane,
  QrCode,
  RotateCw,
  Search,
  Server,
  ShieldCheck,
  Sparkles,
  Trash2,
} from 'lucide-react'
import QRCode from 'qrcode'
import {
  api,
  ApiError,
  type NodeItem,
  type Profile,
  type ProfileNode,
  type Template,
} from '../lib/api'
import { copyText, relativeTime, toast, useAsync } from '../lib/hooks'
import {
  Badge,
  CopyButton,
  EmptyState,
  ErrorNote,
  Modal,
  ProtocolBadge,
  SectionLabel,
  SkeletonRows,
  Spinner,
} from '../components/ui'

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
  const [selected, setSelected] = useState<number[]>([])
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
      const res = await api.get<{ nodes: ProfileNode[]; duplicates: string[] }>(
        `/api/profiles/${profileId}/naming-preview`,
      )
      const nodes = res.nodes ?? []
      setProfileNodes(nodes)
      setSelected(nodes.map((n) => n.nodeId))
      const map: Record<number, string> = {}
      for (const n of nodes) map[n.nodeId] = n.nameOverride
      setOverrides(map)
    } catch {
      /* ignore */
    }
  }, [profileId])

  useEffect(() => {
    void loadSelection()
  }, [loadSelection])

  const saveNodes = async (ids: number[], ov?: Record<number, string>) => {
    setError('')
    try {
      const res = await api.put<ProfileNode[]>(
        `/api/profiles/${profileId}/nodes`,
        ids.map((nodeId) => ({ nodeId, nameOverride: ov?.[nodeId] ?? overrides[nodeId] ?? '' })),
      )
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
            className="mb-1.5 inline-flex items-center gap-1 text-[12px] text-zinc-500 hover:text-zinc-300"
          >
            <ArrowLeft size={12} /> 返回编排列表
          </button>
          <SectionLabel>profile / {p.id}</SectionLabel>
          <h1 className="mt-1 truncate text-base font-semibold tracking-tight">{p.name}</h1>
        </div>
        <div className="flex items-center gap-2">
          {saved && (
            <span className="flex items-center gap-1 text-[12px] text-emerald-300">
              <Check size={13} /> 已保存
            </span>
          )}
          <ProfileMeta profile={p} />
        </div>
      </header>

      <ol className="flex items-center gap-1 rounded-lg border border-zinc-800 bg-zinc-900/40 p-1">
        {STEPS.map((s, i) => {
          const active = step === i
          const done = step > i
          return (
            <li key={s.key} className="flex flex-1 items-center">
              <button
                type="button"
                onClick={() => setStep(i as Step)}
                aria-current={active ? 'step' : undefined}
                className={`flex h-9 flex-1 items-center justify-center gap-2 rounded-md text-[12.5px] transition-colors ${
                  active ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
                }`}
              >
                <s.icon size={13} className={active ? 'text-cyan-300' : ''} />
                <span className="hidden sm:inline">{s.label}</span>
                <span className="mono text-[10.5px] text-zinc-600">{i + 1}</span>
                {done && <Check size={12} className="text-emerald-400" />}
              </button>
              {i < STEPS.length - 1 && <span className="mx-1 h-px w-3 bg-zinc-800 sm:w-6" aria-hidden />}
            </li>
          )
        })}
      </ol>

      {error && <ErrorNote>{error}</ErrorNote>}

      {step === 0 && (
        <NodeSelectStep
          selected={selected}
          onChange={setSelected}
          onNext={async () => {
            const ok = await saveNodes(selected)
            if (ok) setStep(1)
          }}
        />
      )}

      {step === 1 && (
        <NodeOrderStep
          profileId={profileId}
          nodes={profileNodes}
          overrides={overrides}
          duplicates={duplicates}
          onOverrides={setOverrides}
          onReorder={(ids) => setSelected(ids)}
          onRuleSaved={() => {
            profile.reload()
            void loadNaming()
          }}
          onSave={async () => {
            const ok = await saveNodes(selected, overrides)
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
      <span className="mono text-[11px] text-zinc-500">拉取 {relativeTime(profile.lastFetchAt)}</span>
    </div>
  )
}

function NodeSelectStep({
  selected,
  onChange,
  onNext,
}: {
  selected: number[]
  onChange: (ids: number[]) => void
  onNext: () => void
}) {
  const nodes = useAsync(() => api.get<NodeItem[]>('/api/nodes'))
  const [q, setQ] = useState('')
  const [source, setSource] = useState<string>('')
  const [busy, setBusy] = useState(false)

  const list = useMemo(() => {
    let l = nodes.data ?? []
    if (source) l = l.filter((n) => `${n.sourceType}:${n.sourceId}` === source)
    if (q.trim()) {
      const k = q.trim().toLowerCase()
      l = l.filter((n) => n.rawName.toLowerCase().includes(k) || n.host.toLowerCase().includes(k))
    }
    return l
  }, [nodes.data, source, q])

  const sources = useMemo(() => {
    const map = new Map<string, { label: string; type: 'airport' | 'server'; id: number }>()
    for (const n of nodes.data ?? []) {
      map.set(`${n.sourceType}:${n.sourceId}`, {
        label: n.sourceName,
        type: n.sourceType,
        id: n.sourceId,
      })
    }
    return [...map.entries()]
  }, [nodes.data])

  const toggle = (id: number) => {
    const next = selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id]
    onChange(next)
  }

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-0 flex-1">
          <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            className="input pl-8"
            placeholder="搜索节点名或地址…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
        </div>
        <div className="relative">
          <select
            aria-label="按来源筛选"
            className="input h-11 w-[150px] appearance-none pr-7 md:h-10"
            value={source}
            onChange={(e) => setSource(e.target.value)}
          >
            <option value="">全部来源</option>
            {sources.map(([key, s]) => (
              <option key={key} value={key}>
                {s.label}
              </option>
            ))}
          </select>
          <ChevronDown size={13} className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-500" />
        </div>
      </div>

      {nodes.loading ? (
        <SkeletonRows rows={6} />
      ) : nodes.error ? (
        <ErrorNote>{nodes.error.message}</ErrorNote>
      ) : list.length === 0 ? (
        <EmptyState
          icon={<Sparkles size={18} />}
          title="没有可选择的节点"
          description="先到「来源」添加机场并刷新，或为自建 VPS 导入节点。"
        />
      ) : (
        <ul className="divide-y divide-zinc-800/70 overflow-hidden rounded-lg border border-zinc-800">
          {list.map((n) => {
            const checked = selected.includes(n.id)
            return (
              <li key={n.id} className="flex items-center gap-2.5 px-3 py-2">
                <input
                  type="checkbox"
                  aria-label={`选择 ${n.rawName}`}
                  checked={checked}
                  onChange={() => toggle(n.id)}
                  className="h-4 w-4 shrink-0"
                />
                <span
                  aria-hidden
                  className={`h-1.5 w-1.5 shrink-0 rounded-full ${
                    n.sourceType === 'airport' ? 'bg-[var(--airport-accent)]' : 'bg-[var(--server-accent)]'
                  }`}
                />
                <div className="min-w-0 flex-1">
                  <div className="mono truncate text-[12.5px] text-zinc-100">{n.rawName}</div>
                  <div className="mt-0.5 flex items-center gap-2">
                    <ProtocolBadge protocol={n.protocol} />
                    <span className="mono truncate text-[10.5px] text-zinc-500">
                      {n.host}:{n.portRange || n.port}
                    </span>
                  </div>
                </div>
                <span className="hidden shrink-0 text-[11px] text-zinc-600 sm:inline">{n.sourceName}</span>
                {n.missing && <Badge tone="neutral">缺失</Badge>}
                {!n.enabled && <Badge tone="neutral">停用</Badge>}
              </li>
            )
          })}
        </ul>
      )}

      <div className="flex items-center justify-between border-t border-zinc-800 pt-3">
        <span className="mono text-[12px] text-zinc-500">已选 {selected.length} 个节点</span>
        <button
          type="button"
          className="btn btn-primary"
          disabled={selected.length === 0}
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
  nodes,
  overrides,
  duplicates,
  onOverrides,
  onReorder,
  onSave,
  onRuleSaved,
}: {
  profileId: number
  nodes: ProfileNode[]
  overrides: Record<number, string>
  duplicates: string[]
  onOverrides: (v: Record<number, string>) => void
  onReorder: (ids: number[]) => void
  onSave: () => void
  onRuleSaved: () => void
}) {
  const [rule, setRule] = useState({ segments: 'index,region,protocol,rawName', separator: '-', zeroPad: 2, upperProtocol: true })
  const [savingRule, setSavingRule] = useState(false)

  const ids = useMemo(() => nodes.map((n) => n.nodeId), [nodes])

  const move = (index: number, dir: -1 | 1) => {
    const next = [...ids]
    const target = index + dir
    if (target < 0 || target >= next.length) return
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

  return (
    <section className="grid gap-4 lg:grid-cols-[1fr_260px]">
      <div className="space-y-2">
        {nodes.length === 0 ? (
          <EmptyState
            icon={<ListOrdered size={18} />}
            title="还没有节点"
            description="回到第 1 步挑选节点。"
          />
        ) : (
          <ul className="divide-y divide-zinc-800/70 overflow-hidden rounded-lg border border-zinc-800">
            {nodes.map((n, i) => (
              <li key={n.nodeId} className="flex items-center gap-2 px-2.5 py-2">
                <span className="mono w-6 shrink-0 text-center text-[11px] text-zinc-600">
                  {String(i + 1).padStart(2, '0')}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="mono truncate text-[12px] text-zinc-400">{n.rawName}</div>
                  <div className="mt-0.5 flex items-center gap-1.5">
                    <span className="mono truncate text-[12px] text-cyan-200/90">{n.outputName}</span>
                    {duplicates.includes(n.outputName) && <Badge tone="amber">重名</Badge>}
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-0.5">
                  <button
                    type="button"
                    aria-label="上移"
                    onClick={() => move(i, -1)}
                    className="flex h-7 w-7 items-center justify-center rounded text-zinc-600 hover:bg-zinc-800 hover:text-zinc-200"
                  >
                    <ChevronUp size={13} />
                  </button>
                  <button
                    type="button"
                    aria-label="下移"
                    onClick={() => move(i, 1)}
                    className="flex h-7 w-7 items-center justify-center rounded text-zinc-600 hover:bg-zinc-800 hover:text-zinc-200"
                  >
                    <ChevronDown size={13} />
                  </button>
                </div>
              </li>
            ))}
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
            <span className="text-[11.5px] text-zinc-400">片段顺序</span>
            <input
              className="input mono h-9 text-[12px]"
              value={rule.segments}
              onChange={(e) => setRule({ ...rule, segments: e.target.value })}
            />
            <span className="block text-[10.5px] text-zinc-600">
              可选：index / region / protocol / rawName
            </span>
          </label>
          <label className="block space-y-1">
            <span className="text-[11.5px] text-zinc-400">分隔符</span>
            <input
              className="input mono h-9 text-[12px]"
              value={rule.separator}
              onChange={(e) => setRule({ ...rule, separator: e.target.value })}
            />
          </label>
          <label className="block space-y-1">
            <span className="text-[11.5px] text-zinc-400">序号位数</span>
            <input
              type="number"
              min={1}
              max={4}
              className="input mono h-9 text-[12px]"
              value={rule.zeroPad}
              onChange={(e) => setRule({ ...rule, zeroPad: Number(e.target.value) })}
            />
          </label>
          <label className="flex items-center gap-2 text-[11.5px] text-zinc-300">
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
          <p className="mt-1 text-[11px] text-zinc-500">在第 1 步之后，可在预览里逐个覆盖名称。</p>
          <div className="mt-2 space-y-1.5">
            {nodes.slice(0, 8).map((n) => (
              <div key={n.nodeId} className="space-y-1">
                <span className="mono block truncate text-[10.5px] text-zinc-500">{n.rawName}</span>
                <input
                  className="input mono h-8 text-[11.5px]"
                  placeholder={n.outputName}
                  value={overrides[n.nodeId] ?? ''}
                  onChange={(e) => onOverrides({ ...overrides, [n.nodeId]: e.target.value })}
                />
              </div>
            ))}
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
  const [qr, setQr] = useState('')
  const [busy, setBusy] = useState(false)
  const [rotating, setRotating] = useState(false)
  const [showQr, setShowQr] = useState(false)

  useEffect(() => {
    setTemplateId(profile.templateId)
    setEnabled(profile.enabled)
  }, [profile])

  useEffect(() => {
    if (!profile.url) return
    QRCode.toDataURL(profile.url, { margin: 1, width: 220, color: { dark: '#e4e4e7', light: '#18181b' } })
      .then(setQr)
      .catch(() => setQr(''))
  }, [profile.url])

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
            <Layers3 size={14} className="text-cyan-300" />
            <h2 className="text-[13px] font-semibold">模板与状态</h2>
          </div>

          <label className="block space-y-1.5">
            <span className="text-[12px] font-medium text-zinc-300">Clash 模板</span>
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

          <label className="flex items-center gap-2 text-[12.5px] text-zinc-300">
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
            <h2 className="text-[13px] font-semibold">配置预览</h2>
            <div className="ml-auto flex items-center gap-1 rounded-md border border-zinc-800 p-0.5">
              {(['mihomo', 'sing-box', 'loon'] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setTarget(t)}
                  aria-pressed={target === t}
                  className={`rounded px-2 py-1 text-[11.5px] transition-colors ${
                    target === t ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
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
                <p className="mt-2 rounded border border-amber-400/30 bg-amber-400/5 px-2.5 py-1.5 text-[11.5px] text-amber-200">
                  已跳过 {preview.skipped.length} 个不支持的节点：{preview.skipped.join('、')}
                </p>
              )}
              <pre className="mono mt-3 max-h-[420px] overflow-auto rounded border border-zinc-800 bg-zinc-950 p-3 text-[11.5px] leading-relaxed text-zinc-300">
                {preview.content}
              </pre>
            </>
          ) : (
            <p className="mt-3 text-[12px] text-zinc-500">
              选择目标客户端后生成预览，确认无误再复制订阅链接。
            </p>
          )}
        </div>
      </div>

      <aside className="space-y-3">
        <div className="card p-4">
          <div className="flex items-center gap-2">
            <KeyRound size={14} className="text-cyan-300" />
            <h2 className="text-[13px] font-semibold">订阅链接</h2>
          </div>
          {profile.url ? (
            <>
              <div className="mt-3 rounded border border-zinc-800 bg-zinc-950 p-2.5">
                <code className="mono block break-all text-[11px] leading-relaxed text-zinc-300">
                  {profile.url}
                </code>
              </div>
              <div className="mt-2 flex gap-1.5">
                <button
                  type="button"
                  className="btn btn-primary flex-1"
                  onClick={() => {
                    copyText(profile.url)
                    toast('success', '订阅链接已复制')
                  }}
                >
                  <Copy size={13} /> 复制
                </button>
                <button
                  type="button"
                  className="btn btn-ghost"
                  onClick={() => setShowQr(true)}
                  aria-label="显示二维码"
                >
                  <QrCode size={13} />
                </button>
              </div>
              <p className="mt-2 text-[11px] text-zinc-500">
                链接按 User-Agent 自动返回 Mihomo / sing-box / Loon 对应格式。
              </p>
            </>
          ) : (
            <p className="mt-3 rounded border border-amber-400/30 bg-amber-400/5 px-2.5 py-2 text-[11.5px] text-amber-200">
              未配置实例外部地址，只生成了相对路径。请到「设置」填写 external_url 后重新保存。
            </p>
          )}

          <button
            type="button"
            className="btn btn-ghost mt-3 w-full"
            onClick={rotate}
            disabled={rotating}
          >
            {rotating ? <Spinner /> : <RotateCw size={13} />} 轮换令牌
          </button>
        </div>

        <div className="card p-4">
          <SectionLabel>客户端</SectionLabel>
          <ul className="mt-2 space-y-1.5 text-[11.5px] text-zinc-400">
            <li className="flex items-center gap-1.5">
              <Plane size={11} className="text-zinc-600" /> Mihomo：Clash Verge / FlClash
            </li>
            <li className="flex items-center gap-1.5">
              <Server size={11} className="text-zinc-600" /> sing-box：SFA / SFI
            </li>
            <li className="flex items-center gap-1.5">
              <ShieldCheck size={11} className="text-zinc-600" /> Loon（iOS）
            </li>
          </ul>
          <p className="mt-2 text-[11px] text-zinc-600">mieru 协议仅支持 Mihomo。</p>
        </div>
      </aside>

      <Modal open={showQr} title="订阅二维码" onClose={() => setShowQr(false)}>
        {qr && <img src={qr} alt="订阅链接二维码" className="mx-auto w-[220px] rounded" />}
        {profile.url && (
          <p className="mono mt-3 break-all text-center text-[11px] text-zinc-500">{profile.url}</p>
        )}
      </Modal>
    </section>
  )
}

export { Trash2, CopyButton, Plane, Server }
