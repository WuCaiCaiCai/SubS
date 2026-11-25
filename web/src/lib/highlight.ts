// 轻量代码着色：只做 YAML / JSON / INI 三种，够模板与配置预览用。
// 不引第三方高亮库 —— 规则集中在这里，样式由 style.css 的 tok-* 类控制，
// 切主题时颜色自动跟着变。

export type Lang = 'yaml' | 'json' | 'ini'

const ESCAPES: Record<string, string> = {
  '&': '\u0026amp;',
  '<': '\u0026lt;',
  '>': '\u0026gt;',
  '"': '\u0026quot;',
  "'": '\u0026#39;',
}

export function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ESCAPES[c])
}

function span(cls: string, text: string): string {
  return `<span class="${cls}">${escapeHtml(text)}</span>`
}

const NUM = /^-?\d+(\.\d+)?$/
const BOOL = /^(true|false|yes|no|on|off|null|~)$/i

/** 行内值（YAML 右值 / INI 右值）着色 */
function valueTokens(raw: string, lang: Lang): string {
  let out = ''
  let i = 0
  while (i < raw.length) {
    const ch = raw[i]
    // 行内注释：# 前必须是空白
    if (ch === '#' && (i === 0 || /\s/.test(raw[i - 1]))) {
      out += span('tok-comment', raw.slice(i))
      break
    }
    if (ch === '"' || ch === "'") {
      const end = raw.indexOf(ch, i + 1)
      if (end === -1) {
        out += span('tok-string', raw.slice(i))
        break
      }
      out += span('tok-string', raw.slice(i, end + 1))
      i = end + 1
      continue
    }
    if (ch === '&' || ch === '*') {
      const m = /^[&*][\w.-]+/.exec(raw.slice(i))
      if (m) {
        out += span('tok-bool', m[0])
        i += m[0].length
        continue
      }
    }
    if (/[\w]/.test(ch)) {
      let j = i
      while (j < raw.length && /[\w.-]/.test(raw[j])) j++
      const word = raw.slice(i, j)
      if (NUM.test(word)) out += span('tok-number', word)
      else if (BOOL.test(word)) out += span('tok-bool', word)
      else out += escapeHtml(word)
      i = j
      continue
    }
    if (/\s/.test(ch)) {
      out += escapeHtml(ch)
      i++
      continue
    }
    out += span('tok-punct', ch)
    i++
  }
  void lang
  return out
}

/** 找出 YAML 里「键: 值」的分隔位置：冒号后必须是空白或行尾。 */
function splitKeyValue(rest: string): { key: string; value: string } | null {
  let inSingle = false
  let inDouble = false
  for (let i = 0; i < rest.length; i++) {
    const c = rest[i]
    if (c === "'" && !inDouble) inSingle = !inSingle
    else if (c === '"' && !inSingle) inDouble = !inDouble
    else if (c === ':' && !inSingle && !inDouble) {
      const next = rest[i + 1]
      if (next === undefined || next === ' ' || next === '\t') {
        return { key: rest.slice(0, i), value: rest.slice(i + 1) }
      }
    }
  }
  return null
}

function yamlLine(line: string): string {
  const indent = /^\s*/.exec(line)?.[0] ?? ''
  const rest = line.slice(indent.length)
  if (!rest) return escapeHtml(line)

  if (rest.startsWith('#')) {
    const cls = /subs:/i.test(rest) ? 'tok-marker' : 'tok-comment'
    return escapeHtml(indent) + span(cls, rest)
  }

  let prefix = ''
  let body = rest
  if (body === '-' || body.startsWith('- ')) {
    prefix = span('tok-punct', '-')
    body = body === '-' ? '' : body.slice(2)
    if (body && !body.startsWith(' ')) {
      // "-name" 这种紧贴写法
      return escapeHtml(indent) + prefix + escapeHtml(body)
    }
  }

  const kv = splitKeyValue(body)
  if (!kv) {
    return escapeHtml(indent) + prefix + valueTokens(body, 'yaml')
  }
  const keyHtml = /^(["']).*\1$/.test(kv.key.trim())
    ? span('tok-string', kv.key)
    : span('tok-key', kv.key)
  const value = kv.value
  const lead = /^\s*/.exec(value)?.[0] ?? ''
  const restValue = value.slice(lead.length)
  return (
    escapeHtml(indent) +
    prefix +
    keyHtml +
    span('tok-punct', ':') +
    escapeHtml(lead) +
    valueTokens(restValue, 'yaml')
  )
}

function iniLine(line: string): string {
  const indent = /^\s*/.exec(line)?.[0] ?? ''
  const rest = line.slice(indent.length)
  if (!rest) return escapeHtml(line)
  if (rest.startsWith('#') || rest.startsWith(';')) return escapeHtml(indent) + span('tok-comment', rest)
  if (rest.startsWith('[') && rest.endsWith(']')) {
    return escapeHtml(indent) + span('tok-marker', rest)
  }
  const eq = rest.indexOf('=')
  if (eq === -1) return escapeHtml(indent) + valueTokens(rest, 'ini')
  return (
    escapeHtml(indent) +
    span('tok-key', rest.slice(0, eq)) +
    span('tok-punct', '=') +
    valueTokens(rest.slice(eq + 1), 'ini')
  )
}

function jsonCode(code: string): string {
  const re = /("(?:\\.|[^"\\])*")|(\b-?\d+(?:\.\d+)?\b)|\b(true|false|null)\b|([{}[\],:])|(\/\/[^\n]*|#[^\n]*)/g
  let out = ''
  let last = 0
  let m: RegExpExecArray | null
  while ((m = re.exec(code))) {
    out += escapeHtml(code.slice(last, m.index))
    last = m.index + m[0].length
    if (m[1]) {
      // 后面紧跟冒号的是键
      const after = code.slice(last).match(/^\s*:/)
      out += span(after ? 'tok-key' : 'tok-string', m[1])
    } else if (m[2]) out += span('tok-number', m[2])
    else if (m[3]) out += span('tok-bool', m[3])
    else if (m[4]) out += span('tok-punct', m[4])
    else if (m[5]) out += span('tok-comment', m[5])
  }
  out += escapeHtml(code.slice(last))
  return out
}

export function highlight(code: string, lang: Lang): string {
  if (lang === 'json') return jsonCode(code)
  const lines = code.split('\n')
  const html = lines.map((line) => (lang === 'ini' ? iniLine(line) : yamlLine(line))).join('\n')
  return html
}
