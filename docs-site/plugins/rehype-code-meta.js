import { visit } from 'unist-util-visit'

// MDX drops the fence "meta" string (```go title="x.go" {3-5}). Copy it onto
// the <code> element so the CodeBlock component can read titles and line
// highlights.
export default function rehypeCodeMeta() {
  return (tree) => {
    visit(tree, 'element', (node) => {
      if (node.tagName === 'code' && node.data && node.data.meta) {
        node.properties = { ...node.properties, metastring: node.data.meta }
      }
    })
  }
}
