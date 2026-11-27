export function ZetnixLogo({ size = 28, live = false }: { size?: number; live?: boolean }) {
  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg viewBox="0 0 40 40" className="w-full h-full drop-shadow-[0_0_12px_rgba(6,182,212,0.45)]">
        <defs>
          <linearGradient id="zetnixGrad" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="#22d3ee" />
            <stop offset="50%" stopColor="#3b82f6" />
            <stop offset="100%" stopColor="#a855f7" />
          </linearGradient>
        </defs>
        <polygon
          points="20,2 36,11 36,29 20,38 4,29 4,11"
          fill="rgba(15, 23, 42, 0.9)"
          stroke="url(#zetnixGrad)"
          strokeWidth="2.2"
        />
        <path
          d="M12.5 13.5 H27.5 L14 26.5 H27.5"
          fill="none"
          stroke="url(#zetnixGrad)"
          strokeWidth="3.1"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <circle cx="27.5" cy="13.5" r="2.2" fill="#22d3ee" />
        <circle cx="14" cy="26.5" r="2.2" fill="#a855f7" />
      </svg>
      {live && (
        <span className="absolute -top-0.5 -right-0.5 flex h-2.5 w-2.5">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
          <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-400 border border-[#060911]" />
        </span>
      )}
    </div>
  )
}

export function StatusBadge({ status }: { status: string }) {
  const s = (status || '').toLowerCase()
  let cls = 'bg-slate-500/15 text-slate-400 border-slate-500/30'
  let dot = 'bg-slate-400'

  if (['running', 'ready', 'active', 'bound', 'healthy', 'complete', 'succeeded', 'normal'].includes(s)) {
    cls = 'bg-emerald-500/15 text-emerald-400 border-emerald-500/35'
    dot = 'bg-emerald-400'
  } else if (['pending', 'progressing', 'updating', 'degraded', 'warning', 'terminating'].includes(s)) {
    cls = 'bg-amber-500/15 text-amber-400 border-amber-500/35'
    dot = 'bg-amber-400 animate-pulse'
  } else if (['crashloopbackoff', 'imagepullbackoff', 'failed', 'error', 'failing', 'notready', 'oomkilled'].includes(s)) {
    cls = 'bg-rose-500/15 text-rose-400 border-rose-500/35'
    dot = 'bg-rose-400 animate-ping'
  } else if (s === 'scaled to 0' || s === 'suspended') {
    cls = 'bg-slate-500/15 text-slate-400 border-slate-500/30'
    dot = 'bg-slate-500'
  }

  return (
    <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-medium border ${cls}`}>
      <span className="relative flex h-1.5 w-1.5">
        <span className={`inline-flex rounded-full h-1.5 w-1.5 ${dot}`} />
      </span>
      {status}
    </span>
  )
}

export function KindBadge({ kind }: { kind: string }) {
  const map: Record<string, string> = {
    Pod: 'bg-cyan-500/15 text-cyan-400 border-cyan-500/30',
    Deployment: 'bg-indigo-500/15 text-indigo-400 border-indigo-500/30',
    StatefulSet: 'bg-violet-500/15 text-violet-400 border-violet-500/30',
    DaemonSet: 'bg-fuchsia-500/15 text-fuchsia-400 border-fuchsia-500/30',
    CronJob: 'bg-amber-500/15 text-amber-400 border-amber-500/30',
    Job: 'bg-amber-500/15 text-amber-400 border-amber-500/30',
    Service: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30',
    Ingress: 'bg-sky-500/15 text-sky-400 border-sky-500/30',
    PersistentVolumeClaim: 'bg-orange-500/15 text-orange-400 border-orange-500/30',
    Node: 'bg-teal-500/15 text-teal-400 border-teal-500/30',
  }
  const style = map[kind] || 'bg-slate-500/15 text-slate-400 border-slate-500/30'
  const short = kind === 'PersistentVolumeClaim' ? 'PVC' : kind
  return (
    <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-mono font-semibold tracking-wide border ${style}`}>
      {short}
    </span>
  )
}
