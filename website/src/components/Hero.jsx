export default function Hero({ t }) {
  return (
    <section className="relative overflow-hidden pt-32 pb-20 md:pt-44 md:pb-32">
      {/* Background decoration */}
      <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[800px] h-[800px] rounded-full bg-claw-500/5 blur-3xl pointer-events-none" />
      <div className="absolute top-20 right-20 w-2 h-2 rounded-full bg-claw-400/60 float-anim" />
      <div className="absolute top-40 left-32 w-1.5 h-1.5 rounded-full bg-claw-300/40 float-anim" style={{ animationDelay: '1s' }} />
      <div className="absolute bottom-40 right-40 w-1 h-1 rounded-full bg-claw-500/50 float-anim" style={{ animationDelay: '2s' }} />

      <div className="relative mx-auto max-w-6xl px-6">
        <div className="mx-auto max-w-3xl text-center">
          {/* Badge */}
          <div className="mb-8 inline-flex items-center gap-2 rounded-full border border-claw-500/20 bg-claw-500/5 px-4 py-1.5">
            <span className="relative flex h-2 w-2">
              <span className="pulse-ring absolute inline-flex h-full w-full rounded-full bg-claw-400" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-claw-400" />
            </span>
            <span className="font-mono text-xs text-claw-300">{t.hero.badge}</span>
          </div>

          {/* Title */}
          <h1 className="text-5xl font-extrabold leading-tight tracking-tight text-white md:text-7xl">
            {t.hero.title_1}{' '}
            <span className="bg-gradient-to-r from-claw-400 to-claw-200 bg-clip-text text-transparent">
              {t.hero.title_2}
            </span>
          </h1>

          {/* Subtitle */}
          <p className="mt-6 text-lg leading-relaxed text-slate-400 md:text-xl">
            {t.hero.subtitle}
          </p>

          {/* CTA */}
          <div className="mt-10 flex flex-col items-center gap-4 sm:flex-row sm:justify-center">
            <a
              href="#install"
              className="inline-flex items-center gap-2 rounded-lg bg-claw-500 px-6 py-3 text-sm font-semibold text-white shadow-lg shadow-claw-500/25 hover:bg-claw-600 transition-colors"
            >
              <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
              </svg>
              {t.hero.cta_install}
            </a>
            <a
              href="https://github.com/llm-net/asr-claw"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 rounded-lg border border-surface-3 px-6 py-3 text-sm font-semibold text-slate-300 hover:border-claw-500/50 hover:text-claw-400 transition-colors"
            >
              {t.hero.cta_docs}
              <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 5l7 7m0 0l-7 7m7-7H3" />
              </svg>
            </a>
          </div>
        </div>

        {/* Terminal demo */}
        <div className="mx-auto mt-16 max-w-2xl">
          <div className="terminal-glow rounded-xl border border-surface-3 bg-surface-1 overflow-hidden">
            {/* Title bar */}
            <div className="flex items-center gap-2 border-b border-surface-3 px-4 py-3">
              <div className="h-3 w-3 rounded-full bg-red-500/70" />
              <div className="h-3 w-3 rounded-full bg-yellow-500/70" />
              <div className="h-3 w-3 rounded-full bg-green-500/70" />
              <span className="ml-3 font-mono text-xs text-slate-500">~/.agent/workspace</span>
            </div>
            {/* Terminal content */}
            <div className="p-5 font-mono text-sm leading-7">
              <div className="text-slate-500">{t.hero.terminal_comment}</div>
              <div>
                <span className="text-claw-400">$</span>{' '}
                <span className="text-slate-200">asr-claw transcribe --file meeting.wav --lang zh</span>
              </div>
              <div className="mt-2 text-slate-500">{'{'}</div>
              <div className="text-slate-500">
                {'  '}<span className="text-green-400">"ok"</span>: <span className="text-amber-300">true</span>,
              </div>
              <div className="text-slate-500">
                {'  '}<span className="text-green-400">"data"</span>: {'{'}
              </div>
              <div className="text-slate-500">
                {'    '}<span className="text-green-400">"full_text"</span>: <span className="text-amber-200">"..."</span>,
              </div>
              <div className="text-slate-500">
                {'    '}<span className="text-green-400">"engine"</span>: <span className="text-amber-200">"qwen-asr"</span>,
              </div>
              <div className="text-slate-500">
                {'    '}<span className="text-green-400">"audio_duration_sec"</span>: <span className="text-purple-300">12.5</span>
              </div>
              <div className="text-slate-500">{'  }'}</div>
              <div className="text-slate-500">{'}'}</div>

              <div className="mt-4 text-slate-500">{t.hero.terminal_pipe}</div>
              <div>
                <span className="text-claw-400">$</span>{' '}
                <span className="text-slate-200">adb-claw audio capture --stream</span>
                <span className="text-slate-500"> | </span>
                <span className="text-slate-200">asr-claw transcribe --stream</span>
                <span className="cursor-blink text-claw-400 ml-0.5">_</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
