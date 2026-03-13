import { useState } from 'react'

const tabs = [
  { key: 'claude_code', icon: 'C' },
  { key: 'openclaw', icon: 'O' },
  { key: 'binary', icon: 'B' },
]

const codeBlocks = {
  claude_code: {
    step1: 'claude plugin install llm-net/asr-claw',
    step2: 'asr-claw engines install qwen-asr',
    step3: 'cat audio.wav | asr-claw transcribe --lang zh',
  },
  openclaw: {
    step1: 'claw install dionren/asr-claw',
    step2: 'asr-claw engines install qwen-asr',
    step3: 'cat audio.wav | asr-claw transcribe --lang zh',
  },
  binary: {
    step1: `curl -fsSL https://github.com/llm-net/asr-claw/releases/latest/download/install.sh | bash`,
    step2: 'asr-claw engines install qwen-asr',
    step3: 'cat audio.wav | asr-claw transcribe --lang zh',
  },
}

function CopyButton({ text }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <button
      onClick={handleCopy}
      className="absolute top-3 right-3 rounded-md bg-surface-3/50 px-2 py-1 text-xs text-slate-500 hover:bg-surface-3 hover:text-slate-300 transition-colors"
    >
      {copied ? 'Copied!' : 'Copy'}
    </button>
  )
}

export default function Install({ t }) {
  const [activeTab, setActiveTab] = useState('claude_code')
  const code = codeBlocks[activeTab]

  return (
    <section id="install" className="py-24 md:py-32">
      <div className="mx-auto max-w-6xl px-6">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="text-3xl font-bold text-white md:text-4xl">{t.install.title}</h2>
          <p className="mt-4 text-lg text-slate-400">{t.install.subtitle}</p>
        </div>

        <div className="mx-auto mt-12 max-w-2xl">
          {/* Tabs */}
          <div className="flex gap-2 mb-6">
            {tabs.map(({ key, icon }) => (
              <button
                key={key}
                onClick={() => setActiveTab(key)}
                className={`flex items-center gap-2 rounded-lg px-4 py-2.5 text-sm font-medium transition-colors ${
                  activeTab === key
                    ? 'bg-claw-500/10 text-claw-400 border border-claw-500/30'
                    : 'bg-surface-1 text-slate-400 border border-surface-3 hover:border-surface-3/80 hover:text-slate-300'
                }`}
              >
                <span className="font-mono text-xs font-bold">{icon}</span>
                {t.install[key]}
              </button>
            ))}
          </div>

          {/* Install steps */}
          <div className="space-y-4">
            {['step1', 'step2', 'step3'].map((step) => (
              <div key={step} className="relative">
                <div className="code-block">
                  <pre className="pr-16">
                    <span className="text-slate-500">{t.install[step]}</span>
                    {'\n'}
                    <span className="text-claw-400">$ </span>
                    <span className="text-slate-200">{code[step]}</span>
                  </pre>
                </div>
                <CopyButton text={code[step]} />
              </div>
            ))}
          </div>

          {/* Machine-readable install info */}
          <div className="mt-8 rounded-xl border border-dashed border-surface-3 bg-surface-1/30 p-5">
            <div className="mb-3 flex items-center gap-2">
              <div className="h-2 w-2 rounded-full bg-claw-400" />
              <span className="font-mono text-xs font-medium uppercase tracking-wider text-slate-500">
                machine-readable
              </span>
            </div>
            <div className="font-mono text-xs leading-6 text-slate-500">
              <div><span className="text-slate-400">name:</span> asr-claw</div>
              <div><span className="text-slate-400">version:</span> 1.1.1</div>
              <div><span className="text-slate-400">binary:</span> asr-claw</div>
              <div><span className="text-slate-400">protocol:</span> stdin/stdout | json envelope</div>
              <div><span className="text-slate-400">platforms:</span> darwin/arm64, darwin/amd64, linux/arm64, linux/amd64</div>
              <div><span className="text-slate-400">license:</span> MIT</div>
              <div><span className="text-slate-400">repo:</span> github.com/llm-net/asr-claw</div>
              <div><span className="text-slate-400">clawhub:</span> clawhub.ai/dionren/asr-claw</div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
