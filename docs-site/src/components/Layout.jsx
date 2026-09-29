import { useEffect, useState } from 'react'
import Navbar from './Navbar.jsx'
import Sidebar from './Sidebar.jsx'
import TableOfContents from './TableOfContents.jsx'
import Breadcrumbs from './Breadcrumbs.jsx'
import PageNav from './PageNav.jsx'
import SearchDialog from './SearchDialog.jsx'

export function useSearchHotkey(setOpen) {
  useEffect(() => {
    const onKey = (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setOpen(true)
      } else if (e.key === '/' && !['INPUT', 'TEXTAREA'].includes(document.activeElement?.tagName)) {
        e.preventDefault()
        setOpen(true)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [setOpen])
}

export default function Layout({ page, children }) {
  const [menu, setMenu] = useState(false)
  const [search, setSearch] = useState(false)
  useSearchHotkey(setSearch)
  useEffect(() => {
    document.title = `${page.title} · Karim MicroVM OS`
    setMenu(false)
  }, [page])

  return (
    <div className="min-h-screen">
      <Navbar onMenu={() => setMenu(true)} onSearch={() => setSearch(true)} />
      <SearchDialog open={search} onClose={() => setSearch(false)} />

      {menu && (
        <div className="fixed inset-0 z-50 lg:hidden" onClick={() => setMenu(false)}>
          <div className="absolute inset-0 bg-slate-950/60" />
          <div className="absolute inset-y-0 left-0 w-72 overflow-y-auto bg-white p-6 dark:bg-slate-950" onClick={(e) => e.stopPropagation()}>
            <Sidebar currentPath={page.path} onNavigate={() => setMenu(false)} />
          </div>
        </div>
      )}

      <div className="mx-auto flex max-w-[90rem] px-4 sm:px-6">
        <aside className="sticky top-16 hidden h-[calc(100vh-4rem)] w-64 shrink-0 overflow-y-auto border-r border-slate-200 py-8 pr-6 lg:block dark:border-slate-800/80">
          <Sidebar currentPath={page.path} />
        </aside>
        <main className="min-w-0 flex-1 py-10 lg:px-12">
          <Breadcrumbs page={page} />
          <article className="prose prose-karim prose-slate max-w-3xl dark:prose-invert prose-headings:scroll-mt-20 prose-headings:tracking-tight prose-a:no-underline hover:prose-a:underline prose-table:block prose-table:overflow-x-auto">
            {children}
          </article>
          <div className="max-w-3xl">
            <PageNav page={page} />
          </div>
        </main>
        <aside className="sticky top-16 hidden h-[calc(100vh-4rem)] w-60 shrink-0 overflow-y-auto py-10 xl:block">
          <TableOfContents contentKey={page.path} />
        </aside>
      </div>
    </div>
  )
}
