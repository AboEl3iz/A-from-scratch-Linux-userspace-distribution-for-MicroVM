// GitHub Pages has no SPA rewrites: serving index.html as 404.html lets deep
// links such as /subsystems/supervisor load the app, which then routes.
import { copyFileSync } from 'node:fs'
copyFileSync('dist/index.html', 'dist/404.html')
console.log('spa-fallback: dist/404.html written')
