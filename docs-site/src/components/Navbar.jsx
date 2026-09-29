import { Link } from 'react-router-dom'
import { Logo } from './Brand.jsx'
import { useTheme } from '../lib/theme.jsx'

function SunMoon({ dark }) {
  return dark ? (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round"><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></svg>
  ) : (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round"><path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z" /></svg>
  )
}

export default function Navbar({ onMenu, onSearch }) {
  const { theme, toggle } = useTheme()
  return (
    <header className="sticky top-0 z-40 border-b border-slate-200 bg-white/80 backdrop-blur dark:border-slate-800/80 dark:bg-slate-950/80">
      <div className="mx-auto flex h-16 max-w-[90rem] items-center gap-3 px-4 sm:px-6">
        {onMenu && (
          <button type="button" onClick={onMenu} className="rounded-md p-2 text-slate-500 hover:bg-slate-100 lg:hidden dark:hover:bg-slate-900" aria-label="Open navigation">
            <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round"><path d="M4 6h16M4 12h16M4 18h16" /></svg>
          </button>
        )}
        <Link to="/" className="flex items-center gap-2.5">
          <Logo size={30} />
          <span className="text-[15px] font-semibold tracking-tight text-slate-900 dark:text-slate-50">
            Karim <span className="font-mono text-xs font-medium uppercase tracking-widest text-teal-600 dark:text-teal-400">MicroVM OS</span>
          </span>
        </Link>
        <div className="flex-1" />
        <button
          type="button"
          onClick={onSearch}
          className="flex w-full max-w-xs items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-3 py-1.5 text-sm text-slate-500 transition hover:border-slate-300 dark:border-slate-800 dark:bg-slate-900 dark:hover:border-slate-700"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round"><circle cx="11" cy="11" r="7" /><path d="M20 20l-3.5-3.5" /></svg>
          <span className="flex-1 text-left">Search docs…</span>
          <kbd className="hidden rounded border border-slate-300 px-1.5 font-mono text-[11px] sm:inline dark:border-slate-700">Ctrl K</kbd>
        </button>
        <button type="button" onClick={toggle} className="rounded-md p-2 text-slate-500 hover:bg-slate-100 hover:text-slate-900 dark:hover:bg-slate-900 dark:hover:text-slate-100" aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}>
          <SunMoon dark={theme === 'dark'} />
        </button>
      </div>
    </header>
  )
}
