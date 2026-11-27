# Contributing to Zetnix

Thank you for your interest in contributing to **Zetnix**!

## Project Structure

- `cmd/zetnix/` — Main binary entrypoint and CLI wiring
- `internal/config/` — Configuration flags, environment variables, and release version
- `internal/driver/` — `ClusterDriver` contract and `Manager`
- `internal/driver/k8s/` — Live Kubernetes implementation (`k8s.io/client-go`, SSE logs/watch, WebSocket SPDY exec, SSH auto-tunnel)
- `internal/driver/demo/` — Self-contained simulated Kubernetes cluster for local development & testing
- `internal/topology/` — Application topology graph builder
- `internal/api/` — HTTP handlers, authentication middleware, SSE & WebSocket bridges
- `web/` — React + TypeScript + Vite + Tailwind CSS v4 frontend (embedded via `//go:embed`)
- `deploy/k8s/` — Production Kubernetes manifests

## Development Workflow

1. Fork the repository and create a topic branch:
   ```bash
   git checkout -b feat/my-feature
   ```
2. Install frontend dependencies once:
   ```bash
   npm --prefix web install
   ```
3. Run the backend and Vite dev server in two terminals:
   ```bash
   make dev-api
   make dev-ui
   ```
4. Before submitting a pull request, ensure all frontend checks, Go tests, and builds pass:
   ```bash
   npm --prefix web run build
   go test -count=1 -v ./...
   make build
   ```

## Pull Request Guidelines

- Keep PRs focused and well-scoped.
- Add or update unit/integration tests in `internal/` when modifying API routes, drivers, or topology logic.
- Avoid committing secrets, kubeconfigs, or compiled binaries under `bin/`.
