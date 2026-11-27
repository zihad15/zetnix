import {
  Activity,
  AlertTriangle,
  Box,
  Cpu,
  Globe,
  HardDrive,
  Layers,
  MemoryStick,
  Network,
  Server,
  ShieldCheck,
} from 'lucide-react'
import type { ClusterOverview as OverviewData, InspectedResource, NavSection } from '../../types/k8s'
import { formatAge, formatCores, formatMemoryMiB } from '../../lib/api'
import { KindBadge, StatusBadge } from '../shared/StatusBadge'

interface TelemetryPoint {
  ts: string
  cpuPct: number
  memPct: number
}

interface ClusterOverviewProps {
  overview: OverviewData | null
  history: TelemetryPoint[]
  dark: boolean
  onNavigate: (s: NavSection) => void
  onInspect: (res: InspectedResource) => void
}

function RadialGauge({ value, label, sub, color }: { value: number; label: string; sub: string; color: string }) {
  const clamped = Math.max(0, Math.min(100, value))
  const r = 38
  const c = 2 * Math.PI * r
  const offset = c - (clamped / 100) * c

  return (
    <div className="flex items-center gap-4">
      <div className="relative w-24 h-24 shrink-0">
        <svg viewBox="0 0 96 96" className="w-full h-full -rotate-90">
          <circle cx="48" cy="48" r={r} fill="none" stroke="rgba(148,163,184,0.14)" strokeWidth="8" />
          <circle
            cx="48"
            cy="48"
            r={r}
            fill="none"
            stroke={color}
            strokeWidth="8"
            strokeLinecap="round"
            strokeDasharray={c}
            strokeDashoffset={offset}
            className="transition-all duration-700 ease-out"
          />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-xl font-extrabold font-mono">{Math.round(clamped)}%</span>
        </div>
      </div>
      <div>
        <div className="text-xs uppercase tracking-wider text-slate-400 font-semibold">{label}</div>
        <div className="text-sm font-semibold mt-0.5">{sub}</div>
      </div>
    </div>
  )
}

function SparkArea({ points, dataKey, stroke, fillId, fillColor }: { points: TelemetryPoint[]; dataKey: 'cpuPct' | 'memPct'; stroke: string; fillId: string; fillColor: string }) {
  const safe = points.length >= 2 ? points : [...points, ...(points.length === 1 ? [points[0]] : [{ ts: '', cpuPct: 18, memPct: 35 }, { ts: '', cpuPct: 20, memPct: 36 }])]
  const w = 280
  const h = 64
  const coords = safe.map((pt, i) => {
    const x = (i / Math.max(1, safe.length - 1)) * w
    const y = h - (Math.max(2, Math.min(98, pt[dataKey])) / 100) * (h - 8) - 4
    return { x, y }
  })
  const line = coords.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ')
  const area = `${line} L ${w} ${h} L 0 ${h} Z`

  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-full h-16 overflow-visible">
      <defs>
        <linearGradient id={fillId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={fillColor} stopOpacity="0.35" />
          <stop offset="100%" stopColor={fillColor} stopOpacity="0.0" />
        </linearGradient>
      </defs>
      <path d={area} fill={`url(#${fillId})`} />
      <path d={line} fill="none" stroke={stroke} strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx={coords[coords.length - 1].x} cy={coords[coords.length - 1].y} r="3.5" fill={stroke} />
    </svg>
  )
}

export function ClusterOverview({ overview, history, dark, onNavigate, onInspect }: ClusterOverviewProps) {
  if (!overview) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 animate-pulse">
        {[1, 2, 3].map((i) => (
          <div key={i} className={`h-36 rounded-2xl border ${dark ? 'bg-slate-900/60 border-slate-800' : 'bg-white border-slate-200'}`} />
        ))}
      </div>
    )
  }

  const cardCls = `rounded-2xl border backdrop-blur-xl transition-all ${
    dark ? 'bg-[#0b1120]/85 border-slate-800/80' : 'bg-white/90 border-slate-200 shadow-sm'
  }`

  const healthColor = overview.healthScore >= 85 ? '#34d399' : overview.healthScore >= 65 ? '#fbbf24' : '#fb7185'
  const totalPods = Math.max(1, overview.pods.total)

  const inventoryTiles = [
    { label: 'Deployments', val: `${overview.deploymentsReady}/${overview.deploymentsTotal}`, sub: 'Ready', icon: <Layers className="w-4 h-4 text-indigo-400" />, nav: 'workloads' as NavSection },
    { label: 'StatefulSets', val: String(overview.statefulSets), sub: 'Ordered Apps', icon: <Box className="w-4 h-4 text-violet-400" />, nav: 'workloads' as NavSection },
    { label: 'DaemonSets', val: String(overview.daemonSets), sub: 'Node Agents', icon: <Server className="w-4 h-4 text-fuchsia-400" />, nav: 'workloads' as NavSection },
    { label: 'Services', val: String(overview.services), sub: 'Cluster Endpoints', icon: <Network className="w-4 h-4 text-emerald-400" />, nav: 'networking' as NavSection },
    { label: 'Ingresses', val: String(overview.ingresses), sub: 'HTTP/TLS Routes', icon: <Globe className="w-4 h-4 text-cyan-400" />, nav: 'networking' as NavSection },
    { label: 'Storage PVCs', val: String(overview.pvcs), sub: 'Persistent Claims', icon: <HardDrive className="w-4 h-4 text-orange-400" />, nav: 'storage' as NavSection },
  ]

  return (
    <div className="space-y-5">
      {/* Top Hero Row: Cluster Pulse + Compute Telemetry */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {/* Cluster Health Card */}
        <div className={`${cardCls} p-5 flex flex-col justify-between relative overflow-hidden`}>
          <div className="flex items-center justify-between mb-3">
            <span className="inline-flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-cyan-400">
              <ShieldCheck className="w-4 h-4" />
              Cluster Integrity
            </span>
            <span className="text-[11px] font-mono text-slate-400">
              {overview.namespace === 'all' ? 'All Projects' : `ns/${overview.namespace}`}
            </span>
          </div>
          <RadialGauge
            value={overview.healthScore}
            label="Health Index"
            sub={`${overview.nodesReady}/${overview.nodesTotal} Nodes Ready • ${overview.namespacesCount} Projects`}
            color={healthColor}
          />
          <div className="mt-4 pt-3 border-t border-slate-800/60 flex items-center justify-between text-xs">
            <span className="text-slate-400">Pods Running</span>
            <span className="font-mono font-semibold text-emerald-400">
              {overview.pods.running} / {overview.pods.total}
              {overview.pods.crashLoop > 0 && (
                <span className="ml-2 text-rose-400">({overview.pods.crashLoop} CrashLoop)</span>
              )}
            </span>
          </div>
        </div>

        {/* CPU Telemetry Card */}
        <div className={`${cardCls} p-5 flex flex-col justify-between`}>
          <div className="flex items-center justify-between">
            <span className="inline-flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-cyan-400">
              <Cpu className="w-4 h-4" />
              CPU Utilization
            </span>
            <span className="text-xs font-mono font-bold text-cyan-300">
              {overview.metricsAvailable ? `${overview.usage.cpuPercent.toFixed(1)}%` : 'No Metrics Server'}
            </span>
          </div>
          <div className="my-2">
            <SparkArea points={history} dataKey="cpuPct" stroke="#22d3ee" fillId="cpuFill" fillColor="#06b6d4" />
          </div>
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span>Allocated Demand</span>
            <span className="font-mono text-slate-200">
              {formatCores(overview.usage.usedMilliCpu)} / {formatCores(overview.usage.totalMilliCpu)}
            </span>
          </div>
        </div>

        {/* Memory Telemetry Card */}
        <div className={`${cardCls} p-5 flex flex-col justify-between`}>
          <div className="flex items-center justify-between">
            <span className="inline-flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-violet-400">
              <MemoryStick className="w-4 h-4" />
              Memory Footprint
            </span>
            <span className="text-xs font-mono font-bold text-violet-300">
              {overview.metricsAvailable ? `${overview.usage.memoryPercent.toFixed(1)}%` : 'No Metrics Server'}
            </span>
          </div>
          <div className="my-2">
            <SparkArea points={history} dataKey="memPct" stroke="#a855f7" fillId="memFill" fillColor="#8b5cf6" />
          </div>
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span>Working Set</span>
            <span className="font-mono text-slate-200">
              {formatMemoryMiB(overview.usage.usedMemoryMiB)} / {formatMemoryMiB(overview.usage.totalMemoryMiB)}
            </span>
          </div>
        </div>
      </div>

      {/* Pod Phase Distribution Bar */}
      <div className={`${cardCls} p-4`}>
        <div className="flex flex-wrap items-center justify-between gap-2 mb-3">
          <div className="flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-slate-300">
            <Activity className="w-4 h-4 text-cyan-400" />
            Pod Phase Distribution
          </div>
          <div className="flex flex-wrap items-center gap-4 text-xs">
            <span className="flex items-center gap-1.5"><span className="w-2.5 h-2.5 rounded-full bg-emerald-400" />Running: <b>{overview.pods.running}</b></span>
            <span className="flex items-center gap-1.5"><span className="w-2.5 h-2.5 rounded-full bg-amber-400" />Pending: <b>{overview.pods.pending}</b></span>
            <span className="flex items-center gap-1.5"><span className="w-2.5 h-2.5 rounded-full bg-rose-500" />CrashLoop/Fail: <b>{overview.pods.crashLoop + overview.pods.failed}</b></span>
            <span className="flex items-center gap-1.5"><span className="w-2.5 h-2.5 rounded-full bg-cyan-400" />Succeeded: <b>{overview.pods.succeeded}</b></span>
          </div>
        </div>
        <div className="h-3 rounded-full overflow-hidden flex bg-slate-800/80">
          <div style={{ width: `${(overview.pods.running / totalPods) * 100}%` }} className="bg-gradient-to-r from-emerald-500 to-teal-400 transition-all duration-500" />
          <div style={{ width: `${(overview.pods.pending / totalPods) * 100}%` }} className="bg-amber-400 transition-all duration-500" />
          <div style={{ width: `${((overview.pods.crashLoop + overview.pods.failed) / totalPods) * 100}%` }} className="bg-rose-500 transition-all duration-500" />
          <div style={{ width: `${(overview.pods.succeeded / totalPods) * 100}%` }} className="bg-cyan-400 transition-all duration-500" />
        </div>
      </div>

      {/* Inventory Grid + Live Cluster Events */}
      <div className="grid grid-cols-1 xl:grid-cols-12 gap-4">
        <div className="xl:col-span-5 grid grid-cols-2 sm:grid-cols-3 xl:grid-cols-2 gap-3">
          {inventoryTiles.map((tile) => (
            <button
              key={tile.label}
              onClick={() => onNavigate(tile.nav)}
              className={`${cardCls} p-4 text-left hover:border-cyan-500/45 hover:-translate-y-0.5 cursor-pointer group`}
            >
              <div className="flex items-center justify-between mb-2">
                <span className="p-2 rounded-xl bg-slate-800/70 group-hover:bg-cyan-500/15 transition-colors">{tile.icon}</span>
                <span className="text-[11px] text-slate-400 group-hover:text-cyan-300 transition-colors">Explore →</span>
              </div>
              <div className="text-2xl font-extrabold font-mono tracking-tight">{tile.val}</div>
              <div className="text-xs font-semibold mt-0.5">{tile.label}</div>
              <div className="text-[11px] text-slate-400">{tile.sub}</div>
            </button>
          ))}
        </div>

        {/* Live Events Stream */}
        <div className={`${cardCls} xl:col-span-7 p-4 flex flex-col`}>
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2 text-xs font-bold uppercase tracking-wider">
              <AlertTriangle className="w-4 h-4 text-amber-400" />
              Recent Cluster Events
            </div>
            <button
              onClick={() => onNavigate('events')}
              className="text-xs text-cyan-400 hover:text-cyan-300 font-semibold cursor-pointer"
            >
              View All Events →
            </button>
          </div>
          <div className="divide-y divide-slate-800/60 overflow-y-auto max-h-80">
            {overview.recentEvents.length === 0 ? (
              <div className="py-10 text-center text-xs text-slate-500">No recent Kubernetes events in this scope</div>
            ) : (
              overview.recentEvents.map((ev) => (
                <div
                  key={ev.id}
                  onClick={() =>
                    ev.involvedKind &&
                    ev.involvedName &&
                    onInspect({
                      kind: ev.involvedKind,
                      namespace: ev.namespace,
                      name: ev.involvedName,
                    })
                  }
                  className="py-2.5 px-2 rounded-lg hover:bg-slate-800/40 transition-colors flex items-start justify-between gap-3 cursor-pointer"
                >
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <StatusBadge status={ev.type} />
                      <span className="text-xs font-bold font-mono text-slate-200">{ev.reason}</span>
                      <KindBadge kind={ev.involvedKind} />
                      <span className="text-xs font-mono text-cyan-300 truncate">{ev.involvedName}</span>
                    </div>
                    <p className="text-xs text-slate-400 leading-relaxed">{ev.message}</p>
                  </div>
                  <div className="shrink-0 text-right">
                    <div className="text-[11px] font-mono text-slate-400">{formatAge(ev.lastTimestamp)}</div>
                    <div className="text-[10px] font-mono text-slate-500">{ev.namespace}</div>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
