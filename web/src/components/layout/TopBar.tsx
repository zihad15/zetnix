import React, { useState } from 'react'
import {
  ChevronDown,
  EyeOff,
  Flame,
  FolderPlus,
  LogOut,
  Moon,
  Plus,
  RefreshCw,
  Search,
  ServerCog,
  Sun,
  Zap,
} from 'lucide-react'
import type { ClusterStatus, NamespaceSummary } from '../../types/k8s'

export const SYSTEM_NAMESPACES = new Set(['kube-system', 'kube-public', 'kube-node-lease', 'zetnix-system'])

interface TopBarProps {
  cluster: ClusterStatus | null
  namespaces: NamespaceSummary[]
  activeNamespace: string
  onNamespaceChange: (ns: string) => void
  hideSystem: boolean
  onToggleHideSystem: () => void
  onCreateNamespace: (name: string) => Promise<void>
  onOpenClusterModal: () => void
  onOpenCreateYaml: () => void
  onOpenCommandPalette: () => void
  onRefresh: () => void
  onTriggerChaos: () => void
  dark: boolean
  onToggleTheme: () => void
  refreshing: boolean
  onLogout?: () => void
}

export function TopBar({
  cluster,
  namespaces,
  activeNamespace,
  onNamespaceChange,
  hideSystem,
  onToggleHideSystem,
  onCreateNamespace,
  onOpenClusterModal,
  onOpenCreateYaml,
  onOpenCommandPalette,
  onRefresh,
  onTriggerChaos,
  dark,
  onToggleTheme,
  refreshing,
  onLogout,
}: TopBarProps) {
  const [creatingNS, setCreatingNS] = useState(false)
  const [newNS, setNewNS] = useState('')
  const [busyNS, setBusyNS] = useState(false)

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newNS.trim()) return
    setBusyNS(true)
    try {
      await onCreateNamespace(newNS.trim().toLowerCase())
      setNewNS('')
      setCreatingNS(false)
    } finally {
      setBusyNS(false)
    }
  }

  const modeBadge =
    cluster?.mode === 'demo'
      ? 'bg-violet-500/15 text-violet-300 border-violet-500/30'
      : 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30'

  return (
    <header
      className={`h-14 shrink-0 px-4 flex items-center justify-between gap-3 border-b backdrop-blur-xl z-20 ${
        dark ? 'bg-[#080d18]/80 border-slate-800/80' : 'bg-white/85 border-slate-200'
      }`}
    >
      {/* Left: Cluster Switcher + Project/Namespace Selector */}
      <div className="flex items-center gap-2.5 min-w-0">
        <button
          onClick={onOpenClusterModal}
          className={`flex items-center gap-2 px-3 py-1.5 rounded-xl border text-xs font-semibold transition-all cursor-pointer ${
            dark
              ? 'bg-[#0d1424] hover:bg-slate-800/80 border-slate-800 text-slate-100'
              : 'bg-slate-50 hover:bg-slate-100 border-slate-200 text-slate-800'
          }`}
        >
          <ServerCog className="w-3.5 h-3.5 text-cyan-400" />
          <span className="truncate max-w-[140px]">{cluster?.clusterName || 'Connecting...'}</span>
          <span className={`px-1.5 py-0.2 rounded text-[10px] font-mono border ${modeBadge}`}>
            {cluster?.mode === 'demo' ? 'SANDBOX' : cluster?.mode?.toUpperCase() || '...'}
          </span>
          <ChevronDown className="w-3.5 h-3.5 text-slate-400" />
        </button>

        {/* Project / Namespace selector */}
        <div className="flex items-center gap-1.5">
          <div className="relative">
            <select
              value={activeNamespace}
              onChange={(e) => onNamespaceChange(e.target.value)}
              aria-label="Select Project or Namespace"
              className={`appearance-none pl-3 pr-8 py-1.5 rounded-xl border text-xs font-medium cursor-pointer focus:outline-none focus:ring-2 focus:ring-cyan-500/40 ${
                dark
                  ? 'bg-[#0d1424] border-slate-800 text-slate-200'
                  : 'bg-slate-50 border-slate-200 text-slate-800'
              }`}
            >
              <option value="all">{hideSystem ? 'Project: Your Projects' : 'Project: All Namespaces'}</option>
              {namespaces.some((n) => !SYSTEM_NAMESPACES.has(n.name)) && (
                <optgroup label="Your Projects">
                  {namespaces
                    .filter((n) => !SYSTEM_NAMESPACES.has(n.name))
                    .map((ns) => (
                      <option key={ns.name} value={ns.name}>
                        ns / {ns.name} ({ns.podCount} pods)
                      </option>
                    ))}
                </optgroup>
              )}
              {namespaces.some((n) => SYSTEM_NAMESPACES.has(n.name)) && (
                <optgroup label="System Namespaces">
                  {namespaces
                    .filter((n) => SYSTEM_NAMESPACES.has(n.name))
                    .map((ns) => (
                      <option key={ns.name} value={ns.name}>
                        sys / {ns.name} ({ns.podCount} pods)
                      </option>
                    ))}
                </optgroup>
              )}
            </select>
            <ChevronDown className="w-3.5 h-3.5 text-slate-400 absolute right-2.5 top-1/2 -translate-y-1/2 pointer-events-none" />
          </div>

          {activeNamespace === 'all' && (
            <button
              onClick={onToggleHideSystem}
              title="Hide built-in kube-* system namespaces (like OpenShift Developer scope)"
              className={`hidden sm:inline-flex items-center gap-1 px-2.5 py-1.5 rounded-xl border text-[11px] font-semibold cursor-pointer transition-colors ${
                hideSystem
                  ? 'bg-cyan-500/15 border-cyan-500/35 text-cyan-400'
                  : dark
                    ? 'bg-[#0d1424] border-slate-800 text-slate-400 hover:text-slate-200'
                    : 'bg-slate-50 border-slate-200 text-slate-600'
              }`}
            >
              <EyeOff className="w-3.5 h-3.5" />
              Hide kube-*
            </button>
          )}

          {creatingNS ? (
            <form onSubmit={handleCreate} className="flex items-center gap-1">
              <input
                value={newNS}
                onChange={(e) => setNewNS(e.target.value)}
                placeholder="new-project"
                autoFocus
                className={`w-32 px-2.5 py-1 rounded-lg border text-xs font-mono focus:outline-none focus:border-cyan-500 ${
                  dark ? 'bg-[#0d1424] border-slate-700 text-slate-100' : 'bg-white border-slate-300 text-slate-900'
                }`}
              />
              <button
                type="submit"
                disabled={busyNS}
                className="px-2 py-1 rounded-lg bg-cyan-500 text-slate-950 text-xs font-bold cursor-pointer"
              >
                Add
              </button>
              <button
                type="button"
                onClick={() => setCreatingNS(false)}
                className="px-1.5 py-1 text-xs text-slate-400 hover:text-slate-200 cursor-pointer"
              >
                ✕
              </button>
            </form>
          ) : (
            <button
              onClick={() => setCreatingNS(true)}
              title="Create Project / Namespace"
              className={`p-1.5 rounded-xl border transition-colors cursor-pointer ${
                dark
                  ? 'bg-[#0d1424] hover:bg-slate-800 border-slate-800 text-slate-400 hover:text-cyan-300'
                  : 'bg-slate-50 hover:bg-slate-100 border-slate-200 text-slate-600'
              }`}
            >
              <FolderPlus className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      </div>

      {/* Center: Quick Spotlight Search (Cmd+K) */}
      <button
        onClick={onOpenCommandPalette}
        className={`hidden md:flex items-center justify-between gap-4 w-72 px-3 py-1.5 rounded-xl border text-xs transition-all cursor-pointer ${
          dark
            ? 'bg-[#0d1424]/90 hover:border-cyan-500/40 border-slate-800 text-slate-400'
            : 'bg-slate-50 hover:border-cyan-400 border-slate-200 text-slate-500'
        }`}
      >
        <span className="flex items-center gap-2">
          <Search className="w-3.5 h-3.5 text-cyan-400" />
          Jump to pods, workloads, actions...
        </span>
        <kbd className={`px-1.5 py-0.5 rounded font-mono text-[10px] ${dark ? 'bg-slate-800 text-slate-300' : 'bg-slate-200 text-slate-700'}`}>
          ⌘K
        </kbd>
      </button>

      {/* Right: Actions */}
      <div className="flex items-center gap-2">
        {cluster?.mode === 'demo' && (
          <button
            onClick={onTriggerChaos}
            title="Simulate a live Pod failure to see real-time Watch updates"
            className="hidden sm:inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-xl border border-amber-500/35 bg-amber-500/10 hover:bg-amber-500/20 text-amber-300 text-xs font-semibold transition-all cursor-pointer"
          >
            <Flame className="w-3.5 h-3.5" />
            Chaos Sim
          </button>
        )}

        <button
          onClick={onOpenCreateYaml}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-gradient-to-r from-cyan-500 to-blue-500 hover:from-cyan-400 hover:to-blue-400 text-slate-950 text-xs font-bold shadow-[0_0_16px_rgba(6,182,212,0.28)] transition-all cursor-pointer"
        >
          <Plus className="w-3.5 h-3.5 stroke-[2.5]" />
          <span className="hidden sm:inline">Deploy YAML</span>
        </button>

        <button
          onClick={onRefresh}
          title="Refresh cluster state"
          className={`p-1.5 rounded-xl border transition-colors cursor-pointer ${
            dark
              ? 'bg-[#0d1424] hover:bg-slate-800 border-slate-800 text-slate-300'
              : 'bg-slate-50 hover:bg-slate-100 border-slate-200 text-slate-700'
          }`}
        >
          <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin text-cyan-400' : ''}`} />
        </button>

        <button
          onClick={onToggleTheme}
          title="Toggle Obsidian Dark / Daylight theme"
          className={`p-1.5 rounded-xl border transition-colors cursor-pointer ${
            dark
              ? 'bg-[#0d1424] hover:bg-slate-800 border-slate-800 text-slate-300'
              : 'bg-slate-50 hover:bg-slate-100 border-slate-200 text-slate-700'
          }`}
        >
          {dark ? <Sun className="w-4 h-4 text-amber-300" /> : <Moon className="w-4 h-4 text-slate-700" />}
        </button>

        <div className="hidden xl:flex items-center gap-1.5 pl-1 text-[11px] font-mono text-slate-400">
          <Zap className="w-3.5 h-3.5 text-cyan-400" />
          <span>{cluster?.serverVersion || 'k8s'}</span>
        </div>

        {onLogout && (
          <button
            onClick={onLogout}
            title="Sign out"
            className={`p-1.5 rounded-xl border transition-colors cursor-pointer ${
              dark
                ? 'bg-[#0d1424] hover:bg-rose-500/15 border-slate-800 text-slate-400 hover:text-rose-300'
                : 'bg-slate-50 hover:bg-rose-50 border-slate-200 text-slate-600 hover:text-rose-600'
            }`}
          >
            <LogOut className="w-4 h-4" />
          </button>
        )}
      </div>
    </header>
  )
}
