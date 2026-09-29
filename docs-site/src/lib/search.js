import Fuse from 'fuse.js'
import { allPages } from '../nav.js'

// The index is built lazily in the browser from the raw MDX sources, split
// into one record per heading, so results deep-link to the right section.
let fusePromise

function slugify(text) {
  // Mirrors github-slugger (used by rehype-slug) for ordinary headings.
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\p{L}\p{N}\s_-]/gu, '')
    .replace(/\s/g, '-')
}

function plainText(md) {
  return md
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/<[^>]+>/g, ' ')
    .replace(/^import .*$/gm, ' ')
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    .replace(/[`*_>#|]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function toRecords(page, source) {
  const records = []
  let heading = page.title
  let anchor = ''
  let buf = []
  const flush = () => {
    const text = plainText(buf.join('\n'))
    if (text || anchor === '') records.push({ page: page.title, section: page.section, path: page.path, heading, anchor, text })
    buf = []
  }
  for (const line of source.split('\n')) {
    const m = /^(#{2,3})\s+(.+)$/.exec(line)
    if (m) {
      flush()
      heading = m[2].replace(/`/g, '')
      anchor = slugify(heading)
    } else if (!/^#\s/.test(line)) {
      buf.push(line)
    }
  }
  flush()
  return records
}

export function loadSearch() {
  if (!fusePromise) {
    const raw = import.meta.glob('../content/**/*.mdx', { query: '?raw', import: 'default' })
    fusePromise = Promise.all(
      allPages.map(async (page) => toRecords(page, await raw[`../content/${page.file}`]())),
    ).then(
      (lists) =>
        new Fuse(lists.flat(), {
          keys: [
            { name: 'heading', weight: 3 },
            { name: 'page', weight: 2 },
            { name: 'text', weight: 1 },
          ],
          includeMatches: true,
          ignoreLocation: true,
          threshold: 0.35,
          minMatchCharLength: 2,
        }),
    )
  }
  return fusePromise
}
