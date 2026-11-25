// Typed API client. All mutations carry the CSRF header; auth failures bubble up
// as ApiError so the router can bounce to /login.
export class ApiError extends Error {
  status: number
  code: string
  details?: unknown
  constructor(status: number, code: string, message: string, details?: unknown) {
    super(message)
    this.status = status
    this.code = code
    this.details = details
  }
}

let csrfToken = ''

export function setCsrfToken(token: string) {
  csrfToken = token
}

export function getCsrfToken() {
  return csrfToken
}

function readCookie(name: string): string {
  const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`))
  return match ? decodeURIComponent(match[1]) : ''
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (method !== 'GET') headers['X-CSRF-Token'] = csrfToken || readCookie('subs_csrf')
  const res = await fetch(path, {
    method,
    headers,
    credentials: 'same-origin',
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (res.status === 204) return undefined as T
  const text = await res.text()
  let payload: unknown = undefined
  if (text) {
    try {
      payload = JSON.parse(text)
    } catch {
      payload = text
    }
  }
  if (!res.ok) {
    const err = (payload as { error?: { code?: string; message?: string } })?.error
    throw new ApiError(res.status, err?.code ?? 'unknown', err?.message ?? `请求失败 (${res.status})`, payload)
  }
  return payload as T
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  del: <T>(path: string) => request<T>('DELETE', path),
}

// ---- domain types -------------------------------------------------------

export type Protocol = 'hy2' | 'vless' | 'tuic' | 'anytls' | 'mieru'

export interface Me {
  username: string
  csrfToken: string
  initialized: boolean
}

export interface SystemInfo {
  instanceName: string
  version: string
  listenHost: string
  listenPort: number
  externalUrl: string
  diskUsedBytes: number
  airports: number
  servers: number
  nodes: number
  newNodes: number
  missingNodes: number
  profiles: number
  lastBackupAt: string
}

export interface Airport {
  id: number
  name: string
  url: string
  enabled: boolean
  note: string
  refreshIntervalMin: number | null
  lastOkAt: string
  lastError: string
  nodeCount: number
}

export interface VpsServer {
  id: number
  name: string
  address: string
  note: string
  tags: string[]
  nodeCount: number
}

export interface NodeItem {
  id: number
  sourceType: 'airport' | 'server'
  sourceId: number
  sourceName: string
  protocol: Protocol
  rawName: string
  host: string
  port: number
  /** mieru 使用端口范围时 port 为 0，这里给出 "start-end"。 */
  portRange: string
  fingerprint: string
  config: Record<string, unknown>
  enabled: boolean
  missing: boolean
  isNew: boolean
  tags: string[]
  inProfiles: number
}

export interface Template {
  id: number
  name: string
  target: string
  content: string
  lastCheck: string
}

export interface Profile {
  id: number
  name: string
  note: string
  templateId: number
  templateName: string
  enabled: boolean
  namingRule: NamingRule
  token: string
  lastFetchAt: string
  nodeCount: number
  url: string
  items?: ProfileItem[]
}

/** 编排选择项：单节点（kind=node）或整源（kind=source，渲染时展开）。 */
export interface ProfileItem {
  kind: 'node' | 'source'
  refId: number
  sourceType: '' | 'airport' | 'server'
  nameOverride: string
  sortOrder: number
  label: string
  protocol: string
  nodeCount: number
}

export interface NamingRule {
  segments: string[]
  separator: string
  zeroPad: number
  upperProtocol: boolean
}

export interface ProfileNode {
  nodeId: number
  nameOverride: string
  sortOrder: number
  rawName: string
  protocol: Protocol
  sourceType: 'airport' | 'server'
  sourceId: number
  sourceName: string
  outputName: string
}

export interface RefreshStats {
  added: number
  matched: number
  missing: number
  ignored: number
  errors: { line: number; reason: string }[]
}

export interface ImportPreview {
  parsed: { rawName: string; protocol: Protocol; host: string; port: number; portRange?: string }[]
  errors: { line: number; reason: string }[]
  ignored: number
  saved: number
}

export interface BackupResult {
  downloadId: string
  sha256: string
  sizeBytes: number
}
