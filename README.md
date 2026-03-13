# asr-claw

Speech recognition CLI for AI agent automation. Transcribe audio from stdin, files, or URLs with multiple ASR engines — local and cloud.

## Install

### Claude Code

Search `asr-claw` in the plugin marketplace, or load locally:

```bash
claude --plugin-dir /path/to/asr-claw
```

The SessionStart hook automatically downloads the binary on first launch.

### OpenClaw

```bash
claw install dionren/asr-claw
```

### Manual

```bash
curl -fsSL https://github.com/llm-net/asr-claw/releases/latest/download/install.sh | bash
```

Or download a specific binary from [Releases](https://github.com/llm-net/asr-claw/releases).

## Quick Start

```bash
# 1. Install the recommended local engine (downloads pre-built binary + 0.6B model ~1.9GB)
asr-claw engines install qwen-asr

# 2. Verify
asr-claw doctor

# 3. Transcribe
asr-claw transcribe --file meeting.wav --lang zh
cat audio.wav | asr-claw transcribe --lang en
```

## Usage

### Transcribe

```bash
# File input
asr-claw transcribe --file audio.wav --lang zh

# Stdin pipe
cat audio.wav | asr-claw transcribe --lang en

# Streaming (real-time, from adb-claw or ffmpeg)
adb-claw audio capture --stream --duration 60000 | asr-claw transcribe --stream --lang zh

# Subtitle output
asr-claw transcribe --file lecture.wav --format srt > lecture.srt
asr-claw transcribe --file lecture.wav --format vtt > lecture.vtt

# Specify engine
asr-claw transcribe --file audio.wav --engine whisper --lang en
```

| Flag | Default | Description |
|------|---------|-------------|
| `--file <path>` | stdin | Input audio file |
| `--stream` | false | Streaming mode (real-time) |
| `--lang <code>` | zh | Language code |
| `--engine <name>` | auto | ASR engine |
| `--format <fmt>` | json | Output: json, text, srt, vtt |
| `--chunk <sec>` | 0 | Fixed-time chunking (disables VAD) |
| `--rate <hz>` | 16000 | Sample rate for raw PCM input |

### Engine Management

```bash
asr-claw engines list                  # List all engines + status
asr-claw engines install qwen-asr     # Install local engine
asr-claw engines info qwen-asr        # Engine details
asr-claw engines start qwen3-asr      # Start service engine
asr-claw engines stop qwen3-asr       # Stop service engine
asr-claw engines status                # Running engines
```

### Environment Check

```bash
asr-claw doctor
```

### Global Flags

```
-o, --output <format>  # json (default) | text | quiet
--timeout <ms>         # Command timeout (default 60000)
--verbose              # Debug output to stderr
```

## Engines

| Engine | Type | Platform | Streaming | Setup |
|--------|------|----------|-----------|-------|
| **qwen-asr** | Local | macOS, Linux | VAD | `engines install qwen-asr` |
| **qwen3-asr** | vLLM Service | GPU (CUDA) | Native | `engines start qwen3-asr` |
| **whisper** | Local | macOS, Linux | VAD | Manual |
| **doubao** | Cloud API | Any | No | `export DOUBAO_API_KEY=...` |
| **openai** | Cloud API | Any | No | `export OPENAI_API_KEY=...` |
| **deepgram** | Cloud API | Any | Native | `export DEEPGRAM_API_KEY=...` |

**qwen-asr** (recommended for Mac) — Pre-built [antirez/qwen-asr](https://github.com/antirez/qwen-asr) binary with Qwen3-ASR-0.6B model. Uses Apple Accelerate framework, no GPU required.

## Output Format

All commands output a JSON envelope to stdout:

```json
{
  "ok": true,
  "command": "transcribe",
  "data": {
    "segments": [{"index": 0, "start": 0.0, "end": 2.5, "text": "..."}],
    "full_text": "...",
    "engine": "qwen-asr",
    "audio_duration_sec": 5.5
  },
  "duration_ms": 1230,
  "timestamp": "2026-03-13T10:00:00Z"
}
```

Use `-o text` for plain text, `-o quiet` for silent mode.

## With adb-claw

asr-claw pairs with [adb-claw](https://github.com/llm-net/adb-claw) via Unix pipe:

```bash
# Real-time transcription from Android device
adb-claw audio capture --stream --duration 60000 | asr-claw transcribe --stream --lang zh

# Record then transcribe
adb-claw audio capture --duration 30000 --file recording.wav
asr-claw transcribe --file recording.wav --lang zh

# Save audio + transcribe simultaneously
adb-claw audio capture --stream --duration 0 | tee backup.wav | asr-claw transcribe --stream
```

Stream protocol: 44-byte WAV header + continuous raw PCM (16kHz, mono, 16-bit).

## Configuration

Settings are stored in `~/.asr-claw/config.yaml`:

```yaml
default:
  engine: qwen-asr
  lang: zh
  format: json

engines:
  qwen-asr:
    binary: ~/.asr-claw/bin/qwen-asr
    model_path: ~/.asr-claw/models/Qwen3-ASR-0.6B
```

OpenClaw users can also configure via:

```bash
claw config set asr-claw.default_lang en
claw config set asr-claw.model Qwen/Qwen3-ASR-1.7B
claw config set asr-claw.hf_mirror https://hf-mirror.com    # China users
```

## License

MIT
