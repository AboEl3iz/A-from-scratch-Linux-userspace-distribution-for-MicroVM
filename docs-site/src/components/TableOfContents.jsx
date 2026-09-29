import { useEffect, useState } from 'react'

// Reads h2/h3 from the rendered article (ids come from rehype-slug) and
// highlights the heading currently in view.
export default function TableOfContents({ contentKey }) {
  const [items, setItems] = useState([])
  const [active, setActive] = useState('')

  useEffect(() => {
    let observer
    const collect = () => {
      const nodes = [...document.querySelectorAll('article h2[id], article h3[id]')]
      setItems(nodes.map((n) => ({ id: n.id, text: n.textContent.replace(/#$/, ''), level: n.tagName === 'H3' ? 3 : 2 })))
      observer = new IntersectionObserver(
        (entries) => {
          const visible = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
          if (visible[0]) setActive(visible[0].target.id)
        },
        { rootMargin: '-80px 0px -70% 0px' },
      )
      nodes.forEach((n) => observer.observe(n))
    }
    // Lazy MDX: wait for the article to render.
    const article = document.querySelector('article')
    const mo = new MutationObserver(() => {
      observer?.disconnect()
      collect()
    })
    if (article) mo.observe(article, { childList: true, subtree: true })
    collect()
    return () => {
      mo.disconnect()
      observer?.disconnect()
    }
  }, [contentKey])

  if (items.length === 0) return null
  return (
    <nav aria-label="On this page" className="text-sm">
      <div className="mb-3 font-mono text-[11px] font-semibold uppercase tracking-wider text-slate-500">On this page</div>
      <ul className="space-y-1.5">
        {items.map((it) => (
          <li key={it.id} className={it.level === 3 ? 'pl-3' : ''}>
            <a
              href={`#${it.id}`}
              className={`block leading-5 transition ${active === it.id ? 'text-teal-600 dark:text-teal-400' : 'text-slate-500 hover:text-slate-900 dark:hover:text-slate-200'}`}
            >
              {it.text}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  )
}
