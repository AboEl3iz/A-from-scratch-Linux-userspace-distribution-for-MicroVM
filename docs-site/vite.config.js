import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import mdx from '@mdx-js/rollup'
import remarkGfm from 'remark-gfm'
import rehypeSlug from 'rehype-slug'
import rehypeCodeMeta from './plugins/rehype-code-meta.js'

// DOCS_BASE is set by the GitHub Pages workflow to "/<repository>/".
export default defineConfig({
  base: process.env.DOCS_BASE || '/',
  plugins: [
    {
      enforce: 'pre',
      ...mdx({
        providerImportSource: '@mdx-js/react',
        remarkPlugins: [remarkGfm],
        rehypePlugins: [rehypeSlug, rehypeCodeMeta],
      }),
    },
    react({ include: /\.(jsx|js|mdx)$/ }),
  ],
  build: {
    chunkSizeWarningLimit: 1500,
  },
})
