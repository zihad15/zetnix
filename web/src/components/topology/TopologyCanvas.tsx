import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  AlertCircle,
  Box,
  Database,
  Globe,
  Maximize2,
  Network,
  Search,
  ZoomIn,
  ZoomOut,
} from 'lucide-react'
import type { InspectedResource, TopologyGraph, TopologyNode } from '../../types/k8s'

interface TopologyCanvasProps {
  graph: TopologyGraph | null
  dark: boolean
  onInspect: (res: InspectedResource) => void
}

type KindFilter = 'all' | 'Workload' | 'Service' | 'Ingress' | 'PersistentVolumeClaim'

const statusRingColor = (status: string) => {
  switch (status) {
    case 'healthy':
      return '#34d399'
    case 'degraded':
      return '#fbbf24'
    case 'failing':
      return '#fb7185'
    case 'pending':
      return '#38bdf8'
    default:
      return '#94a3b8'
  }
}

export function TopologyCanvas({ graph, dark, onInspect }: TopologyCanvasProps) {
  const [zoom, setZoom] = useState(0.95)
  const [pan, setPan] = useState({ x: 30, y: 20 })
  const [search, setSearch] = useState('')
  const [kindFilter, setKindFilter] = useState<KindFilter>('all')
  const [onlyIssues, setOnlyIssues] = useState(false)
  const [positions, setPositions] = useState<Record<string, { x: number; y: number }>>({})
  const [draggingNode, setDraggingNode] = useState<{ id: string; startX: number; startY: number; origX: number; origY: number } | null>(null)
  const [panning, setPanning] = useState<{ startX: number; startY: number; origX: number; origY: number } | null>(null)
  const movedRef = useRef(false)
  const viewportRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!graph) return
    setPositions((prev) => {
      const next: Record<string, { x: number; y: number }> = {}
      for (const n of graph.nodes) {
        next[n.id] = prev[n.id] || { x: n.x, y: n.y }
      }
      return next
    })
  }, [graph])

  const filteredNodes = useMemo(() => {
    if (!graph) return []
    const q = search.trim().toLowerCase()
    return graph.nodes.filter((n) => {
      if (onlyIssues && n.status === 'healthy') return false
      if (kindFilter !== 'all') {
        if (kindFilter === 'Workload') {
          if (!['Deployment', 'StatefulSet', 'DaemonSet', 'CronJob', 'Job', 'Pod'].includes(n.kind)) return false
        } else if (n.kind !== kindFilter) {
          return false
        }
      }
      if (!q) return true
      return (
        n.name.toLowerCase().includes(q) ||
        n.kind.toLowerCase().includes(q) ||
        n.namespace.toLowerCase().includes(q) ||
        (n.appGroup || '').toLowerCase().includes(q)
      )
    })
  }, [graph, search, kindFilter, onlyIssues])

  const visibleIds = useMemo(() => new Set(filteredNodes.map((n) => n.id)), [filteredNodes])

  const visibleEdges = useMemo(() => {
    if (!graph) return []
    return graph.edges.filter((e) => visibleIds.has(e.source) && visibleIds.has(e.target))
  }, [graph, visibleIds])

  const getPos = useCallback((n: TopologyNode) => positions[n.id] || { x: n.x, y: n.y }, [positions])

  const fitToScreen = useCallback(
    (resetPos = false) => {
      if (!graph || graph.nodes.length === 0) {
        setZoom(0.95)
        setPan({ x: 30, y: 20 })
        return
      }
      let nextPos = positions
      if (resetPos) {
        const clean: Record<string, { x: number; y: number }> = {}
        for (const n of graph.nodes) clean[n.id] = { x: n.x, y: n.y }
        setPositions(clean)
        nextPos = clean
      }
      const list = filteredNodes.length > 0 ? filteredNodes : graph.nodes
      let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity
      for (const n of list) {
        const p = nextPos[n.id] || { x: n.x, y: n.y }
        minX = Math.min(minX, p.x - 120)
        minY = Math.min(minY, p.y - 70)
        maxX = Math.max(maxX, p.x + 120)
        maxY = Math.max(maxY, p.y + 70)
      }
      const vw = viewportRef.current?.clientWidth || 1000
      const vh = viewportRef.current?.clientHeight || 650
      const bw = Math.max(240, maxX - minX)
      const bh = Math.max(200, maxY - minY)
      const nextZoom = Math.max(0.45, Math.min(1.15, Math.min((vw - 80) / bw, (vh - 80) / bh)))
      setZoom(nextZoom)
      setPan({
        x: (vw - bw * nextZoom) / 2 - minX * nextZoom,
        y: (vh - bh * nextZoom) / 2 - minY * nextZoom,
      })
    },
    [graph, filteredNodes, positions],
  )

  const groupBoxes = useMemo(() => {
    if (!graph) return []
    return graph.groups
      .map((g) => {
        const members = filteredNodes.filter((n) => g.nodeIds.includes(n.id))
        if (members.length < 2) return null
        let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity
        for (const m of members) {
          const p = getPos(m)
          minX = Math.min(minX, p.x - 108)
          minY = Math.min(minY, p.y - 68)
          maxX = Math.max(maxX, p.x + 108)
          maxY = Math.max(maxY, p.y + 68)
        }
        return {
          ...g,
          x: minX,
          y: minY,
          w: maxX - minX,
          h: maxY - minY,
        }
      })
      .filter((b): b is NonNullable<typeof b> => b !== null)
  }, [graph, filteredNodes, getPos])

  const handleMouseMove = (e: React.MouseEvent) => {
    if (draggingNode) {
      const dx = (e.clientX - draggingNode.startX) / zoom
      const dy = (e.clientY - draggingNode.startY) / zoom
      if (Math.abs(dx) > 3 || Math.abs(dy) > 3) movedRef.current = true
      setPositions((prev) => ({
        ...prev,
        [draggingNode.id]: { x: draggingNode.origX + dx, y: draggingNode.origY + dy },
      }))
      return
    }
    if (panning) {
      setPan({
        x: panning.origX + (e.clientX - panning.startX),
        y: panning.origY + (e.clientY - panning.startY),
      })
    }
  }

  const handleMouseUp = () => {
    setDraggingNode(null)
    setPanning(null)
  }

  const iconForKind = (kind: string) => {
    switch (kind) {
      case 'Ingress':
        return <Globe className="w-4 h-4 text-sky-400" />
      case 'Service':
        return <Network className="w-4 h-4 text-emerald-400" />
      case 'PersistentVolumeClaim':
        return <Database className="w-4 h-4 text-orange-400" />
      default:
        return <Box className="w-4 h-4 text-cyan-400" />
    }
  }

  return (
    <div
      className={`flex flex-col h-[calc(100vh-120px)] rounded-2xl border overflow-hidden relative select-none ${
        dark ? 'border-slate-800/80 bg-[#060a14]/90' : 'border-slate-200 bg-white/95 shadow-sm'
      }`}
    >
      {/* Canvas Toolbar */}
      <div
        className={`flex flex-wrap items-center justify-between gap-2 px-4 py-2.5 border-b z-10 ${
          dark ? 'border-slate-800/80 bg-[#0a101f]/90' : 'border-slate-200 bg-slate-50/95'
        }`}
      >
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative">
            <Search className="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Filter topology nodes..."
              className={`pl-8 pr-3 py-1 rounded-xl border text-xs focus:outline-none focus:border-cyan-500 w-48 ${
                dark
                  ? 'bg-[#0e1628] border-slate-800 text-slate-100 placeholder-slate-500'
                  : 'bg-white border-slate-300 text-slate-900 placeholder-slate-400'
              }`}
            />
          </div>

          <div
            className={`hidden sm:flex items-center gap-0.5 p-0.5 rounded-xl border text-[11px] font-medium ${
              dark ? 'bg-[#0e1628] border-slate-800' : 'bg-white border-slate-200'
            }`}
          >
            {(
              [
                ['all', 'All'],
                ['Ingress', 'Routes'],
                ['Service', 'Services'],
                ['Workload', 'Workloads'],
                ['PersistentVolumeClaim', 'Storage'],
              ] as [KindFilter, string][]
            ).map(([k, lbl]) => (
              <button
                key={k}
                onClick={() => setKindFilter(k)}
                className={`px-2 py-1 rounded-lg cursor-pointer transition-colors ${
                  kindFilter === k
                    ? dark
                      ? 'bg-cyan-500/20 text-cyan-300'
                      : 'bg-cyan-50 text-cyan-700'
                    : dark
                      ? 'text-slate-400 hover:text-slate-200'
                      : 'text-slate-500 hover:text-slate-900'
                }`}
              >
                {lbl}
              </button>
            ))}
          </div>

          <button
            onClick={() => setOnlyIssues((v) => !v)}
            className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-xl border text-xs font-medium cursor-pointer transition-colors ${
              onlyIssues
                ? 'bg-rose-500/20 border-rose-500/40 text-rose-400'
                : dark
                  ? 'bg-[#0e1628] border-slate-800 text-slate-400 hover:text-slate-200'
                  : 'bg-white border-slate-200 text-slate-600 hover:text-slate-900'
            }`}
          >
            <AlertCircle className="w-3.5 h-3.5" />
            Issues
          </button>
        </div>

        <div className="flex items-center gap-3 text-[11px] text-slate-400">
          <span className="hidden lg:flex items-center gap-1.5"><span className="w-2 h-2 rounded-full bg-emerald-400" />Healthy</span>
          <span className="hidden lg:flex items-center gap-1.5"><span className="w-2 h-2 rounded-full bg-amber-400" />Degraded</span>
          <span className="hidden lg:flex items-center gap-1.5"><span className="w-2 h-2 rounded-full bg-rose-400" />Failing</span>
          <div className={`flex items-center gap-1 pl-2 border-l ${dark ? 'border-slate-800' : 'border-slate-200'}`}>
            <button onClick={() => setZoom((z) => Math.max(0.45, z - 0.12))} className="p-1.5 rounded-lg hover:bg-slate-800/40 cursor-pointer" title="Zoom Out">
              <ZoomOut className="w-3.5 h-3.5" />
            </button>
            <span className={`font-mono w-11 text-center ${dark ? 'text-slate-200' : 'text-slate-700'}`}>{Math.round(zoom * 100)}%</span>
            <button onClick={() => setZoom((z) => Math.min(1.75, z + 0.12))} className="p-1.5 rounded-lg hover:bg-slate-800/40 cursor-pointer" title="Zoom In">
              <ZoomIn className="w-3.5 h-3.5" />
            </button>
            <button onClick={() => fitToScreen(true)} className="p-1.5 rounded-lg hover:bg-slate-800/40 cursor-pointer" title="Fit & Reset Topology Layout">
              <Maximize2 className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>

      {/* Interactive Pan/Zoom Workspace */}
      <div
        ref={viewportRef}
        className={`flex-1 relative overflow-hidden cursor-grab active:cursor-grabbing ${dark ? 'zetnix-grid-bg' : 'zetnix-light-grid-bg'}`}
        onMouseDown={(e) => {
          if (e.target === e.currentTarget || (e.target as HTMLElement).dataset.canvas === 'true') {
            setPanning({ startX: e.clientX, startY: e.clientY, origX: pan.x, origY: pan.y })
          }
        }}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseUp}
        onWheel={(e) => {
          const delta = e.deltaY > 0 ? -0.06 : 0.06
          setZoom((z) => Math.max(0.45, Math.min(1.75, z + delta)))
        }}
      >
        <div
          data-canvas="true"
          style={{
            transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
            transformOrigin: '0 0',
          }}
          className="absolute inset-0"
        >
          {/* SVG Layer for Application Group Hulls & Animated Edges */}
          <svg width="2400" height="1400" className="overflow-visible pointer-events-none">
            <defs>
              <marker id="arrowCyan" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                <path d="M 0 1 L 10 5 L 0 9 z" fill="#22d3ee" />
              </marker>
              <marker id="arrowViolet" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                <path d="M 0 1 L 10 5 L 0 9 z" fill="#a855f7" />
              </marker>
            </defs>

            {groupBoxes.map((box) => (
              <g key={box.id}>
                <rect
                  x={box.x}
                  y={box.y}
                  width={box.w}
                  height={box.h}
                  rx={22}
                  fill={dark ? 'rgba(30, 41, 59, 0.22)' : 'rgba(14, 165, 233, 0.05)'}
                  stroke={dark ? 'rgba(56, 189, 248, 0.25)' : 'rgba(14, 165, 233, 0.35)'}
                  strokeWidth="1.5"
                  strokeDasharray="6 6"
                />
                <text x={box.x + 16} y={box.y + 22} fill={dark ? '#7dd3fc' : '#0284c7'} fontSize="11" fontWeight="700" fontFamily="monospace">
                  app / {box.name} ({box.namespace})
                </text>
              </g>
            ))}

            {visibleEdges.map((edge) => {
              const srcNode = filteredNodes.find((n) => n.id === edge.source)
              const dstNode = filteredNodes.find((n) => n.id === edge.target)
              if (!srcNode || !dstNode) return null
              const s = getPos(srcNode)
              const t = getPos(dstNode)
              const midX = (s.x + t.x) / 2
              const midY = (s.y + t.y) / 2
              const path = `M ${s.x} ${s.y + 24} C ${s.x} ${midY}, ${t.x} ${midY}, ${t.x} ${t.y - 28}`
              const stroke = edge.type === 'mounts' ? '#a855f7' : '#06b6d4'
              const marker = edge.type === 'mounts' ? 'url(#arrowViolet)' : 'url(#arrowCyan)'
              return (
                <g key={edge.id}>
                  <path d={path} fill="none" stroke={stroke} strokeOpacity="0.22" strokeWidth="2.5" />
                  <path d={path} fill="none" stroke={stroke} strokeWidth="1.8" className="edge-flow" markerEnd={marker} />
                  {edge.label && (
                    <g transform={`translate(${midX}, ${midY})`}>
                      <rect
                        x={-(edge.label.length * 3.3 + 8)}
                        y={-10}
                        width={edge.label.length * 6.6 + 16}
                        height={18}
                        rx={6}
                        fill={dark ? '#0a1222' : '#f1f5f9'}
                        stroke={dark ? 'rgba(148,163,184,0.22)' : 'rgba(15,23,42,0.12)'}
                      />
                      <text y={2} textAnchor="middle" fill={dark ? '#cbd5e1' : '#334155'} fontSize="10" fontFamily="monospace">
                        {edge.label}
                      </text>
                    </g>
                  )}
                </g>
              )
            })}
          </svg>

          {filteredNodes.map((node) => {
            const pos = getPos(node)
            const ringColor = statusRingColor(node.status)
            const isWorkload = ['Deployment', 'StatefulSet', 'DaemonSet', 'CronJob', 'Job', 'Pod'].includes(node.kind)
            const desired = Math.max(1, node.desiredReplicas || 1)
            const ready = node.readyReplicas || 0
            const ratio = Math.min(1, ready / desired)
            const circ = 2 * Math.PI * 24

            return (
              <div
                key={node.id}
                style={{ left: pos.x - 92, top: pos.y - 42 }}
                onMouseDown={(e) => {
                  e.stopPropagation()
                  movedRef.current = false
                  setDraggingNode({
                    id: node.id,
                    startX: e.clientX,
                    startY: e.clientY,
                    origX: pos.x,
                    origY: pos.y,
                  })
                }}
                onClick={(e) => {
                  e.stopPropagation()
                  if (movedRef.current) return
                  onInspect({ kind: node.kind, namespace: node.namespace, name: node.name, raw: node })
                }}
                className={`absolute w-[184px] rounded-2xl border shadow-xl px-3 py-2.5 cursor-pointer transition-colors group ${
                  dark
                    ? 'border-slate-700/80 bg-[#0b1222]/95 hover:border-cyan-400/70'
                    : 'border-slate-200 bg-white/95 hover:border-cyan-500'
                }`}
              >
                <div className="flex items-center gap-2.5">
                  <div className="relative w-12 h-12 shrink-0 flex items-center justify-center">
                    <svg viewBox="0 0 56 56" className="w-12 h-12 -rotate-90">
                      <circle cx="28" cy="28" r="24" fill={dark ? 'rgba(15,23,42,0.8)' : '#f8fafc'} stroke="rgba(148,163,184,0.22)" strokeWidth="4" />
                      <circle
                        cx="28"
                        cy="28"
                        r="24"
                        fill="none"
                        stroke={ringColor}
                        strokeWidth="4"
                        strokeLinecap="round"
                        strokeDasharray={circ}
                        strokeDashoffset={isWorkload ? circ * (1 - ratio) : 0}
                      />
                    </svg>
                    <div className="absolute inset-0 flex items-center justify-center">
                      {iconForKind(node.kind)}
                    </div>
                  </div>

                  <div className="min-w-0 flex-1">
                    <div className="text-[10px] font-mono uppercase tracking-wider text-slate-400 truncate">
                      {node.kind === 'PersistentVolumeClaim' ? 'PVC' : node.kind}
                    </div>
                    <div
                      className={`text-xs font-bold truncate transition-colors ${
                        dark ? 'text-slate-100 group-hover:text-cyan-300' : 'text-slate-900 group-hover:text-cyan-700'
                      }`}
                    >
                      {node.name}
                    </div>
                    <div className="text-[10px] font-mono text-slate-400 truncate">{node.namespace}</div>
                  </div>
                </div>

                {node.pods && node.pods.length > 0 ? (
                  <div className={`mt-2 pt-1.5 border-t flex items-center justify-between ${dark ? 'border-slate-800/80' : 'border-slate-200'}`}>
                    <div className="flex items-center gap-1 flex-wrap">
                      {node.pods.map((p) => {
                        const bad = p.statusReason === 'CrashLoopBackOff' || p.phase === 'Failed' || p.statusReason === 'ImagePullBackOff'
                        const warn = p.phase === 'Pending'
                        return (
                          <span
                            key={p.name}
                            title={`${p.name} (${p.statusReason})`}
                            className={`w-2.5 h-2.5 rounded-full border ${dark ? 'border-slate-950' : 'border-white'} ${
                              bad ? 'bg-rose-400 animate-ping' : warn ? 'bg-amber-400' : 'bg-emerald-400'
                            }`}
                          />
                        )
                      })}
                    </div>
                    <span className={`text-[10px] font-mono ${dark ? 'text-slate-300' : 'text-slate-600'}`}>
                      {node.readyReplicas}/{node.desiredReplicas}
                    </span>
                  </div>
                ) : (
                  <div className={`mt-2 pt-1.5 border-t flex items-center justify-between text-[10px] font-mono text-slate-400 truncate ${dark ? 'border-slate-800/80' : 'border-slate-200'}`}>
                    <span className="truncate">
                      {node.meta?.hosts || node.meta?.ports || node.meta?.capacity || node.status}
                    </span>
                  </div>
                )}
              </div>
            )
          })}
        </div>

        {filteredNodes.length === 0 && (
          <div className="absolute inset-0 flex flex-col items-center justify-center text-slate-400 text-sm gap-2">
            <span>No resources match the topology filter in this namespace.</span>
            {(search || onlyIssues || kindFilter !== 'all') && (
              <button
                onClick={() => {
                  setSearch('')
                  setOnlyIssues(false)
                  setKindFilter('all')
                }}
                className="px-3 py-1 rounded-xl bg-cyan-500/15 text-cyan-400 border border-cyan-500/30 text-xs font-semibold cursor-pointer"
              >
                Reset Filters
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
