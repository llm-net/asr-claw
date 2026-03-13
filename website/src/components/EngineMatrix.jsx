const engines = [
  { name: 'qwen-asr', type: 'localCli', mac: true, gpu: false, streaming: 'vad', install: 'engines install qwen-asr', recommended: true },
  { name: 'qwen3-asr', type: 'vllmService', mac: false, gpu: true, streaming: 'native', install: 'engines start qwen3-asr', recommended: false },
  { name: 'whisper', type: 'localCli', mac: true, gpu: false, streaming: 'vad', install: 'Manual', recommended: false },
  { name: 'doubao', type: 'cloudApi', mac: true, gpu: null, streaming: null, install: 'DOUBAO_API_KEY', recommended: false },
  { name: 'openai', type: 'cloudApi', mac: true, gpu: null, streaming: null, install: 'OPENAI_API_KEY', recommended: false },
  { name: 'deepgram', type: 'cloudApi', mac: true, gpu: null, streaming: 'native', install: 'DEEPGRAM_API_KEY', recommended: false },
]

function Badge({ children, variant = 'default' }) {
  const styles = {
    default: 'bg-surface-3/50 text-slate-400',
    green: 'bg-green-500/10 text-green-400 border border-green-500/20',
    blue: 'bg-claw-500/10 text-claw-400 border border-claw-500/20',
    amber: 'bg-amber-500/10 text-amber-400 border border-amber-500/20',
  }
  return (
    <span className={`inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium ${styles[variant]}`}>
      {children}
    </span>
  )
}

export default function EngineMatrix({ t }) {
  const em = t.engineMatrix

  return (
    <section id="engines" className="py-24 md:py-32">
      <div className="mx-auto max-w-6xl px-6">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="text-3xl font-bold text-white md:text-4xl">{em.title}</h2>
          <p className="mt-4 text-lg text-slate-400">{em.subtitle}</p>
        </div>

        {/* Desktop table */}
        <div className="mt-16 hidden md:block">
          <div className="terminal-glow overflow-hidden rounded-xl border border-surface-3">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-surface-3 bg-surface-1">
                  {[em.engine, em.type, em.mac, em.gpu, em.streaming, em.install].map((h) => (
                    <th key={h} className="px-6 py-4 text-left font-mono text-xs font-medium uppercase tracking-wider text-slate-500">
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {engines.map((e, i) => (
                  <tr
                    key={e.name}
                    className={`border-b border-surface-3/50 ${e.recommended ? 'bg-claw-500/5' : i % 2 === 0 ? 'bg-surface-0' : 'bg-surface-1/30'}`}
                  >
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <span className="font-mono font-semibold text-white">{e.name}</span>
                        {e.recommended && <Badge variant="blue">{em.recommended}</Badge>}
                      </div>
                    </td>
                    <td className="px-6 py-4 text-slate-400">{em[e.type]}</td>
                    <td className="px-6 py-4">
                      {e.mac ? (
                        <span className="text-green-400">&#10003;</span>
                      ) : (
                        <span className="text-slate-600">&#10005;</span>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      {e.gpu === true ? (
                        <span className="text-green-400">&#10003;</span>
                      ) : e.gpu === false ? (
                        <span className="text-slate-500">Accelerate</span>
                      ) : (
                        <span className="text-slate-600">&mdash;</span>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      {e.streaming === 'native' ? (
                        <Badge variant="green">{em.native}</Badge>
                      ) : e.streaming === 'vad' ? (
                        <Badge variant="amber">{em.vad}</Badge>
                      ) : (
                        <span className="text-slate-600">&mdash;</span>
                      )}
                    </td>
                    <td className="px-6 py-4 font-mono text-xs text-slate-400">{e.install}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* Mobile cards */}
        <div className="mt-16 grid gap-4 md:hidden">
          {engines.map((e) => (
            <div
              key={e.name}
              className={`rounded-xl border p-5 ${e.recommended ? 'border-claw-500/30 bg-claw-500/5' : 'border-surface-3 bg-surface-1/50'}`}
            >
              <div className="flex items-center gap-2 mb-3">
                <span className="font-mono font-semibold text-white">{e.name}</span>
                {e.recommended && <Badge variant="blue">{em.recommended}</Badge>}
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs">
                <div className="text-slate-500">{em.type}</div>
                <div className="text-slate-300">{em[e.type]}</div>
                <div className="text-slate-500">{em.streaming}</div>
                <div>
                  {e.streaming === 'native' ? (
                    <Badge variant="green">{em.native}</Badge>
                  ) : e.streaming === 'vad' ? (
                    <Badge variant="amber">{em.vad}</Badge>
                  ) : (
                    <span className="text-slate-600">&mdash;</span>
                  )}
                </div>
                <div className="text-slate-500">{em.install}</div>
                <div className="font-mono text-slate-400">{e.install}</div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
