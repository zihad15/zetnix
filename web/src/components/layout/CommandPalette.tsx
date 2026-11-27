import { useEffect, useMemo, useState } from 'react'
import { ArrowRight, Box, Layers, Network, Search, Server, Sparkles } from 'lucide-react'
import type { InspectedResource, NavSection, PodSummary, ServiceSummary, WorkloadSummary } from '../../types/k8s'

interface CommandPaletteProps {
  open: boolean
  onClose: () => void
  pods: PodSummary[]
  workloads: WorkloadSummary[]
  services: ServiceSummary[]
  onNavigate: (s: NavSection) => void
  onInspect: (res: InspectedResource) => void
}

export function CommandPalette({
  open,
  onClose,
  pods,
  workloads,
  services,
  onNavigate,
  onInspect,
}: CommandPaletteProps) {
  const [q, setQ] = useState('')

  useEffect(() => {
    if (open) setQ('')
  }, [open])

  const items = useMemo(() => {
    const query = q.trim().toLowerCase()
    const navItems = [
      { id: 'overview', label: 'Command Center (Overview)', type: 'View', action: () => onNavigate('overview') },
      { id: 'topology', label: 'Interactive Topology Mesh', type: 'View', action: () => onNavigate('topology') },
      { id: 'workloads', label: 'Workloads Explorer', type: 'View', action: () => onNavigate('workloads') },
      { id: 'pods', label: 'Pods, Logs & Web Terminals', type: 'View', action: () => onNavigate('pods') },
      { id: 'networking', label: 'Services & Ingress Routes', type: 'View', action: () => onNavigate('networking') },
      { id: 'storage', label: 'Storage, ConfigMaps & Secrets', type: 'View', action: () => onNavigate('storage') },
      { id: 'nodes', label: 'Cluster Nodes & Compute', type: 'View', action: () => onNavigate('nodes') },
      { id: 'catalog', label: 'Developer App Catalog', type: 'View', action: () => onNavigate('catalog') },
      { id: 'yaml', label: 'YAML Studio', type: 'View', action: () => onNavigate('yaml') },
    ]

    const resItems = [
      ...workloads.map((w) => ({
        id: `${w.kind}/${w.namespace}/${w.name}`,
        label: `${w.name}`,
        sub: `${w.kind} • ${w.namespace} (${w.readyReplicas}/${w.desiredReplicas})`,
        type: w.kind,
        action: () => onInspect({ kind: w.kind, namespace: w.namespace, name: w.name, raw: w }),
      })),
      ...pods.map((p) => ({
        id: `Pod/${p.namespace}/${p.name}`,
        label: p.name,
        sub: `Pod • ${p.namespace} (${p.statusReason})`,
        type: 'Pod',
        action: () => onInspect({ kind: 'Pod', namespace: p.namespace, name: p.name, raw: p }),
      })),
      ...services.map((s) => ({
        id: `Service/${s.namespace}/${s.name}`,
        label: s.name,
        sub: `Service • ${s.namespace} (${s.clusterIp})`,
        type: 'Service',
        action: () => onInspect({ kind: 'Service', namespace: s.namespace, name: s.name, raw: s }),
      })),
    ]

    const all = [...navItems.map((n) => ({ ...n, sub: 'Navigate console view' })), ...resItems]
    if (!query) return all.slice(0, 14)
    return all
      .filter((it) => it.label.toLowerCase().includes(query) || it.sub.toLowerCase().includes(query) || it.type.toLowerCase().includes(query))
      .slice(0, 16)
  }, [q, pods, workloads, services, onNavigate, onInspect])

  if (!open) return null

  const iconFor = (type: string) => {
    if (type === 'Pod') return <Box className="w-4 h-4 text-cyan-400" />
    if (type === 'Service') return <Network className="w-4 h-4 text-emerald-400" />
    if (type === 'View') return <Sparkles className="w-4 h-4 text-violet-400" />
    if (type === 'Node') return <Server className="w-4 h-4 text-teal-400" />
    return <Layers className="w-4 h-4 text-indigo-400" />
  }

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[12vh] px-4 bg-black/65 backdrop-blur-sm" onClick={onClose}>
      <div
        className="w-full max-w-xl rounded-2xl border border-slate-700/80 bg-[#0b1120] shadow-2xl overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-3 px-4 py-3 border-b border-slate-800">
          <Search className="w-4 h-4 text-cyan-400" />
          <input
            autoFocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') onClose()
              if (e.key === 'Enter' && items[0]) {
                items[0].action()
                onClose()
              }
            }}
            placeholder="Search pods, deployments, services, or console views..."
            className="w-full bg-transparent text-sm text-slate-100 placeholder-slate-500 focus:outline-none"
          />
          <kbd className="px-1.5 py-0.5 text-[10px] font-mono rounded bg-slate-800 text-slate-400">ESC</kbd>
        </div>
        <div className="max-h-96 overflow-y-auto p-2 space-y-1">
          {items.length === 0 ? (
            <div className="py-10 text-center text-xs text-slate-500">No matching resources or views</div>
          ) : (
            items.map((item) => (
              <button
                key={item.id}
                onClick={() => {
                  item.action()
                  onClose()
                }}
                className="w-full flex items-center justify-between px-3 py-2.5 rounded-xl hover:bg-slate-800/70 text-left transition-colors cursor-pointer group"
              >
                <div className="flex items-center gap-3 min-w-0">
                  {iconFor(item.type)}
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-slate-100 truncate">{item.label}</div>
                    <div className="text-[11px] text-slate-400 truncate">{item.sub}</div>
                  </div>
                </div>
                <ArrowRight className="w-3.5 h-3.5 text-slate-500 group-hover:text-cyan-400 transition-colors" />
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
