import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import Navbar from './Navbar.jsx'
import SearchDialog from './SearchDialog.jsx'
import { Icon, Logo } from './Brand.jsx'
import { useSearchHotkey } from './Layout.jsx'

const subsystems = [
  { icon: 'supervisor', title: 'Supervisor & Init', to: '/subsystems/supervisor', text: 'A single WNOWAIT reaper, one actor per service, desired-state restarts and a C init that never exits.' },
  { icon: 'security', title: 'Security & Isolation', to: '/subsystems/security', text: 'karim-secd seccomp and capability wrapper, unprivileged service identities and an authenticated control plane.' },
  { icon: 'observability', title: 'eBPF Engine', to: '/subsystems/ebpf', text: 'tp_btf probes with real CO-RE, a single ring-buffer reader and metrics that never fabricate data.' },
  { icon: 'networking', title: 'VSOCK Transport', to: '/subsystems/vsock-transport', text: 'AF_VSOCK on Go’s netpoller: correct EOF, deadlines, close-unblocks and connection limits.' },
  { icon: 'storage', title: 'OCI Layer Extraction', to: '/subsystems/oci-layers', text: 'os.Root-confined extraction, in-root symlink resolution, digest pinning and mksquashfs pseudo metadata.' },
  { icon: 'rca', title: 'Post-Mortems', to: '/postmortems', text: 'Six root-cause analyses: what broke, why, how it was fixed and how the fix was proven.' },
]

export default function Home() {
  const [search, setSearch] = useState(false)
  useSearchHotkey(setSearch)
  useEffect(() => {
    document.title = 'Karim MicroVM OS Documentation'
  }, [])
  return (
    <div className="min-h-screen">
      <Navbar onSearch={() => setSearch(true)} />
      <SearchDialog open={search} onClose={() => setSearch(false)} />
      <section className="relative overflow-hidden border-b border-slate-200 dark:border-slate-800">
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_top,rgba(20,184,166,0.12),transparent_60%)]" />
        <div className="relative mx-auto max-w-6xl px-6 py-20 sm:py-28">
          <Logo size={72} />
          <h1 className="mt-8 max-w-3xl text-4xl font-bold tracking-tight text-slate-900 sm:text-5xl dark:text-slate-50">
            A small, supervised, observable MicroVM operating system.
          </h1>
          <p className="mt-6 max-w-2xl text-lg leading-8 text-slate-600 dark:text-slate-400">
            Karim boots a Linux 6.6 guest under QEMU/KVM with a static C init, a Go supervisor with cgroup v2 isolation,
            an AF_VSOCK control plane, CO-RE eBPF telemetry and transactional snapshots — all driven by one host CLI.
          </p>
          <div className="mt-10 flex flex-wrap gap-3">
            <Link to="/getting-started" className="rounded-lg bg-teal-600 px-5 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:bg-teal-500">Get started</Link>
            <Link to="/architecture/overview" className="rounded-lg border border-slate-300 px-5 py-2.5 text-sm font-semibold text-slate-900 transition hover:border-teal-500 dark:border-slate-700 dark:text-slate-100">Architecture overview</Link>
            <Link to="/reference/cli" className="rounded-lg px-5 py-2.5 text-sm font-semibold text-slate-600 transition hover:text-teal-600 dark:text-slate-400 dark:hover:text-teal-400">CLI reference →</Link>
          </div>
          <pre className="mt-12 max-w-2xl overflow-x-auto rounded-xl border border-slate-200 bg-slate-50 p-5 font-mono text-[13px] leading-6 text-slate-700 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-300">
{`$ make kernel all && make run
$ karim --target vsock://3:1024 ps
SERVICE     STATE     PID  EXEC             MEMORY  SECCOMP      RESTART
kv_store    RUNNING   84   /bin/kv_store    16MB    net-service  always
karim-obsd  RUNNING   79   /sbin/karim-obsd 64MB    unrestricted always`}
          </pre>
        </div>
      </section>
      <section className="mx-auto max-w-6xl px-6 py-16">
        <h2 className="font-mono text-xs font-semibold uppercase tracking-widest text-slate-500">Subsystems</h2>
        <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {subsystems.map((s) => (
            <Link key={s.title} to={s.to} className="group rounded-2xl border border-slate-200 p-6 transition hover:-translate-y-0.5 hover:border-teal-500/60 dark:border-slate-800 dark:hover:border-teal-500/50">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-slate-100 text-slate-700 dark:bg-slate-900 dark:text-slate-200">
                <Icon name={s.icon} size={22} />
              </div>
              <h3 className="mt-4 font-semibold text-slate-900 group-hover:text-teal-600 dark:text-slate-50 dark:group-hover:text-teal-400">{s.title}</h3>
              <p className="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-400">{s.text}</p>
            </Link>
          ))}
        </div>
      </section>
      <footer className="border-t border-slate-200 py-8 text-center text-xs text-slate-500 dark:border-slate-800">
        Karim MicroVM OS documentation · MIT licensed
      </footer>
    </div>
  )
}
