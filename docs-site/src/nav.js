// Information architecture: the single source of truth for routes, titles,
// sidebar structure, breadcrumbs, prev/next links and search labels.
export const sections = [
  {
    title: 'Introduction',
    icon: 'logo',
    pages: [
      { path: '/getting-started', title: 'Getting Started', file: 'getting-started.mdx' },
      { path: '/architecture/overview', title: 'Architecture Overview', file: 'architecture/overview.mdx' },
    ],
  },
  {
    title: 'Subsystems',
    icon: 'supervisor',
    groups: [
      {
        title: 'Runtime',
        pages: [
          { path: '/subsystems/supervisor', title: 'Supervisor Internals', file: 'subsystems/supervisor.mdx', icon: 'supervisor' },
          { path: '/subsystems/pid1-lifecycle', title: 'PID 1 & Lifecycle', file: 'subsystems/pid1-lifecycle.mdx', icon: 'supervisor' },
          { path: '/subsystems/security', title: 'Security & Isolation', file: 'subsystems/security.mdx', icon: 'security' },
        ],
      },
      {
        title: 'I/O & Storage',
        pages: [
          { path: '/subsystems/vsock-transport', title: 'VSOCK Transport', file: 'subsystems/vsock-transport.mdx', icon: 'networking' },
          { path: '/subsystems/oci-layers', title: 'OCI Layer Extraction', file: 'subsystems/oci-layers.mdx', icon: 'storage' },
          { path: '/subsystems/dynamic-oci-runtime', title: 'Dynamic OCI Runtime', file: 'subsystems/dynamic-oci-runtime.mdx', icon: 'storage' },
        ],
      },
      {
        title: 'Observability & Hypervisor',
        pages: [
          { path: '/subsystems/ebpf', title: 'eBPF Engine', file: 'subsystems/ebpf.mdx', icon: 'observability' },
          { path: '/subsystems/snapshots', title: 'Snapshots & QMP', file: 'subsystems/snapshots.mdx', icon: 'networking' },
        ],
      },
    ],
  },
  {
    title: 'Post-Mortems',
    icon: 'rca',
    pages: [
      { path: '/postmortems', title: 'Overview', file: 'postmortems/index.mdx', icon: 'rca' },
      { path: '/postmortems/pm-001-reaper-race', title: 'PM-001 Reaper race', file: 'postmortems/pm-001-reaper-race.mdx', icon: 'rca' },
      { path: '/postmortems/pm-002-pid1-panic', title: 'PM-002 PID 1 panics', file: 'postmortems/pm-002-pid1-panic.mdx', icon: 'rca' },
      { path: '/postmortems/pm-003-vsock-spin', title: 'PM-003 VSOCK CPU spin', file: 'postmortems/pm-003-vsock-spin.mdx', icon: 'rca' },
      { path: '/postmortems/pm-004-oci-traversal', title: 'PM-004 OCI traversal', file: 'postmortems/pm-004-oci-traversal.mdx', icon: 'rca' },
      { path: '/postmortems/pm-005-ebpf-dormant', title: 'PM-005 Dormant eBPF', file: 'postmortems/pm-005-ebpf-dormant.mdx', icon: 'rca' },
      { path: '/postmortems/pm-006-snapshots', title: 'PM-006 Phantom snapshots', file: 'postmortems/pm-006-snapshots.mdx', icon: 'rca' },
    ],
  },
  {
    title: 'Reference',
    icon: 'networking',
    pages: [
      { path: '/reference/cli', title: 'karim CLI', file: 'reference/cli.mdx' },
      { path: '/reference/service-config', title: 'Service Definitions', file: 'reference/service-config.mdx' },
      { path: '/reference/control-plane', title: 'Control-Plane API', file: 'reference/control-plane.mdx' },
    ],
  },
]

export function sectionPages(section) {
  return section.pages || section.groups.flatMap((g) => g.pages)
}

// Flat, ordered list with parent labels, for routing and prev/next.
export const allPages = sections.flatMap((section) =>
  (section.groups || [{ title: null, pages: section.pages }]).flatMap((group) =>
    group.pages.map((p) => ({ ...p, section: section.title, group: group.title })),
  ),
)

export function findPage(path) {
  return allPages.find((p) => p.path === path)
}
