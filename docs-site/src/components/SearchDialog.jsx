import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { loadSearch } from '../lib/search.js'

function snippet(text, matches) {
  const m = matches?.find((x) => x.key === 'text')?.indices?.[0]
  if (!m) return text.slice(0, 140)
  const start = Math.max(0, m[0] - 50)
  return (start > 0 ? '…' : '') + text.slice(start, start + 150) + '…'
}

export default function SearchDialog({ open, onClose }) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState([])
  const [selected, setSelected] = useState(0)
  const [fuse, setFuse] = useState(null)
  const input = useRef(null)
  const navigate = useNavigate()

  useEffect(() => {
    if (!open) return
    setQuery('')
    setSelected(0)
    loadSearch().then(setFuse)
    setTimeout(() => input.current?.focus(), 0)
  }, [open])

  useEffect(() => {
    if (!fuse || query.trim().length < 2) return setResults([])
    setResults(fuse.search(query.trim(), { limit: 12 }))
    setSelected(0)
  }, [query, fuse])

  if (!open) return null

  const go = (r) => {
    const { path, anchor } = r.item
    navigate(anchor ? `${path}#${anchor}` : path)
    onClose()
  }
  const onKey = (e) => {
    if (e.key === 'Escape') onClose()
    else if (e.key === 'ArrowDown') { e.preventDefault(); setSelected((s) => Math.min(s + 1, results.length - 1)) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setSelected((s) => Math.max(s - 1, 0)) }
    else if (e.key === 'Enter' && results[selected]) go(results[selected])
  }

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-slate-950/60 p-4 pt-[12vh] backdrop-blur-sm" onMouseDown={onClose}>
      <div role="dialog" aria-modal="true" aria-label="Search documentation" className="w-full max-w-xl overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-2xl dark:border-slate-800 dark:bg-slate-900" onMouseDown={(e) => e.stopPropagation()}>
        <div className="flex items-center gap-3 border-b border-slate-200 px-4 dark:border-slate-800">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" className="text-slate-400"><circle cx="11" cy="11" r="7" /><path d="M20 20l-3.5-3.5" /></svg>
          <input
            ref={input}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onKey}
            placeholder="Search: reaper, __data_loc, RESOLVE_IN_ROOT, snapshot…"
            className="h-12 flex-1 bg-transparent text-sm text-slate-900 outline-none placeholder:text-slate-400 dark:text-slate-100"
          />
          <kbd className="rounded border border-slate-300 px-1.5 font-mono text-[11px] text-slate-400 dark:border-slate-700">Esc</kbd>
        </div>
        <ul className="max-h-[60vh] overflow-y-auto p-2">
          {!fuse && <li className="p-4 text-sm text-slate-500">Building index…</li>}
          {fuse && query.trim().length >= 2 && results.length === 0 && <li className="p-4 text-sm text-slate-500">No results for “{query}”.</li>}
          {results.map((r, i) => (
            <li key={`${r.item.path}#${r.item.anchor}`}>
              <button
                type="button"
                onMouseEnter={() => setSelected(i)}
                onClick={() => go(r)}
                className={`w-full rounded-lg px-3 py-2.5 text-left ${i === selected ? 'bg-teal-500/10' : ''}`}
              >
                <div className="text-[11px] font-mono uppercase tracking-wider text-slate-400">{r.item.section} · {r.item.page}</div>
                <div className="text-sm font-medium text-slate-900 dark:text-slate-100">{r.item.heading}</div>
                <div className="mt-0.5 line-clamp-2 text-xs text-slate-500">{snippet(r.item.text, r.matches)}</div>
              </button>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}
