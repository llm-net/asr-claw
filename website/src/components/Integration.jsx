function CodeBlock({ children }) {
  return (
    <div className="code-block">
      <pre className="text-slate-300">{children}</pre>
    </div>
  )
}

export default function Integration({ t }) {
  return (
    <section id="integration" className="py-24 md:py-32">
      <div className="mx-auto max-w-6xl px-6">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="text-3xl font-bold text-white md:text-4xl">{t.integration.title}</h2>
          <p className="mt-4 text-lg text-slate-400">{t.integration.subtitle}</p>
        </div>

        <div className="mt-16 grid gap-8 lg:grid-cols-2">
          {/* adb-claw integration */}
          <div className="rounded-xl border border-surface-3 bg-surface-1/50 p-6">
            <div className="mb-1 font-mono text-xs font-medium uppercase tracking-wider text-claw-400">
              01
            </div>
            <h3 className="mb-2 text-lg font-semibold text-white">{t.integration.with_adb}</h3>
            <p className="mb-4 text-sm text-slate-400">{t.integration.with_adb_desc}</p>
            <CodeBlock>{`# Real-time transcription from Android
adb-claw audio capture --stream --duration 0 \\
  | asr-claw transcribe --stream --lang zh

# Record then transcribe
adb-claw audio capture --duration 30000 --file rec.wav
asr-claw transcribe --file rec.wav`}</CodeBlock>
          </div>

          {/* ffmpeg integration */}
          <div className="rounded-xl border border-surface-3 bg-surface-1/50 p-6">
            <div className="mb-1 font-mono text-xs font-medium uppercase tracking-wider text-claw-400">
              02
            </div>
            <h3 className="mb-2 text-lg font-semibold text-white">{t.integration.with_ffmpeg}</h3>
            <p className="mb-4 text-sm text-slate-400">{t.integration.with_ffmpeg_desc}</p>
            <CodeBlock>{`# Convert any format and transcribe
ffmpeg -i video.mp4 -ar 16000 -ac 1 \\
  -f wav pipe:1 | asr-claw transcribe

# From URL
ffmpeg -i https://example.com/stream.m3u8 \\
  -ar 16000 -ac 1 -f wav pipe:1 \\
  | asr-claw transcribe --stream`}</CodeBlock>
          </div>

          {/* Agent integration */}
          <div className="rounded-xl border border-surface-3 bg-surface-1/50 p-6">
            <div className="mb-1 font-mono text-xs font-medium uppercase tracking-wider text-claw-400">
              03
            </div>
            <h3 className="mb-2 text-lg font-semibold text-white">{t.integration.with_agent}</h3>
            <p className="mb-4 text-sm text-slate-400">{t.integration.with_agent_desc}</p>
            <CodeBlock>{`# In your agent's tool call
result=$(asr-claw transcribe --file audio.wav)

# Parse with jq
text=$(echo "$result" | jq -r '.data.full_text')
ok=$(echo "$result" | jq -r '.ok')

# Act on the result
if [ "$ok" = "true" ]; then
  echo "Heard: $text"
fi`}</CodeBlock>
          </div>

          {/* Output format */}
          <div className="rounded-xl border border-surface-3 bg-surface-1/50 p-6">
            <div className="mb-1 font-mono text-xs font-medium uppercase tracking-wider text-claw-400">
              04
            </div>
            <h3 className="mb-2 text-lg font-semibold text-white">{t.integration.output_title}</h3>
            <p className="mb-4 text-sm text-slate-400">{t.integration.output_desc}</p>
            <CodeBlock>{`{
  "ok": true,
  "command": "transcribe",
  "data": {
    "segments": [
      {"index": 0, "start": 0.0,
       "end": 2.5, "text": "..."}
    ],
    "full_text": "...",
    "engine": "qwen-asr",
    "audio_duration_sec": 5.5
  },
  "duration_ms": 1230,
  "timestamp": "2026-03-13T10:00:00Z"
}`}</CodeBlock>
          </div>
        </div>
      </div>
    </section>
  )
}
