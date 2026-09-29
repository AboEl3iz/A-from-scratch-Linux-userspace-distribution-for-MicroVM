import { Link } from 'react-router-dom'
import { allPages } from '../nav.js'

export default function PageNav({ page }) {
  const i = allPages.findIndex((p) => p.path === page.path)
  const prev = allPages[i - 1]
  const next = allPages[i + 1]
  const card = 'group flex-1 rounded-xl border border-slate-200 p-4 transition hover:border-teal-500/60 dark:border-slate-800'
  return (
    <div className="mt-16 flex gap-4 border-t border-slate-200 pt-8 dark:border-slate-800">
      {prev ? (
        <Link to={prev.path} className={card}>
          <div className="text-xs text-slate-500">← Previous</div>
          <div className="mt-1 font-medium text-slate-900 group-hover:text-teal-600 dark:text-slate-100 dark:group-hover:text-teal-400">{prev.title}</div>
        </Link>
      ) : <div className="flex-1" />}
      {next ? (
        <Link to={next.path} className={`${card} text-right`}>
          <div className="text-xs text-slate-500">Next →</div>
          <div className="mt-1 font-medium text-slate-900 group-hover:text-teal-600 dark:text-slate-100 dark:group-hover:text-teal-400">{next.title}</div>
        </Link>
      ) : <div className="flex-1" />}
    </div>
  )
}
