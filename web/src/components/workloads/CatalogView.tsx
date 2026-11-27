import { ArrowRight, Sparkles } from 'lucide-react'
import { CATALOG_TEMPLATES, type CatalogTemplate } from '../../lib/templates'

export function CatalogView({
  activeNamespace,
  dark,
  onLaunchTemplate,
}: {
  activeNamespace: string
  dark: boolean
  onLaunchTemplate: (tpl: CatalogTemplate) => void
}) {
  const ns = activeNamespace === 'all' ? 'production' : activeNamespace

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 className="text-lg font-extrabold flex items-center gap-2">
            <Sparkles className="w-5 h-5 text-cyan-400" />
            Zetnix Developer Catalog
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">
            One-click OpenShift-style curated stacks ready to customize and deploy into <span className="font-mono text-cyan-300">{ns}</span>.
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-2 gap-4">
        {CATALOG_TEMPLATES.map((tpl) => (
          <div
            key={tpl.id}
            className={`rounded-2xl border bg-gradient-to-br p-5 flex flex-col justify-between transition-all hover:-translate-y-0.5 ${
              dark ? tpl.accent : 'from-slate-50 to-white border-slate-200'
            }`}
          >
            <div>
              <div className="flex items-center justify-between gap-2 mb-3">
                <span className="px-2.5 py-0.5 rounded-full text-[11px] font-mono font-semibold bg-slate-900/70 text-cyan-300 border border-slate-700">
                  {tpl.category}
                </span>
                <span className="text-[11px] font-mono text-slate-400">{tpl.version}</span>
              </div>
              <h3 className={`text-base font-bold ${dark ? 'text-slate-100' : 'text-slate-900'}`}>{tpl.title}</h3>
              <p className={`text-xs mt-1.5 leading-relaxed ${dark ? 'text-slate-300' : 'text-slate-600'}`}>{tpl.description}</p>
            </div>

            <div className="mt-5 pt-3 border-t border-slate-800/50 flex items-center justify-between">
              <span className="text-[11px] font-mono text-slate-400">{tpl.badge}</span>
              <button
                onClick={() => onLaunchTemplate(tpl)}
                className="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-cyan-500 hover:bg-cyan-400 text-slate-950 text-xs font-bold cursor-pointer transition-colors"
              >
                Deploy Stack
                <ArrowRight className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
