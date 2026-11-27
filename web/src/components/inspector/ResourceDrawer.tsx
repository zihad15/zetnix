import { useEffect, useState } from 'react'
import {
  Copy,
  FileCode2,
  Globe,
  HardDrive,
  Info,
  KeyRound,
  Lock,
  Minus,
  Network,
  Plus,
  RotateCw,
  ScrollText,
  Server,
  Terminal,
  Trash2,
  X,
} from 'lucide-react'
import type {
  ConfigMapSummary,
  IngressSummary,
  InspectedResource,
  NodeSummary,
  PodSummary,
  PVCSummary,
  SecretSummary,
  ServiceSummary,
  WorkloadSummary,
} from '../../types/k8s'
import { api, formatAge, formatCores, formatMemoryMiB } from '../../lib/api'
import { KindBadge, StatusBadge } from '../shared/StatusBadge'
import { LogViewer } from './LogViewer'
import { WebTerminal } from './WebTerminal'
import { YamlStudio } from './YamlStudio'

interface ResourceDrawerProps {
  resource: InspectedResource | null
  pods: PodSummary[]
  workloads: WorkloadSummary[]
  nodes: NodeSummary[]
  services: ServiceSummary[]
  ingresses: IngressSummary[]
  configMaps: ConfigMapSummary[]
  secrets: SecretSummary[]
  pvcs: PVCSummary[]
  onClose: () => void
  onChanged: (notice: string) => void
  onInspect: (res: InspectedResource) => void
}

function LabelChips({ labels }: { labels?: Record<string, string> }) {
  const entries = Object.entries(labels || {})
  if (entries.length === 0) return null
  return (
    <div>
      <h3 className="text-xs font-bold uppercase tracking-wider text-slate-300 mb-2">Labels</h3>
      <div className="flex flex-wrap gap-1.5">
        {entries.map(([k, v]) => (
          <span
            key={k}
            className="px-2 py-0.5 rounded-lg bg-[#0d1424] border border-slate-800 text-[11px] font-mono text-slate-300"
          >
            <span className="text-cyan-400">{k}</span>={v}
          </span>
        ))}
      </div>
    </div>
  )
}

export function ResourceDrawer({
  resource,
  pods,
  workloads,
  nodes,
  services,
  ingresses,
  configMaps,
  secrets,
  pvcs,
  onClose,
  onChanged,
  onInspect,
}: ResourceDrawerProps) {
  const [tab, setTab] = useState<'details' | 'logs' | 'terminal' | 'yaml'>('details')
  const [yamlText, setYamlText] = useState('')
  const [loadingYaml, setLoadingYaml] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [actionBusy, setActionBusy] = useState(false)
  const [replicasDraft, setReplicasDraft] = useState(1)
  const [copiedKey, setCopiedKey] = useState<string | null>(null)

  useEffect(() => {
    if (!resource) return
    setTab(resource.initialTab || 'details')
    setConfirmDelete(false)
    setLoadingYaml(true)
    api
      .getYaml(resource.kind, resource.namespace, resource.name)
      .then((res) => setYamlText(res.yaml))
      .catch((err) => setYamlText(`# Unable to fetch YAML: ${err.message}`))
      .finally(() => setLoadingYaml(false))
  }, [resource])

  const matchedPod =
    resource?.kind === 'Pod'
      ? pods.find((p) => p.namespace === resource.namespace && p.name === resource.name)
      : undefined

  const matchedWorkload = ['Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob'].includes(resource?.kind || '')
    ? workloads.find((w) => w.kind === resource?.kind && w.namespace === resource?.namespace && w.name === resource?.name)
    : undefined

  const matchedNode = resource?.kind === 'Node' ? nodes.find((n) => n.name === resource.name) : undefined
  const matchedService =
    resource?.kind === 'Service' ? services.find((s) => s.namespace === resource.namespace && s.name === resource.name) : undefined
  const matchedIngress =
    resource?.kind === 'Ingress' ? ingresses.find((i) => i.namespace === resource.namespace && i.name === resource.name) : undefined
  const matchedConfigMap =
    resource?.kind === 'ConfigMap' ? configMaps.find((c) => c.namespace === resource.namespace && c.name === resource.name) : undefined
  const matchedSecret =
    resource?.kind === 'Secret' ? secrets.find((s) => s.namespace === resource.namespace && s.name === resource.name) : undefined
  const matchedPVC =
    resource?.kind === 'PersistentVolumeClaim'
      ? pvcs.find((p) => p.namespace === resource.namespace && p.name === resource.name)
      : undefined

  const childPods = matchedWorkload
    ? pods.filter((p) => {
        if (p.namespace !== matchedWorkload.namespace || !p.ownerName) return false
        if (p.ownerName === matchedWorkload.name) return true
        if (matchedWorkload.kind === 'Deployment' || matchedWorkload.kind === 'CronJob') {
          const lastDash = p.ownerName.lastIndexOf('-')
          return lastDash > 0 && p.ownerName.slice(0, lastDash) === matchedWorkload.name
        }
        return false
      })
    : []

  const nodePods = matchedNode ? pods.filter((p) => p.nodeName === matchedNode.name) : []

  useEffect(() => {
    if (matchedWorkload) {
      setReplicasDraft(matchedWorkload.desiredReplicas)
    }
  }, [matchedWorkload])

  if (!resource) return null

  const isPod = resource.kind === 'Pod'
  const isScalable = resource.kind === 'Deployment' || resource.kind === 'StatefulSet'
  const isRestartable = isScalable || resource.kind === 'DaemonSet'
  const containers = matchedPod?.containers.map((c) => c.name) || []

  const handleScale = async (next: number) => {
    if (next < 0) return
    setActionBusy(true)
    try {
      await api.scaleWorkload(resource.kind, resource.namespace, resource.name, next)
      setReplicasDraft(next)
      onChanged(`Scaled ${resource.kind}/${resource.name} to ${next} replicas`)
    } catch (e) {
      onChanged(`Scale failed: ${(e as Error).message}`)
    } finally {
      setActionBusy(false)
    }
  }

  const handleRestart = async () => {
    setActionBusy(true)
    try {
      await api.restartWorkload(resource.kind, resource.namespace, resource.name)
      onChanged(`Triggered rollout restart for ${resource.kind}/${resource.name}`)
    } catch (e) {
      onChanged(`Restart failed: ${(e as Error).message}`)
    } finally {
      setActionBusy(false)
    }
  }

  const handleDelete = async () => {
    setActionBusy(true)
    try {
      await api.deleteResource(resource.kind, resource.namespace, resource.name)
      onChanged(`Deleted ${resource.kind}/${resource.name}`)
      onClose()
    } catch (e) {
      onChanged(`Delete failed: ${(e as Error).message}`)
    } finally {
      setActionBusy(false)
    }
  }

  const parentWorkloadForPod =
    matchedPod && matchedPod.ownerName
      ? workloads.find((w) => {
          if (w.namespace !== matchedPod.namespace) return false
          if (w.name === matchedPod.ownerName) return true
          if (w.kind === 'Deployment' && matchedPod.ownerKind === 'ReplicaSet') {
            const idx = matchedPod.ownerName!.lastIndexOf('-')
            return idx > 0 && matchedPod.ownerName!.slice(0, idx) === w.name
          }
          return false
        })
      : undefined

  return (
    <div className="fixed inset-0 z-40 flex justify-end bg-black/55 backdrop-blur-xs" onClick={onClose}>
      <div
        className="w-full max-w-3xl h-full bg-[#090f1d] border-l border-slate-800 shadow-2xl flex flex-col text-slate-100"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="px-5 py-4 border-b border-slate-800/90 flex flex-wrap items-center justify-between gap-3 bg-[#0c1324]">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <KindBadge kind={resource.kind} />
              <span className="text-xs font-mono text-slate-400">{resource.namespace || 'cluster-scoped'}</span>
              {matchedPod && <StatusBadge status={matchedPod.statusReason} />}
              {matchedWorkload && <StatusBadge status={matchedWorkload.status} />}
              {matchedNode && <StatusBadge status={matchedNode.status} />}
              {matchedPVC && <StatusBadge status={matchedPVC.status} />}
            </div>
            <h2 className="text-lg font-bold text-slate-100 truncate mt-1 font-mono">{resource.name}</h2>
          </div>

          <div className="flex items-center gap-2">
            {isScalable && (
              <div className="inline-flex items-center rounded-xl bg-[#090f1d] border border-slate-700 p-0.5">
                <button
                  disabled={actionBusy || replicasDraft <= 0}
                  onClick={() => handleScale(replicasDraft - 1)}
                  className="p-1.5 hover:bg-slate-800 rounded-lg text-slate-300 disabled:opacity-40 cursor-pointer"
                  title="Scale down"
                >
                  <Minus className="w-3.5 h-3.5" />
                </button>
                <span className="px-2.5 text-xs font-mono font-bold text-cyan-300">{replicasDraft}</span>
                <button
                  disabled={actionBusy}
                  onClick={() => handleScale(replicasDraft + 1)}
                  className="p-1.5 hover:bg-slate-800 rounded-lg text-slate-300 cursor-pointer"
                  title="Scale up"
                >
                  <Plus className="w-3.5 h-3.5" />
                </button>
              </div>
            )}

            {isRestartable && (
              <button
                disabled={actionBusy}
                onClick={handleRestart}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-[#090f1d] border border-slate-700 hover:border-cyan-500 text-xs font-semibold text-slate-200 cursor-pointer"
              >
                <RotateCw className={`w-3.5 h-3.5 text-cyan-400 ${actionBusy ? 'animate-spin' : ''}`} />
                Restart
              </button>
            )}

            {resource.kind !== 'Node' && (
              confirmDelete ? (
                <button
                  disabled={actionBusy}
                  onClick={handleDelete}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-rose-500 text-white text-xs font-bold cursor-pointer"
                >
                  Confirm Delete
                </button>
              ) : (
                <button
                  onClick={() => setConfirmDelete(true)}
                  className="p-2 rounded-xl bg-[#090f1d] border border-slate-700 hover:border-rose-500/60 text-slate-400 hover:text-rose-400 cursor-pointer"
                  title="Delete resource"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              )
            )}

            <button
              onClick={onClose}
              className="p-2 rounded-xl bg-[#090f1d] border border-slate-700 hover:bg-slate-800 text-slate-400 hover:text-slate-100 cursor-pointer"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Drawer Tabs */}
        <div className="px-5 flex items-center gap-1 border-b border-slate-800 bg-[#0a101f]">
          {[
            { id: 'details' as const, label: 'Details', icon: <Info className="w-3.5 h-3.5" />, show: true },
            { id: 'logs' as const, label: 'Live Logs', icon: <ScrollText className="w-3.5 h-3.5" />, show: isPod },
            { id: 'terminal' as const, label: 'Terminal (Exec)', icon: <Terminal className="w-3.5 h-3.5" />, show: isPod },
            { id: 'yaml' as const, label: 'YAML Manifest', icon: <FileCode2 className="w-3.5 h-3.5" />, show: true },
          ]
            .filter((t) => t.show)
            .map((t) => (
              <button
                key={t.id}
                onClick={() => setTab(t.id)}
                className={`inline-flex items-center gap-1.5 px-3.5 py-2.5 text-xs font-semibold border-b-2 transition-colors cursor-pointer ${
                  tab === t.id
                    ? 'border-cyan-400 text-cyan-300'
                    : 'border-transparent text-slate-400 hover:text-slate-200'
                }`}
              >
                {t.icon}
                {t.label}
              </button>
            ))}
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-5">
          {tab === 'details' && (
            <div className="space-y-5">
              {matchedPod && (
                <>
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                    <div
                      onClick={() => matchedPod.nodeName && onInspect({ kind: 'Node', namespace: '', name: matchedPod.nodeName })}
                      className="p-3 rounded-xl bg-[#0d1424] border border-slate-800 hover:border-cyan-500/40 cursor-pointer"
                    >
                      <div className="text-[11px] text-slate-400">Node</div>
                      <div className="text-xs font-mono font-bold text-cyan-300 mt-1 truncate">{matchedPod.nodeName || '—'}</div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Pod IP</div>
                      <div className="text-xs font-mono font-bold text-slate-100 mt-1">{matchedPod.podIp || '—'}</div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">CPU / Memory</div>
                      <div className="text-xs font-mono font-bold text-slate-100 mt-1">
                        {formatCores(matchedPod.cpuUsageMilli)} / {formatMemoryMiB(matchedPod.memoryUsageMiB)}
                      </div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Restarts / Age</div>
                      <div className="text-xs font-mono font-bold text-slate-100 mt-1">
                        {matchedPod.restarts} • {formatAge(matchedPod.createdAt)}
                      </div>
                    </div>
                  </div>

                  {parentWorkloadForPod && (
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800 flex items-center justify-between">
                      <span className="text-xs text-slate-400">Controlled By</span>
                      <button
                        onClick={() =>
                          onInspect({
                            kind: parentWorkloadForPod.kind,
                            namespace: parentWorkloadForPod.namespace,
                            name: parentWorkloadForPod.name,
                            raw: parentWorkloadForPod,
                          })
                        }
                        className="inline-flex items-center gap-2 text-xs font-mono font-bold text-cyan-300 hover:underline cursor-pointer"
                      >
                        <KindBadge kind={parentWorkloadForPod.kind} />
                        {parentWorkloadForPod.name}
                      </button>
                    </div>
                  )}

                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <h3 className="text-xs font-bold uppercase tracking-wider text-slate-300">Containers</h3>
                      <div className="flex items-center gap-2">
                        <button onClick={() => setTab('logs')} className="text-xs text-cyan-400 hover:text-cyan-300 font-semibold cursor-pointer">
                          Stream Logs →
                        </button>
                        <button onClick={() => setTab('terminal')} className="text-xs text-emerald-400 hover:text-emerald-300 font-semibold cursor-pointer">
                          Open Shell →
                        </button>
                      </div>
                    </div>
                    <div className="space-y-2.5">
                      {matchedPod.containers.map((c) => (
                        <div key={c.name} className="p-3.5 rounded-xl bg-[#0d1424] border border-slate-800 space-y-2">
                          <div className="flex items-center justify-between">
                            <span className="font-mono text-sm font-bold text-slate-100">{c.name}</span>
                            <StatusBadge status={c.reason || c.state} />
                          </div>
                          <div className="text-xs font-mono text-slate-400 break-all">{c.image}</div>
                          <div className="flex flex-wrap items-center gap-4 text-[11px] font-mono text-slate-400">
                            <span>Ready: <b className="text-slate-200">{String(c.ready)}</b></span>
                            <span>Restarts: <b className="text-slate-200">{c.restartCount}</b></span>
                            <span>CPU: <b className="text-cyan-300">{formatCores(c.cpuUsageMilli)}</b></span>
                            <span>Mem: <b className="text-violet-300">{formatMemoryMiB(c.memoryUsageMiB)}</b></span>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                  <LabelChips labels={matchedPod.labels} />
                </>
              )}

              {matchedWorkload && (
                <>
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Replicas</div>
                      <div className="text-sm font-mono font-bold text-cyan-300 mt-1">
                        {matchedWorkload.readyReplicas} / {matchedWorkload.desiredReplicas}
                      </div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Strategy</div>
                      <div className="text-xs font-mono font-bold text-slate-100 mt-1">{matchedWorkload.strategy || matchedWorkload.schedule || 'Standard'}</div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Updated</div>
                      <div className="text-sm font-mono font-bold text-slate-100 mt-1">{matchedWorkload.updatedReplicas}</div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Age</div>
                      <div className="text-sm font-mono font-bold text-slate-100 mt-1">{formatAge(matchedWorkload.createdAt)}</div>
                    </div>
                  </div>

                  <div>
                    <h3 className="text-xs font-bold uppercase tracking-wider text-slate-300 mb-2">
                      Managed Pods ({childPods.length})
                    </h3>
                    <div className="space-y-2">
                      {childPods.length === 0 ? (
                        <div className="p-4 rounded-xl bg-[#0d1424] border border-slate-800 text-xs text-slate-500">
                          No active pods scheduled for this workload.
                        </div>
                      ) : (
                        childPods.map((p) => (
                          <div
                            key={p.name}
                            className="p-3 rounded-xl bg-[#0d1424] border border-slate-800 flex flex-wrap items-center justify-between gap-2"
                          >
                            <div className="min-w-0">
                              <div className="flex items-center gap-2">
                                <button
                                  onClick={() => onInspect({ kind: 'Pod', namespace: p.namespace, name: p.name, raw: p })}
                                  className="text-xs font-mono font-bold text-cyan-300 hover:underline truncate cursor-pointer"
                                >
                                  {p.name}
                                </button>
                                <StatusBadge status={p.statusReason} />
                              </div>
                              <div className="text-[11px] font-mono text-slate-400 mt-1">
                                {p.nodeName} • {p.podIp || 'no-ip'} • {formatCores(p.cpuUsageMilli)} • {formatMemoryMiB(p.memoryUsageMiB)}
                              </div>
                            </div>
                            <div className="flex items-center gap-1.5">
                              <button
                                onClick={() => onInspect({ kind: 'Pod', namespace: p.namespace, name: p.name, initialTab: 'logs', raw: p })}
                                className="px-2.5 py-1 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs text-slate-200 cursor-pointer"
                              >
                                Logs
                              </button>
                              <button
                                onClick={() => onInspect({ kind: 'Pod', namespace: p.namespace, name: p.name, initialTab: 'terminal', raw: p })}
                                className="px-2.5 py-1 rounded-lg bg-cyan-500/15 hover:bg-cyan-500/25 border border-cyan-500/30 text-xs text-cyan-300 cursor-pointer"
                              >
                                Shell
                              </button>
                            </div>
                          </div>
                        ))
                      )}
                    </div>
                  </div>
                  <LabelChips labels={matchedWorkload.labels} />
                </>
              )}

              {matchedNode && (
                <>
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Internal IP</div>
                      <div className="text-xs font-mono font-bold text-cyan-300 mt-1">{matchedNode.internalIp || '—'}</div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">CPU Used</div>
                      <div className="text-xs font-mono font-bold mt-1">
                        {formatCores(matchedNode.cpuUsedMilli)} / {matchedNode.cpuCapacityCores.toFixed(1)}c
                      </div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Memory Used</div>
                      <div className="text-xs font-mono font-bold mt-1">
                        {formatMemoryMiB(matchedNode.memUsedMiB)} / {formatMemoryMiB(matchedNode.memCapacityMiB)}
                      </div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Pods Capacity</div>
                      <div className="text-xs font-mono font-bold mt-1">{matchedNode.podsAllocated} / {matchedNode.podsCapacity}</div>
                    </div>
                  </div>

                  <div className="p-4 rounded-xl bg-[#0d1424] border border-slate-800 text-xs font-mono space-y-1.5">
                    <div className="flex items-center gap-2 text-slate-300 font-bold mb-2">
                      <Server className="w-4 h-4 text-teal-400" />
                      Host & Runtime Info
                    </div>
                    <div className="text-slate-400">OS: <span className="text-slate-200">{matchedNode.osImage} ({matchedNode.architecture})</span></div>
                    <div className="text-slate-400">Kubelet: <span className="text-slate-200">{matchedNode.kubeletVersion}</span></div>
                    <div className="text-slate-400">Runtime: <span className="text-slate-200">{matchedNode.containerRuntime}</span></div>
                  </div>

                  <div>
                    <h3 className="text-xs font-bold uppercase tracking-wider text-slate-300 mb-2">Node Conditions</h3>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                      {matchedNode.conditions.map((c) => (
                        <div key={c.type} className="p-3 rounded-xl bg-[#0d1424] border border-slate-800 flex items-center justify-between text-xs">
                          <span className="font-mono font-bold text-slate-200">{c.type}</span>
                          <span className={`font-mono px-2 py-0.5 rounded-full text-[11px] ${
                            (c.type === 'Ready' ? c.status === 'True' : c.status === 'False')
                              ? 'bg-emerald-500/15 text-emerald-300'
                              : 'bg-amber-500/15 text-amber-300'
                          }`}>
                            {c.status}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>

                  <div>
                    <h3 className="text-xs font-bold uppercase tracking-wider text-slate-300 mb-2">
                      Scheduled Pods ({nodePods.length})
                    </h3>
                    <div className="space-y-2">
                      {nodePods.map((p) => (
                        <div key={`${p.namespace}/${p.name}`} className="p-3 rounded-xl bg-[#0d1424] border border-slate-800 flex items-center justify-between gap-2 text-xs">
                          <div className="min-w-0">
                            <button
                              onClick={() => onInspect({ kind: 'Pod', namespace: p.namespace, name: p.name, raw: p })}
                              className="font-mono font-bold text-cyan-300 hover:underline truncate block cursor-pointer"
                            >
                              {p.name}
                            </button>
                            <span className="font-mono text-[11px] text-slate-400">{p.namespace} • {p.podIp}</span>
                          </div>
                          <div className="flex items-center gap-1.5">
                            <StatusBadge status={p.statusReason} />
                            <button
                              onClick={() => onInspect({ kind: 'Pod', namespace: p.namespace, name: p.name, initialTab: 'logs', raw: p })}
                              className="px-2 py-1 rounded bg-slate-800 text-slate-200 cursor-pointer"
                            >
                              Logs
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                </>
              )}

              {matchedService && (
                <>
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Type</div>
                      <div className="text-xs font-mono font-bold text-emerald-300 mt-1">{matchedService.type}</div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Cluster IP</div>
                      <div className="text-xs font-mono font-bold mt-1">{matchedService.clusterIp}</div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">External IP</div>
                      <div className="text-xs font-mono font-bold text-cyan-300 mt-1">{matchedService.externalIp || '—'}</div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Endpoints</div>
                      <div className="text-xs font-mono font-bold mt-1">{matchedService.endpoints} ready</div>
                    </div>
                  </div>

                  <div className="p-4 rounded-xl bg-[#0d1424] border border-slate-800 space-y-2">
                    <div className="flex items-center gap-2 text-xs font-bold text-emerald-400">
                      <Network className="w-4 h-4" />
                      Port Mappings
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {matchedService.ports.map((p, idx) => (
                        <span key={idx} className="px-3 py-1 rounded-lg bg-[#060a14] border border-slate-800 text-xs font-mono text-slate-200">
                          {p.name ? `${p.name}: ` : ''}{p.port} → {p.targetPort}/{p.protocol}
                          {p.nodePort ? ` (NodePort ${p.nodePort})` : ''}
                        </span>
                      ))}
                    </div>
                  </div>
                  <LabelChips labels={matchedService.selector} />
                </>
              )}

              {matchedIngress && (
                <>
                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">Ingress Class</div>
                      <div className="text-xs font-mono font-bold mt-1">{matchedIngress.className || 'default'}</div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">TLS Encryption</div>
                      <div className="text-xs font-mono font-bold mt-1 flex items-center gap-1.5">
                        {matchedIngress.tls ? <><Lock className="w-3.5 h-3.5 text-emerald-400" /> Enabled</> : 'HTTP Only'}
                      </div>
                    </div>
                    <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                      <div className="text-[11px] text-slate-400">LoadBalancer</div>
                      <div className="text-xs font-mono font-bold text-cyan-300 mt-1">{matchedIngress.address || '—'}</div>
                    </div>
                  </div>
                  <div className="p-4 rounded-xl bg-[#0d1424] border border-slate-800 space-y-2.5">
                    <div className="flex items-center gap-2 text-xs font-bold text-sky-400">
                      <Globe className="w-4 h-4" />
                      Routing Rules
                    </div>
                    {matchedIngress.rules.map((r, i) => (
                      <div key={i} className="p-2.5 rounded-lg bg-[#060a14] border border-slate-800 flex items-center justify-between text-xs font-mono">
                        <span className="text-sky-300">{r.host || '*'}{r.path}</span>
                        <span className="text-emerald-300">→ {r.serviceName}:{r.servicePort}</span>
                      </div>
                    ))}
                  </div>
                </>
              )}

              {matchedConfigMap && (
                <div className="space-y-3">
                  <h3 className="text-xs font-bold uppercase tracking-wider text-cyan-400">
                    Data Keys ({matchedConfigMap.keys.length})
                  </h3>
                  {Object.entries(matchedConfigMap.data || {}).map(([k, v]) => (
                    <div key={k} className="rounded-xl bg-[#0d1424] border border-slate-800 overflow-hidden">
                      <div className="px-3.5 py-2 border-b border-slate-800 flex items-center justify-between bg-[#0a101f]">
                        <span className="font-mono text-xs font-bold text-cyan-300">{k}</span>
                        <button
                          onClick={() => {
                            navigator.clipboard.writeText(v)
                            setCopiedKey(k)
                            setTimeout(() => setCopiedKey(null), 1500)
                          }}
                          className="inline-flex items-center gap-1 text-[11px] text-slate-400 hover:text-slate-200 cursor-pointer"
                        >
                          <Copy className="w-3 h-3" />
                          {copiedKey === k ? 'Copied' : 'Copy'}
                        </button>
                      </div>
                      <pre className="p-3 text-xs font-mono text-slate-200 overflow-auto max-h-48 whitespace-pre-wrap">{v}</pre>
                    </div>
                  ))}
                </div>
              )}

              {matchedSecret && (
                <div className="p-4 rounded-xl bg-[#0d1424] border border-slate-800 space-y-3">
                  <div className="flex items-center justify-between">
                    <span className="flex items-center gap-2 text-xs font-bold text-violet-400">
                      <KeyRound className="w-4 h-4" />
                      Secret Metadata (Values Masked)
                    </span>
                    <span className="text-xs font-mono text-slate-400">{matchedSecret.type} • {matchedSecret.dataSize} B</span>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {matchedSecret.keys.map((k) => (
                      <span key={k} className="px-2.5 py-1 rounded-lg bg-[#060a14] border border-slate-800 text-xs font-mono text-violet-300">
                        {k}: ••••••••
                      </span>
                    ))}
                  </div>
                  <p className="text-[11px] text-slate-400">Switch to the YAML Manifest tab if you need to inspect or update encoded values.</p>
                </div>
              )}

              {matchedPVC && (
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                  <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                    <div className="text-[11px] text-slate-400 flex items-center gap-1"><HardDrive className="w-3 h-3 text-orange-400" />Capacity</div>
                    <div className="text-sm font-mono font-bold text-slate-100 mt-1">{matchedPVC.capacity || '—'}</div>
                  </div>
                  <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                    <div className="text-[11px] text-slate-400">StorageClass</div>
                    <div className="text-xs font-mono font-bold mt-1">{matchedPVC.storageClass || 'default'}</div>
                  </div>
                  <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                    <div className="text-[11px] text-slate-400">Access Modes</div>
                    <div className="text-xs font-mono font-bold mt-1">{matchedPVC.accessModes.join(', ')}</div>
                  </div>
                  <div className="p-3 rounded-xl bg-[#0d1424] border border-slate-800">
                    <div className="text-[11px] text-slate-400">Volume</div>
                    <div className="text-xs font-mono font-bold text-cyan-300 mt-1 truncate">{matchedPVC.volumeName || '—'}</div>
                  </div>
                </div>
              )}

              {!matchedPod && !matchedWorkload && !matchedNode && !matchedService && !matchedIngress && !matchedConfigMap && !matchedSecret && !matchedPVC && (
                <div className="p-4 rounded-xl bg-[#0d1424] border border-slate-800">
                  <div className="text-xs text-slate-400 mb-2">Resource Snapshot</div>
                  <pre className="text-xs font-mono text-cyan-100 overflow-auto max-h-96">
                    {JSON.stringify(resource.raw || { kind: resource.kind, namespace: resource.namespace, name: resource.name }, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          )}

          {tab === 'logs' && isPod && (
            <LogViewer namespace={resource.namespace} pod={resource.name} containers={containers} />
          )}

          {tab === 'terminal' && isPod && (
            <WebTerminal namespace={resource.namespace} pod={resource.name} containers={containers} />
          )}

          {tab === 'yaml' && (
            loadingYaml ? (
              <div className="py-12 text-center text-xs text-slate-400 font-mono">Loading live resource YAML...</div>
            ) : (
              <YamlStudio
                initialYaml={yamlText}
                title={`${resource.kind} / ${resource.name}`}
                onApplied={(summary) => onChanged(summary)}
              />
            )
          )}
        </div>
      </div>
    </div>
  )
}
