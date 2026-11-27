import { Component, StrictMode, type ErrorInfo, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import '@xterm/xterm/css/xterm.css'
import './index.css'
import App from './App'

class ErrorBoundary extends Component<{ children: ReactNode }, { err: Error | null }> {
  state: { err: Error | null } = { err: null }

  static getDerivedStateFromError(err: Error) {
    return { err }
  }

  componentDidCatch(err: Error, info: ErrorInfo) {
    console.error('Zetnix UI Error:', err, info)
  }

  render() {
    if (this.state.err) {
      return (
        <div className="h-screen flex items-center justify-center p-6 bg-[#060911] text-slate-100">
          <div className="max-w-lg w-full rounded-2xl border border-rose-500/40 bg-[#0b1120] p-6 space-y-3">
            <div className="text-sm font-bold text-rose-400">Zetnix UI encountered a rendering error</div>
            <pre className="text-xs font-mono text-rose-200 bg-[#060911] p-3 rounded-xl overflow-auto">
              {this.state.err.message}
            </pre>
            <button
              onClick={() => window.location.reload()}
              className="px-4 py-2 rounded-xl bg-cyan-500 text-slate-950 text-xs font-bold cursor-pointer"
            >
              Reload Console
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </StrictMode>,
)
