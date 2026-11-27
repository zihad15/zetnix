import { useEffect, useMemo, useState } from 'react'
import yaml from 'js-yaml'
import { CheckCircle2, Copy, Play, RotateCcw, Sparkles, XCircle } from 'lucide-react'
import { api } from '../../lib/api'

interface YamlStudioProps {
  initialYaml: string
  title?: string
  onApplied?: (summary: string) => void
}

export function YamlStudio({ initialYaml, title = 'Kubernetes Manifest Studio', onApplied }: YamlStudioProps) {
  const [code, setCode] = useState(initialYaml)
  const [busy, setBusy] = useState(false)
  const [feedback, setFeedback] = useState<{ type: 'ok' | 'err'; text: string } | null>(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    setCode(initialYaml)
    setFeedback(null)
  }, [initialYaml])

  const syntaxCheck = useMemo(() => {
    try {
      const docs = yaml.loadAll(code)
      const count = docs.filter((d) => d && typeof d === 'object').length
      return { valid: count > 0, count, err: count === 0 ? 'Empty YAML document' : '' }
    } catch (e) {
      return { valid: false, count: 0, err: (e as Error).message }
    }
  }, [code])

  const lineCount = useMemo(() => Math.max(12, code.split('\n').length), [code])

  const handleApply = async () => {
    if (!syntaxCheck.valid) return
    setBusy(true)
    setFeedback(null)
    try {
      const res = await api.applyYaml(code)
      const summary = res.applied.map((a) => `${a.kind}/${a.name} (${a.action})`).join(', ')
      setFeedback({ type: 'ok', text: `Applied: ${summary}` })
      onApplied?.(summary)
    } catch (e) {
      setFeedback({ type: 'err', text: (e as Error).message })
    } finally {
      setBusy(false)
    }
  }

  const handleFormat = () => {
    try {
      const docs = yaml.loadAll(code).filter(Boolean)
      const formatted = docs.map((d) => yaml.dump(d, { indent: 2, lineWidth: 120 })).join('---\n')
      setCode(formatted)
      setFeedback(null)
    } catch (e) {
      setFeedback({ type: 'err', text: (e as Error).message })
    }
  }

  return (
    <div className="flex flex-col h-full gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="text-xs font-bold uppercase tracking-wider text-slate-300">{title}</span>
          {syntaxCheck.valid ? (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] bg-emerald-500/12 text-emerald-300 border border-emerald-500/30">
              <CheckCircle2 className="w-3 h-3" />
              Valid ({syntaxCheck.count} doc{syntaxCheck.count > 1 ? 's' : ''})
            </span>
          ) : (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] bg-rose-500/12 text-rose-300 border border-rose-500/30 max-w-xs truncate">
              <XCircle className="w-3 h-3 shrink-0" />
              {syntaxCheck.err}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={handleFormat}
            className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-xl bg-[#0d1424] border border-slate-700 hover:border-slate-500 text-xs text-slate-200 cursor-pointer"
          >
            <Sparkles className="w-3.5 h-3.5 text-violet-400" />
            Format
          </button>
          <button
            onClick={() => {
              navigator.clipboard.writeText(code)
              setCopied(true)
              setTimeout(() => setCopied(false), 1500)
            }}
            className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-xl bg-[#0d1424] border border-slate-700 hover:border-slate-500 text-xs text-slate-200 cursor-pointer"
          >
            <Copy className="w-3.5 h-3.5" />
            {copied ? 'Copied' : 'Copy'}
          </button>
          <button
            onClick={() => setCode(initialYaml)}
            className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-xl bg-[#0d1424] border border-slate-700 hover:border-slate-500 text-xs text-slate-300 cursor-pointer"
          >
            <RotateCcw className="w-3.5 h-3.5" />
            Reset
          </button>
          <button
            onClick={handleApply}
            disabled={busy || !syntaxCheck.valid}
            className="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-gradient-to-r from-cyan-500 to-blue-500 hover:from-cyan-400 hover:to-blue-400 disabled:opacity-45 text-slate-950 text-xs font-bold cursor-pointer"
          >
            <Play className="w-3.5 h-3.5 fill-current" />
            {busy ? 'Applying...' : 'Apply Manifest'}
          </button>
        </div>
      </div>

      {feedback && (
        <div
          className={`px-3 py-2 rounded-xl border text-xs font-mono ${
            feedback.type === 'ok'
              ? 'bg-emerald-500/12 border-emerald-500/35 text-emerald-300'
              : 'bg-rose-500/12 border-rose-500/35 text-rose-300'
          }`}
        >
          {feedback.text}
        </div>
      )}

      <div className="flex-1 flex rounded-xl border border-slate-800 bg-[#04070e] overflow-hidden min-h-[340px]">
        <div className="py-3 px-2.5 bg-[#070b15] border-r border-slate-800/80 text-right font-mono text-[12px] leading-5 text-slate-600 select-none overflow-hidden">
          {Array.from({ length: lineCount }, (_, idx) => (
            <div key={idx}>{idx + 1}</div>
          ))}
        </div>
        <textarea
          value={code}
          onChange={(e) => setCode(e.target.value)}
          spellCheck={false}
          className="mono flex-1 p-3 bg-transparent text-[12px] leading-5 text-cyan-100 focus:outline-none resize-none overflow-auto"
        />
      </div>
    </div>
  )
}
