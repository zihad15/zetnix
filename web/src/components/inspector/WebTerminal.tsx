import { useEffect, useRef, useState } from 'react'
import { Terminal as XTerm } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { RefreshCw, TerminalSquare } from 'lucide-react'

interface WebTerminalProps {
  namespace: string
  pod: string
  containers: string[]
}

export function WebTerminal({ namespace, pod, containers }: WebTerminalProps) {
  const [container, setContainer] = useState(containers[0] || '')
  const [shell, setShell] = useState('/bin/sh')
  const [connState, setConnState] = useState<'connecting' | 'connected' | 'closed'>('connecting')
  const [nonce, setNonce] = useState(0)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    setContainer((prev) => (containers.includes(prev) ? prev : containers[0] || ''))
  }, [containers])

  useEffect(() => {
    if (!containerRef.current || !namespace || !pod) return

    const term = new XTerm({
      cursorBlink: true,
      fontFamily: "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace",
      fontSize: 13,
      lineHeight: 1.35,
      theme: {
        background: '#04070e',
        foreground: '#e2e8f0',
        cursor: '#22d3ee',
        selectionBackground: 'rgba(34, 211, 238, 0.25)',
        black: '#0f172a',
        red: '#fb7185',
        green: '#34d399',
        yellow: '#fbbf24',
        blue: '#60a5fa',
        magenta: '#c084fc',
        cyan: '#22d3ee',
        white: '#f1f5f9',
      },
    })

    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(containerRef.current)
    fit.fit()
    term.focus()

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const params = new URLSearchParams({ shell })
    if (container) params.set('container', container)
    const wsUrl = `${proto}//${window.location.host}/api/v1/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(pod)}/exec?${params}`

    setConnState('connecting')
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      setConnState('connected')
      ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    }
    ws.onmessage = (ev) => {
      if (typeof ev.data === 'string') {
        term.write(ev.data)
      }
    }
    ws.onclose = () => {
      setConnState('closed')
      term.write('\r\n\x1b[90m[WebSocket PTY disconnected]\x1b[0m\r\n')
    }
    ws.onerror = () => {
      setConnState('closed')
    }

    const dataSub = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'stdin', data }))
      }
    })

    const resizeSub = term.onResize(({ cols, rows }) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols, rows }))
      }
    })

    const ro = new ResizeObserver(() => {
      try {
        fit.fit()
      } catch {
        // ignore resize error during unmount
      }
    })
    ro.observe(containerRef.current)

    return () => {
      ro.disconnect()
      dataSub.dispose()
      resizeSub.dispose()
      ws.close()
      term.dispose()
    }
  }, [namespace, pod, container, shell, nonce])

  return (
    <div className="flex flex-col h-full">
      <div className="flex flex-wrap items-center justify-between gap-2 pb-3 border-b border-slate-800/80">
        <div className="flex items-center gap-2">
          <TerminalSquare className="w-4 h-4 text-cyan-400" />
          <span className="text-xs font-mono text-slate-300">
            exec://{namespace}/{pod}
          </span>
          {containers.length > 1 && (
            <select
              value={container}
              onChange={(e) => setContainer(e.target.value)}
              className="px-2 py-1 rounded-lg bg-[#0d1424] border border-slate-700 text-xs font-mono text-slate-200"
            >
              {containers.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
          )}
          <select
            value={shell}
            onChange={(e) => setShell(e.target.value)}
            className="px-2 py-1 rounded-lg bg-[#0d1424] border border-slate-700 text-xs font-mono text-slate-200"
          >
            <option value="/bin/sh">/bin/sh</option>
            <option value="/bin/bash">/bin/bash</option>
          </select>
        </div>

        <div className="flex items-center gap-2">
          <span
            className={`text-[10px] font-mono px-2 py-0.5 rounded-full border ${
              connState === 'connected'
                ? 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30'
                : connState === 'connecting'
                  ? 'bg-amber-500/15 text-amber-300 border-amber-500/30'
                  : 'bg-rose-500/15 text-rose-300 border-rose-500/30'
            }`}
          >
            {connState.toUpperCase()}
          </span>
          <button
            onClick={() => setNonce((n) => n + 1)}
            className="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-[#0d1424] border border-slate-700 text-xs text-slate-200 hover:border-cyan-500 cursor-pointer"
          >
            <RefreshCw className="w-3 h-3" />
            Reconnect
          </button>
        </div>
      </div>

      <div
        ref={containerRef}
        className="flex-1 mt-3 rounded-xl overflow-hidden border border-slate-800 bg-[#04070e] p-2 min-h-[340px]"
      />
    </div>
  )
}
