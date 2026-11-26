import { useEffect, useState } from 'react'
import {
  Apple,
  ArrowUpRight,
  Copy,
  Download,
  FileJson2,
  FileText,
  FileArchive,
  Link2,
  Plane,
  QrCode,
  RotateCw,
  Server,
  Smartphone,
  Terminal,
  Zap,
} from 'lucide-react'
import QRCode from 'qrcode'
import { copyText } from '../lib/hooks'
import { toast } from '../lib/hooks'
import { resolveTheme } from '../lib/theme'
import { Modal, SectionLabel, Spinner } from './ui'

const TARGETS = [
  { id: 'mihomo', label: 'Mihomo', file: 'subs.yaml', icon: Server, hint: 'Clash Verge / FlClash / mihomo 内核' },
  { id: 'sing-box', label: 'sing-box', file: 'subs.json', icon: FileJson2, hint: 'SFA / SFI / sing-box 内核' },
  { id: 'loon', label: 'Loon', file: 'subs.conf', icon: FileText, hint: 'iOS Loon' },
] as const

// 说明独占一行，避免客户端名称较长时挤压操作入口。
function OutputRow({
  icon: Icon,
  label,
  hint,
  action,
  href,
  onClick,
}: {
  icon: React.ComponentType<{ size?: number | string; strokeWidth?: number; className?: string }>
  label: string
  hint: string
  action: React.ReactNode
  href?: string
  onClick?: () => void
}) {
  const body = (
    <>
      <Icon size={14} strokeWidth={1.9} className="shrink-0 text-accent" />
      <span className="min-w-0 flex-1">
        <span className="block text-body font-medium leading-snug text-fg-soft">{label}</span>
        <span className="mt-0.5 block text-meta leading-snug text-fg-subtle">{hint}</span>
      </span>
      <span className="col-start-2 flex shrink-0 items-center gap-1 text-meta text-fg-subtle transition-colors group-hover:text-accent sm:col-auto">
        {action}
      </span>
    </>
  )
  const cls =
    'group grid min-h-11 grid-cols-[auto_minmax(0,1fr)] items-center gap-x-2.5 gap-y-1 rounded-lg border border-line bg-surface px-3 py-2 text-left transition-colors hover:border-accent/50 hover:bg-raised focus-visible:border-accent/50 sm:flex'
  if (href) {
    return (
      <a href={href} className={cls}>
        {body}
      </a>
    )
  }
  return (
    <button type="button" className={cls} onClick={onClick}>
      {body}
    </button>
  )
}

function withParams(url: string, params: Record<string, string>): string {
  const sep = url.includes('?') ? '&' : '?'
  const qs = Object.entries(params)
    .map(([k, v]) => `${k}=${encodeURIComponent(v)}`)
    .join('&')
  return url + sep + qs
}

/** 订阅链接的各种导出方式：复制、二维码、下载文件、客户端一键导入。 */
export function SubscriptionOutput({
  url,
  name,
  slug,
  onRotate,
  rotating,
}: {
  url: string | null
  name: string
  slug?: string
  onRotate: () => void
  rotating: boolean
}) {
  const [showQr, setShowQr] = useState(false)
  const [qr, setQr] = useState('')
  const [busy, setBusy] = useState('')

  // 二维码颜色跟随当前主题，浅色底用深色点阵。
  useEffect(() => {
    if (!showQr || !url) return
    const light = resolveTheme(
      (localStorage.getItem('subs-theme') as 'system' | 'light' | 'dark') || 'system',
    ) === 'light'
    void QRCode.toDataURL(url, {
      margin: 2,
      width: 640,
      errorCorrectionLevel: 'M',
      color: { dark: light ? '#101013' : '#e9e9ee', light: light ? '#ffffff' : '#141417' },
    })
      .then(setQr)
      .catch(() => setQr(''))
  }, [showQr, url])

  if (!url) {
    return (
      <div className="card p-4">
        <div className="flex items-center gap-2">
          <Link2 size={15} className="text-accent" />
          <h2 className="card-title">订阅输出</h2>
        </div>
        <p className="mt-3 rounded-lg border border-warning/35 bg-warning/8 px-3 py-2 text-[0.75rem] text-warning">
          未配置实例外部地址，只生成了相对路径。请到「设置」填写 external_url 后重新保存。
        </p>
      </div>
    )
  }

  const copy = async (value: string, label: string) => {
    const ok = await copyText(value)
    toast(ok ? 'success' : 'error', ok ? `${label}已复制` : '复制失败，请手动选择')
  }

  const download = async (target: string, file: string) => {
    setBusy(target)
    try {
      const res = await fetch(withParams(url, { target, download: '1' }), { credentials: 'omit' })
      if (!res.ok) throw new Error(String(res.status))
      const blob = await res.blob()
      const href = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = href
      a.download = file
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(href)
      toast('success', `${file} 已开始下载`)
    } catch {
      toast('error', '下载失败，请检查外部地址是否可访问')
    } finally {
      setBusy('')
    }
  }

  // 打包下载：三端配置 + README 打成一个 zip，给够不到这台机器的场景用。
  const downloadBundle = async () => {
    setBusy('bundle')
    try {
      const res = await fetch(`${url}/bundle.zip`, { credentials: 'omit' })
      if (!res.ok) throw new Error(String(res.status))
      const blob = await res.blob()
      const href = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = href
      a.download = `${slug || (name || 'subs').replace(/[^\w\-.]/g, '') || 'subs'}-subs.zip`
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(href)
      toast('success', '订阅包已开始下载')
    } catch {
      toast('error', '打包下载失败，请检查外部地址是否可访问')
    } finally {
      setBusy('')
    }
  }

  // 下载文件名优先用订阅名（slug），没有则退回 subs.yaml 等通用名；
  // 与服务端 contentDisposition 的行为保持一致。
  const fileFor = (t: { id: string; file: string }) => {
    if (!slug) return t.file
    const ext = t.file.slice(t.file.indexOf('.'))
    return `${slug}${ext}`
  }
  const importLinks = [
    {
      label: 'Clash / Mihomo',
      hint: 'Clash Verge Rev、FlClash、ClashX Meta',
      href: `clash://install-config?url=${encodeURIComponent(url)}&name=${encodeURIComponent(slug || name)}`,
      icon: Server,
      action: '打开客户端',
    },
    {
      label: 'sing-box',
      hint: 'SFA / SFI',
      href: `sing-box://import-remote-profile?url=${encodeURIComponent(url)}#${encodeURIComponent(slug || name)}`,
      icon: Plane,
      action: '打开客户端',
    },
  ]

  return (
    <>
      <div className="card space-y-3.5 p-4">
        <div className="flex items-center gap-2">
          <Link2 size={15} className="text-accent" />
          <h2 className="card-title">订阅输出</h2>
          <span className="meta ml-auto">按客户端自动适配</span>
        </div>

        <div className="rounded-lg border border-line bg-canvas px-3 py-2.5">
          <code className="mono block break-all text-[0.7188rem] leading-relaxed text-fg-soft">
            {url}
          </code>
        </div>

        <div className="flex gap-1.5">
          <button type="button" className="btn btn-primary h-11 flex-1 text-[0.8125rem]" onClick={() => copy(url, '订阅链接')}>
            <Copy size={14} /> 复制链接
          </button>
          <button
            type="button"
            className="btn btn-ghost h-11 px-3"
            onClick={() => setShowQr(true)}
            aria-label="显示二维码"
          >
            <QrCode size={16} />
          </button>
        </div>

        <div>
          <SectionLabel>一键导入</SectionLabel>
          <div className="mt-2 grid gap-1.5">
            {importLinks.map((l) => (
              <OutputRow
                key={l.label}
                icon={l.icon}
                label={l.label}
                hint={l.hint}
                href={l.href}
                action={
                  <>
                    {l.action}
                    <ArrowUpRight size={13} />
                  </>
                }
              />
            ))}
            {/* Loon 使用应用内粘贴订阅地址的导入流程。 */}
            <OutputRow
              icon={Apple}
              label="Loon"
              hint="复制链接后在应用内「订阅」粘贴"
              onClick={() => void copy(url, '订阅链接')}
              action={
                <>
                  复制链接
                  <Copy size={13} />
                </>
              }
            />
          </div>
        </div>

        <div>
          <SectionLabel>下载配置文件</SectionLabel>
          <div className="mt-2 grid gap-1.5 sm:grid-cols-3">
            {TARGETS.map((t) => (
              <button
                key={t.id}
                type="button"
                onClick={() => download(t.id, fileFor(t))}
                disabled={busy !== ''}
                title={t.hint}
                className="flex flex-col items-start gap-0.5 rounded-lg border border-line bg-surface px-3 py-2 text-left transition-colors hover:border-accent/50 hover:bg-raised disabled:opacity-50"
              >
                <span className="flex items-center gap-1.5 text-body font-medium text-fg-soft">
                  {busy === t.id ? <Spinner /> : <Download size={13} />} {t.label}
                </span>
                <span className="mono text-meta text-fg-subtle">{fileFor(t)}</span>
              </button>
            ))}
          </div>
          <button
            type="button"
            onClick={downloadBundle}
            disabled={busy !== ''}
            className="mt-1.5 flex w-full items-center gap-2.5 rounded-lg border border-line bg-surface px-3 py-2 text-left transition-colors hover:border-accent/50 hover:bg-raised disabled:opacity-50"
          >
            {busy === 'bundle' ? <Spinner /> : <FileArchive size={14} strokeWidth={1.9} className="shrink-0 text-accent" />}
            <span className="min-w-0 flex-1">
              <span className="block text-body font-medium leading-snug text-fg-soft">打包下载订阅文件</span>
              <span className="mt-0.5 block text-meta leading-snug text-fg-subtle">
                三端配置 + README 打成一个 zip，换到够不到本机的设备上用
              </span>
            </span>
            <Download size={14} className="shrink-0 text-fg-subtle" />
          </button>
        </div>

        <details className="group">
          <summary className="flex cursor-pointer items-center gap-1.5 text-[0.75rem] text-fg-muted hover:text-fg-soft">
            <Terminal size={13} /> 固定格式的订阅地址
            <span className="text-fg-subtle">（脚本或客户端不支持 UA 协商时用）</span>
          </summary>
          <ul className="mt-2 space-y-1.5">
            {TARGETS.map((t) => (
              <li key={t.id} className="flex items-center gap-2 rounded-md border border-line bg-canvas px-2 py-1.5">
                <span className="mono w-16 shrink-0 text-[0.6562rem] text-fg-subtle">{t.id}</span>
                <code className="mono min-w-0 flex-1 truncate text-[0.6562rem] text-fg-muted">
                  {withParams(url, { target: t.id })}
                </code>
                <button
                  type="button"
                  aria-label={`复制 ${t.label} 订阅地址`}
                  className="flex h-7 w-7 shrink-0 items-center justify-center rounded border border-line text-fg-muted hover:border-line-strong hover:text-fg"
                  onClick={() => copy(withParams(url, { target: t.id }), `${t.label} 地址`)}
                >
                  <Copy size={12} />
                </button>
              </li>
            ))}
          </ul>
        </details>

        <div className="flex items-center gap-2 border-t border-line pt-3">
          <Zap size={13} className="text-fg-subtle" />
          <p className="text-[0.7188rem] text-fg-muted">
            链接含访问令牌，等同于密码；泄露后在下方轮换即可作废旧链接。
          </p>
        </div>

        <button type="button" className="btn btn-ghost h-11 w-full" onClick={onRotate} disabled={rotating}>
          {rotating ? <Spinner /> : <RotateCw size={14} />} 轮换访问令牌
        </button>
      </div>

      <Modal open={showQr} title="订阅二维码" description={name} onClose={() => setShowQr(false)}>
        <div className="flex flex-col items-center gap-3">
          {qr ? (
            <img
              src={qr}
              alt="订阅链接二维码"
              className="w-[260px] rounded-lg border border-line bg-surface p-2"
            />
          ) : (
            <div className="flex h-[260px] w-[260px] items-center justify-center rounded-lg border border-line bg-canvas text-[0.75rem] text-fg-muted">
              生成中…
            </div>
          )}
          <div className="flex items-center gap-2">
            <button
              type="button"
              className="btn btn-ghost h-10"
              onClick={() => copy(url, '订阅链接')}
            >
              <Copy size={14} /> 复制链接
            </button>
            <a
              className="btn btn-primary h-10"
              href={qr || '#'}
              download={`${name || 'subs'}-qr.png`}
              aria-disabled={!qr}
            >
              <Smartphone size={14} /> 保存二维码
            </a>
          </div>
          <p className="text-center text-[0.6875rem] text-fg-subtle">
            用手机客户端扫码即可导入；链接尾部令牌泄露后请轮换。
          </p>
        </div>
      </Modal>
    </>
  )
}
