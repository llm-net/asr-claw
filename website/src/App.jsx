import { useState } from 'react'
import translations from './i18n'
import Header from './components/Header'
import Hero from './components/Hero'
import Features from './components/Features'
import EngineMatrix from './components/EngineMatrix'
import Integration from './components/Integration'
import Install from './components/Install'
import Footer from './components/Footer'

export default function App() {
  const [lang, setLang] = useState('en')
  const t = translations[lang]

  const toggleLang = () => setLang(lang === 'en' ? 'zh' : 'en')

  return (
    <div className="min-h-screen grid-bg">
      <Header t={t} lang={lang} toggleLang={toggleLang} />
      <Hero t={t} />
      <Features t={t} />
      <EngineMatrix t={t} />
      <Integration t={t} />
      <Install t={t} />
      <Footer t={t} />
    </div>
  )
}
