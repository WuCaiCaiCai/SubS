import { useEffect, useState } from 'react'
import {
  Apple,
  Copy,
  Download,
  FileJson2,
  FileText,
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
  onRotate,
  rotating,
}: {
  url: string | null
  name: string
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

  const importLinks = [
    {
      label: 'Clash / Mihomo',
      hint: 'Clash Verge Rev、FlClash',
      href: `clash://install-config?url=${encodeURIComponent(url)}&name=${encodeURIComponent(name)}`,
      icon: Server,
    },
    {
      label: 'sing-box',
      hint: 'SFA / SFI',
      href: `sing-box://import-remote-profile?url=${encodeURIComponent(url)}#${encodeURIComponent(name)}`,
      icon: Plane,
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
              <a
                key={l.label}
                href={l.href}
                className="flex items-center gap-2.5 rounded-lg border border-line bg-surface px-3 py-2 text-[0.7812rem] text-fg-soft transition-colors hover:border-accent/50 hover:bg-raised hover:text-fg"
              >
                <l.icon size={15} className="text-accent" />
                <span className="font-medium">{l.label}</span>
                <span className="meta ml-auto">{l.hint}</span>
              </a>
            ))}
            <div className="flex items-center gap-2.5 rounded-lg border border-dashed border-line px-3 py-2 text-[0.7188rem] text-fg-muted">
              <Apple size={15} className="text-fg-subtle" />
              <span>Loon：在应用内「订阅」粘贴上面的链接</span>
            </div>
          </div>
        </div>

        <div>
          <SectionLabel>下载配置文件</SectionLabel>
          <div className="mt-2 grid gap-1.5 sm:grid-cols-3">
            {TARGETS.map((t) => (
              <button
                key={t.id}
                type="button"
                onClick={() => download(t.id, t.file)}
                disabled={busy !== ''}
                title={t.hint}
                className="flex flex-col items-start gap-0.5 rounded-lg border border-line bg-surface px-3 py-2 text-left transition-colors hover:border-accent/50 hover:bg-raised disabled:opacity-50"
              >
                <span className="flex items-center gap-1.5 text-[0.75rem] font-medium text-fg-soft">
                  {busy === t.id ? <Spinner /> : <Download size={13} />} {t.label}
                </span>
                <span className="mono text-[0.6562rem] text-fg-subtle">{t.file}</span>
              </button>
            ))}
          </div>
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
