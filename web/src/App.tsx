import { useCallback, useEffect, useState } from 'react'
import {
  Activity,
  Boxes,
  DatabaseBackup,
  Layers3,
  LogOut,
  Settings2,
  ShieldCheck,
  Trash2,
  TriangleAlert,
} from 'lucide-react'
import { api, setCsrfToken, type Me } from './lib/api'
import { toast } from './lib/hooks'
import { Toaster } from './components/ui'
import { LoginPage } from './pages/Login'
import { DashboardPage } from './pages/Dashboard'
import { SourcesPage } from './pages/Sources'
import { NodesPage } from './pages/Nodes'
import { ProfilesPage } from './pages/Profiles'
import { ProfileEditorPage } from './pages/ProfileEditor'
import { TemplatesPage } from './pages/Templates'
import { SettingsPage } from './pages/Settings'
import { BackupPage } from './pages/Backup'
import { DestroyPage } from './pages/Destroy'

export type Route =
  | { name: 'dashboard' }
  | { name: 'sources' }
  | { name: 'nodes' }
  | { name: 'profiles' }
  | { name: 'profile'; id: number }
  | { name: 'templates' }
  | { name: 'settings' }
  | { name: 'backup' }
  | { name: 'destroy' }

const NAV = [
  { name: 'dashboard', label: '总览', icon: Activity, path: '/' },
  { name: 'sources', label: '来源', icon: Layers3, path: '/sources' },
  { name: 'nodes', label: '节点库', icon: Boxes, path: '/nodes' },
  { name: 'profiles', label: '编排', icon: ShieldCheck, path: '/profiles' },
] as const

const SECONDARY = [
  { name: 'templates', label: '模板', icon: Layers3, path: '/templates' },
  { name: 'backup', label: '备份', icon: DatabaseBackup, path: '/backup' },
  { name: 'settings', label: '设置', icon: Settings2, path: '/settings' },
  { name: 'destroy', label: '销毁', icon: Trash2, path: '/destroy' },
] as const

function parseRoute(path: string): Route {
  if (path === '/' || path === '') return { name: 'dashboard' }
  if (path === '/sources') return { name: 'sources' }
  if (path === '/nodes') return { name: 'nodes' }
  if (path === '/profiles') return { name: 'profiles' }
  const profileMatch = path.match(/^\/profiles\/(\d+)$/)
  if (profileMatch) return { name: 'profile', id: Number(profileMatch[1]) }
  if (path === '/templates') return { name: 'templates' }
  if (path === '/settings') return { name: 'settings' }
  if (path === '/backup') return { name: 'backup' }
  if (path === '/destroy') return { name: 'destroy' }
  return { name: 'dashboard' }
}

export function navigate(path: string) {
  if (window.location.pathname === path) return
  window.history.pushState({}, '', path)
  window.dispatchEvent(new PopStateEvent('popstate'))
}

export function App() {
  const [me, setMe] = useState<Me | null>(null)
  const [checking, setChecking] = useState(true)
  const [route, setRoute] = useState<Route>(() => parseRoute(window.location.pathname))

  const loadMe = useCallback(async () => {
    try {
      const res = await api.get<Me>('/api/auth/me')
      setCsrfToken(res.csrfToken)
      setMe(res)
    } catch {
      setMe(null)
    } finally {
      setChecking(false)
    }
  }, [])

  useEffect(() => {
    void loadMe()
  }, [loadMe])

  useEffect(() => {
    const onPop = () => setRoute(parseRoute(window.location.pathname))
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  if (checking) {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-zinc-950">
        <div className="mono text-[12px] text-zinc-600">连接实例中…</div>
      </div>
    )
  }

  if (!me) {
    return (
      <>
        <LoginPage onAuthed={loadMe} />
        <Toaster />
      </>
    )
  }

  const logout = async () => {
    try {
      await api.post('/api/auth/logout')
    } catch {
      /* ignore */
    }
    setMe(null)
    navigate('/')
    toast('info', '已退出登录')
  }

  const go = (path: string) => navigate(path)

  return (
    <div className="min-h-dvh bg-zinc-950">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-50 focus:rounded focus:bg-zinc-900 focus:px-3 focus:py-2"
      >
        跳到主内容
      </a>

      {/* Desktop sidebar */}
      <aside className="fixed inset-y-0 left-0 hidden w-56 flex-col border-r border-zinc-800 bg-zinc-950 md:flex">
        <div className="flex items-center gap-2 px-4 py-4">
          <div className="flex h-7 w-7 items-center justify-center rounded border border-cyan-400/30 bg-cyan-400/10 font-mono text-[13px] font-semibold text-cyan-300">
            S
          </div>
          <div className="leading-tight">
            <div className="text-[13px] font-semibold">SubS</div>
            <div className="mono text-[9.5px] uppercase tracking-[0.14em] text-zinc-600">
              private sub ops
            </div>
          </div>
        </div>

        <nav className="mt-2 flex flex-col gap-0.5 px-2" aria-label="主导航">
          {NAV.map((item) => (
            <NavButton
              key={item.name}
              active={route.name === item.name || (route.name === 'profile' && item.name === 'profiles')}
              icon={<item.icon size={16} />}
              label={item.label}
              onClick={() => go(item.path)}
            />
          ))}
          <div className="my-2 border-t border-zinc-800/80" />
          {SECONDARY.map((item) => (
            <NavButton
              key={item.name}
              active={route.name === item.name}
              icon={<item.icon size={16} />}
              label={item.label}
              onClick={() => go(item.path)}
              danger={item.name === 'destroy'}
            />
          ))}
        </nav>

        <div className="mt-auto space-y-2 border-t border-zinc-800 p-3">
          <div className="flex items-center gap-1.5 text-[11px] text-zinc-600">
            <ShieldCheck size={12} />
            <span>本地运行 · 无遥测</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="mono truncate text-[11.5px] text-zinc-400">{me.username}</span>
            <button
              type="button"
              onClick={logout}
              aria-label="退出登录"
              className="flex h-8 w-8 items-center justify-center rounded text-zinc-500 hover:bg-zinc-800 hover:text-zinc-200"
            >
              <LogOut size={14} />
            </button>
          </div>
        </div>
      </aside>

      {/* Mobile top bar */}
      <header className="sticky top-0 z-30 flex items-center justify-between border-b border-zinc-800 bg-zinc-950/90 px-4 py-3 backdrop-blur md:hidden">
        <div className="flex items-center gap-2">
          <div className="flex h-6 w-6 items-center justify-center rounded border border-cyan-400/30 bg-cyan-400/10 font-mono text-[11px] text-cyan-300">
            S
          </div>
          <span className="text-[13px] font-semibold">SubS</span>
        </div>
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={() => go('/settings')}
            aria-label="设置"
            className="flex h-9 w-9 items-center justify-center rounded text-zinc-400 hover:bg-zinc-800"
          >
            <Settings2 size={16} />
          </button>
          <button
            type="button"
            onClick={logout}
            aria-label="退出登录"
            className="flex h-9 w-9 items-center justify-center rounded text-zinc-400 hover:bg-zinc-800"
          >
            <LogOut size={16} />
          </button>
        </div>
      </header>

      <main
        id="main"
        className="mx-auto w-full max-w-6xl px-4 pb-28 pt-5 md:pb-12 md:pl-64 md:pr-6 md:pt-8"
      >
        {route.name === 'dashboard' && <DashboardPage onNavigate={go} />}
        {route.name === 'sources' && <SourcesPage onNavigate={go} />}
        {route.name === 'nodes' && <NodesPage onNavigate={go} />}
        {route.name === 'profiles' && <ProfilesPage onNavigate={go} />}
        {route.name === 'profile' && <ProfileEditorPage profileId={route.id} onNavigate={go} />}
        {route.name === 'templates' && <TemplatesPage />}
        {route.name === 'settings' && <SettingsPage />}
        {route.name === 'backup' && <BackupPage />}
        {route.name === 'destroy' && <DestroyPage onNavigate={go} />}
      </main>

      {/* Mobile bottom tabs */}
      <nav
        className="fixed inset-x-0 bottom-0 z-30 border-t border-zinc-800 bg-zinc-950/95 pb-[env(safe-area-inset-bottom)] backdrop-blur md:hidden"
        aria-label="底部导航"
      >
        <div className="grid grid-cols-4">
          {NAV.map((item) => {
            const active = route.name === item.name || (route.name === 'profile' && item.name === 'profiles')
            return (
              <button
                key={item.name}
                type="button"
                onClick={() => go(item.path)}
                aria-current={active ? 'page' : undefined}
                className={`flex h-14 flex-col items-center justify-center gap-0.5 text-[10.5px] transition-colors ${
                  active ? 'text-cyan-300' : 'text-zinc-500'
                }`}
              >
                <item.icon size={18} />
                {item.label}
              </button>
            )
          })}
        </div>
      </nav>

      <Toaster />
    </div>
  )
}

function NavButton({
  active,
  icon,
  label,
  onClick,
  danger,
}: {
  active: boolean
  icon: React.ReactNode
  label: string
  onClick: () => void
  danger?: boolean
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-current={active ? 'page' : undefined}
      className={`flex h-9 items-center gap-2.5 rounded-md px-2.5 text-left text-[13px] transition-colors ${
        active
          ? 'bg-zinc-800/70 text-zinc-100'
          : danger
            ? 'text-red-400/70 hover:bg-red-500/10 hover:text-red-300'
            : 'text-zinc-400 hover:bg-zinc-900 hover:text-zinc-200'
      }`}
    >
      <span className={active ? 'text-cyan-300' : danger ? '' : 'text-zinc-500'}>{icon}</span>
      {label}
      {active && <span className="ml-auto h-3.5 w-0.5 rounded-full bg-cyan-400" aria-hidden />}
    </button>
  )
}

export { TriangleAlert }
