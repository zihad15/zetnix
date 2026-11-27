import { useMemo, useState } from 'react'
import { AlertCircle, Minus, Plus, RotateCw, ScrollText, Search, Terminal } from 'lucide-react'
import type { InspectedResource, PodSummary, WorkloadSummary } from '../../types/k8s'
import { api, formatAge, formatCores, formatMemoryMiB } from '../../lib/api'
import { KindBadge, StatusBadge } from '../shared/StatusBadge'

interface WorkloadsViewProps {
  mode: 'workloads' | 'pods'
  workloads: WorkloadSummary[]
  pods: PodSummary[]
  dark: boolean
  onInspect: (res: InspectedResource) => void
  onChanged: (msg: string) => void
}

export function WorkloadsView({ mode, workloads, pods, dark, onInspect, onChanged }: WorkloadsViewProps) {
  const [search, setSearch] = useState('')
  const [kindFilter, setKindFilter] = useState('all')
  const [onlyIssues, setOnlyIssues] = useState(false)
  const [busyKey, setBusyKey] = useState<string | null>(null)

  const filteredWorkloads = useMemo(() => {
    const q = search.trim().toLowerCase()
    return workloads.filter((w) => {
      if (kindFilter !== 'all' && w.kind !== kindFilter) return false
      if (onlyIssues && ['Ready', 'Active', 'Complete'].includes(w.status)) return false
      if (!q) return true
      return (
        w.name.toLowerCase().includes(q) ||
        w.namespace.toLowerCase().includes(q) ||
        w.images.some((i) => i.toLowerCase().includes(q))
      )
    })
  }, [workloads, search, kindFilter, onlyIssues])

  const filteredPods = useMemo(() => {
    const q = search.trim().toLowerCase()
    return pods.filter((p) => {
      if (onlyIssues && ['Running', 'Succeeded'].includes(p.statusReason)) return false
      if (!q) return true
      return (
        p.name.toLowerCase().includes(q) ||
        p.namespace.toLowerCase().includes(q) ||
        p.nodeName.toLowerCase().includes(q) ||
        p.statusReason.toLowerCase().includes(q)
      )
    })
  }, [pods, search, onlyIssues])

  const cardCls = `rounded-2xl border overflow-hidden ${
    dark ? 'bg-[#0b1120]/85 border-slate-800/80' : 'bg-white/95 border-slate-200 shadow-sm'
  }`

  const quickScale = async (w: WorkloadSummary, delta: number) => {
    const next = Math.max(0, w.desiredReplicas + delta)
    setBusyKey(`${w.kind}/${w.namespace}/${w.name}`)
    try {
      await api.scaleWorkload(w.kind, w.namespace, w.name, next)
      onChanged(`Scaled ${w.kind}/${w.name} to ${next}`)
    } catch (e) {
      onChanged(`Scale failed: ${(e as Error).message}`)
    } finally {
      setBusyKey(null)
    }
  }

  const quickRestart = async (w: WorkloadSummary) => {
    setBusyKey(`${w.kind}/${w.namespace}/${w.name}`)
    try {
      await api.restartWorkload(w.kind, w.namespace, w.name)
      onChanged(`Triggered rollout restart for ${w.kind}/${w.name}`)
    } catch (e) {
      onChanged(`Restart failed: ${(e as Error).message}`)
    } finally {
      setBusyKey(null)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative">
            <Search className="w-3.5 h-3.5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={mode === 'pods' ? 'Search pods, nodes, status...' : 'Search workloads, images...'}
              className={`pl-9 pr-3 py-1.5 rounded-xl border text-xs w-64 focus:outline-none focus:border-cyan-500 ${
                dark ? 'bg-[#0b1120] border-slate-800 text-slate-100' : 'bg-white border-slate-300 text-slate-900'
              }`}
            />
          </div>

          {mode === 'workloads' && (
            <div className={`flex flex-wrap items-center gap-1 p-1 rounded-xl border text-xs ${dark ? 'bg-[#0b1120] border-slate-800' : 'bg-white border-slate-200'}`}>
              {['all', 'Deployment', 'StatefulSet', 'DaemonSet', 'CronJob', 'Job'].map((k) => (
                <button
                  key={k}
                  onClick={() => setKindFilter(k)}
                  className={`px-2.5 py-1 rounded-lg font-medium cursor-pointer transition-colors ${
                    kindFilter === k
                      ? dark
                        ? 'bg-cyan-500/20 text-cyan-300'
                        : 'bg-cyan-50 text-cyan-700'
                      : dark
                        ? 'text-slate-400 hover:text-slate-200'
                        : 'text-slate-500 hover:text-slate-900'
                  }`}
                >
                  {k === 'all' ? 'All' : k}
                </button>
              ))}
            </div>
          )}

          <button
            onClick={() => setOnlyIssues((v) => !v)}
            className={`inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-xl border text-xs font-medium cursor-pointer transition-colors ${
              onlyIssues
                ? 'bg-rose-500/20 border-rose-500/40 text-rose-400'
                : dark
                  ? 'bg-[#0b1120] border-slate-800 text-slate-400 hover:text-slate-200'
                  : 'bg-white border-slate-200 text-slate-600 hover:text-slate-900'
            }`}
          >
            <AlertCircle className="w-3.5 h-3.5" />
            Issues Only
          </button>
        </div>
        <div className="text-xs font-mono text-slate-400">
          {mode === 'pods' ? `${filteredPods.length} Pods` : `${filteredWorkloads.length} Workloads`}
        </div>
      </div>

      {mode === 'workloads' ? (
        <div className={cardCls}>
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className={`border-b text-[11px] uppercase tracking-wider font-semibold ${dark ? 'border-slate-800 text-slate-400 bg-[#0d1424]/70' : 'border-slate-200 text-slate-500 bg-slate-50'}`}>
                  <th className="py-3 px-4">Workload</th>
                  <th className="py-3 px-3">Kind</th>
                  <th className="py-3 px-3">Namespace</th>
                  <th className="py-3 px-3">Status</th>
                  <th className="py-3 px-3">Replicas</th>
                  <th className="py-3 px-3">Images</th>
                  <th className="py-3 px-3">Age</th>
                  <th className="py-3 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className={`divide-y text-xs ${dark ? 'divide-slate-800/60' : 'divide-slate-200'}`}>
                {filteredWorkloads.map((w) => {
                  const key = `${w.kind}/${w.namespace}/${w.name}`
                  const scalable = w.kind === 'Deployment' || w.kind === 'StatefulSet'
                  return (
                    <tr key={key} className={`transition-colors ${dark ? 'hover:bg-slate-800/35' : 'hover:bg-slate-50'}`}>
                      <td className="py-3 px-4">
                        <button
                          onClick={() => onInspect({ kind: w.kind, namespace: w.namespace, name: w.name, raw: w })}
                          className={`font-mono font-bold hover:underline cursor-pointer ${dark ? 'text-cyan-300' : 'text-cyan-700'}`}
                        >
                          {w.name}
                        </button>
                      </td>
                      <td className="py-3 px-3"><KindBadge kind={w.kind} /></td>
                      <td className="py-3 px-3 font-mono text-slate-400">{w.namespace}</td>
                      <td className="py-3 px-3"><StatusBadge status={w.status} /></td>
                      <td className="py-3 px-3 font-mono">
                        {w.kind === 'CronJob' ? (
                          <span className="text-amber-400">{w.schedule}</span>
                        ) : (
                          <span className={w.readyReplicas < w.desiredReplicas ? 'text-amber-500 font-bold' : 'text-emerald-500 font-bold'}>
                            {w.readyReplicas}/{w.desiredReplicas}
                          </span>
                        )}
                      </td>
                      <td className="py-3 px-3 font-mono text-slate-400 max-w-[260px] truncate">{w.images.join(', ')}</td>
                      <td className="py-3 px-3 font-mono text-slate-400">{formatAge(w.createdAt)}</td>
                      <td className="py-3 px-4 text-right">
                        <div className="inline-flex items-center gap-1.5">
                          {scalable && (
                            <div className={`inline-flex items-center rounded-lg border ${dark ? 'border-slate-700 bg-[#0d1424]' : 'border-slate-300 bg-white'}`}>
                              <button
                                disabled={busyKey === key || w.desiredReplicas <= 0}
                                onClick={() => quickScale(w, -1)}
                                className="p-1 hover:bg-slate-800/30 disabled:opacity-40 cursor-pointer"
                                title="Scale -1"
                              >
                                <Minus className="w-3 h-3" />
                              </button>
                              <button
                                disabled={busyKey === key}
                                onClick={() => quickScale(w, 1)}
                                className="p-1 hover:bg-slate-800/30 cursor-pointer"
                                title="Scale +1"
                              >
                                <Plus className="w-3 h-3" />
                              </button>
                            </div>
                          )}
                          {(scalable || w.kind === 'DaemonSet') && (
                            <button
                              disabled={busyKey === key}
                              onClick={() => quickRestart(w)}
                              className={`p-1.5 rounded-lg border cursor-pointer ${dark ? 'border-slate-700 bg-[#0d1424] hover:border-cyan-500 text-slate-300' : 'border-slate-300 bg-white hover:border-cyan-500 text-slate-700'}`}
                              title="Rollout Restart"
                            >
                              <RotateCw className={`w-3.5 h-3.5 ${busyKey === key ? 'animate-spin text-cyan-400' : ''}`} />
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  )
                })}
                {filteredWorkloads.length === 0 && (
                  <tr>
                    <td colSpan={8} className="py-12 text-center text-slate-500">
                      No workloads match your filter.{' '}
                      {(search || onlyIssues || kindFilter !== 'all') && (
                        <button
                          onClick={() => {
                            setSearch('')
                            setOnlyIssues(false)
                            setKindFilter('all')
                          }}
                          className="text-cyan-400 underline ml-1 cursor-pointer"
                        >
                          Clear filters
                        </button>
                      )}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      ) : (
        <div className={cardCls}>
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className={`border-b text-[11px] uppercase tracking-wider font-semibold ${dark ? 'border-slate-800 text-slate-400 bg-[#0d1424]/70' : 'border-slate-200 text-slate-500 bg-slate-50'}`}>
                  <th className="py-3 px-4">Pod</th>
                  <th className="py-3 px-3">Namespace</th>
                  <th className="py-3 px-3">Status</th>
                  <th className="py-3 px-3">Ready</th>
                  <th className="py-3 px-3">Restarts</th>
                  <th className="py-3 px-3">Node</th>
                  <th className="py-3 px-3">CPU / Mem</th>
                  <th className="py-3 px-3">Age</th>
                  <th className="py-3 px-4 text-right">Streams</th>
                </tr>
              </thead>
              <tbody className={`divide-y text-xs ${dark ? 'divide-slate-800/60' : 'divide-slate-200'}`}>
                {filteredPods.map((p) => (
                  <tr key={`${p.namespace}/${p.name}`} className={`transition-colors ${dark ? 'hover:bg-slate-800/35' : 'hover:bg-slate-50'}`}>
                    <td className="py-3 px-4">
                      <button
                        onClick={() => onInspect({ kind: 'Pod', namespace: p.namespace, name: p.name, raw: p })}
                        className={`font-mono font-bold hover:underline cursor-pointer ${dark ? 'text-cyan-300' : 'text-cyan-700'}`}
                      >
                        {p.name}
                      </button>
                    </td>
                    <td className="py-3 px-3 font-mono text-slate-400">{p.namespace}</td>
                    <td className="py-3 px-3"><StatusBadge status={p.statusReason} /></td>
                    <td className="py-3 px-3 font-mono">{p.ready}</td>
                    <td className={`py-3 px-3 font-mono ${p.restarts > 0 ? 'text-amber-500 font-bold' : 'text-slate-400'}`}>{p.restarts}</td>
                    <td className="py-3 px-3 font-mono text-slate-400">{p.nodeName || '—'}</td>
                    <td className="py-3 px-3 font-mono">
                      {formatCores(p.cpuUsageMilli)} / {formatMemoryMiB(p.memoryUsageMiB)}
                    </td>
                    <td className="py-3 px-3 font-mono text-slate-400">{formatAge(p.createdAt)}</td>
                    <td className="py-3 px-4 text-right">
                      <div className="inline-flex items-center gap-1.5">
                        <button
                          onClick={() => onInspect({ kind: 'Pod', namespace: p.namespace, name: p.name, initialTab: 'logs', raw: p })}
                          className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-lg border cursor-pointer ${
                            dark ? 'bg-[#0d1424] border-slate-700 hover:border-cyan-500 text-slate-200' : 'bg-white border-slate-300 hover:border-cyan-500 text-slate-700'
                          }`}
                        >
                          <ScrollText className="w-3 h-3 text-cyan-500" />
                          Logs
                        </button>
                        <button
                          onClick={() => onInspect({ kind: 'Pod', namespace: p.namespace, name: p.name, initialTab: 'terminal', raw: p })}
                          className="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-cyan-500/15 border border-cyan-500/35 hover:bg-cyan-500/25 text-cyan-400 cursor-pointer"
                        >
                          <Terminal className="w-3 h-3" />
                          Shell
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
                {filteredPods.length === 0 && (
                  <tr>
                    <td colSpan={9} className="py-12 text-center text-slate-500">
                      No pods match your filter.{' '}
                      {(search || onlyIssues) && (
                        <button
                          onClick={() => {
                            setSearch('')
                            setOnlyIssues(false)
                          }}
                          className="text-cyan-400 underline ml-1 cursor-pointer"
                        >
                          Clear filters
                        </button>
                      )}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
