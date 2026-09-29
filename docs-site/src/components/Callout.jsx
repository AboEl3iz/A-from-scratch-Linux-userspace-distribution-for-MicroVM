import { Icon } from './Brand.jsx'

const styles = {
  note: { box: 'border-sky-500/40 bg-sky-500/5', title: 'text-sky-600 dark:text-sky-400', icon: 'observability', label: 'Note' },
  success: { box: 'border-teal-500/40 bg-teal-500/5', title: 'text-teal-600 dark:text-teal-400', icon: 'supervisor', label: 'Verified' },
  warning: { box: 'border-rose-500/40 bg-rose-500/5', title: 'text-rose-600 dark:text-rose-400', icon: 'rca', label: 'Warning' },
  open: { box: 'border-amber-500/40 bg-amber-500/5', title: 'text-amber-600 dark:text-amber-400', icon: 'rca', label: 'Open issue' },
  design: { box: 'border-indigo-500/40 bg-indigo-500/5', title: 'text-indigo-600 dark:text-indigo-400', icon: 'security', label: 'Design note' },
}

export default function Callout({ type = 'note', title, children }) {
  const s = styles[type] || styles.note
  return (
    <aside className={`not-prose my-6 rounded-xl border-l-4 px-5 py-4 ${s.box}`}>
      <div className={`mb-1 flex items-center gap-2 text-sm font-semibold ${s.title}`}>
        <Icon name={s.icon} size={16} />
        {title || s.label}
      </div>
      <div className="text-sm leading-6 text-slate-700 dark:text-slate-300 [&_code]:rounded [&_code]:bg-slate-200/60 [&_code]:px-1 [&_code]:font-mono [&_code]:text-[0.85em] dark:[&_code]:bg-slate-800 [&_p]:my-1">
        {children}
      </div>
    </aside>
  )
}
