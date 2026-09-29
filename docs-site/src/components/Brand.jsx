import logo from '../brand/logo.svg?raw'
import wordmark from '../brand/wordmark.svg?raw'
import supervisor from '../brand/icon-supervisor.svg?raw'
import security from '../brand/icon-security.svg?raw'
import observability from '../brand/icon-observability.svg?raw'
import networking from '../brand/icon-networking.svg?raw'
import storage from '../brand/icon-storage.svg?raw'
import rca from '../brand/icon-rca.svg?raw'
import network from '../brand/icon-network.svg?raw'
import overlay from '../brand/icon-overlay.svg?raw'
import builder from '../brand/icon-builder.svg?raw'
import kernel from '../brand/icon-kernel.svg?raw'
import snapshot from '../brand/icon-snapshot.svg?raw'
import testing from '../brand/icon-testing.svg?raw'
import reference from '../brand/icon-reference.svg?raw'

// The standalone SVG files in src/brand are the single source of truth; they
// are inlined so currentColor follows the surrounding text color and theme.
export const svgs = {
  logo, wordmark, supervisor, security, observability, networking, storage, rca,
  network, overlay, builder, kernel, snapshot, testing, reference,
}

function sized(svg, size) {
  return svg.replace(/\swidth="\d+"\sheight="\d+"/, ` width="${size}" height="${size}"`)
}

export function Icon({ name, size = 20, className = '' }) {
  const svg = svgs[name]
  if (!svg) return null
  return <span aria-hidden="true" className={`inline-flex shrink-0 ${className}`} dangerouslySetInnerHTML={{ __html: sized(svg, size) }} />
}

export function Logo({ size = 32, className = '' }) {
  return <Icon name="logo" size={size} className={className} />
}

export function Wordmark({ className = '' }) {
  return <span role="img" aria-label="Karim MicroVM OS" className={`inline-flex text-slate-900 dark:text-slate-50 ${className}`} dangerouslySetInnerHTML={{ __html: wordmark }} />
}

const iconCatalog = [
  ['supervisor', 'Supervisor & Init', 'icon-supervisor.svg'],
  ['security', 'Security & Isolation', 'icon-security.svg'],
  ['observability', 'eBPF Observability', 'icon-observability.svg'],
  ['networking', 'VSOCK Transport', 'icon-networking.svg'],
  ['network', 'Guest Networking', 'icon-network.svg'],
  ['storage', 'OCI Layers', 'icon-storage.svg'],
  ['overlay', 'Runtime Storage', 'icon-overlay.svg'],
  ['snapshot', 'Snapshots & QMP', 'icon-snapshot.svg'],
  ['builder', 'Image Builder', 'icon-builder.svg'],
  ['kernel', 'Kernel', 'icon-kernel.svg'],
  ['testing', 'Build & Test', 'icon-testing.svg'],
  ['reference', 'Reference', 'icon-reference.svg'],
  ['rca', 'Post-Mortems', 'icon-rca.svg'],
]

export function BrandGallery() {
  return (
    <div className="not-prose my-8 space-y-6">
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="flex flex-col items-center justify-center gap-3 rounded-xl border border-slate-200 bg-slate-50 p-8 dark:border-slate-800 dark:bg-slate-950">
          <Logo size={96} />
          <code className="text-xs text-slate-500">logo.svg</code>
        </div>
        <div className="flex flex-col items-center justify-center gap-3 rounded-xl border border-slate-200 bg-slate-50 p-8 dark:border-slate-800 dark:bg-slate-950">
          <Wordmark className="h-12 w-auto [&>svg]:h-12 [&>svg]:w-auto" />
          <code className="text-xs text-slate-500">wordmark.svg</code>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {iconCatalog.map(([name, title, file]) => (
          <div key={name} className="flex flex-col items-center gap-2 rounded-xl border border-slate-200 bg-white p-4 text-center dark:border-slate-800 dark:bg-slate-900">
            <Icon name={name} size={36} className="text-slate-700 dark:text-slate-200" />
            <span className="text-sm font-medium text-slate-900 dark:text-slate-100">{title}</span>
            <code className="text-[11px] text-slate-500">{file}</code>
          </div>
        ))}
      </div>
    </div>
  )
}
