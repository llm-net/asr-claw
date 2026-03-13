import { useState } from 'react'

export default function Header({ t, lang, toggleLang }) {
  const [menuOpen, setMenuOpen] = useState(false)

  return (
    <header className="fixed top-0 left-0 right-0 z-50 border-b border-white/5 bg-surface-0/80 backdrop-blur-xl">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
        <a href="#" className="flex items-center gap-3 group">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-claw-500/10 border border-claw-500/20 font-mono text-sm font-bold text-claw-400 group-hover:bg-claw-500/20 transition-colors">
            &gt;_
          </div>
          <span className="font-mono text-lg font-semibold text-white">
            asr-claw
          </span>
        </a>

        <nav className="hidden md:flex items-center gap-8">
          {[
            ['#features', t.nav.features],
            ['#engines', t.nav.engines],
            ['#integration', t.nav.integration],
            ['#install', t.nav.install],
          ].map(([href, label]) => (
            <a
              key={href}
              href={href}
              className="text-sm text-slate-400 hover:text-claw-400 transition-colors"
            >
              {label}
            </a>
          ))}
        </nav>

        <div className="flex items-center gap-4">
          <button
            onClick={toggleLang}
            className="rounded-md border border-surface-3 px-3 py-1.5 font-mono text-xs text-slate-400 hover:border-claw-500/50 hover:text-claw-400 transition-colors"
          >
            {t.meta.lang_switch}
          </button>
          <a
            href="https://github.com/llm-net/asr-claw"
            target="_blank"
            rel="noopener noreferrer"
            className="hidden sm:flex items-center gap-2 rounded-lg bg-surface-2 px-4 py-2 text-sm text-slate-300 hover:bg-surface-3 transition-colors"
          >
            <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
            </svg>
            GitHub
          </a>
          <button
            className="md:hidden text-slate-400"
            onClick={() => setMenuOpen(!menuOpen)}
          >
            <svg className="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              {menuOpen ? (
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              ) : (
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
              )}
            </svg>
          </button>
        </div>
      </div>

      {menuOpen && (
        <div className="md:hidden border-t border-white/5 bg-surface-0/95 backdrop-blur-xl px-6 py-4">
          {[
            ['#features', t.nav.features],
            ['#engines', t.nav.engines],
            ['#integration', t.nav.integration],
            ['#install', t.nav.install],
          ].map(([href, label]) => (
            <a
              key={href}
              href={href}
              onClick={() => setMenuOpen(false)}
              className="block py-2 text-sm text-slate-400 hover:text-claw-400 transition-colors"
            >
              {label}
            </a>
          ))}
        </div>
      )}
    </header>
  )
}
