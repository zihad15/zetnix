import { FileCode2, X } from 'lucide-react'
import { YamlStudio } from '../inspector/YamlStudio'

interface CreateResourceModalProps {
  open: boolean
  initialYaml: string
  onClose: () => void
  onApplied: (summary: string) => void
}

export function CreateResourceModal({ open, initialYaml, onClose, onApplied }: CreateResourceModalProps) {
  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm" onClick={onClose}>
      <div
        className="w-full max-w-4xl h-[80vh] rounded-2xl border border-slate-700/80 bg-[#090f1d] shadow-2xl flex flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-5 py-3.5 border-b border-slate-800 bg-[#0d1424] flex items-center justify-between">
          <div className="flex items-center gap-2">
            <FileCode2 className="w-4 h-4 text-cyan-400" />
            <span className="text-sm font-bold text-slate-100">Deploy Multi-Document Kubernetes YAML</span>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-lg hover:bg-slate-800 text-slate-400 cursor-pointer">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="flex-1 p-5 overflow-hidden">
          <YamlStudio
            initialYaml={initialYaml}
            title="Import / Edit Manifest"
            onApplied={(summary) => {
              onApplied(summary)
              onClose()
            }}
          />
        </div>
      </div>
    </div>
  )
}
