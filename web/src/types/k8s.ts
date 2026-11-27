export type ConnectionMode = 'in-cluster' | 'kubeconfig' | 'uploaded' | 'ssh-tunnel' | 'token' | 'demo'

export interface ClusterContext {
  id: string
  name: string
  cluster: string
  user: string
  mode: ConnectionMode
  active: boolean
}

export interface ClusterStatus {
  mode: ConnectionMode
  connected: boolean
  clusterName: string
  context: string
  serverVersion: string
  platform: string
  apiServerUrl?: string
  metricsAvailable: boolean
  error?: string
  contexts: ClusterContext[]
}

export interface ResourceUsage {
  usedMilliCpu: number
  totalMilliCpu: number
  cpuPercent: number
  usedMemoryMiB: number
  totalMemoryMiB: number
  memoryPercent: number
  podsRunning: number
  podsCapacity: number
}

export interface PodPhaseCounts {
  total: number
  running: number
  pending: number
  succeeded: number
  failed: number
  crashLoop: number
  terminating: number
}

export interface EventSummary {
  id: string
  namespace: string
  type: string
  reason: string
  message: string
  involvedKind: string
  involvedName: string
  count: number
  firstTimestamp: string
  lastTimestamp: string
}

export interface ClusterOverview {
  namespace: string
  healthScore: number
  nodesTotal: number
  nodesReady: number
  namespacesCount: number
  deploymentsTotal: number
  deploymentsReady: number
  statefulSets: number
  daemonSets: number
  jobs: number
  services: number
  ingresses: number
  pvcs: number
  pods: PodPhaseCounts
  usage: ResourceUsage
  metricsAvailable: boolean
  recentEvents: EventSummary[]
  updatedAt: string
}

export interface NamespaceSummary {
  name: string
  status: string
  labels?: Record<string, string>
  podCount: number
  createdAt: string
}

export interface NodeCondition {
  type: string
  status: string
  reason?: string
  message?: string
}

export interface NodeSummary {
  name: string
  roles: string[]
  status: string
  kubeletVersion: string
  osImage: string
  containerRuntime: string
  architecture: string
  internalIp: string
  cpuCapacityCores: number
  cpuUsedMilli: number
  memCapacityMiB: number
  memUsedMiB: number
  podsCapacity: number
  podsAllocated: number
  conditions: NodeCondition[]
  labels?: Record<string, string>
  createdAt: string
}

export interface ContainerInfo {
  name: string
  image: string
  ready: boolean
  restartCount: number
  state: string
  reason?: string
  cpuUsageMilli: number
  memoryUsageMiB: number
  ports?: number[]
}

export interface PodSummary {
  name: string
  namespace: string
  phase: string
  statusReason: string
  ready: string
  restarts: number
  nodeName: string
  podIp: string
  cpuUsageMilli: number
  memoryUsageMiB: number
  ownerKind?: string
  ownerName?: string
  containers: ContainerInfo[]
  labels?: Record<string, string>
  createdAt: string
}

export interface WorkloadSummary {
  kind: string
  name: string
  namespace: string
  desiredReplicas: number
  readyReplicas: number
  updatedReplicas: number
  availableReplicas: number
  status: string
  images: string[]
  strategy?: string
  schedule?: string
  labels?: Record<string, string>
  selector?: Record<string, string>
  createdAt: string
}

export interface ServicePort {
  name?: string
  protocol: string
  port: number
  targetPort: string
  nodePort?: number
}

export interface ServiceSummary {
  name: string
  namespace: string
  type: string
  clusterIp: string
  externalIp?: string
  ports: ServicePort[]
  selector?: Record<string, string>
  labels?: Record<string, string>
  endpoints: number
  createdAt: string
}

export interface IngressRuleSummary {
  host: string
  path: string
  serviceName: string
  servicePort: string
}

export interface IngressSummary {
  name: string
  namespace: string
  className?: string
  tls: boolean
  hosts: string[]
  rules: IngressRuleSummary[]
  address?: string
  labels?: Record<string, string>
  createdAt: string
}

export interface ConfigMapSummary {
  name: string
  namespace: string
  keys: string[]
  data?: Record<string, string>
  labels?: Record<string, string>
  createdAt: string
}

export interface SecretSummary {
  name: string
  namespace: string
  type: string
  keys: string[]
  dataSize: number
  labels?: Record<string, string>
  createdAt: string
}

export interface PVCSummary {
  name: string
  namespace: string
  status: string
  volumeName?: string
  capacity: string
  accessModes: string[]
  storageClass?: string
  labels?: Record<string, string>
  createdAt: string
}

export interface WatchEvent {
  eventType: string
  kind: string
  namespace: string
  name: string
  status?: string
  message?: string
  timestamp: string
}

export interface TopologyNode {
  id: string
  kind: string
  name: string
  namespace: string
  appGroup?: string
  status: 'healthy' | 'degraded' | 'failing' | 'pending' | string
  desiredReplicas?: number
  readyReplicas?: number
  pods?: PodSummary[]
  meta?: Record<string, string>
  x: number
  y: number
}

export interface TopologyEdge {
  id: string
  source: string
  target: string
  type: string
  label?: string
}

export interface TopologyGroup {
  id: string
  name: string
  namespace: string
  nodeIds: string[]
}

export interface TopologyGraph {
  namespace: string
  groups: TopologyGroup[]
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

export interface ApplyResult {
  applied: {
    kind: string
    namespace?: string
    name: string
    action: string
  }[]
}

export interface AuthStatus {
  authRequired: boolean
  authenticated: boolean
  user: string
}

export interface ClusterSnapshot {
  cluster: ClusterStatus
  overview: ClusterOverview
  topology: TopologyGraph
  namespaces: NamespaceSummary[]
  nodes: NodeSummary[]
  pods: PodSummary[]
  workloads: WorkloadSummary[]
  services: ServiceSummary[]
  ingresses: IngressSummary[]
  configMaps: ConfigMapSummary[]
  secrets: SecretSummary[]
  pvcs: PVCSummary[]
  events: EventSummary[]
  errors?: string[]
}

export type Perspective = 'admin' | 'developer'

export type NavSection =
  | 'overview'
  | 'topology'
  | 'workloads'
  | 'pods'
  | 'networking'
  | 'storage'
  | 'nodes'
  | 'events'
  | 'catalog'
  | 'yaml'

export interface InspectedResource {
  kind: string
  name: string
  namespace: string
  initialTab?: 'details' | 'logs' | 'terminal' | 'yaml'
  raw?: unknown
}
