import type {
  ApplyResult,
  AuthStatus,
  ClusterOverview,
  ClusterSnapshot,
  ClusterStatus,
  ConfigMapSummary,
  EventSummary,
  IngressSummary,
  NamespaceSummary,
  NodeSummary,
  PodSummary,
  PVCSummary,
  SecretSummary,
  ServiceSummary,
  TopologyGraph,
  WorkloadSummary,
} from '../types/k8s'

async function req<T>(url: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(url, {
      headers: { 'Content-Type': 'application/json' },
      ...init,
    })
  } catch {
    throw new Error('Cannot reach Zetnix backend server. Make sure ./bin/zetnix is running and refresh this page.')
  }
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error((data as { error?: string }).error || `Request failed (${res.status})`)
  }
  return data as T
}

const nsQuery = (ns: string) => encodeURIComponent(ns || 'all')

export const api = {
  authStatus: () => req<AuthStatus>('/api/v1/auth/status'),
  login: (username: string, password: string) =>
    req<{ authenticated: boolean; user: string }>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () =>
    req<{ authenticated: boolean }>('/api/v1/auth/logout', {
      method: 'POST',
    }),
  snapshot: (ns: string) => req<ClusterSnapshot>(`/api/v1/snapshot?namespace=${nsQuery(ns)}`),
  clusterStatus: () => req<ClusterStatus>('/api/v1/cluster'),
  switchCluster: (id: string) =>
    req<ClusterStatus>('/api/v1/cluster/switch', {
      method: 'POST',
      body: JSON.stringify({ id }),
    }),
  disconnectCluster: (id: string) =>
    req<ClusterStatus>('/api/v1/cluster/disconnect', {
      method: 'POST',
      body: JSON.stringify({ id }),
    }),
  connectCluster: (payload: { name: string; context?: string; kubeconfig: string; insecureSkipTlsVerify?: boolean }) =>
    req<ClusterStatus>('/api/v1/cluster/connect', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  connectSSHCluster: (payload: {
    name?: string
    host: string
    port: string
    username: string
    password?: string
    privateKey?: string
    passphrase?: string
    remoteKubeconfigPath?: string
    useSudo?: boolean
    insecureSkipTlsVerify?: boolean
    trustHostKey?: boolean
  }) =>
    req<ClusterStatus>('/api/v1/cluster/connect-ssh', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  connectTokenCluster: (payload: {
    name?: string
    apiServerUrl: string
    bearerToken: string
    caData?: string
    insecureSkipTlsVerify?: boolean
  }) =>
    req<ClusterStatus>('/api/v1/cluster/connect-token', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),

  overview: (ns: string) => req<ClusterOverview>(`/api/v1/overview?namespace=${nsQuery(ns)}`),
  topology: (ns: string) => req<TopologyGraph>(`/api/v1/topology?namespace=${nsQuery(ns)}`),

  namespaces: () => req<NamespaceSummary[]>('/api/v1/namespaces'),
  createNamespace: (name: string) =>
    req<NamespaceSummary>('/api/v1/namespaces', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  nodes: () => req<NodeSummary[]>('/api/v1/nodes'),
  pods: (ns: string) => req<PodSummary[]>(`/api/v1/pods?namespace=${nsQuery(ns)}`),
  workloads: (ns: string, kind = 'all') =>
    req<WorkloadSummary[]>(`/api/v1/workloads?kind=${encodeURIComponent(kind)}&namespace=${nsQuery(ns)}`),
  services: (ns: string) => req<ServiceSummary[]>(`/api/v1/services?namespace=${nsQuery(ns)}`),
  ingresses: (ns: string) => req<IngressSummary[]>(`/api/v1/ingresses?namespace=${nsQuery(ns)}`),
  configMaps: (ns: string) => req<ConfigMapSummary[]>(`/api/v1/configmaps?namespace=${nsQuery(ns)}`),
  secrets: (ns: string) => req<SecretSummary[]>(`/api/v1/secrets?namespace=${nsQuery(ns)}`),
  pvcs: (ns: string) => req<PVCSummary[]>(`/api/v1/pvcs?namespace=${nsQuery(ns)}`),
  events: (ns: string) => req<EventSummary[]>(`/api/v1/events?namespace=${nsQuery(ns)}`),

  getYaml: (kind: string, namespace: string, name: string) =>
    req<{ yaml: string }>(
      `/api/v1/yaml?kind=${encodeURIComponent(kind)}&namespace=${encodeURIComponent(namespace)}&name=${encodeURIComponent(name)}`,
    ),
  applyYaml: (yaml: string) =>
    req<ApplyResult>('/api/v1/yaml/apply', {
      method: 'POST',
      body: JSON.stringify({ yaml }),
    }),
  deleteResource: (kind: string, namespace: string, name: string) =>
    req<{ deleted: boolean }>(
      `/api/v1/resources?kind=${encodeURIComponent(kind)}&namespace=${encodeURIComponent(namespace)}&name=${encodeURIComponent(name)}`,
      { method: 'DELETE' },
    ),
  scaleWorkload: (kind: string, namespace: string, name: string, replicas: number) =>
    req<{ scaled: boolean; replicas: number }>('/api/v1/workloads/scale', {
      method: 'POST',
      body: JSON.stringify({ kind, namespace, name, replicas }),
    }),
  restartWorkload: (kind: string, namespace: string, name: string) =>
    req<{ restarted: boolean }>('/api/v1/workloads/restart', {
      method: 'POST',
      body: JSON.stringify({ kind, namespace, name }),
    }),
  triggerChaos: (ns: string) =>
    req<{ message: string }>(`/api/v1/demo/chaos?namespace=${nsQuery(ns)}`, {
      method: 'POST',
    }),
}

export function formatAge(iso: string): string {
  if (!iso) return '—'
  const diff = Math.max(0, Date.now() - new Date(iso).getTime())
  const s = Math.floor(diff / 1000)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m`
  const h = Math.floor(m / 60)
  if (h < 48) return `${h}h`
  const d = Math.floor(h / 24)
  return `${d}d`
}

export function formatCores(milli: number): string {
  if (!milli) return '0m'
  if (milli >= 1000) return `${(milli / 1000).toFixed(2)}c`
  return `${milli}m`
}

export function formatMemoryMiB(mib: number): string {
  if (!mib) return '0 MiB'
  if (mib >= 1024) return `${(mib / 1024).toFixed(1)} GiB`
  return `${mib} MiB`
}
