# Karim MicroVM OS — documentation site

React 18 + Vite + Tailwind CSS + MDX, with Mermaid diagrams, local fuzzy search (Fuse.js), dark/light themes and GitHub Pages deployment.

```bash
npm ci                 # install exact dependencies (package-lock.json)
npm run dev            # http://localhost:5173 with hot reload
npm run check:diagrams # parse every Mermaid diagram (also run in CI)
npm run build          # production build in dist/ (+ 404.html SPA fallback)
npm run preview        # serve dist/ locally
```

For a GitHub *project* page the site lives under `/<repository>/`; build with `DOCS_BASE=/<repository>/ npm run build`. The deploy workflow sets this automatically.

## Layout

```text
src/brand/        logo.svg, wordmark.svg, icon-*.svg (single source for all brand assets)
src/content/      MDX pages
src/nav.js        sidebar, routes, breadcrumbs, prev/next
src/components/   Layout, Navbar, Sidebar, TableOfContents, SearchDialog, CodeBlock, Mermaid, Diagrams
plugins/          rehype plugin passing code-fence meta (title="…", {1,3-5}) to CodeBlock
scripts/          SPA fallback and Mermaid validation
```

Add a page: create `src/content/<section>/<name>.mdx` and register it in `src/nav.js`.
Code fences accept `title="file.go"` and line highlights `{3,7-9}`; a `mermaid` fence renders a diagram.
