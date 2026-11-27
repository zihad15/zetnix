import { useEffect, useMemo, useRef, useState } from 'react'
import { Download, Pause, Play, Search, Trash2 } from 'lucide-react'

interface LogViewerProps {
  namespace: string
  pod: string
  containers: string[]
}

export function LogViewer({ namespace, pod, containers }: LogViewerProps) {
  const [container, setContainer] = useState(containers[0] || '')
  const [follow, setFollow] = useState(true)
  const [tail, setTail] = useState(250)
  const [filter, setFilter] = useState('')
  const [wrap, setWrap] = useState(false)
  const [lines, setLines] = useState<string[]>([])
  const [status, setStatus] = useState<'connecting' | 'streaming' | 'ended' | 'error'>('connecting')
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    setContainer((prev) => (containers.includes(prev) ? prev : containers[0] || ''))
  }, [containers])

  useEffect(() => {
    if (!pod || !namespace) return
    setLines([])
    setStatus('connecting')

    const params = new URLSearchParams({
      tail: String(tail),
      follow: follow ? 'true' : 'false',
    })
    if (container) params.set('container', container)

    const url = `/api/v1/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(pod)}/logs?${params}`
    const es = new EventSource(url)

    es.onopen = () => setStatus('streaming')
    es.addEventListener('log', (ev) => {
      setStatus('streaming')
      try {
        const payload = JSON.parse((ev as MessageEvent).data) as { line: string }
        setLines((prev) => {
          const next = [...prev, payload.line]
          return next.length > 2000 ? next.slice(next.length - 2000) : next
        })
      } catch {
        // ignore malformed chunk
      }
    })
    es.addEventListener('done', () => {
      setStatus('ended')
      es.close()
    })
    es.onerror = () => {
      setStatus((prev) => (prev === 'ended' ? 'ended' : 'error'))
      es.close()
    }

    return () => {
      es.close()
    }
  }, [namespace, pod, container, tail, follow])

  useEffect(() => {
    if (follow) {
      bottomRef.current?.scrollIntoView({ behavior: 'auto' })
    }
  }, [lines, follow])

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return lines
    return lines.filter((l) => l.toLowerCase().includes(q))
  }, [lines, filter])

  const downloadLogs = () => {
    const blob = new Blob([filtered.join('\n')], { type: 'text/plain;charset=utf-8' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = `${namespace}-${pod}.log`
    a.click()
  }

  const lineStyle = (line: string) => {
    const u = line.toUpperCase()
    if (u.includes('ERROR') || u.includes('PANIC') || u.includes('FATAL') || u.includes('OOMKILLED') || u.includes('CRASHLOOP')) {
      return 'text-rose-300 bg-rose-500/8'
    }
    if (u.includes('WARN')) {
      return 'text-amber-300'
    }
    if (u.includes('INF') || u.includes('INFO')) {
      return 'text-slate-200'
    }
    return 'text-slate-300'
  }

  return (
    <div className="flex flex-col h-full">
      {/* Controls */}
      <div className="flex flex-wrap items-center justify-between gap-2 pb-3 border-b border-slate-800/80">
        <div className="flex flex-wrap items-center gap-2">
          {containers.length > 1 && (
            <select
              value={container}
              onChange={(e) => setContainer(e.target.value)}
              className="px-2.5 py-1 rounded-lg bg-[#0d1424] border border-slate-700 text-xs font-mono text-slate-200"
            >
              {containers.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
          )}
          <select
            value={tail}
            onChange={(e) => setTail(Number(e.target.value))}
            className="px-2 py-1 rounded-lg bg-[#0d1424] border border-slate-700 text-xs font-mono text-slate-200"
          >
            <option value={100}>Tail 100</option>
            <option value={250}>Tail 250</option>
            <option value={1000}>Tail 1000</option>
          </select>

          <div className="relative">
            <Search className="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
            <input
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Grep logs..."
              className="pl-8 pr-2.5 py-1 rounded-lg bg-[#0d1424] border border-slate-700 text-xs font-mono text-slate-100 w-44 focus:outline-none focus:border-cyan-500"
            />
          </div>
        </div>

        <div className="flex items-center gap-1.5">
          <span
            className={`text-[10px] font-mono px-2 py-0.5 rounded-full border ${
              status === 'streaming'
                ? 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30'
                : status === 'error'
                  ? 'bg-rose-500/15 text-rose-300 border-rose-500/30'
                  : 'bg-slate-700/40 text-slate-300 border-slate-700'
            }`}
          >
            {status.toUpperCase()}
          </span>
          <button
            onClick={() => setFollow((f) => !f)}
            className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-lg border text-xs font-medium cursor-pointer ${
              follow ? 'bg-cyan-500/15 border-cyan-500/35 text-cyan-300' : 'bg-[#0d1424] border-slate-700 text-slate-300'
            }`}
          >
            {follow ? <Pause className="w-3 h-3" /> : <Play className="w-3 h-3" />}
            {follow ? 'Follow' : 'Paused'}
          </button>
          <button
            onClick={() => setWrap((w) => !w)}
            className={`px-2 py-1 rounded-lg border text-xs font-mono cursor-pointer ${
              wrap ? 'bg-slate-700 text-slate-100 border-slate-600' : 'bg-[#0d1424] border-slate-700 text-slate-400'
            }`}
          >
            Wrap
          </button>
          <button
            onClick={() => setLines([])}
            title="Clear buffer"
            className="p-1.5 rounded-lg bg-[#0d1424] border border-slate-700 text-slate-400 hover:text-slate-200 cursor-pointer"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={downloadLogs}
            title="Download .log"
            className="p-1.5 rounded-lg bg-[#0d1424] border border-slate-700 text-slate-400 hover:text-cyan-300 cursor-pointer"
          >
            <Download className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Log output */}
      <div className="flex-1 mt-3 rounded-xl bg-[#04070e] border border-slate-800/90 p-3 overflow-auto font-mono text-[12px] leading-5">
        {filtered.length === 0 ? (
          <div className="text-slate-500 py-10 text-center">
            {status === 'connecting' ? 'Connecting to pod log stream...' : 'No log lines matching filter'}
          </div>
        ) : (
          filtered.map((line, i) => (
            <div
              key={i}
              className={`px-1.5 py-0.5 rounded flex gap-3 hover:bg-slate-800/40 ${lineStyle(line)} ${wrap ? 'whitespace-pre-wrap break-all' : 'whitespace-pre'}`}
            >
              <span className="text-slate-600 select-none w-8 text-right shrink-0">{i + 1}</span>
              <span>{line}</span>
            </div>
          ))
        )}
        <div ref={bottomRef} />
      </div>
    </div>
  )
}
