import React from 'react'
import {
  Activity,
  Box,
  Boxes,
  Cpu,
  FileCode2,
  GitBranch,
  HardDrive,
  Layers,
  Network,
  Radio,
  Server,
  Sparkles,
  Terminal,
} from 'lucide-react'
import type { NavSection, Perspective } from '../../types/k8s'
import { ZetnixLogo } from '../shared/StatusBadge'

interface SidebarProps {
  perspective: Perspective
  onPerspectiveChange: (p: Perspective) => void
  activeSection: NavSection
  onSelectSection: (s: NavSection) => void
  watchConnected: boolean
  dark: boolean
  counts?: {
    pods?: number
    workloads?: number
    nodes?: number
    warnings?: number
  }
}

export function Sidebar({
  perspective,
  onPerspectiveChange,
  activeSection,
  onSelectSection,
  watchConnected,
  dark,
  counts,
}: SidebarProps) {
  const adminNav: { id: NavSection; label: string; icon: React.ReactNode; badge?: number }[] = [
    { id: 'overview', label: 'Command Center', icon: <Activity className="w-4 h-4" /> },
    { id: 'topology', label: 'Topology Mesh', icon: <GitBranch className="w-4 h-4" /> },
    { id: 'workloads', label: 'Workloads', icon: <Layers className="w-4 h-4" />, badge: counts?.workloads },
    { id: 'pods', label: 'Pods & Shells', icon: <Box className="w-4 h-4" />, badge: counts?.pods },
    { id: 'networking', label: 'Services & Routes', icon: <Network className="w-4 h-4" /> },
    { id: 'storage', label: 'Storage & Secrets', icon: <HardDrive className="w-4 h-4" /> },
    { id: 'nodes', label: 'Nodes & Compute', icon: <Server className="w-4 h-4" />, badge: counts?.nodes },
    { id: 'events', label: 'Events Stream', icon: <Radio className="w-4 h-4" />, badge: counts?.warnings },
    { id: 'catalog', label: 'App Catalog', icon: <Sparkles className="w-4 h-4" /> },
    { id: 'yaml', label: 'YAML Studio', icon: <FileCode2 className="w-4 h-4" /> },
  ]

  const devNav: { id: NavSection; label: string; icon: React.ReactNode; badge?: number }[] = [
    { id: 'topology', label: 'App Topology', icon: <GitBranch className="w-4 h-4" /> },
    { id: 'catalog', label: 'Developer Catalog', icon: <Sparkles className="w-4 h-4" /> },
    { id: 'workloads', label: 'Deployments', icon: <Boxes className="w-4 h-4" />, badge: counts?.workloads },
    { id: 'pods', label: 'Pods & Logs', icon: <Terminal className="w-4 h-4" />, badge: counts?.pods },
    { id: 'networking', label: 'Routes & Ingress', icon: <Network className="w-4 h-4" /> },
    { id: 'storage', label: 'Config & Volumes', icon: <HardDrive className="w-4 h-4" /> },
    { id: 'yaml', label: 'Import YAML', icon: <FileCode2 className="w-4 h-4" /> },
  ]

  const items = perspective === 'admin' ? adminNav : devNav

  return (
    <aside
      className={`w-64 shrink-0 flex flex-col border-r transition-colors ${
        dark
          ? 'bg-[#080d18]/90 border-slate-800/80 text-slate-200'
          : 'bg-white/90 border-slate-200 text-slate-800'
      } backdrop-blur-xl`}
    >
      {/* Brand header */}
      <div className={`px-4 py-4 flex items-center justify-between border-b ${dark ? 'border-slate-800/70' : 'border-slate-200'}`}>
        <div className="flex items-center gap-3">
          <ZetnixLogo size={32} live={watchConnected} />
          <div>
            <div className="flex items-center gap-1.5">
              <span className="font-extrabold tracking-tight text-base bg-gradient-to-r from-cyan-400 via-blue-400 to-violet-400 bg-clip-text text-transparent">
                ZETNIX
              </span>
              <span className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-cyan-500/15 text-cyan-400 border border-cyan-500/25">
                v1.0.0
              </span>
            </div>
            <p className={`text-[11px] ${dark ? 'text-slate-400' : 'text-slate-500'}`}>Kubernetes Control Plane</p>
          </div>
        </div>
      </div>

      {/* OpenShift-inspired Perspective Switcher */}
      <div className="px-3 pt-3 pb-2">
        <div
          className={`grid grid-cols-2 p-1 rounded-xl border text-xs font-semibold ${
            dark ? 'bg-[#0d1424] border-slate-800/80' : 'bg-slate-100 border-slate-200'
          }`}
        >
          <button
            onClick={() => {
              onPerspectiveChange('admin')
              if (activeSection === 'catalog') onSelectSection('overview')
            }}
            className={`flex items-center justify-center gap-1.5 py-1.5 rounded-lg transition-all cursor-pointer ${
              perspective === 'admin'
                ? 'bg-gradient-to-r from-cyan-500/20 to-blue-500/20 text-cyan-300 border border-cyan-500/35 shadow-sm'
                : dark
                  ? 'text-slate-400 hover:text-slate-200'
                  : 'text-slate-500 hover:text-slate-900'
            }`}
          >
            <Cpu className="w-3.5 h-3.5" />
            Platform
          </button>
          <button
            onClick={() => {
              onPerspectiveChange('developer')
              if (activeSection === 'overview' || activeSection === 'nodes') onSelectSection('topology')
            }}
            className={`flex items-center justify-center gap-1.5 py-1.5 rounded-lg transition-all cursor-pointer ${
              perspective === 'developer'
                ? 'bg-gradient-to-r from-violet-500/20 to-cyan-500/20 text-violet-300 border border-violet-500/35 shadow-sm'
                : dark
                  ? 'text-slate-400 hover:text-slate-200'
                  : 'text-slate-500 hover:text-slate-900'
            }`}
          >
            <GitBranch className="w-3.5 h-3.5" />
            Developer
          </button>
        </div>
      </div>

      {/* Navigation items */}
      <nav className="flex-1 overflow-y-auto px-3 py-2 space-y-1">
        {items.map((item) => {
          const active = activeSection === item.id
          return (
            <button
              key={item.id}
              onClick={() => onSelectSection(item.id)}
              className={`w-full flex items-center justify-between px-3 py-2 rounded-xl text-[13px] font-medium transition-all cursor-pointer ${
                active
                  ? dark
                    ? 'bg-cyan-500/12 text-cyan-300 border border-cyan-500/30 shadow-[0_0_16px_rgba(6,182,212,0.1)]'
                    : 'bg-cyan-50 text-cyan-700 border border-cyan-200'
                  : dark
                    ? 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/45 border border-transparent'
                    : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100 border border-transparent'
              }`}
            >
              <span className="flex items-center gap-2.5">
                <span className={active ? 'text-cyan-400' : ''}>{item.icon}</span>
                {item.label}
              </span>
              {typeof item.badge === 'number' && item.badge > 0 && (
                <span
                  className={`text-[11px] font-mono px-1.5 py-0.2 rounded-md ${
                    item.id === 'events'
                      ? 'bg-amber-500/20 text-amber-300'
                      : dark
                        ? 'bg-slate-800 text-slate-300'
                        : 'bg-slate-200 text-slate-700'
                  }`}
                >
                  {item.badge}
                </span>
              )}
            </button>
          )
        })}
      </nav>

      {/* Bottom Live Watch Status Card */}
      <div className={`p-3 border-t ${dark ? 'border-slate-800/80' : 'border-slate-200'}`}>
        <div
          className={`rounded-xl p-2.5 border text-xs ${
            dark ? 'bg-[#0c1322] border-slate-800/80' : 'bg-slate-50 border-slate-200'
          }`}
        >
          <div className="flex items-center justify-between mb-1">
            <span className="font-semibold flex items-center gap-1.5">
              <span className={`w-2 h-2 rounded-full ${watchConnected ? 'bg-emerald-400 animate-pulse' : 'bg-amber-400'}`} />
              {watchConnected ? 'Watch Stream Active' : 'Reconnecting Watch'}
            </span>
            <span className="font-mono text-[10px] text-cyan-400">SSE</span>
          </div>
          <p className={`text-[11px] leading-relaxed ${dark ? 'text-slate-400' : 'text-slate-500'}`}>
            Real-time Kubernetes state sync without manual refresh.
          </p>
        </div>
      </div>
    </aside>
  )
}
