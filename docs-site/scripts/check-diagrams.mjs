// Parses every ```mermaid block in src/content with Mermaid's own parser, so
// a syntax error fails CI instead of showing up as a broken diagram.
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { JSDOM } from 'jsdom'

const dom = new JSDOM('<!doctype html><html><body></body></html>', { pretendToBeVisual: true })
globalThis.window = dom.window
globalThis.document = dom.window.document
globalThis.DOMParser = dom.window.DOMParser
Object.defineProperty(globalThis, 'navigator', { value: dom.window.navigator, configurable: true })

const { default: mermaid } = await import('mermaid')
mermaid.initialize({ startOnLoad: false })

function* mdxFiles(dir) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) yield* mdxFiles(p)
    else if (p.endsWith('.mdx')) yield p
  }
}

let total = 0
let failed = 0
for (const file of mdxFiles('src/content')) {
  const src = readFileSync(file, 'utf8')
  for (const m of src.matchAll(/```mermaid\n([\s\S]*?)```/g)) {
    total++
    try {
      await mermaid.parse(m[1])
    } catch (e) {
      failed++
      console.error(`✗ ${file}: ${String(e.message || e).split('\n').slice(0, 3).join(' ')}`)
    }
  }
}
console.log(`${total - failed}/${total} diagrams parse`)
process.exit(failed ? 1 : 0)
