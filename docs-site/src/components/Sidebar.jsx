import { useState } from 'react'
import { NavLink } from 'react-router-dom'
import { sections, sectionPages } from '../nav.js'
import { Icon } from './Brand.jsx'

function PageLink({ page, onNavigate }) {
  return (
    <NavLink
      to={page.path}
      end
      onClick={onNavigate}
      className={({ isActive }) =>
        `-ml-px block border-l py-1.5 pl-4 text-sm transition ${
          isActive
            ? 'border-teal-500 font-medium text-teal-700 dark:text-teal-400'
            : 'border-transparent text-slate-600 hover:border-slate-400 hover:text-slate-900 dark:text-slate-400 dark:hover:border-slate-600 dark:hover:text-slate-200'
        }`
      }
    >
      {page.title}
    </NavLink>
  )
}

function Section({ section, currentPath, onNavigate }) {
  const containsCurrent = sectionPages(section).some((p) => p.path === currentPath)
  const [open, setOpen] = useState(true)
  return (
    <div className="mb-6">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="mb-2 flex w-full items-center gap-2 text-left text-[13px] font-semibold text-slate-900 dark:text-slate-100"
        aria-expanded={open}
      >
        <Icon name={section.icon} size={16} className={containsCurrent ? 'text-teal-500' : 'text-slate-400'} />
        <span className="flex-1">{section.title}</span>
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" className={`text-slate-400 transition ${open ? 'rotate-90' : ''}`}><path d="M9 6l6 6-6 6" /></svg>
      </button>
      {open && (
        <div className="border-l border-slate-200 dark:border-slate-800">
          {section.groups
            ? section.groups.map((g) => (
                <div key={g.title} className="mb-2">
                  <div className="py-1.5 pl-4 font-mono text-[11px] uppercase tracking-wider text-slate-400 dark:text-slate-500">{g.title}</div>
                  {g.pages.map((p) => <PageLink key={p.path} page={p} onNavigate={onNavigate} />)}
                </div>
              ))
            : section.pages.map((p) => <PageLink key={p.path} page={p} onNavigate={onNavigate} />)}
        </div>
      )}
    </div>
  )
}

export default function Sidebar({ currentPath, onNavigate }) {
  return (
    <nav aria-label="Documentation">
      {sections.map((s) => (
        <Section key={s.title} section={s} currentPath={currentPath} onNavigate={onNavigate} />
      ))}
    </nav>
  )
}
