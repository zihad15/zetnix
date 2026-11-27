import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Lock } from 'lucide-react'
import { api } from './lib/api'
import { CATALOG_TEMPLATES } from './lib/templates'
import type {
  ClusterOverview as OverviewData,
  ClusterStatus,
  ConfigMapSummary,
  EventSummary,
  IngressSummary,
  InspectedResource,
  NamespaceSummary,
  NavSection,
  NodeSummary,
  Perspective,
  PodSummary,
  PVCSummary,
  SecretSummary,
  ServiceSummary,
  TopologyGraph,
  WatchEvent,
  WorkloadSummary,
} from './types/k8s'
import { Sidebar } from './components/layout/Sidebar'
import { SYSTEM_NAMESPACES, TopBar } from './components/layout/TopBar'
import { CommandPalette } from './components/layout/CommandPalette'
import { ClusterOverview } from './components/overview/ClusterOverview'
import { TopologyCanvas } from './components/topology/TopologyCanvas'
import { WorkloadsView } from './components/workloads/WorkloadsView'
import {
  EventsView,
  NetworkingView,
  NodesView,
  StorageAndConfigView,
} from './components/workloads/InfrastructureViews'
import { CatalogView } from './components/workloads/CatalogView'
import { YamlStudio } from './components/inspector/YamlStudio'
import { ResourceDrawer } from './components/inspector/ResourceDrawer'
import { ClusterModal } from './components/cluster/ClusterModal'
import { CreateResourceModal } from './components/cluster/CreateResourceModal'
import { ZetnixLogo } from './components/shared/StatusBadge'

const DEFAULT_CREATE_YAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-zetnix
  namespace: production
  labels:
    app: hello-zetnix
    app.kubernetes.io/part-of: commerce-core
spec:
  replicas: 2
  selector:
    matchLabels:
      app: hello-zetnix
  template:
    metadata:
      labels:
        app: hello-zetnix
    spec:
      containers:
        - name: web
          image: nginx:1.27-alpine
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: hello-zetnix
  namespace: production
  labels:
    app.kubernetes.io/part-of: commerce-core
spec:
  selector:
    app: hello-zetnix
  ports:
    - port: 80
      targetPort: 80
`

export default function App() {
  const [authChecking, setAuthChecking] = useState(true)
  const [authRequired, setAuthRequired] = useState(false)
  const [authenticated, setAuthenticated] = useState(false)
  const [loginUser, setLoginUser] = useState('admin')
  const [loginPass, setLoginPass] = useState('')
  const [loginErr, setLoginErr] = useState('')
  const [loginBusy, setLoginBusy] = useState(false)

  const [dark, setDark] = useState(true)
  const [perspective, setPerspective] = useState<Perspective>('admin')
  const [section, setSection] = useState<NavSection>('overview')
  const [activeNS, setActiveNS] = useState('all')
  const [hideSystem, setHideSystem] = useState(false)

  const [cluster, setCluster] = useState<ClusterStatus | null>(null)
  const [overview, setOverview] = useState<OverviewData | null>(null)
  const [topology, setTopology] = useState<TopologyGraph | null>(null)
  const [namespaces, setNamespaces] = useState<NamespaceSummary[]>([])
  const [nodes, setNodes] = useState<NodeSummary[]>([])
  const [pods, setPods] = useState<PodSummary[]>([])
  const [workloads, setWorkloads] = useState<WorkloadSummary[]>([])
  const [services, setServices] = useState<ServiceSummary[]>([])
  const [ingresses, setIngresses] = useState<IngressSummary[]>([])
  const [configMaps, setConfigMaps] = useState<ConfigMapSummary[]>([])
  const [secrets, setSecrets] = useState<SecretSummary[]>([])
  const [pvcs, setPvcs] = useState<PVCSummary[]>([])
  const [events, setEvents] = useState<EventSummary[]>([])

  const [history, setHistory] = useState<{ ts: string; cpuPct: number; memPct: number }[]>([])
  const [watchConnected, setWatchConnected] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [toast, setToast] = useState<{ id: number; text: string } | null>(null)

  const [inspected, setInspected] = useState<InspectedResource | null>(null)
  const [clusterModalOpen, setClusterModalOpen] = useState(false)
  const [createYamlOpen, setCreateYamlOpen] = useState(false)
  const [createYamlDraft, setCreateYamlDraft] = useState(DEFAULT_CREATE_YAML)
  const [paletteOpen, setPaletteOpen] = useState(false)

  const loadingRef = useRef(false)
  const watchDebounceRef = useRef<number | null>(null)

  const showToast = useCallback((text: string) => {
    const id = Date.now()
    setToast({ id, text })
    setTimeout(() => {
      setToast((cur) => (cur?.id === id ? null : cur))
    }, 3600)
  }, [])

  useEffect(() => {
    api
      .authStatus()
      .then((st) => {
        setAuthRequired(st.authRequired)
        setAuthenticated(st.authenticated)
        if (st.user) setLoginUser(st.user)
      })
      .catch(() => {
        setAuthenticated(true)
      })
      .finally(() => setAuthChecking(false))
  }, [])

  const loadAll = useCallback(
    async (ns: string) => {
      if (authChecking || !authenticated || loadingRef.current) return
      loadingRef.current = true
      setRefreshing(true)
      try {
        const snap = await api.snapshot(ns)
        setCluster(snap.cluster)
        setOverview(snap.overview)
        setTopology(snap.topology)
        setNamespaces(Array.isArray(snap.namespaces) ? snap.namespaces : [])
        setNodes(Array.isArray(snap.nodes) ? snap.nodes : [])
        setPods(Array.isArray(snap.pods) ? snap.pods : [])
        setWorkloads(Array.isArray(snap.workloads) ? snap.workloads : [])
        setServices(Array.isArray(snap.services) ? snap.services : [])
        setIngresses(Array.isArray(snap.ingresses) ? snap.ingresses : [])
        setConfigMaps(Array.isArray(snap.configMaps) ? snap.configMaps : [])
        setSecrets(Array.isArray(snap.secrets) ? snap.secrets : [])
        setPvcs(Array.isArray(snap.pvcs) ? snap.pvcs : [])
        setEvents(Array.isArray(snap.events) ? snap.events : [])
        if (snap.overview) {
          setHistory((prev) => {
            const next = [
              ...prev,
              {
                ts: snap.overview.updatedAt,
                cpuPct: snap.overview.usage.cpuPercent,
                memPct: snap.overview.usage.memoryPercent,
              },
            ]
            return next.slice(-24)
          })
        }
        if (snap.errors && snap.errors.length > 0) {
          showToast(`Sync partial: ${snap.errors[0]}`)
        }
      } catch (err) {
        const msg = (err as Error).message
        if (msg.toLowerCase().includes('authentication required')) {
          setAuthenticated(false)
        } else {
          showToast(`Sync warning: ${msg}`)
        }
      } finally {
        loadingRef.current = false
        setRefreshing(false)
      }
    },
    [authChecking, authenticated, showToast],
  )

  useEffect(() => {
    if (authChecking || !authenticated) return
    loadAll(activeNS)
    const timer = setInterval(() => loadAll(activeNS), 15000)
    return () => clearInterval(timer)
  }, [activeNS, authChecking, authenticated, loadAll])

  const clusterKey = `${cluster?.mode || ''}:${cluster?.clusterName || ''}:${cluster?.context || ''}`

  // Real-time Kubernetes SSE Watch stream (debounced + re-subscribes on cluster switch)
  useEffect(() => {
    if (authChecking || !authenticated) return
    const es = new EventSource(`/api/v1/watch?namespace=${encodeURIComponent(activeNS)}`)
    es.addEventListener('ready', () => setWatchConnected(true))
    es.onopen = () => setWatchConnected(true)
    es.addEventListener('watch', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as WatchEvent
        if (watchDebounceRef.current) window.clearTimeout(watchDebounceRef.current)
        watchDebounceRef.current = window.setTimeout(() => {
          loadAll(activeNS)
        }, 450)
        if (data.kind !== 'Event' || data.status === 'BackOff') {
          showToast(`${data.eventType}: ${data.kind} ${data.namespace}/${data.name} ${data.status ? `(${data.status})` : ''}`)
        }
      } catch {
        // ignore parse errors
      }
    })
    es.onerror = () => setWatchConnected(false)
    return () => {
      if (watchDebounceRef.current) window.clearTimeout(watchDebounceRef.current)
      es.close()
    }
  }, [activeNS, authChecking, authenticated, clusterKey, loadAll, showToast])

  // Cmd+K / Ctrl+K shortcut
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setPaletteOpen((v) => !v)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const filterUserScope = activeNS === 'all' && hideSystem

  const visiblePods = useMemo(
    () => (filterUserScope ? pods.filter((p) => !SYSTEM_NAMESPACES.has(p.namespace)) : pods),
    [pods, filterUserScope],
  )
  const visibleWorkloads = useMemo(
    () => (filterUserScope ? workloads.filter((w) => !SYSTEM_NAMESPACES.has(w.namespace)) : workloads),
    [workloads, filterUserScope],
  )
  const visibleServices = useMemo(
    () => (filterUserScope ? services.filter((s) => !SYSTEM_NAMESPACES.has(s.namespace)) : services),
    [services, filterUserScope],
  )
  const visibleIngresses = useMemo(
    () => (filterUserScope ? ingresses.filter((i) => !SYSTEM_NAMESPACES.has(i.namespace)) : ingresses),
    [ingresses, filterUserScope],
  )
  const visibleConfigMaps = useMemo(
    () => (filterUserScope ? configMaps.filter((c) => !SYSTEM_NAMESPACES.has(c.namespace)) : configMaps),
    [configMaps, filterUserScope],
  )
  const visibleSecrets = useMemo(
    () => (filterUserScope ? secrets.filter((s) => !SYSTEM_NAMESPACES.has(s.namespace)) : secrets),
    [secrets, filterUserScope],
  )
  const visiblePvcs = useMemo(
    () => (filterUserScope ? pvcs.filter((p) => !SYSTEM_NAMESPACES.has(p.namespace)) : pvcs),
    [pvcs, filterUserScope],
  )
  const visibleEvents = useMemo(
    () => (filterUserScope ? events.filter((e) => !SYSTEM_NAMESPACES.has(e.namespace)) : events),
    [events, filterUserScope],
  )
  const visibleTopology = useMemo(() => {
    if (!topology || !filterUserScope) return topology
    const nodes = topology.nodes.filter((n) => !SYSTEM_NAMESPACES.has(n.namespace))
    const ids = new Set(nodes.map((n) => n.id))
    return {
      ...topology,
      nodes,
      edges: topology.edges.filter((e) => ids.has(e.source) && ids.has(e.target)),
      groups: topology.groups.filter((g) => !SYSTEM_NAMESPACES.has(g.namespace)),
    }
  }, [topology, filterUserScope])

  const defaultTargetNS = useMemo(() => {
    if (activeNS !== 'all') return activeNS
    if (namespaces.some((n) => n.name === 'production')) return 'production'
    const firstUser = namespaces.find((n) => !SYSTEM_NAMESPACES.has(n.name))
    return firstUser?.name || 'default'
  }, [activeNS, namespaces])

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoginBusy(true)
    setLoginErr('')
    try {
      await api.login(loginUser.trim(), loginPass)
      setLoginPass('')
      setAuthenticated(true)
    } catch (ex) {
      setLoginErr((ex as Error).message)
    } finally {
      setLoginBusy(false)
    }
  }

  const handleLogout = async () => {
    await api.logout().catch(() => {})
    setAuthenticated(false)
  }

  if (authChecking) {
    return (
      <div className="h-screen flex items-center justify-center bg-[#060911] text-slate-300 font-mono text-xs">
        Initializing Zetnix Control Plane...
      </div>
    )
  }

  if (authRequired && !authenticated) {
    return (
      <div className="h-screen flex items-center justify-center p-4 bg-[#060911] zetnix-grid-bg text-slate-100">
        <form
          onSubmit={handleLogin}
          className="w-full max-w-sm rounded-2xl border border-slate-800 bg-[#0b1120]/95 backdrop-blur-xl p-6 shadow-2xl space-y-4"
        >
          <div className="flex items-center gap-3">
            <ZetnixLogo size={34} live={false} />
            <div>
              <div className="font-extrabold text-base tracking-tight bg-gradient-to-r from-cyan-400 via-blue-400 to-violet-400 bg-clip-text text-transparent">
                ZETNIX v1.0.0
              </div>
              <div className="text-xs text-slate-400">Protected Kubernetes Console</div>
            </div>
          </div>

          {loginErr && (
            <div className="p-2.5 rounded-xl border border-rose-500/40 bg-rose-500/12 text-rose-300 text-xs font-mono">
              {loginErr}
            </div>
          )}

          <div className="space-y-2.5 text-xs">
            <input
              value={loginUser}
              onChange={(e) => setLoginUser(e.target.value)}
              placeholder="Username"
              required
              className="w-full px-3 py-2.5 rounded-xl bg-[#060a14] border border-slate-800 text-slate-100 font-mono focus:outline-none focus:border-cyan-500"
            />
            <div className="relative">
              <Lock className="w-3.5 h-3.5 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
              <input
                type="password"
                value={loginPass}
                onChange={(e) => setLoginPass(e.target.value)}
                placeholder="Password / Access Token"
                required
                autoFocus
                className="w-full pl-9 pr-3 py-2.5 rounded-xl bg-[#060a14] border border-slate-800 text-slate-100 font-mono focus:outline-none focus:border-cyan-500"
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={loginBusy}
            className="w-full py-2.5 rounded-xl bg-gradient-to-r from-cyan-500 to-blue-500 hover:from-cyan-400 hover:to-blue-400 text-slate-950 font-bold text-xs cursor-pointer disabled:opacity-50"
          >
            {loginBusy ? 'Signing in...' : 'Sign In to Zetnix'}
          </button>
        </form>
      </div>
    )
  }

  const handleCreateNamespace = async (name: string) => {
    await api.createNamespace(name)
    showToast(`Created namespace "${name}"`)
    setActiveNS(name)
    await loadAll(name)
  }

  const handleTriggerChaos = async () => {
    try {
      const res = await api.triggerChaos(activeNS)
      showToast(`Chaos triggered: ${res.message}`)
      await loadAll(activeNS)
    } catch (e) {
      showToast((e as Error).message)
    }
  }

  const handlePerspectiveChange = (next: Perspective) => {
    setPerspective(next)
    if (next === 'developer') {
      setHideSystem(true)
      if (activeNS === 'all' || SYSTEM_NAMESPACES.has(activeNS)) {
        const firstUser = namespaces.find((n) => !SYSTEM_NAMESPACES.has(n.name))
        if (firstUser) setActiveNS(firstUser.name)
      }
    }
  }

  const handleNamespaceChange = (ns: string) => {
    setActiveNS(ns)
    if (ns !== 'all' && SYSTEM_NAMESPACES.has(ns)) {
      setHideSystem(false)
    }
  }

  const warningCount = visibleEvents.filter((e) => e.type === 'Warning').length

  return (
    <div className={`h-screen flex overflow-hidden ${dark ? 'bg-[#060911] text-slate-100' : 'bg-slate-50 text-slate-900'}`}>
      <Sidebar
        perspective={perspective}
        onPerspectiveChange={handlePerspectiveChange}
        activeSection={section}
        onSelectSection={setSection}
        watchConnected={watchConnected}
        dark={dark}
        counts={{
          pods: visiblePods.length,
          workloads: visibleWorkloads.length,
          nodes: nodes.length,
          warnings: warningCount,
        }}
      />

      <div className="flex-1 flex flex-col min-w-0">
        <TopBar
          cluster={cluster}
          namespaces={namespaces}
          activeNamespace={activeNS}
          onNamespaceChange={handleNamespaceChange}
          hideSystem={hideSystem}
          onToggleHideSystem={() => setHideSystem((v) => !v)}
          onCreateNamespace={handleCreateNamespace}
          onOpenClusterModal={() => setClusterModalOpen(true)}
          onOpenCreateYaml={() => {
            setCreateYamlDraft(DEFAULT_CREATE_YAML.replace(/namespace: production/g, `namespace: ${defaultTargetNS}`))
            setCreateYamlOpen(true)
          }}
          onOpenCommandPalette={() => setPaletteOpen(true)}
          onRefresh={() => loadAll(activeNS)}
          onTriggerChaos={handleTriggerChaos}
          dark={dark}
          onToggleTheme={() => setDark((d) => !d)}
          refreshing={refreshing}
          onLogout={authRequired ? handleLogout : undefined}
        />

        <main className={`flex-1 overflow-y-auto p-5 ${dark ? 'zetnix-grid-bg' : 'zetnix-light-grid-bg'}`}>
          {section === 'overview' && (
            <ClusterOverview
              overview={overview}
              pods={visiblePods}
              history={history}
              dark={dark}
              onNavigate={setSection}
              onInspect={setInspected}
            />
          )}

          {section === 'topology' && (
            <TopologyCanvas graph={visibleTopology} dark={dark} onInspect={setInspected} />
          )}

          {(section === 'workloads' || section === 'pods') && (
            <WorkloadsView
              mode={section}
              workloads={visibleWorkloads}
              pods={visiblePods}
              dark={dark}
              onInspect={setInspected}
              onChanged={(msg) => {
                showToast(msg)
                loadAll(activeNS)
              }}
            />
          )}

          {section === 'networking' && (
            <NetworkingView services={visibleServices} ingresses={visibleIngresses} dark={dark} onInspect={setInspected} />
          )}

          {section === 'storage' && (
            <StorageAndConfigView
              pvcs={visiblePvcs}
              configMaps={visibleConfigMaps}
              secrets={visibleSecrets}
              activeNamespace={defaultTargetNS}
              dark={dark}
              onInspect={setInspected}
              onCreateManifest={(yamlTemplate) => {
                setCreateYamlDraft(yamlTemplate)
                setCreateYamlOpen(true)
              }}
            />
          )}

          {section === 'nodes' && <NodesView nodes={nodes} dark={dark} onInspect={setInspected} />}

          {section === 'events' && <EventsView events={visibleEvents} dark={dark} onInspect={setInspected} />}

          {section === 'catalog' && (
            <CatalogView
              activeNamespace={defaultTargetNS}
              dark={dark}
              onLaunchTemplate={(tpl) => {
                setCreateYamlDraft(tpl.buildYaml(defaultTargetNS))
                setCreateYamlOpen(true)
              }}
            />
          )}

          {section === 'yaml' && (
            <div className="h-[calc(100vh-130px)] rounded-2xl border border-slate-800 bg-[#090f1d]/95 p-5">
              <YamlStudio
                initialYaml={CATALOG_TEMPLATES[0].buildYaml(defaultTargetNS)}
                onApplied={(summary) => {
                  showToast(`Applied: ${summary}`)
                  loadAll(activeNS)
                }}
              />
            </div>
          )}
        </main>
      </div>

      <ResourceDrawer
        resource={inspected}
        pods={pods}
        workloads={workloads}
        nodes={nodes}
        services={services}
        ingresses={ingresses}
        configMaps={configMaps}
        secrets={secrets}
        pvcs={pvcs}
        onClose={() => setInspected(null)}
        onChanged={(msg) => {
          showToast(msg)
          loadAll(activeNS)
        }}
        onInspect={setInspected}
      />

      <ClusterModal
        open={clusterModalOpen}
        cluster={cluster}
        onClose={() => setClusterModalOpen(false)}
        onUpdated={(next) => {
          setCluster(next)
          setHistory([])
          setActiveNS('all')
          loadAll('all')
          showToast(`Switched active cluster to ${next.clusterName}`)
        }}
      />

      <CreateResourceModal
        open={createYamlOpen}
        initialYaml={createYamlDraft}
        onClose={() => setCreateYamlOpen(false)}
        onApplied={(summary) => {
          showToast(`Deployed: ${summary}`)
          loadAll(activeNS)
        }}
      />

      <CommandPalette
        open={paletteOpen}
        onClose={() => setPaletteOpen(false)}
        pods={visiblePods}
        workloads={visibleWorkloads}
        services={visibleServices}
        onNavigate={setSection}
        onInspect={setInspected}
      />

      {toast && (
        <div className="fixed bottom-5 right-5 z-50 px-4 py-2.5 rounded-xl border border-cyan-500/40 bg-[#0b1326]/95 text-cyan-200 text-xs font-mono shadow-2xl backdrop-blur-xl">
          ⚡ {toast.text}
        </div>
      )}
    </div>
  )
}
