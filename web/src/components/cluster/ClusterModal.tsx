import React, { useEffect, useState } from 'react'
import { FileUp, Key, KeyRound, Lock, Server, ShieldAlert, Terminal, Trash2, Upload, X } from 'lucide-react'
import type { ClusterStatus } from '../../types/k8s'
import { api } from '../../lib/api'

interface ClusterModalProps {
  open: boolean
  cluster: ClusterStatus | null
  onClose: () => void
  onUpdated: (next: ClusterStatus) => void
}

type SetupTab = 'ssh' | 'kubeconfig' | 'token'

export function ClusterModal({ open, cluster, onClose, onUpdated }: ClusterModalProps) {
  const [tab, setTab] = useState<SetupTab>('ssh')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')

  // SSH Auto-Tunnel state
  const [sshLabel, setSshLabel] = useState('')
  const [sshHost, setSshHost] = useState('')
  const [sshPort, setSshPort] = useState('22')
  const [sshUser, setSshUser] = useState('root')
  const [sshAuthType, setSshAuthType] = useState<'password' | 'key'>('password')
  const [sshPassword, setSshPassword] = useState('')
  const [sshKey, setSshKey] = useState('')
  const [sshPassphrase, setSshPassphrase] = useState('')
  const [remoteKubePath, setRemoteKubePath] = useState('')
  const [useSudo, setUseSudo] = useState(true)
  const [sshInsecureTLS, setSshInsecureTLS] = useState(true)
  const [trustHostKey, setTrustHostKey] = useState(true)

  // Kubeconfig YAML state
  const [kubeLabel, setKubeLabel] = useState('')
  const [kubeconfig, setKubeconfig] = useState('')
  const [kubeInsecureTLS, setKubeInsecureTLS] = useState(false)

  // Direct API + Token state
  const [tokenLabel, setTokenLabel] = useState('')
  const [apiServerUrl, setApiServerUrl] = useState('')
  const [bearerToken, setBearerToken] = useState('')
  const [caData, setCaData] = useState('')
  const [tokenInsecureTLS, setTokenInsecureTLS] = useState(true)

  useEffect(() => {
    if (open) setErr('')
  }, [open])

  if (!open) return null

  const handleSwitch = async (id: string) => {
    setBusy(true)
    setErr('')
    try {
      const next = await api.switchCluster(id)
      onUpdated(next)
      onClose()
    } catch (e) {
      setErr((e as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const handleDisconnect = async (e: React.MouseEvent, id: string) => {
    e.stopPropagation()
    setBusy(true)
    setErr('')
    try {
      const next = await api.disconnectCluster(id)
      onUpdated(next)
    } catch (ex) {
      setErr((ex as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const handleConnectSSH = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setErr('')
    try {
      const next = await api.connectSSHCluster({
        name: sshLabel.trim(),
        host: sshHost.trim(),
        port: sshPort.trim() || '22',
        username: sshUser.trim() || 'root',
        password: sshAuthType === 'password' ? sshPassword : '',
        privateKey: sshAuthType === 'key' ? sshKey : '',
        passphrase: sshAuthType === 'key' ? sshPassphrase : '',
        remoteKubeconfigPath: remoteKubePath.trim(),
        useSudo,
        insecureSkipTlsVerify: sshInsecureTLS,
        trustHostKey,
      })
      onUpdated(next)
      setSshPassword('')
      setSshPassphrase('')
      onClose()
    } catch (ex) {
      setErr((ex as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const handleConnectKubeconfig = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!kubeconfig.trim()) return
    setBusy(true)
    setErr('')
    try {
      const next = await api.connectCluster({
        name: kubeLabel.trim(),
        kubeconfig,
        insecureSkipTlsVerify: kubeInsecureTLS,
      })
      onUpdated(next)
      setKubeconfig('')
      setKubeLabel('')
      onClose()
    } catch (ex) {
      setErr((ex as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const handleConnectToken = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setErr('')
    try {
      const next = await api.connectTokenCluster({
        name: tokenLabel.trim(),
        apiServerUrl: apiServerUrl.trim(),
        bearerToken: bearerToken.trim(),
        caData: caData.trim(),
        insecureSkipTlsVerify: tokenInsecureTLS,
      })
      onUpdated(next)
      setBearerToken('')
      onClose()
    } catch (ex) {
      setErr((ex as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const readFileText = (file: File | undefined, onLoaded: (text: string) => void) => {
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => {
      if (typeof reader.result === 'string') {
        onLoaded(reader.result)
      }
    }
    reader.readAsText(file)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm" onClick={onClose}>
      <div
        className="w-full max-w-2xl rounded-2xl border border-slate-700/80 bg-[#090f1d] shadow-2xl overflow-hidden max-h-[90vh] flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-5 py-4 border-b border-slate-800 flex items-center justify-between bg-[#0d1424]">
          <div className="flex items-center gap-2.5">
            <Server className="w-4 h-4 text-cyan-400" />
            <h2 className="text-sm font-bold text-slate-100">Cluster Connections & In-UI Setup</h2>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-lg hover:bg-slate-800 text-slate-400 cursor-pointer">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-5 overflow-y-auto space-y-5 text-xs">
          {err && (
            <div className="p-3 rounded-xl border border-rose-500/40 bg-rose-500/12 text-rose-300 font-mono leading-relaxed">
              {err}
            </div>
          )}

          {/* Available Contexts */}
          <div>
            <div className="text-[11px] font-bold uppercase tracking-wider text-slate-400 mb-2">
              Connected & Available Clusters
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
              {(cluster?.contexts || []).map((ctx) => {
                const removable = ctx.id.startsWith('ssh:') || ctx.id.startsWith('uploaded:') || ctx.id.startsWith('token:')
                return (
                  <div
                    key={ctx.id}
                    onClick={() => !busy && handleSwitch(ctx.id)}
                    className={`p-3 rounded-xl border text-left transition-all cursor-pointer flex items-start justify-between gap-2 ${
                      ctx.active
                        ? 'bg-cyan-500/12 border-cyan-500/45 text-cyan-100'
                        : 'bg-[#0d1424] border-slate-800 hover:border-slate-600 text-slate-300'
                    }`}
                  >
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-mono font-bold truncate">{ctx.name}</span>
                        <span className="text-[10px] font-mono uppercase px-1.5 py-0.5 rounded bg-slate-800 text-cyan-300 shrink-0">
                          {ctx.mode}
                        </span>
                      </div>
                      <div className="text-[11px] text-slate-400 mt-1 truncate">
                        {ctx.cluster} • {ctx.user}
                      </div>
                    </div>
                    {removable && (
                      <button
                        type="button"
                        onClick={(e) => handleDisconnect(e, ctx.id)}
                        title="Disconnect & remove cluster"
                        className="p-1 rounded-lg hover:bg-rose-500/20 text-slate-500 hover:text-rose-300 shrink-0 cursor-pointer"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    )}
                  </div>
                )
              })}
            </div>
          </div>

          {/* 3-Tab In-UI Setup Wizard */}
          <div className="rounded-xl border border-slate-800 bg-[#0c1322] overflow-hidden">
            <div className="grid grid-cols-3 border-b border-slate-800 bg-[#0a101d] p-1 gap-1">
              <button
                type="button"
                onClick={() => {
                  setTab('ssh')
                  setErr('')
                }}
                className={`flex items-center justify-center gap-1.5 py-2 rounded-lg font-semibold transition-all cursor-pointer ${
                  tab === 'ssh' ? 'bg-cyan-500/15 text-cyan-300 border border-cyan-500/35' : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <Terminal className="w-3.5 h-3.5" />
                VPS SSH Auto-Tunnel
              </button>
              <button
                type="button"
                onClick={() => {
                  setTab('kubeconfig')
                  setErr('')
                }}
                className={`flex items-center justify-center gap-1.5 py-2 rounded-lg font-semibold transition-all cursor-pointer ${
                  tab === 'kubeconfig' ? 'bg-cyan-500/15 text-cyan-300 border border-cyan-500/35' : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <FileUp className="w-3.5 h-3.5" />
                Kubeconfig File / YAML
              </button>
              <button
                type="button"
                onClick={() => {
                  setTab('token')
                  setErr('')
                }}
                className={`flex items-center justify-center gap-1.5 py-2 rounded-lg font-semibold transition-all cursor-pointer ${
                  tab === 'token' ? 'bg-cyan-500/15 text-cyan-300 border border-cyan-500/35' : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <KeyRound className="w-3.5 h-3.5" />
                API URL + Token
              </button>
            </div>

            <div className="p-4">
              {tab === 'ssh' && (
                <form onSubmit={handleConnectSSH} className="space-y-3">
                  <div className="flex items-center justify-between text-slate-400">
                    <span>Zetnix automatically creates an in-process SSH tunnel and reads your VPS kubeconfig.</span>
                    <span className="text-[11px] text-slate-500 flex items-center gap-1 shrink-0">
                      <ShieldAlert className="w-3 h-3" /> In-memory only
                    </span>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-4 gap-2">
                    <input
                      value={sshHost}
                      onChange={(e) => setSshHost(e.target.value)}
                      required
                      placeholder="VPS IP / Host"
                      className="sm:col-span-2 px-3 py-2 rounded-xl bg-[#060a14] border border-slate-800 text-slate-100 font-mono focus:outline-none focus:border-cyan-500"
                    />
                    <input
                      value={sshPort}
                      onChange={(e) => setSshPort(e.target.value)}
                      placeholder="SSH Port (22)"
                      className="px-3 py-2 rounded-xl bg-[#060a14] border border-slate-800 text-slate-100 font-mono focus:outline-none focus:border-cyan-500"
                    />
                    <input
                      value={sshUser}
                      onChange={(e) => setSshUser(e.target.value)}
                      placeholder="Username"
                      className="px-3 py-2 rounded-xl bg-[#060a14] border border-slate-800 text-slate-100 font-mono focus:outline-none focus:border-cyan-500"
                    />
                  </div>

                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => setSshAuthType('password')}
                      className={`px-3 py-1.5 rounded-lg border font-semibold cursor-pointer ${
                        sshAuthType === 'password'
                          ? 'bg-slate-800 border-cyan-500/40 text-cyan-300'
                          : 'bg-[#060a14] border-slate-800 text-slate-400'
                      }`}
                    >
                      Password Auth
                    </button>
                    <button
                      type="button"
                      onClick={() => setSshAuthType('key')}
                      className={`px-3 py-1.5 rounded-lg border font-semibold cursor-pointer ${
                        sshAuthType === 'key'
                          ? 'bg-slate-800 border-cyan-500/40 text-cyan-300'
                          : 'bg-[#060a14] border-slate-800 text-slate-400'
                      }`}
                    >
                      SSH Private Key
                    </button>
                    <input
                      value={sshLabel}
                      onChange={(e) => setSshLabel(e.target.value)}
                      placeholder="Label (e.g. vps-prod)"
                      className="flex-1 px-3 py-1.5 rounded-lg bg-[#060a14] border border-slate-800 text-slate-200 font-mono focus:outline-none focus:border-cyan-500"
                    />
                  </div>

                  {sshAuthType === 'password' ? (
                    <input
                      type="password"
                      value={sshPassword}
                      onChange={(e) => setSshPassword(e.target.value)}
                      required
                      placeholder="SSH Password"
                      className="w-full px-3 py-2 rounded-xl bg-[#060a14] border border-slate-800 text-slate-100 font-mono focus:outline-none focus:border-cyan-500"
                    />
                  ) : (
                    <div className="space-y-2">
                      <div className="flex items-center justify-between gap-2">
                        <label className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#060a14] border border-slate-700 hover:border-cyan-500 text-slate-200 cursor-pointer">
                          <Upload className="w-3.5 h-3.5 text-cyan-400" />
                          Load Private Key File
                          <input
                            type="file"
                            className="hidden"
                            onChange={(e) => readFileText(e.target.files?.[0], setSshKey)}
                          />
                        </label>
                        <input
                          type="password"
                          value={sshPassphrase}
                          onChange={(e) => setSshPassphrase(e.target.value)}
                          placeholder="Key passphrase (optional)"
                          className="flex-1 px-3 py-1.5 rounded-lg bg-[#060a14] border border-slate-800 text-slate-200 font-mono focus:outline-none focus:border-cyan-500"
                        />
                      </div>
                      <textarea
                        value={sshKey}
                        onChange={(e) => setSshKey(e.target.value)}
                        required
                        rows={4}
                        placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                        className="w-full p-2.5 rounded-xl bg-[#060a14] border border-slate-800 text-cyan-100 font-mono text-[11px] focus:outline-none focus:border-cyan-500"
                      />
                    </div>
                  )}

                  <input
                    value={remoteKubePath}
                    onChange={(e) => setRemoteKubePath(e.target.value)}
                    placeholder="Remote kubeconfig path (leave blank to auto-detect ~/.kube/config, k3s.yaml, admin.conf)"
                    className="w-full px-3 py-2 rounded-xl bg-[#060a14] border border-slate-800 text-slate-200 font-mono text-[11px] focus:outline-none focus:border-cyan-500"
                  />

                  <div className="flex flex-wrap items-center justify-between gap-3 pt-1">
                    <div className="flex flex-wrap items-center gap-4 text-[11px] text-slate-300">
                      <label className="inline-flex items-center gap-1.5 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={useSudo}
                          onChange={(e) => setUseSudo(e.target.checked)}
                          className="accent-cyan-400"
                        />
                        Use sudo if kubeconfig is root-owned
                      </label>
                      <label className="inline-flex items-center gap-1.5 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={sshInsecureTLS}
                          onChange={(e) => setSshInsecureTLS(e.target.checked)}
                          className="accent-cyan-400"
                        />
                        Allow Self-Signed K8s TLS
                      </label>
                      <label className="inline-flex items-center gap-1.5 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={trustHostKey}
                          onChange={(e) => setTrustHostKey(e.target.checked)}
                          className="accent-cyan-400"
                        />
                        Auto-Trust SSH Host Key
                      </label>
                    </div>

                    <button
                      type="submit"
                      disabled={busy || !sshHost.trim()}
                      className="px-4 py-2 rounded-xl bg-gradient-to-r from-cyan-500 to-blue-500 text-slate-950 font-bold disabled:opacity-45 cursor-pointer"
                    >
                      {busy ? 'Opening Tunnel...' : 'Connect via SSH Tunnel'}
                    </button>
                  </div>
                </form>
              )}

              {tab === 'kubeconfig' && (
                <form onSubmit={handleConnectKubeconfig} className="space-y-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <label className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#060a14] border border-slate-700 hover:border-cyan-500 text-slate-200 cursor-pointer">
                      <Upload className="w-3.5 h-3.5 text-cyan-400" />
                      Upload Kubeconfig File (.yaml / config)
                      <input
                        type="file"
                        className="hidden"
                        onChange={(e) => readFileText(e.target.files?.[0], setKubeconfig)}
                      />
                    </label>
                    <input
                      value={kubeLabel}
                      onChange={(e) => setKubeLabel(e.target.value)}
                      placeholder="Display label (optional)"
                      className="flex-1 min-w-[180px] px-3 py-1.5 rounded-lg bg-[#060a14] border border-slate-800 text-slate-200 font-mono focus:outline-none focus:border-cyan-500"
                    />
                  </div>

                  <textarea
                    value={kubeconfig}
                    onChange={(e) => setKubeconfig(e.target.value)}
                    rows={6}
                    placeholder="apiVersion: v1&#10;kind: Config&#10;clusters: ..."
                    className="w-full p-3 rounded-xl bg-[#060a14] border border-slate-800 text-cyan-100 font-mono text-[11px] focus:outline-none focus:border-cyan-500"
                  />

                  <div className="flex items-center justify-between">
                    <label className="inline-flex items-center gap-1.5 text-[11px] text-slate-300 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={kubeInsecureTLS}
                        onChange={(e) => setKubeInsecureTLS(e.target.checked)}
                        className="accent-cyan-400"
                      />
                      Allow Self-Signed TLS (skip CA verification)
                    </label>
                    <button
                      type="submit"
                      disabled={busy || !kubeconfig.trim()}
                      className="px-4 py-2 rounded-xl bg-gradient-to-r from-cyan-500 to-blue-500 text-slate-950 font-bold disabled:opacity-45 cursor-pointer"
                    >
                      {busy ? 'Testing API...' : 'Connect Kubeconfig'}
                    </button>
                  </div>
                </form>
              )}

              {tab === 'token' && (
                <form onSubmit={handleConnectToken} className="space-y-3">
                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                    <input
                      value={apiServerUrl}
                      onChange={(e) => setApiServerUrl(e.target.value)}
                      required
                      placeholder="https://10.0.0.10:6443"
                      className="sm:col-span-2 px-3 py-2 rounded-xl bg-[#060a14] border border-slate-800 text-slate-100 font-mono focus:outline-none focus:border-cyan-500"
                    />
                    <input
                      value={tokenLabel}
                      onChange={(e) => setTokenLabel(e.target.value)}
                      placeholder="Label (e.g. prod)"
                      className="px-3 py-2 rounded-xl bg-[#060a14] border border-slate-800 text-slate-100 font-mono focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                  <div className="relative">
                    <Key className="w-3.5 h-3.5 text-slate-500 absolute left-3 top-3" />
                    <textarea
                      value={bearerToken}
                      onChange={(e) => setBearerToken(e.target.value)}
                      required
                      rows={3}
                      placeholder="ServiceAccount Bearer Token (eyJhbGci...)"
                      className="w-full pl-9 pr-3 py-2 rounded-xl bg-[#060a14] border border-slate-800 text-cyan-100 font-mono text-[11px] focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                  {!tokenInsecureTLS && (
                    <textarea
                      value={caData}
                      onChange={(e) => setCaData(e.target.value)}
                      rows={3}
                      placeholder="-----BEGIN CERTIFICATE----- (Optional CA PEM)"
                      className="w-full p-2.5 rounded-xl bg-[#060a14] border border-slate-800 text-slate-200 font-mono text-[11px] focus:outline-none focus:border-cyan-500"
                    />
                  )}
                  <div className="flex items-center justify-between">
                    <label className="inline-flex items-center gap-1.5 text-[11px] text-slate-300 cursor-pointer">
                      <Lock className="w-3 h-3 text-amber-400" />
                      <input
                        type="checkbox"
                        checked={tokenInsecureTLS}
                        onChange={(e) => setTokenInsecureTLS(e.target.checked)}
                        className="accent-cyan-400"
                      />
                      Skip TLS CA Verification (self-signed API server)
                    </label>
                    <button
                      type="submit"
                      disabled={busy || !apiServerUrl.trim() || !bearerToken.trim()}
                      className="px-4 py-2 rounded-xl bg-gradient-to-r from-cyan-500 to-blue-500 text-slate-950 font-bold disabled:opacity-45 cursor-pointer"
                    >
                      {busy ? 'Connecting...' : 'Connect with Token'}
                    </button>
                  </div>
                </form>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
