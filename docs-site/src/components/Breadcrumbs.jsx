import { Link } from 'react-router-dom'

export default function Breadcrumbs({ page }) {
  const crumbs = [page.section, page.group].filter(Boolean)
  return (
    <nav aria-label="Breadcrumb" className="mb-6 flex flex-wrap items-center gap-1.5 text-[13px] text-slate-500">
      <Link to="/" className="hover:text-slate-900 dark:hover:text-slate-200">Docs</Link>
      {crumbs.map((c) => (
        <span key={c} className="flex items-center gap-1.5">
          <span className="text-slate-300 dark:text-slate-700">/</span>
          <span>{c}</span>
        </span>
      ))}
      <span className="text-slate-300 dark:text-slate-700">/</span>
      <span className="font-medium text-slate-900 dark:text-slate-200">{page.title}</span>
    </nav>
  )
}
