// Inline, responsive SVG diagrams. Colors come from Tailwind classes so they
// follow the light/dark theme.
const box = 'fill-white stroke-slate-300 dark:fill-slate-900 dark:stroke-slate-700'
const label = 'fill-slate-900 dark:fill-slate-100'
const sub = 'fill-slate-500 dark:fill-slate-400'

function Frame({ children, caption, viewBox }) {
  return (
    <figure className="not-prose my-8">
      <div className="rounded-xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-950">
        <svg viewBox={viewBox} className="h-auto w-full" role="img" aria-label={caption} fontFamily="Inter, ui-sans-serif, system-ui, sans-serif">
          {children}
        </svg>
      </div>
      {caption && <figcaption className="mt-2 text-center text-sm text-slate-500">{caption}</figcaption>}
    </figure>
  )
}

function Node({ x, y, w, h, title, detail, accent = 'stroke-teal-500' }) {
  return (
    <g>
      <rect x={x} y={y} width={w} height={h} rx="10" className={`${box} ${accent}`} strokeWidth="1.5" />
      <text x={x + 14} y={y + 24} fontSize="14" fontWeight="600" className={label}>{title}</text>
      {detail && detail.map((d, i) => (
        <text key={i} x={x + 14} y={y + 44 + i * 17} fontSize="12" className={sub}>{d}</text>
      ))}
    </g>
  )
}

export function TopologyDiagram() {
  return (
    <Frame viewBox="0 0 860 470" caption="Host/guest boundary: the only guest-facing channels are AF_VSOCK (control plane) and QMP (hypervisor).">
      <defs>
        <marker id="arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
          <path d="M0 0L10 5L0 10z" className="fill-slate-400" />
        </marker>
      </defs>
      {/* Host */}
      <rect x="10" y="10" width="250" height="450" rx="16" className="fill-indigo-500/5 stroke-indigo-500/60" strokeDasharray="6 5" strokeWidth="1.5" />
      <text x="28" y="40" fontSize="12" fontFamily="JetBrains Mono, monospace" className="fill-indigo-500" letterSpacing="2">HOST</text>
      <Node x={28} y={60} w={214} h={86} title="karim CLI" detail={['ps · stop · logs · trace', 'apply · run · shutdown']} accent="stroke-indigo-500" />
      <Node x={28} y={170} w={214} h={86} title="Snapshot orchestrator" detail={['saga: freeze → save → thaw', 'QMP client (id demux)']} accent="stroke-indigo-500" />
      <Node x={28} y={280} w={214} h={86} title="QEMU / KVM" detail={['vhost-vsock-pci (CID 3)', 'vmgenid · virtio-blk (ro)']} accent="stroke-indigo-500" />
      <text x="28" y="400" fontSize="12" className={sub}>QMP: unix:/tmp/qmp.sock</text>
      <text x="28" y="420" fontSize="12" className={sub}>Images: bzImage · initramfs.cpio</text>
      <text x="28" y="440" fontSize="12" className={sub}>rootfs.sqsh (read-only)</text>

      {/* Guest */}
      <rect x="320" y="10" width="530" height="450" rx="16" className="fill-teal-500/5 stroke-teal-500/60" strokeWidth="1.5" />
      <text x="338" y="40" fontSize="12" fontFamily="JetBrains Mono, monospace" className="fill-teal-500" letterSpacing="2">GUEST MICROVM</text>
      <Node x={338} y={60} w={236} h={86} title="karim-init (PID 1, C)" detail={['reaps orphans · restarts svcd', 'final sync + reboot(2)']} />
      <Node x={596} y={60} w={236} h={86} title="Control plane" detail={['AF_VSOCK :1024 (host CID 2)', 'unix /run/karim/vsock.sock 0600']} />
      <Node x={338} y={170} w={494} h={86} title="karim-svcd (child subreaper)" detail={['single Reaper · Supervisor registry · one actor per service', 'cgroup v2 manager · power button · netd · stored']} />
      <Node x={338} y={280} w={152} h={86} title="karim-secd" detail={['NNP · caps', 'seccomp → exec']} accent="stroke-rose-500" />
      <Node x={509} y={280} w={152} h={86} title="services" detail={['user = nobody', 'cgroup per service']} />
      <Node x={680} y={280} w={152} h={86} title="karim-obsd" detail={['tp_btf probes', 'ring buffer · :9100']} accent="stroke-sky-500" />
      <rect x="338" y="390" width="494" height="52" rx="10" className="fill-slate-100 stroke-slate-300 dark:fill-slate-900 dark:stroke-slate-700" strokeWidth="1.5" />
      <text x="354" y="421" fontSize="13" fontWeight="600" className={label}>Linux 6.6.45 · BTF · cgroup v2 · seccomp · tracepoints · vmgenid</text>

      <path d="M242 90 H290 V52 H714 V60" className="stroke-slate-400" strokeWidth="1.5" markerEnd="url(#arr)" fill="none" />
      <text x="530" y="46" fontSize="11" className={sub}>framed JSON RPC over AF_VSOCK</text>
      <path d="M242 213 C300 213 290 323 262 323" className="stroke-slate-400" strokeWidth="1.5" markerEnd="url(#arr)" fill="none" />
      <text x="268" y="275" fontSize="11" className={sub}>QMP</text>
      <path d="M456 146 V170" className="stroke-slate-400" strokeWidth="1.5" markerEnd="url(#arr)" fill="none" />
      <path d="M414 256 V280" className="stroke-slate-400" strokeWidth="1.5" markerEnd="url(#arr)" fill="none" />
      <path d="M585 256 V280" className="stroke-slate-400" strokeWidth="1.5" markerEnd="url(#arr)" fill="none" />
      <path d="M756 256 V280" className="stroke-slate-400" strokeWidth="1.5" markerEnd="url(#arr)" fill="none" />
      <path d="M714 146 V170" className="stroke-slate-400" strokeWidth="1.5" markerEnd="url(#arr)" fill="none" />
    </Frame>
  )
}

export function DataLocDiagram() {
  const cells = [
    { x: 20, w: 200, t: 'struct trace_entry', d: 'common_type · flags · pid (8 B)' },
    { x: 220, w: 190, t: '__data_loc_filename', d: 'u32: len << 16 | offset', hl: true },
    { x: 410, w: 110, t: 'pid', d: 's32' },
    { x: 520, w: 110, t: 'old_pid', d: 's32' },
    { x: 630, w: 210, t: '__data[] (variable)', d: '"/bin/sample_app\\0"', hl2: true },
  ]
  return (
    <Frame viewBox="0 0 860 190" caption="sched_process_exec record: the filename is not at the start of ctx — it lives at ctx + (loc & 0xFFFF).">
      {cells.map((c) => (
        <g key={c.t}>
          <rect x={c.x} y="40" width={c.w} height="70" className={`${c.hl ? 'fill-teal-500/10 stroke-teal-500' : c.hl2 ? 'fill-sky-500/10 stroke-sky-500' : box}`} strokeWidth="1.5" />
          <text x={c.x + 12} y="70" fontSize="13" fontWeight="600" className={label} fontFamily="JetBrains Mono, monospace">{c.t}</text>
          <text x={c.x + 12} y="92" fontSize="12" className={sub}>{c.d}</text>
        </g>
      ))}
      <text x="20" y="30" fontSize="12" className={sub} fontFamily="JetBrains Mono, monospace">offset 0</text>
      <path d="M315 110 C315 160 735 160 735 110" fill="none" className="stroke-teal-500" strokeWidth="1.5" strokeDasharray="5 4" />
      <text x="430" y="172" fontSize="12" className="fill-teal-600 dark:fill-teal-400">offset = loc &amp; 0xFFFF</text>
      <text x="20" y="140" fontSize="12" className="fill-rose-500">old probe: bpf_probe_read_str(dst, n, ctx) → copied the header bytes</text>
    </Frame>
  )
}

export function FrameDiagram() {
  return (
    <Frame viewBox="0 0 860 150" caption="Control-plane frame: 4-byte big-endian length, then a JSON RPCRequest/RPCResponse (≤ 10 MiB).">
      <rect x="20" y="30" width="200" height="64" className="fill-indigo-500/10 stroke-indigo-500" strokeWidth="1.5" />
      <text x="36" y="58" fontSize="13" fontWeight="600" className={label} fontFamily="JetBrains Mono, monospace">length : u32 BE</text>
      <text x="36" y="80" fontSize="12" className={sub}>validated ≤ MaxFrameSize</text>
      <rect x="220" y="30" width="620" height="64" className={box} strokeWidth="1.5" />
      <text x="236" y="58" fontSize="13" fontWeight="600" className={label} fontFamily="JetBrains Mono, monospace">{'{"id":"req-1","command":"stop_service","service":"kv_store"}'}</text>
      <text x="236" y="80" fontSize="12" className={sub}>buffered as bytes arrive (64 KiB initial), never preallocated from the header</text>
      <text x="20" y="124" fontSize="12" className={sub}>Header and payload are written with a single Write; a short write is an error, never a silent truncation.</text>
    </Frame>
  )
}

function Arrow({ d, id = 'arr2', dashed }) {
  return <path d={d} className="stroke-slate-400" strokeWidth="1.5" fill="none" markerEnd={`url(#${id})`} strokeDasharray={dashed ? '5 4' : undefined} />
}

function ArrowDefs({ id = 'arr2' }) {
  return (
    <defs>
      <marker id={id} viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
        <path d="M0 0L10 5L0 10z" className="fill-slate-400" />
      </marker>
    </defs>
  )
}

export function NetworkDiagram() {
  return (
    <Frame viewBox="0 0 860 330" caption="QEMU user-mode networking (slirp): the guest sits alone on 10.0.2.0/24; inbound traffic only reaches it through hostfwd rules.">
      <ArrowDefs id="arrnet" />
      {/* Host */}
      <rect x="10" y="10" width="250" height="310" rx="16" className="fill-indigo-500/5 stroke-indigo-500/60" strokeDasharray="6 5" strokeWidth="1.5" />
      <text x="28" y="40" fontSize="12" fontFamily="JetBrains Mono, monospace" className="fill-indigo-500" letterSpacing="2">HOST</text>
      <Node x={28} y={60} w={214} h={70} title="localhost:8080" detail={['hostfwd → guest :8080 (httpd)']} accent="stroke-indigo-500" />
      <Node x={28} y={150} w={214} h={70} title="localhost:8000" detail={['hostfwd → guest :80']} accent="stroke-indigo-500" />
      <Node x={28} y={240} w={214} h={64} title="Host resolver / uplink" detail={['NAT for guest egress']} accent="stroke-indigo-500" />

      {/* Slirp */}
      <rect x="300" y="60" width="200" height="244" rx="12" className="fill-sky-500/5 stroke-sky-500/70" strokeWidth="1.5" />
      <text x="316" y="86" fontSize="14" fontWeight="600" className={label}>QEMU slirp</text>
      <text x="316" y="106" fontSize="12" className={sub}>10.0.2.0/24 (virtual)</text>
      <text x="316" y="140" fontSize="12" fontFamily="JetBrains Mono, monospace" className="fill-sky-600 dark:fill-sky-400">10.0.2.2</text>
      <text x="316" y="157" fontSize="12" className={sub}>gateway</text>
      <text x="316" y="190" fontSize="12" fontFamily="JetBrains Mono, monospace" className="fill-sky-600 dark:fill-sky-400">10.0.2.3</text>
      <text x="316" y="207" fontSize="12" className={sub}>DNS forwarder</text>
      <text x="316" y="250" fontSize="12" className={sub}>virtio-net-pci</text>
      <text x="316" y="267" fontSize="12" className={sub}>netdev user,id=net0</text>

      {/* Guest */}
      <rect x="540" y="10" width="310" height="310" rx="16" className="fill-teal-500/5 stroke-teal-500/60" strokeWidth="1.5" />
      <text x="558" y="40" fontSize="12" fontFamily="JetBrains Mono, monospace" className="fill-teal-500" letterSpacing="2">GUEST</text>
      <Node x={558} y={60} w={274} h={88} title="eth0  10.0.2.15/24" detail={['RTM_NEWLINK  IFF_UP', 'RTM_NEWADDR  IFA_LOCAL', 'RTM_NEWROUTE 0.0.0.0/0 via .2']} />
      <Node x={558} y={168} w={274} h={70} title="/etc/resolv.conf" detail={['nameserver 10.0.2.3 · 1.1.1.1']} />
      <Node x={558} y={258} w={274} h={46} title="lo  (not configured — see below)" accent="stroke-rose-500" />

      <Arrow id="arrnet" d="M242 95 H300" />
      <Arrow id="arrnet" d="M242 185 H300" />
      <Arrow id="arrnet" d="M300 272 H242" />
      <Arrow id="arrnet" d="M500 104 H558" />
      <Arrow id="arrnet" d="M500 200 H558" dashed />
    </Frame>
  )
}

export function MountLayoutDiagram() {
  const rows = [
    { y: 50, path: '/', fs: 'initramfs (rootfs)', note: 'writable · init, daemons, busybox, service TOMLs', cls: 'stroke-teal-500' },
    { y: 104, path: '/proc  /sys  /dev', fs: 'proc · sysfs · devtmpfs', note: 'mounted by karim-init', cls: 'stroke-slate-400' },
    { y: 158, path: '/sys/fs/cgroup', fs: 'cgroup2', note: 'unified hierarchy for service cgroups', cls: 'stroke-slate-400' },
    { y: 212, path: '/mnt/lower', fs: 'squashfs  /dev/vda  (MS_RDONLY)', note: 'rootfs.sqsh: layers, workloads, passwd', cls: 'stroke-indigo-500' },
    { y: 266, path: '/var  /tmp  /run', fs: 'tmpfs size=64M', note: 'ephemeral, lost on reboot', cls: 'stroke-sky-500' },
    { y: 320, path: '/run/karim/overlay', fs: 'tmpfs size=64M', note: 'upper/ + work/ prepared, overlay not mounted on /', cls: 'stroke-sky-500' },
  ]
  return (
    <Frame viewBox="0 0 860 390" caption="Guest mount table after karim-svcd's storage step. Entries from /mnt/lower are exposed at / by symlink, not by an overlay mount.">
      <text x="20" y="32" fontSize="12" fontFamily="JetBrains Mono, monospace" className={sub} letterSpacing="1">MOUNT POINT</text>
      <text x="270" y="32" fontSize="12" fontFamily="JetBrains Mono, monospace" className={sub} letterSpacing="1">FILESYSTEM</text>
      <text x="560" y="32" fontSize="12" fontFamily="JetBrains Mono, monospace" className={sub} letterSpacing="1">CONTENT</text>
      {rows.map((r) => (
        <g key={r.path}>
          <rect x="20" y={r.y} width="820" height="44" rx="8" className={`${box} ${r.cls}`} strokeWidth="1.5" />
          <text x="36" y={r.y + 27} fontSize="13" fontWeight="600" fontFamily="JetBrains Mono, monospace" className={label}>{r.path}</text>
          <text x="270" y={r.y + 27} fontSize="12" className={label}>{r.fs}</text>
          <text x="560" y={r.y + 27} fontSize="12" className={sub}>{r.note}</text>
        </g>
      ))}
    </Frame>
  )
}
