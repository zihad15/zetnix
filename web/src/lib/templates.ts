export interface CatalogTemplate {
  id: string
  title: string
  category: 'Database' | 'Gateway' | 'Runtime' | 'Cache' | 'Automation'
  badge: string
  version: string
  description: string
  accent: string
  buildYaml: (namespace: string) => string
}

export const CATALOG_TEMPLATES: CatalogTemplate[] = [
  {
    id: 'nginx-edge',
    title: 'NGINX Edge Gateway',
    category: 'Gateway',
    badge: 'Deployment + Service',
    version: '1.27-alpine',
    description: 'High-throughput reverse proxy & static edge gateway pre-wired with health probes and ClusterIP service.',
    accent: 'from-cyan-500/20 to-emerald-500/10 border-cyan-500/30',
    buildYaml: (ns) => `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-gateway
  namespace: ${ns}
  labels:
    app: web-gateway
    app.kubernetes.io/part-of: edge-platform
spec:
  replicas: 2
  selector:
    matchLabels:
      app: web-gateway
  template:
    metadata:
      labels:
        app: web-gateway
    spec:
      containers:
        - name: nginx
          image: nginx:1.27-alpine
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: web-gateway
  namespace: ${ns}
  labels:
    app.kubernetes.io/part-of: edge-platform
spec:
  type: ClusterIP
  selector:
    app: web-gateway
  ports:
    - name: http
      port: 80
      targetPort: 80
`,
  },
  {
    id: 'postgres-16',
    title: 'PostgreSQL 16 HA Node',
    category: 'Database',
    badge: 'StatefulSet + Service',
    version: '16.4-alpine',
    description: 'Production-ready relational database StatefulSet with persistent storage identity and headless service.',
    accent: 'from-blue-500/20 to-indigo-500/10 border-blue-500/30',
    buildYaml: (ns) => `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: analytics-db
  namespace: ${ns}
  labels:
    app: analytics-db
    app.kubernetes.io/part-of: data-tier
spec:
  replicas: 1
  serviceName: analytics-db
  selector:
    matchLabels:
      app: analytics-db
  template:
    metadata:
      labels:
        app: analytics-db
    spec:
      containers:
        - name: postgres
          image: postgres:16.4-alpine
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_DB
              value: zetnix
            - name: POSTGRES_USER
              value: zetnix
            - name: POSTGRES_PASSWORD
              value: change-me-in-secret
---
apiVersion: v1
kind: Service
metadata:
  name: analytics-db
  namespace: ${ns}
  labels:
    app.kubernetes.io/part-of: data-tier
spec:
  selector:
    app: analytics-db
  ports:
    - name: pg
      port: 5432
      targetPort: 5432
`,
  },
  {
    id: 'redis-cache',
    title: 'Redis 7 In-Memory Cache',
    category: 'Cache',
    badge: 'StatefulSet + Service',
    version: '7.4-alpine',
    description: 'Ultra-low-latency key-value cache and Pub/Sub broker ready for microservices.',
    accent: 'from-rose-500/20 to-orange-500/10 border-rose-500/30',
    buildYaml: (ns) => `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: session-redis
  namespace: ${ns}
  labels:
    app: session-redis
    app.kubernetes.io/part-of: cache-mesh
spec:
  replicas: 1
  serviceName: session-redis
  selector:
    matchLabels:
      app: session-redis
  template:
    metadata:
      labels:
        app: session-redis
    spec:
      containers:
        - name: redis
          image: redis:7.4-alpine
          ports:
            - containerPort: 6379
---
apiVersion: v1
kind: Service
metadata:
  name: session-redis
  namespace: ${ns}
  labels:
    app.kubernetes.io/part-of: cache-mesh
spec:
  selector:
    app: session-redis
  ports:
    - name: redis
      port: 6379
      targetPort: 6379
`,
  },
  {
    id: 'go-microservice',
    title: 'Go HTTP/gRPC Microservice',
    category: 'Runtime',
    badge: 'Deployment + ConfigMap',
    version: 'v1.0',
    description: 'Zero-bloat Go API service template wired with ConfigMap environment injection and rolling update strategy.',
    accent: 'from-teal-500/20 to-cyan-500/10 border-teal-500/30',
    buildYaml: (ns) => `apiVersion: v1
kind: ConfigMap
metadata:
  name: billing-worker-cfg
  namespace: ${ns}
data:
  LOG_LEVEL: info
  HTTP_PORT: "8080"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: billing-worker
  namespace: ${ns}
  labels:
    app: billing-worker
    app.kubernetes.io/part-of: commerce-core
spec:
  replicas: 2
  selector:
    matchLabels:
      app: billing-worker
  template:
    metadata:
      labels:
        app: billing-worker
    spec:
      containers:
        - name: billing-worker
          image: ghcr.io/zetnix/sample-api:latest
          ports:
            - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: billing-worker
  namespace: ${ns}
  labels:
    app.kubernetes.io/part-of: commerce-core
spec:
  selector:
    app: billing-worker
  ports:
    - name: http
      port: 8080
      targetPort: 8080
`,
  },
]
