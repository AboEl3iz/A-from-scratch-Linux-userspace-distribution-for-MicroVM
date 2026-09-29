import { useState } from 'react'
import { Highlight, themes } from 'prism-react-renderer'
import '../lib/prism-languages.js'
import { useTheme } from '../lib/theme.jsx'
import Mermaid from './Mermaid.jsx'

// Fence meta: ```go title="reaper.go" {3,7-9}
function parseMeta(meta = '') {
  const title = /title="([^"]+)"/.exec(meta)?.[1]
  const lines = new Set()
  const range = /\{([\d,\s-]+)\}/.exec(meta)?.[1]
  if (range) {
    for (const part of range.split(',')) {
      const [a, b] = part.trim().split('-').map(Number)
      for (let i = a; i <= (b || a); i++) lines.add(i)
    }
  }
  return { title, lines }
}

const aliases = { sh: 'bash', shell: 'bash', console: 'bash', golang: 'go', jsonl: 'json' }

export default function CodeBlock({ children, ...rest }) {
  const code = children?.props ?? rest
  const className = code.className || ''
  const raw = String(code.children ?? '').replace(/\n$/, '')
  const lang = (/language-(\S+)/.exec(className)?.[1] || 'text').toLowerCase()
  const { theme } = useTheme()
  const [copied, setCopied] = useState(false)

  if (lang === 'mermaid') return <Mermaid chart={raw} />

  const { title, lines } = parseMeta(code.metastring)
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(raw)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      /* clipboard unavailable (insecure context) */
    }
  }

  return (
    <div className="not-prose group relative my-6 overflow-hidden rounded-xl border border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/70">
      <div className="flex items-center justify-between border-b border-slate-200 px-4 py-2 text-xs dark:border-slate-800">
        <span className="font-mono text-slate-500">{title || lang}</span>
        <button
          type="button"
          onClick={copy}
          className="rounded-md px-2 py-1 font-medium text-slate-500 transition hover:bg-slate-200 hover:text-slate-900 dark:hover:bg-slate-800 dark:hover:text-slate-100"
          aria-label="Copy code to clipboard"
        >
          {copied ? 'Copied ✓' : 'Copy'}
        </button>
      </div>
      <Highlight code={raw} language={aliases[lang] || lang} theme={theme === 'dark' ? themes.nightOwl : themes.github}>
        {({ tokens, getLineProps, getTokenProps }) => (
          <pre className="overflow-x-auto py-4 font-mono text-[13px] leading-6" style={{ background: 'transparent' }}>
            {tokens.map((line, i) => {
              const hl = lines.has(i + 1)
              const { className: lc, ...lineProps } = getLineProps({ line })
              return (
                <div
                  key={i}
                  {...lineProps}
                  className={`${lc} px-4 ${hl ? 'border-l-2 border-teal-500 bg-teal-500/10' : 'border-l-2 border-transparent'}`}
                >
                  {line.map((token, j) => (
                    <span key={j} {...getTokenProps({ token })} />
                  ))}
                </div>
              )
            })}
          </pre>
        )}
      </Highlight>
    </div>
  )
}
