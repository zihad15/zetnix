import { useState } from 'react'
import { Cpu, Globe, HardDrive, KeyRound, Lock, Network, Radio, Server } from 'lucide-react'
import type {
  ConfigMapSummary,
  EventSummary,
  IngressSummary,
  InspectedResource,
  NodeSummary,
  PVCSummary,
  SecretSummary,
  ServiceSummary,
} from '../../types/k8s'
import { formatAge, formatCores, formatMemoryMiB } from '../../lib/api'
import { KindBadge, StatusBadge } from '../shared/StatusBadge'

export function NetworkingView({
  services,
  ingresses,
  dark,
  onInspect,
}: {
  services: ServiceSummary[]
  ingresses: IngressSummary[]
  dark: boolean
  onInspect: (res: InspectedResource) => void
}) {
  const cardCls = `rounded-2xl border overflow-hidden ${dark ? 'bg-[#0b1120]/85 border-slate-800/80' : 'bg-white border-slate-200'}`

  return (
    <div className="space-y-6">
      <div className={cardCls}>
        <div className="px-4 py-3 border-b border-slate-800/80 flex items-center justify-between">
          <div className="flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-cyan-400">
            <Globe className="w-4 h-4" />
            Ingresses & Public Routes ({ingresses.length})
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse text-xs">
            <thead>
              <tr className="border-b border-slate-800 text-[11px] uppercase text-slate-400">
                <th className="py-3 px-4">Route Name</th>
                <th className="py-3 px-3">Namespace</th>
                <th className="py-3 px-3">Hosts</th>
                <th className="py-3 px-3">Backends</th>
                <th className="py-3 px-3">TLS</th>
                <th className="py-3 px-3">Age</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {ingresses.map((ing) => (
                <tr key={`${ing.namespace}/${ing.name}`} className="hover:bg-slate-800/35">
                  <td className="py-3 px-4">
                    <button
                      onClick={() => onInspect({ kind: 'Ingress', namespace: ing.namespace, name: ing.name, raw: ing })}
                      className="font-mono font-bold text-cyan-300 hover:underline cursor-pointer"
                    >
                      {ing.name}
                    </button>
                  </td>
                  <td className="py-3 px-3 font-mono text-slate-400">{ing.namespace}</td>
                  <td className="py-3 px-3 font-mono text-sky-300">{ing.hosts.join(', ') || '—'}</td>
                  <td className="py-3 px-3 font-mono text-slate-300">
                    {ing.rules.map((r) => `${r.path} → ${r.serviceName}:${r.servicePort}`).join(' | ')}
                  </td>
                  <td className="py-3 px-3">
                    {ing.tls ? (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                        <Lock className="w-3 h-3" /> HTTPS
                      </span>
                    ) : (
                      <span className="text-slate-500">HTTP</span>
                    )}
                  </td>
                  <td className="py-3 px-3 font-mono text-slate-400">{formatAge(ing.createdAt)}</td>
                </tr>
              ))}
              {ingresses.length === 0 && (
                <tr><td colSpan={6} className="py-8 text-center text-slate-500">No ingresses in this namespace</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      <div className={cardCls}>
        <div className="px-4 py-3 border-b border-slate-800/80 flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-emerald-400">
          <Network className="w-4 h-4" />
          Kubernetes Services ({services.length})
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse text-xs">
            <thead>
              <tr className="border-b border-slate-800 text-[11px] uppercase text-slate-400">
                <th className="py-3 px-4">Service</th>
                <th className="py-3 px-3">Namespace</th>
                <th className="py-3 px-3">Type</th>
                <th className="py-3 px-3">Cluster IP</th>
                <th className="py-3 px-3">External IP</th>
                <th className="py-3 px-3">Ports</th>
                <th className="py-3 px-3">Endpoints</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {services.map((s) => (
                <tr key={`${s.namespace}/${s.name}`} className="hover:bg-slate-800/35">
                  <td className="py-3 px-4">
                    <button
                      onClick={() => onInspect({ kind: 'Service', namespace: s.namespace, name: s.name, raw: s })}
                      className="font-mono font-bold text-emerald-300 hover:underline cursor-pointer"
                    >
                      {s.name}
                    </button>
                  </td>
                  <td className="py-3 px-3 font-mono text-slate-400">{s.namespace}</td>
                  <td className="py-3 px-3 font-mono">{s.type}</td>
                  <td className="py-3 px-3 font-mono text-slate-300">{s.clusterIp}</td>
                  <td className="py-3 px-3 font-mono text-cyan-300">{s.externalIp || '—'}</td>
                  <td className="py-3 px-3 font-mono text-slate-300">
                    {s.ports.map((p) => `${p.port}→${p.targetPort}/${p.protocol}`).join(', ')}
                  </td>
                  <td className="py-3 px-3 font-mono">{s.endpoints}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

export function StorageAndConfigView({
  pvcs,
  configMaps,
  secrets,
  dark,
  onInspect,
}: {
  pvcs: PVCSummary[]
  configMaps: ConfigMapSummary[]
  secrets: SecretSummary[]
  dark: boolean
  onInspect: (res: InspectedResource) => void
}) {
  const cardCls = `rounded-2xl border overflow-hidden ${dark ? 'bg-[#0b1120]/85 border-slate-800/80' : 'bg-white border-slate-200'}`

  return (
    <div className="space-y-6">
      <div className={cardCls}>
        <div className="px-4 py-3 border-b border-slate-800/80 flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-orange-400">
          <HardDrive className="w-4 h-4" />
          Persistent Volume Claims ({pvcs.length})
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse text-xs">
            <thead>
              <tr className="border-b border-slate-800 text-[11px] uppercase text-slate-400">
                <th className="py-3 px-4">PVC Name</th>
                <th className="py-3 px-3">Namespace</th>
                <th className="py-3 px-3">Status</th>
                <th className="py-3 px-3">Capacity</th>
                <th className="py-3 px-3">StorageClass</th>
                <th className="py-3 px-3">Access Modes</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {pvcs.map((p) => (
                <tr key={`${p.namespace}/${p.name}`} className="hover:bg-slate-800/35">
                  <td className="py-3 px-4">
                    <button
                      onClick={() => onInspect({ kind: 'PersistentVolumeClaim', namespace: p.namespace, name: p.name, raw: p })}
                      className="font-mono font-bold text-orange-300 hover:underline cursor-pointer"
                    >
                      {p.name}
                    </button>
                  </td>
                  <td className="py-3 px-3 font-mono text-slate-400">{p.namespace}</td>
                  <td className="py-3 px-3"><StatusBadge status={p.status} /></td>
                  <td className="py-3 px-3 font-mono font-bold text-slate-100">{p.capacity}</td>
                  <td className="py-3 px-3 font-mono text-slate-300">{p.storageClass || 'default'}</td>
                  <td className="py-3 px-3 font-mono text-slate-400">{p.accessModes.join(', ')}</td>
                </tr>
              ))}
              {pvcs.length === 0 && (
                <tr><td colSpan={6} className="py-8 text-center text-slate-500">No PVCs found</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div className={cardCls}>
          <div className="px-4 py-3 border-b border-slate-800/80 text-xs font-bold uppercase tracking-wider text-cyan-400">
            ConfigMaps ({configMaps.length})
          </div>
          <div className="divide-y divide-slate-800/60 text-xs">
            {configMaps.map((cm) => (
              <div
                key={`${cm.namespace}/${cm.name}`}
                onClick={() => onInspect({ kind: 'ConfigMap', namespace: cm.namespace, name: cm.name, raw: cm })}
                className="p-3.5 hover:bg-slate-800/35 cursor-pointer flex items-center justify-between gap-2"
              >
                <div>
                  <div className="font-mono font-bold text-cyan-300">{cm.name}</div>
                  <div className="text-[11px] font-mono text-slate-400 mt-0.5">
                    {cm.namespace} • keys: {cm.keys.join(', ') || 'none'}
                  </div>
                </div>
                <span className="font-mono text-slate-500">{formatAge(cm.createdAt)}</span>
              </div>
            ))}
          </div>
        </div>

        <div className={cardCls}>
          <div className="px-4 py-3 border-b border-slate-800/80 flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-violet-400">
            <KeyRound className="w-4 h-4" />
            Secrets ({secrets.length})
          </div>
          <div className="divide-y divide-slate-800/60 text-xs">
            {secrets.map((sec) => (
              <div
                key={`${sec.namespace}/${sec.name}`}
                onClick={() => onInspect({ kind: 'Secret', namespace: sec.namespace, name: sec.name, raw: sec })}
                className="p-3.5 hover:bg-slate-800/35 cursor-pointer flex items-center justify-between gap-2"
              >
                <div>
                  <div className="font-mono font-bold text-violet-300">{sec.name}</div>
                  <div className="text-[11px] font-mono text-slate-400 mt-0.5">
                    {sec.namespace} • {sec.type} • {sec.keys.length} keys
                  </div>
                </div>
                <span className="font-mono text-slate-500">{sec.dataSize} B</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

export function NodesView({
  nodes,
  dark,
  onInspect,
}: {
  nodes: NodeSummary[]
  dark: boolean
  onInspect: (res: InspectedResource) => void
}) {
  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-4">
      {nodes.map((n) => {
        const cpuTotalMilli = Math.max(1, n.cpuCapacityCores * 1000)
        const cpuPct = Math.min(100, (n.cpuUsedMilli / cpuTotalMilli) * 100)
        const memPct = Math.min(100, (n.memUsedMiB / Math.max(1, n.memCapacityMiB)) * 100)
        return (
          <div
            key={n.name}
            onClick={() => onInspect({ kind: 'Node', namespace: '', name: n.name, raw: n })}
            className={`rounded-2xl border p-5 cursor-pointer transition-all hover:border-cyan-500/45 ${
              dark ? 'bg-[#0b1120]/85 border-slate-800/80' : 'bg-white border-slate-200'
            }`}
          >
            <div className="flex items-start justify-between gap-2 mb-3">
              <div>
                <div className="flex items-center gap-2">
                  <Server className="w-4 h-4 text-teal-400" />
                  <span className="font-mono font-bold text-sm text-slate-100">{n.name}</span>
                </div>
                <div className="flex flex-wrap gap-1.5 mt-1.5">
                  {n.roles.map((r) => (
                    <span key={r} className="px-2 py-0.5 rounded-md text-[10px] font-mono bg-teal-500/15 text-teal-300 border border-teal-500/30">
                      {r}
                    </span>
                  ))}
                </div>
              </div>
              <StatusBadge status={n.status} />
            </div>

            <div className="space-y-3 my-4 text-xs">
              <div>
                <div className="flex justify-between font-mono mb-1">
                  <span className="text-slate-400 flex items-center gap-1"><Cpu className="w-3 h-3" />CPU</span>
                  <span>{formatCores(n.cpuUsedMilli)} / {n.cpuCapacityCores.toFixed(1)}c</span>
                </div>
                <div className="h-2 rounded-full bg-slate-800 overflow-hidden">
                  <div className="h-full bg-gradient-to-r from-cyan-400 to-blue-500" style={{ width: `${cpuPct}%` }} />
                </div>
              </div>
              <div>
                <div className="flex justify-between font-mono mb-1">
                  <span className="text-slate-400">Memory</span>
                  <span>{formatMemoryMiB(n.memUsedMiB)} / {formatMemoryMiB(n.memCapacityMiB)}</span>
                </div>
                <div className="h-2 rounded-full bg-slate-800 overflow-hidden">
                  <div className="h-full bg-gradient-to-r from-violet-400 to-fuchsia-500" style={{ width: `${memPct}%` }} />
                </div>
              </div>
            </div>

            <div className="pt-3 border-t border-slate-800/70 grid grid-cols-2 gap-2 text-[11px] font-mono text-slate-400">
              <div>IP: <span className="text-slate-200">{n.internalIp || '—'}</span></div>
              <div>Pods: <span className="text-slate-200">{n.podsAllocated}/{n.podsCapacity}</span></div>
              <div className="truncate col-span-2">{n.kubeletVersion} • {n.containerRuntime}</div>
            </div>
          </div>
        )
      })}
    </div>
  )
}

export function EventsView({
  events,
  dark,
  onInspect,
}: {
  events: EventSummary[]
  dark: boolean
  onInspect: (res: InspectedResource) => void
}) {
  const [onlyWarn, setOnlyWarn] = useState(false)
  const list = onlyWarn ? events.filter((e) => e.type === 'Warning') : events

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-slate-300">
          <Radio className="w-4 h-4 text-cyan-400" />
          Cluster Event Log ({list.length})
        </div>
        <button
          onClick={() => setOnlyWarn((v) => !v)}
          className={`px-3 py-1.5 rounded-xl border text-xs font-semibold cursor-pointer ${
            onlyWarn ? 'bg-amber-500/20 border-amber-500/40 text-amber-300' : 'bg-[#0b1120] border-slate-800 text-slate-400'
          }`}
        >
          Warnings Only
        </button>
      </div>

      <div className={`rounded-2xl border divide-y divide-slate-800/60 ${dark ? 'bg-[#0b1120]/85 border-slate-800/80' : 'bg-white border-slate-200'}`}>
        {list.map((ev) => (
          <div
            key={ev.id}
            onClick={() => ev.involvedKind && ev.involvedName && onInspect({ kind: ev.involvedKind, namespace: ev.namespace, name: ev.involvedName })}
            className="p-4 hover:bg-slate-800/35 cursor-pointer flex items-start justify-between gap-4"
          >
            <div className="space-y-1 min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <StatusBadge status={ev.type} />
                <span className="font-mono text-xs font-bold text-slate-100">{ev.reason}</span>
                <KindBadge kind={ev.involvedKind} />
                <span className="font-mono text-xs text-cyan-300">{ev.involvedName}</span>
                <span className="font-mono text-[11px] text-slate-500">({ev.namespace})</span>
              </div>
              <p className="text-xs text-slate-300">{ev.message}</p>
            </div>
            <div className="shrink-0 text-right font-mono text-xs text-slate-400">
              <div>{formatAge(ev.lastTimestamp)}</div>
              <div className="text-[10px] text-slate-500">x{ev.count || 1}</div>
            </div>
          </div>
        ))}
        {list.length === 0 && <div className="py-12 text-center text-xs text-slate-500">No events to display</div>}
      </div>
    </div>
  )
}
