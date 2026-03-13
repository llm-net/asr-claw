export default function Footer({ t }) {
  return (
    <footer className="border-t border-surface-3 py-16">
      <div className="mx-auto max-w-6xl px-6">
        <div className="flex flex-col items-center gap-6 md:flex-row md:justify-between">
          <div className="text-center md:text-left">
            <div className="flex items-center justify-center gap-3 md:justify-start">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-claw-500/10 border border-claw-500/20 font-mono text-sm font-bold text-claw-400">
                &gt;_
              </div>
              <span className="font-mono text-lg font-semibold text-white">asr-claw</span>
            </div>
            <p className="mt-3 text-sm font-medium text-slate-300">{t.footer.built_for}</p>
            <p className="mt-1 max-w-md text-sm text-slate-500">{t.footer.tagline}</p>
          </div>

          <div className="flex items-center gap-6">
            <a
              href="https://github.com/llm-net/asr-claw"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-slate-400 hover:text-claw-400 transition-colors"
            >
              {t.footer.source}
            </a>
            <a
              href="https://github.com/llm-net/asr-claw/tree/main/docs"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-slate-400 hover:text-claw-400 transition-colors"
            >
              {t.footer.docs}
            </a>
            <a
              href="https://github.com/llm-net/asr-claw/issues"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-slate-400 hover:text-claw-400 transition-colors"
            >
              {t.footer.issues}
            </a>
          </div>
        </div>

        <div className="mt-10 border-t border-surface-3 pt-6 text-center">
          <p className="font-mono text-xs text-slate-600">
            {t.footer.license} &middot; asr-claw.llm.net
          </p>
        </div>
      </div>
    </footer>
  )
}
