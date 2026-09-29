import { Suspense, lazy, useEffect } from 'react'
import { Routes, Route, useLocation } from 'react-router-dom'
import { MDXProvider } from '@mdx-js/react'
import Layout from './components/Layout.jsx'
import Home from './components/Home.jsx'
import NotFound from './components/NotFound.jsx'
import { mdxComponents } from './components/MDXComponents.jsx'
import { allPages } from './nav.js'

const modules = import.meta.glob('./content/**/*.mdx')
const pageComponents = Object.fromEntries(
  allPages.map((p) => [p.path, lazy(modules[`./content/${p.file}`])]),
)

function ScrollManager() {
  const { pathname, hash } = useLocation()
  useEffect(() => {
    if (hash) {
      // Headings render after lazy MDX resolves; retry briefly.
      let tries = 0
      const tick = () => {
        const el = document.getElementById(decodeURIComponent(hash.slice(1)))
        if (el) el.scrollIntoView({ block: 'start' })
        else if (tries++ < 20) setTimeout(tick, 50)
      }
      tick()
    } else {
      window.scrollTo(0, 0)
    }
  }, [pathname, hash])
  return null
}

export default function App() {
  return (
    <MDXProvider components={mdxComponents}>
      <ScrollManager />
      <Routes>
        <Route path="/" element={<Home />} />
        {allPages.map((page) => {
          const Page = pageComponents[page.path]
          return (
            <Route
              key={page.path}
              path={page.path}
              element={
                <Layout page={page}>
                  <Suspense fallback={<div className="animate-pulse text-sm text-slate-500">Loading…</div>}>
                    <Page />
                  </Suspense>
                </Layout>
              }
            />
          )
        })}
        <Route path="*" element={<NotFound />} />
      </Routes>
    </MDXProvider>
  )
}
