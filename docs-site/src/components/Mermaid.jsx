import { useEffect, useId, useState } from 'react'
import { useTheme } from '../lib/theme.jsx'

// Mermaid is ~1 MB; load it only on pages that contain a diagram.
let mermaidPromise
function loadMermaid() {
  mermaidPromise ??= import('mermaid').then((m) => m.default)
  return mermaidPromise
}

const palettes = {
  dark: {
    background: '#020617', primaryColor: '#0f172a', primaryBorderColor: '#14b8a6', primaryTextColor: '#f8fafc',
    secondaryColor: '#1e1b4b', secondaryBorderColor: '#6366f1', tertiaryColor: '#0f172a', tertiaryBorderColor: '#334155',
    lineColor: '#64748b', textColor: '#cbd5e1', mainBkg: '#0f172a', nodeBorder: '#14b8a6', clusterBkg: '#0b1224',
    clusterBorder: '#334155', titleColor: '#f8fafc', edgeLabelBackground: '#0f172a', actorBkg: '#0f172a',
    actorBorder: '#6366f1', actorTextColor: '#f8fafc', signalColor: '#cbd5e1', signalTextColor: '#cbd5e1',
    labelBoxBkgColor: '#0f172a', labelBoxBorderColor: '#6366f1', noteBkgColor: '#1e293b', noteTextColor: '#e2e8f0',
    noteBorderColor: '#475569', activationBkgColor: '#134e4a', activationBorderColor: '#14b8a6', stateBkg: '#0f172a',
    stateLabelColor: '#f8fafc', nodeTextColor: '#f8fafc', labelColor: '#f8fafc', stateBorder: '#14b8a6',
    transitionColor: '#64748b', transitionLabelColor: '#cbd5e1', compositeBackground: '#0b1224', compositeTitleBackground: '#0f172a',
    innerEndBackground: '#14b8a6', specialStateColor: '#14b8a6', altBackground: '#0b1224',
  },
  light: {
    background: '#ffffff', primaryColor: '#f0fdfa', primaryBorderColor: '#0d9488', primaryTextColor: '#0f172a',
    secondaryColor: '#eef2ff', secondaryBorderColor: '#6366f1', tertiaryColor: '#f8fafc', tertiaryBorderColor: '#cbd5e1',
    lineColor: '#64748b', textColor: '#334155', mainBkg: '#f0fdfa', nodeBorder: '#0d9488', clusterBkg: '#f8fafc',
    clusterBorder: '#cbd5e1', titleColor: '#0f172a', edgeLabelBackground: '#ffffff', actorBkg: '#eef2ff',
    actorBorder: '#6366f1', actorTextColor: '#0f172a', signalColor: '#334155', signalTextColor: '#334155',
    labelBoxBkgColor: '#eef2ff', labelBoxBorderColor: '#6366f1', noteBkgColor: '#fefce8', noteTextColor: '#334155',
    noteBorderColor: '#e2e8f0', activationBkgColor: '#ccfbf1', activationBorderColor: '#0d9488', stateBkg: '#f0fdfa',
    stateLabelColor: '#0f172a', nodeTextColor: '#0f172a', labelColor: '#0f172a', stateBorder: '#0d9488',
    transitionColor: '#64748b', transitionLabelColor: '#334155', compositeBackground: '#f8fafc', compositeTitleBackground: '#f0fdfa',
    innerEndBackground: '#0d9488', specialStateColor: '#0d9488', altBackground: '#f8fafc',
  },
}

let renderQueue = Promise.resolve()

export default function Mermaid({ chart, caption }) {
  const { theme } = useTheme()
  const id = 'mmd-' + useId().replace(/[^a-zA-Z0-9]/g, '')
  const [svg, setSvg] = useState('')
  const [error, setError] = useState(null)

  useEffect(() => {
    let cancelled = false
    // mermaid.render uses global state: serialize renders across diagrams.
    renderQueue = renderQueue.then(async () => {
      try {
        const mermaid = await loadMermaid()
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: 'strict',
          theme: 'base',
          fontFamily: 'Inter, ui-sans-serif, system-ui, sans-serif',
          themeVariables: { ...palettes[theme], fontSize: '14px' },
          flowchart: { curve: 'basis', padding: 14 },
          sequence: { mirrorActors: false },
        })
        const { svg } = await mermaid.render(`${id}-${theme}`, chart.trim())
        if (!cancelled) {
          setSvg(svg)
          setError(null)
        }
      } catch (e) {
        if (!cancelled) setError(String(e?.message || e))
      }
    })
    return () => {
      cancelled = true
    }
  }, [chart, theme, id])

  return (
    <figure className="not-prose my-8">
      <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-950">
        {error ? (
          <pre className="whitespace-pre-wrap text-sm text-rose-500">Diagram error: {error}</pre>
        ) : svg ? (
          <div className="flex justify-center [&_svg]:h-auto [&_svg]:max-w-full" dangerouslySetInnerHTML={{ __html: svg }} />
        ) : (
          <div className="h-40 animate-pulse rounded-lg bg-slate-100 dark:bg-slate-900" />
        )}
      </div>
      {caption && <figcaption className="mt-2 text-center text-sm text-slate-500">{caption}</figcaption>}
    </figure>
  )
}
