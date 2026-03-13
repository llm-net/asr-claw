# asr-claw — 语音识别 CLI 设计文档

> 独立项目，与 adb-claw 通过 Unix pipe 协作。纯工具层，不含 LLM/Agent 逻辑。

---

## 0. 上游依赖状态

adb-claw `audio capture` 命令已实现（v1.5.0+），asr-claw 的上游数据源已就绪。

**adb-claw audio capture 实际接口**：

```bash
adb-claw audio capture                            # 流式 WAV 到 stdout（默认 10s）
adb-claw audio capture --duration 30000           # 流式 30s
adb-claw audio capture --duration 0               # 流式无限（直到 Ctrl+C 或 pipe 断开）
adb-claw audio capture --file recording.wav       # 存文件，返回 JSON envelope
adb-claw audio capture --rate 16000               # 自定义采样率（默认 16000）
adb-claw audio capture --stream                   # 显式标记流式（等价于默认行为）
```

关键特性：
- **默认即流式**：不加 `--file` 就输出 WAV 到 stdout，天然支持 pipe
- **默认 10 秒**：`--duration` 默认 10000ms，长时间采集需显式指定 `--duration 0`
- **设备静音**：采集期间 REMOTE_SUBMIX 会静音设备扬声器
- **Android 11+**：需要 SDK 30+，Go 侧预检版本并给出友好错误
- **信号处理**：`signal.NotifyContext` 处理 Ctrl+C 优雅退出，SIGPIPE 正常关闭
- **懒加载 DEX**：~7KB 音频采集 DEX 首次使用时推送，md5 校验避免重复推送

**主要使用场景**：抖音/直播间音频采集 + ASR 转写。adb-claw 的抖音 App Profile 已将 `asr-claw` 列为"推荐能力"（见 `skills/apps/douyin.md` → 推荐能力表）。

---

## 1. 定位与关系

### 1.1 两个项目的职责边界

| 项目 | 职责 | 类比 |
|------|------|------|
| **adb-claw** | Android 设备控制 CLI — 截屏、UI 树、输入、音频采集 | 摄像头 + 麦克风 |
| **asr-claw** | 语音识别 CLI — 音频 → 文本 | OCR / 语音转写引擎 |

两者各自独立：

- **独立仓库**，独立版本号，独立发布
- **独立 Skill**：各自作为 Claude Code Skill + OpenClaw Skill 发布
- 通过标准 **Unix pipe** 协作，无任何代码依赖

### 1.2 职责分离原则

adb-claw 只负责设备音频 → 主机 PCM 流，不做 ASR。这与 screenshot 只输出图片、不做 OCR 是同一设计原则：

```
截屏场景：  adb-claw screenshot → PNG 图片 → (Agent 调用视觉模型理解)
音频场景：  adb-claw audio capture → WAV 流 → asr-claw transcribe → 文本
```

工具层只做数据采集和格式转换，语义理解交给专门的工具或 Agent。

### 1.3 协作方式

```bash
# 实时采集 + 转写（主要场景：直播间语音监控）
adb-claw audio capture --stream --duration 60000 | asr-claw transcribe --stream --lang zh

# 录制后转写（离线场景）
adb-claw audio capture --duration 30000 --file recording.wav
asr-claw transcribe --file recording.wav --lang zh --engine whisper

# tee 同时保存和转写
adb-claw audio capture --stream --duration 0 | tee backup.wav | asr-claw transcribe --stream --lang zh
```

两个 CLI 通过 stdin/stdout 的 WAV 流连接，中间可以插入任意处理环节（tee、ffmpeg 等），完全符合 Unix 哲学。

**注意**：adb-claw 默认采集 10 秒。直播监控场景务必显式传 `--duration 60000` 或 `--duration 0`。

---

## 2. 架构

```
┌─────────────────────────────────────────────────────────────┐
│                        asr-claw                              │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────┐  │
│  │  Input Layer  │  │ Preprocessing│  │   ASR Engine      │  │
│  │              │→│              │→│                   │  │
│  │ • stdin      │  │ • WAV detect │  │ • qwen3-asr      │  │
│  │ • --file     │  │ • Resample   │  │ • qwen3-asr-rs   │  │
│  │ • --url      │  │ • VAD segment│  │ • whisper.cpp     │  │
│  │              │  │ • Silence det│  │ • doubao          │  │
│  └──────────────┘  └──────────────┘  │ • openai          │  │
│                                       │ • deepgram        │  │
│                                       └────────┬──────────┘  │
│                                                │              │
│                                       ┌────────▼──────────┐  │
│                                       │   Output Layer     │  │
│                                       │ • json (envelope)  │  │
│                                       │ • text (plain)     │  │
│                                       │ • stream (jsonl)   │  │
│                                       │ • srt / vtt        │  │
│                                       └───────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 2.1 Input Layer（输入层）

三种输入来源，统一转为 PCM 流交给后续处理：

| 来源 | 用法 | 说明 |
|------|------|------|
| **stdin** | `cat audio.wav \| asr-claw transcribe` | 默认模式，支持 pipe |
| **--file** | `asr-claw transcribe --file recording.wav` | 本地文件 |
| **--url** | `asr-claw transcribe --url https://...` | HTTP(S) 远程文件 |

stdin 是最重要的输入方式 — 它使 asr-claw 能与 adb-claw、ffmpeg 或任何能输出音频的程序组合。

### 2.2 Preprocessing（预处理层）

- **Format detection**：检测 44 字节 WAV header，判断采样率/位深/声道数；无 header 则按 `--rate`/`--bits` 参数当作 raw PCM
- **Resample**：如果输入采样率与引擎要求不匹配，自动重采样（如 48kHz → 16kHz）
- **VAD + 智能切分**：基于能量检测的语音活动检测（Voice Activity Detection），按语句边界切分音频段（详见第 6 节）
- **Silence detection**：RMS 能量检测，识别静音段

### 2.3 ASR Engine Layer（引擎层）

可插拔引擎架构，两种类型：

| 引擎 | 类型 | 连接方式 | 说明 |
|------|------|---------|------|
| **qwen3-asr** | Service | HTTP/WebSocket → vLLM | 原生流式，需 GPU |
| **qwen3-asr-rs** | CLI | 子进程 | Rust 本地推理 |
| **whisper.cpp** | CLI | 子进程 | C++ 本地推理 |
| **doubao** | Cloud API | HTTPS | 火山引擎 Seed-ASR 2.0 |
| **openai** | Cloud API | HTTPS | OpenAI Whisper API |
| **deepgram** | Cloud API | WebSocket | 原生流式 |

### 2.4 Output Layer（输出层）

| 格式 | Flag | 说明 |
|------|------|------|
| **json** | `--format json`（默认） | JSON envelope，与 adb-claw 风格一致 |
| **text** | `--format text` | 纯文本，一段一行 |
| **stream** | `--stream` + `--format json` | JSON Lines，每段一个 JSON 对象 |
| **srt** | `--format srt` | SubRip 字幕格式 |
| **vtt** | `--format vtt` | WebVTT 字幕格式 |

---

## 3. 命令树

```
asr-claw
├── transcribe [--file <path> | stdin]     # 语音转文字（核心命令）
│   --lang zh                              # 语言（默认 zh）
│   --engine qwen3-asr-rs                  # 引擎（默认按优先级自动选择）
│   --stream                               # 流式模式（边听边转）
│   --chunk 3                              # 退化方案：固定分段秒数
│   --format text|json|srt|vtt             # 输出格式
│   --rate 16000                           # 指定输入采样率（raw PCM 时必需）
│   --bits 16                              # 指定输入位深（raw PCM 时必需）
│
├── engines                                # 引擎管理
│   ├── list                               # 列出所有引擎及状态
│   ├── install <engine>                   # 下载引擎模型/二进制
│   ├── start <engine>                     # 启动 service 类型引擎
│   ├── stop <engine>                      # 停止 service 类型引擎
│   ├── status                             # 查看运行中的引擎
│   └── info <engine>                      # 引擎详细信息
│
├── doctor                                 # 环境检查
└── skill                                  # 输出 skill.json
```

### 全局 Flags

```
-o, --output <format>     # json（默认）| text | quiet
--timeout <ms>            # 命令超时（默认 60000）
--verbose                 # 调试输出到 stderr
```

### 用法示例

```bash
# 基本用法：转写本地文件
asr-claw transcribe --file meeting.wav --lang zh

# Pipe 模式：从 adb-claw 实时转写
adb-claw audio capture --stream | asr-claw transcribe --stream --lang zh

# 指定引擎
asr-claw transcribe --file lecture.wav --engine whisper --lang en

# 生成字幕文件
asr-claw transcribe --file video-audio.wav --format srt > subtitles.srt

# 云端引擎
asr-claw transcribe --file call.wav --engine doubao --lang zh
```

---

## 4. 引擎接口设计

### 4.1 核心接口

```go
// Engine 是所有 ASR 引擎的基础接口
// CLI 类型引擎（whisper.cpp、qwen3-asr-rs）只需实现此接口
type Engine interface {
    // Info 返回引擎能力描述
    Info() Capability

    // TranscribeFile 转写单个音频文件
    // path: WAV 文件路径（16kHz, 16bit, mono）
    // lang: 语言代码（zh, en, ja 等）
    TranscribeFile(path string, lang string) ([]Segment, error)
}

// StreamEngine 扩展 Engine，支持原生流式转写
// Service 类型引擎（qwen3-asr vLLM、deepgram）实现此接口
type StreamEngine interface {
    Engine

    // StreamSession 创建一个流式会话
    StreamSession(opts Options) (Session, error)
}

// Session 表示一个活跃的流式转写会话
type Session interface {
    // Feed 发送一块 PCM 数据，返回当前已识别的文本（可能含未确认部分）
    Feed(pcm []byte) (text string, err error)

    // Finish 结束流式输入，返回最终确认的文本
    Finish() (text string, err error)

    // Close 释放会话资源
    Close()
}
```

### 4.2 数据结构

```go
// Capability 描述引擎的能力和要求
type Capability struct {
    Name           string   `json:"name"`             // 引擎名称，如 "qwen3-asr"
    Type           string   `json:"type"`             // "cli" | "service" | "cloud"
    Languages      []string `json:"languages"`        // 支持的语言列表
    NeedsModel     bool     `json:"needs_model"`      // 是否需要下载模型
    NeedsAPIKey    bool     `json:"needs_api_key"`    // 是否需要 API Key
    NativeStream   bool     `json:"native_stream"`    // 是否支持原生流式
    Connection     string   `json:"connection"`       // "subprocess" | "http" | "websocket"
    SampleRate     int      `json:"sample_rate"`      // 要求的采样率（通常 16000）
    Installed      bool     `json:"installed"`        // 是否已安装/可用
}

// Segment 表示一段识别结果
type Segment struct {
    Index    int     `json:"index"`              // 段序号（从 0 开始）
    Start    float64 `json:"start"`              // 开始时间（秒）
    End      float64 `json:"end"`                // 结束时间（秒）
    Text     string  `json:"text"`               // 识别文本
    Lang     string  `json:"lang,omitempty"`     // 检测到的语言
    Confidence float64 `json:"confidence,omitempty"` // 置信度（0-1，部分引擎支持）
}

// Options 流式会话选项
type Options struct {
    Lang       string `json:"lang"`        // 语言
    SampleRate int    `json:"sample_rate"` // 采样率
    Channels   int    `json:"channels"`    // 声道数
    BitsPerSample int `json:"bits"`       // 位深
}
```

### 4.3 引擎注册

```go
// registry.go

var engines = map[string]func() Engine{}

func Register(name string, factory func() Engine) {
    engines[name] = factory
}

func Get(name string) (Engine, error) {
    factory, ok := engines[name]
    if !ok {
        return nil, fmt.Errorf("unknown engine: %s", name)
    }
    return factory(), nil
}

func List() []Capability {
    var caps []Capability
    for _, factory := range engines {
        e := factory()
        caps = append(caps, e.Info())
    }
    return caps
}

// 引擎自注册（各引擎 init() 中调用）
func init() {
    Register("qwen3-asr", NewQwen3ASR)
    Register("qwen3-asr-rs", NewQwen3ASRRS)
    Register("whisper", NewWhisper)
    Register("doubao", NewDoubao)
    Register("openai", NewOpenAI)
    Register("deepgram", NewDeepgram)
}
```

---

## 5. 引擎能力矩阵

| 引擎 | 类型 | 需要模型 | 需要 API Key | 原生流式 | 连接方式 | 语言支持 | 备注 |
|------|------|---------|-------------|---------|---------|---------|------|
| **qwen3-asr** | Service (本地) | Yes (~2GB) | No | **Yes** | HTTP/WebSocket → vLLM | 多语言 | 需 GPU，vLLM 后端 |
| **qwen3-asr-rs** | CLI (本地) | Yes (~2GB) | No | No | 子进程 | 多语言 | Rust 实现，CPU 可用 |
| **whisper.cpp** | CLI (本地) | Yes (~1.5GB) | No | No | 子进程 | 多语言 | C++ 实现，CPU 可用 |
| **doubao** | Cloud API | No | **Yes** | No | HTTPS | 中/英/日/韩 | 火山引擎 Seed-ASR 2.0 |
| **openai** | Cloud API | No | **Yes** | No | HTTPS | 多语言 | OpenAI Whisper API |
| **deepgram** | Cloud API | No | **Yes** | **Yes** | WebSocket | 多语言 | 原生流式，低延迟 |

**引擎选择优先级**（未指定 `--engine` 时自动选择）：

1. 已启动的 service 引擎（qwen3-asr）
2. 已安装的 CLI 引擎（qwen3-asr-rs > whisper）
3. 已配置的 cloud 引擎（doubao > openai > deepgram）

---

## 6. VAD + 语音段切分（核心）

这是 asr-claw 最关键的模块。音频流式输入是连续的，但 ASR 引擎（CLI 类型）需要完整的音频段才能识别。如何切分直接决定识别质量。

### 6.1 为什么固定时间切分是错误的

固定 N 秒切分（如每 3 秒切一次）会在单词/句子中间切断：

```
原始语音：  "今天天气真不错，我们|去公园散步吧"
                               ↑
                          3秒切分点

段1：  "今天天气真不错，我们"     → 识别："今天天气真不错我们"（缺尾巴）
段2：  "去公园散步吧"            → 识别："去公园散步吧"（缺开头语境）
```

问题：
- **切断语句**：在语义单元中间切开，丢失上下文
- **切断词语**：尤其中文，字间无空格，任何位置都可能切断词语
- **引擎困惑**：半句话送给引擎，识别准确率显著下降
- **时间戳不准**：固定切分的时间戳与实际语句不对应

### 6.2 正确方案：VAD 智能切分

基于语音活动检测（Voice Activity Detection），在自然停顿处切分：

```
原始语音：  "今天天气真不错。[停顿500ms] 我们去公园散步吧。[停顿800ms] ..."
                            ↑                              ↑
                       自然切分点                       自然切分点

段1：  "今天天气真不错。"       → 完整句子，识别准确
段2：  "我们去公园散步吧。"     → 完整句子，识别准确
```

### 6.3 状态机

```
                     speech detected (RMS > threshold)
           ┌─────────────────────────────────────────────┐
           │                                             ▼
       ┌───────┐                                  ┌──────────┐
       │ IDLE  │                                  │ SPEAKING │
       └───────┘                                  └────┬─────┘
           ▲                                           │
           │                         silence > silenceDuration
           │                                           │
           │                                           ▼
           │                                  ┌───────────────┐
           │         segment too short        │ SEGMENT_READY │
           │◄─────── (< minDuration) ─────────┤               │
           │                                  └───────┬───────┘
           │                                          │
           │         segment valid                    │
           │◄──── (emit segment, reset) ──────────────┘
```

状态流转：
1. **IDLE**：等待语音开始。持续监测 RMS 能量，低于阈值时保持 IDLE
2. **SPEAKING**：检测到语音。记录 PCM 数据，持续累积
3. **SEGMENT_READY**：检测到足够长的静音，表示一句话说完。将累积的 PCM 作为一个完整段输出

### 6.4 关键参数

```go
const (
    // silenceThresholdRMS 低于此值判定为静音
    // 16-bit PCM 归一化到 [-1, 1] 后的 RMS 值
    silenceThresholdRMS = 0.01

    // silenceDuration 连续静音达到此时长则认为是句子边界
    silenceDuration = 500 * time.Millisecond

    // maxSegmentDuration 单段最大时长（安全阀）
    // 持续说话不停顿时强制切分，防止内存无限增长
    maxSegmentDuration = 15 * time.Second

    // minSegmentDuration 最短有效段时长
    // 低于此值的段丢弃（滤除咳嗽、呼吸、点击声等）
    minSegmentDuration = 300 * time.Millisecond

    // frameSize 每次读取的帧大小
    // 20ms @ 16kHz × 2 bytes = 640 bytes
    frameSize = 640
)
```

### 6.5 VADSegmenter 实现

```go
// vad.go

type VADState int

const (
    StateIdle         VADState = iota
    StateSpeaking
    StateSegmentReady
)

type VADSegmenter struct {
    state           VADState
    sampleRate      int
    silenceThreshold float64
    silenceDuration time.Duration
    maxDuration     time.Duration
    minDuration     time.Duration

    buffer          []byte        // 当前段的 PCM 累积
    speechStart     time.Time     // 当前段开始时间
    silenceStart    time.Time     // 静音开始时间
    totalOffset     time.Duration // 全局时间偏移
}

func NewVADSegmenter(sampleRate int) *VADSegmenter {
    return &VADSegmenter{
        state:            StateIdle,
        sampleRate:       sampleRate,
        silenceThreshold: silenceThresholdRMS,
        silenceDuration:  500 * time.Millisecond,
        maxDuration:      15 * time.Second,
        minDuration:      300 * time.Millisecond,
    }
}

// AudioSegment 表示切分出的一个完整语音段
type AudioSegment struct {
    PCM       []byte        // 原始 PCM 数据
    Start     time.Duration // 在整体音频中的起始时间
    End       time.Duration // 在整体音频中的结束时间
    Duration  time.Duration // 段时长
}

// Feed 输入一帧 PCM 数据（20ms），返回完整的语音段或 nil
// 调用者持续喂帧，当一句话说完时收到 AudioSegment
func (v *VADSegmenter) Feed(frame []byte) *AudioSegment {
    rms := calcRMS(frame)
    isSpeech := rms > v.silenceThreshold
    now := time.Now()

    switch v.state {
    case StateIdle:
        if isSpeech {
            // 检测到语音开始
            v.state = StateSpeaking
            v.buffer = make([]byte, 0, v.sampleRate*2*15) // 预分配 15s
            v.buffer = append(v.buffer, frame...)
            v.speechStart = now
            v.silenceStart = time.Time{}
        }

    case StateSpeaking:
        v.buffer = append(v.buffer, frame...)

        // 检查是否超过最大段时长（安全阀）
        elapsed := now.Sub(v.speechStart)
        if elapsed >= v.maxDuration {
            return v.flushAtBestCutPoint()
        }

        if !isSpeech {
            // 进入静音
            if v.silenceStart.IsZero() {
                v.silenceStart = now
            }
            // 静音持续足够长 → 句子边界
            if now.Sub(v.silenceStart) >= v.silenceDuration {
                return v.flushSegment()
            }
        } else {
            // 语音恢复，重置静音计时
            v.silenceStart = time.Time{}
        }
    }

    return nil
}

// flushSegment 输出当前累积的段并重置状态
func (v *VADSegmenter) flushSegment() *AudioSegment {
    duration := pcmDuration(len(v.buffer), v.sampleRate)

    // 过短的段丢弃（噪声/呼吸/点击）
    if duration < v.minDuration {
        v.reset()
        return nil
    }

    seg := &AudioSegment{
        PCM:      v.buffer,
        Start:    v.totalOffset,
        End:      v.totalOffset + duration,
        Duration: duration,
    }
    v.totalOffset += duration
    v.reset()
    return seg
}

// reset 重置状态机到 IDLE
func (v *VADSegmenter) reset() {
    v.state = StateIdle
    v.buffer = nil
    v.speechStart = time.Time{}
    v.silenceStart = time.Time{}
}

// calcRMS 计算一帧 16-bit PCM 的 RMS 能量（归一化到 0~1）
func calcRMS(frame []byte) float64 {
    samples := len(frame) / 2
    if samples == 0 {
        return 0
    }
    var sumSq float64
    for i := 0; i < len(frame)-1; i += 2 {
        sample := int16(frame[i]) | int16(frame[i+1])<<8 // little-endian
        normalized := float64(sample) / 32768.0
        sumSq += normalized * normalized
    }
    return math.Sqrt(sumSq / float64(samples))
}

// pcmDuration 根据 PCM 字节数和采样率计算时长
func pcmDuration(bytes int, sampleRate int) time.Duration {
    samples := bytes / 2 // 16-bit = 2 bytes per sample
    return time.Duration(float64(samples) / float64(sampleRate) * float64(time.Second))
}
```

### 6.6 最佳切分点搜索（flushAtBestCutPoint）

当持续说话超过 `maxSegmentDuration` 时，不能在任意位置粗暴切断。应向前扫描找到最安静的位置切分：

```go
// flushAtBestCutPoint 在最大时长触发时，回溯寻找最佳切分点
// 策略：扫描最后 2 秒的 PCM，找到最安静的 20ms 窗口，在那里切分
func (v *VADSegmenter) flushAtBestCutPoint() *AudioSegment {
    scanWindow := 2 * time.Second
    scanBytes := int(scanWindow.Seconds()) * v.sampleRate * 2 // 2 bytes per sample
    windowBytes := frameSize // 20ms 窗口

    if len(v.buffer) < scanBytes {
        // 数据不足 2 秒，直接切
        return v.flushSegment()
    }

    // 从 buffer 末尾向前扫描 2 秒
    searchStart := len(v.buffer) - scanBytes
    bestPos := len(v.buffer) // 默认切末尾
    bestRMS := math.MaxFloat64

    for pos := searchStart; pos+windowBytes <= len(v.buffer); pos += windowBytes {
        window := v.buffer[pos : pos+windowBytes]
        rms := calcRMS(window)
        if rms < bestRMS {
            bestRMS = rms
            bestPos = pos
        }
    }

    // 在最佳切分点分割
    segData := make([]byte, bestPos)
    copy(segData, v.buffer[:bestPos])
    remaining := make([]byte, len(v.buffer)-bestPos)
    copy(remaining, v.buffer[bestPos:])

    duration := pcmDuration(len(segData), v.sampleRate)

    seg := &AudioSegment{
        PCM:      segData,
        Start:    v.totalOffset,
        End:      v.totalOffset + duration,
        Duration: duration,
    }
    v.totalOffset += duration

    // 保留剩余部分，继续累积
    v.buffer = remaining
    v.speechStart = time.Now()
    v.silenceStart = time.Time{}
    // 状态保持 StateSpeaking

    return seg
}
```

### 6.7 切分方案对比

| | 固定时间切分 | VAD 智能切分 | 原生流式引擎 |
|---|---|---|---|
| **切分依据** | 固定 N 秒 | 语音静音边界 | 引擎内部模型 |
| **句子完整性** | 差 — 大概率切断句子 | 好 — 在停顿处切 | 最佳 — 模型自主判断 |
| **延迟** | 固定 N 秒 | 可变，取决于说话节奏 | 最低，增量输出 |
| **实现复杂度** | 最低 | 中等 | 需引擎支持 |
| **适用引擎** | 所有 | CLI 类型引擎 | qwen3-asr, deepgram |
| **asr-claw 中的角色** | `--chunk N` 退化方案 | **默认方案** | StreamEngine 接口 |

---

## 7. 流式数据协议

### 7.1 adb-claw → asr-claw 的 WAV 流

adb-claw `audio capture` 已按此协议实现。两个进程之间通过 Unix pipe 传输标准 WAV 流：

```
┌──────────────────────────────────────────┐
│            44-byte WAV Header            │
│  RIFF tag, format=PCM, rate=16000,       │
│  bits=16, channels=1,                    │
│  data_size=0x7FFFFFFF (streaming marker) │
├──────────────────────────────────────────┤
│                                          │
│        Continuous raw PCM bytes          │
│        (16-bit signed LE, mono)          │
│                                          │
│              ... 持续写入 ...             │
│                                          │
└──────────────────────────────────────────┘
```

### 7.2 WAV Header 结构

```
Offset  Size  Field            Value (streaming)
0       4     ChunkID          "RIFF"
4       4     ChunkSize        0x7FFFFFFF       ← 流式标记：未知总长度
8       4     Format           "WAVE"
12      4     Subchunk1ID      "fmt "
16      4     Subchunk1Size    16
20      2     AudioFormat      1 (PCM)
22      2     NumChannels      1 (mono)
24      4     SampleRate       16000
28      4     ByteRate         32000 (16000 × 1 × 2)
32      2     BlockAlign       2 (1 × 16/8)
34      2     BitsPerSample    16
36      4     Subchunk2ID      "data"
40      4     Subchunk2Size    0x7FFFFFFF       ← 流式标记：未知数据长度
44      ...   PCM data         连续原始 PCM 字节
```

### 7.3 为什么用 WAV 而不是自定义协议

| 方案 | 优点 | 缺点 |
|------|------|------|
| **WAV 流** | 自描述（采样率/位深/声道）、所有工具都能读、简单 | header 中长度字段需用 0x7FFFFFFF 表示流式 |
| 自定义 header | 可以更紧凑 | 非标准，调试困难，其他工具无法直接读 |
| Raw PCM | 最简单 | 缺少格式元数据，接收方必须事先知道参数 |
| 协议 buffer | 类型安全 | 过度设计，引入编译依赖 |

WAV 流是最佳平衡点：

- **自描述**：接收方读 44 字节 header 就知道所有格式参数，无需额外协商
- **通用**：`ffmpeg`、`sox`、`aplay` 等标准工具都能处理
- **简单**：就是一个 header + 连续 PCM 字节，实现代码不到 50 行
- **可调试**：pipe 到文件就是标准 WAV，任何播放器都能打开（修正 header 长度后）

### 7.4 asr-claw 的 WAV 检测

```go
// wav.go

type WAVHeader struct {
    SampleRate    int
    BitsPerSample int
    Channels      int
    IsStreaming   bool // data_size == 0x7FFFFFFF
}

// DetectWAV 尝试从 reader 读取 WAV header
// 成功返回 header 信息；非 WAV 数据返回 nil（reader 已消费的 44 字节需要回填）
func DetectWAV(r io.Reader) (*WAVHeader, error) {
    var buf [44]byte
    _, err := io.ReadFull(r, buf[:])
    if err != nil {
        return nil, err
    }

    // 检查 RIFF + WAVE 标记
    if string(buf[0:4]) != "RIFF" || string(buf[8:12]) != "WAVE" {
        return nil, nil // 不是 WAV，可能是 raw PCM
    }

    h := &WAVHeader{
        Channels:      int(binary.LittleEndian.Uint16(buf[22:24])),
        SampleRate:    int(binary.LittleEndian.Uint32(buf[24:28])),
        BitsPerSample: int(binary.LittleEndian.Uint16(buf[34:36])),
    }

    dataSize := binary.LittleEndian.Uint32(buf[40:44])
    h.IsStreaming = dataSize == 0x7FFFFFFF

    return h, nil
}
```

---

## 8. 流式 Pipeline 流程

### 8.1 CLI 引擎的流式 Pipeline

对于不支持原生流式的 CLI 引擎（whisper.cpp、qwen3-asr-rs），asr-claw 通过 VAD 切分实现"伪流式"：

```
stdin PCM    VADSegmenter         Temp WAV          Engine              Output
  │              │                   │                 │                  │
  │  20ms frame  │                   │                 │                  │
  ├─────────────►│                   │                 │                  │
  │              │ (accumulating...) │                 │                  │
  ├─────────────►│                   │                 │                  │
  │              │ (accumulating...) │                 │                  │
  ├─────────────►│                   │                 │                  │
  │              │ (silence > 500ms) │                 │                  │
  │              │                   │                 │                  │
  │              │  AudioSegment     │                 │                  │
  │              ├──────────────────►│                 │                  │
  │              │              write segment.wav      │                  │
  │              │                   ├────────────────►│                  │
  │              │                   │    TranscribeFile()                │
  │              │                   │                 ├─────────────────►│
  │              │                   │                 │  {"text": "..."}  │
  │              │              delete segment.wav     │                  │
  │              │                   │                 │                  │
```

```go
// transcribe.go

func runStreamTranscribe(r io.Reader, engine Engine, lang string, w *output.Writer) error {
    // 1. 检测 WAV header
    header, err := audio.DetectWAV(r)
    if err != nil {
        return fmt.Errorf("读取输入失败: %w", err)
    }

    sampleRate := 16000
    if header != nil {
        sampleRate = header.SampleRate
    }

    // 2. 初始化 VAD 切分器
    vad := audio.NewVADSegmenter(sampleRate)

    // 3. 按 20ms 帧读取
    frameBytes := sampleRate * 2 * 20 / 1000 // 20ms @ 16bit mono
    frame := make([]byte, frameBytes)
    segIndex := 0

    for {
        _, err := io.ReadFull(r, frame)
        if err == io.EOF || err == io.ErrUnexpectedEOF {
            // 流结束，处理最后残余
            if seg := vad.Flush(); seg != nil {
                if err := processSegment(seg, segIndex, engine, lang, w); err != nil {
                    w.WriteError("TRANSCRIBE_ERROR", err.Error(), "")
                }
            }
            break
        }
        if err != nil {
            return fmt.Errorf("读取 PCM 帧失败: %w", err)
        }

        // 4. 喂给 VAD
        seg := vad.Feed(frame)
        if seg == nil {
            continue // 还在累积，等待句子结束
        }

        // 5. 收到完整语音段，转写
        if err := processSegment(seg, segIndex, engine, lang, w); err != nil {
            w.WriteError("TRANSCRIBE_ERROR", err.Error(), "")
            continue
        }
        segIndex++
    }

    return nil
}

// processSegment 将语音段写成临时 WAV 文件，送引擎识别，输出结果
func processSegment(seg *audio.AudioSegment, index int, engine Engine, lang string, w *output.Writer) error {
    // 写临时 WAV 文件
    tmpFile, err := os.CreateTemp("", "asr-claw-*.wav")
    if err != nil {
        return err
    }
    defer os.Remove(tmpFile.Name())

    if err := audio.WriteWAV(tmpFile, seg.PCM, 16000); err != nil {
        tmpFile.Close()
        return err
    }
    tmpFile.Close()

    // 引擎转写
    segments, err := engine.TranscribeFile(tmpFile.Name(), lang)
    if err != nil {
        return err
    }

    // 输出结果（调整时间偏移）
    for i, s := range segments {
        s.Index = index
        s.Start += seg.Start.Seconds()
        s.End += seg.End.Seconds()
        segments[i] = s
    }

    w.WriteStreamSegment(segments)
    return nil
}
```

### 8.2 原生流式引擎的 Pipeline

对于支持原生流式的引擎（qwen3-asr via vLLM、deepgram），数据流更直接：

```
stdin PCM         asr-claw              Engine Service         Output
  │                  │                      │                    │
  │   20ms frame     │                      │                    │
  ├─────────────────►│                      │                    │
  │                  │  500ms chunk buffer  │                    │
  ├─────────────────►│                      │                    │
  │                  │  ···                 │                    │
  ├─────────────────►│                      │                    │
  │                  │  (buffer full)       │                    │
  │                  ├─────────────────────►│                    │
  │                  │  Feed(pcm) via WS    │                    │
  │                  │                      │                    │
  │                  │◄─────────────────────┤                    │
  │                  │  incremental text    ├───────────────────►│
  │                  │                      │  {"text": "今天"}   │
  │                  │                      │                    │
  ├─────────────────►│                      │                    │
  │                  ├─────────────────────►│                    │
  │                  │◄─────────────────────┤                    │
  │                  │  updated text        ├───────────────────►│
  │                  │                      │  {"text": "今天天气"}│
  │                  │                      │                    │
  │     EOF          │                      │                    │
  ├─────────────────►│                      │                    │
  │                  ├─────────────────────►│                    │
  │                  │  Finish()            │                    │
  │                  │◄─────────────────────┤                    │
  │                  │  final text          ├───────────────────►│
  │                  │                      │  {"text": "..."}   │
```

```go
func runNativeStreamTranscribe(r io.Reader, engine StreamEngine, lang string, w *output.Writer) error {
    header, err := audio.DetectWAV(r)
    if err != nil {
        return err
    }

    sampleRate := 16000
    if header != nil {
        sampleRate = header.SampleRate
    }

    // 创建流式会话
    session, err := engine.StreamSession(Options{
        Lang:       lang,
        SampleRate: sampleRate,
        Channels:   1,
        BitsPerSample: 16,
    })
    if err != nil {
        return fmt.Errorf("创建流式会话失败: %w", err)
    }
    defer session.Close()

    // 按 500ms 块读取并发送
    chunkBytes := sampleRate * 2 / 2 // 500ms @ 16bit mono
    buf := make([]byte, chunkBytes)

    for {
        n, err := io.ReadFull(r, buf)
        if n > 0 {
            text, ferr := session.Feed(buf[:n])
            if ferr != nil {
                return ferr
            }
            if text != "" {
                w.WriteStreamText(text)
            }
        }
        if err == io.EOF || err == io.ErrUnexpectedEOF {
            break
        }
        if err != nil {
            return err
        }
    }

    // 结束并获取最终文本
    finalText, err := session.Finish()
    if err != nil {
        return err
    }
    if finalText != "" {
        w.WriteStreamText(finalText)
    }

    return nil
}
```

---

## 9. 配置管理

### 9.1 目录结构

```
~/.asr-claw/
├── config.yaml          # 全局配置
├── models/              # 下载的模型文件
│   ├── qwen3-asr/       # Qwen3-ASR 模型
│   └── whisper/         # Whisper 模型
│       └── ggml-large-v3.bin
├── bin/                 # 下载的引擎二进制
│   ├── qwen3-asr-rs     # Rust 版 Qwen3-ASR
│   └── whisper-cpp      # whisper.cpp main 二进制
└── cache/               # 临时文件缓存
    └── segments/        # 转写过程中的临时 WAV 段
```

### 9.2 config.yaml 示例

```yaml
# ~/.asr-claw/config.yaml

# 默认设置
default:
  engine: qwen3-asr-rs    # 默认引擎
  lang: zh                # 默认语言
  format: json            # 默认输出格式

# 引擎配置
engines:
  # CLI 引擎 — 本地 Rust 推理
  qwen3-asr-rs:
    model_path: ~/.asr-claw/models/qwen3-asr
    # 或指定已有路径
    # model_path: /opt/models/qwen3-asr

  # CLI 引擎 — 本地 whisper.cpp
  whisper:
    binary: ~/.asr-claw/bin/whisper-cpp
    model: ~/.asr-claw/models/whisper/ggml-large-v3.bin
    threads: 4             # CPU 线程数

  # Service 引擎 — vLLM 后端
  qwen3-asr:
    endpoint: http://localhost:8000
    model_name: Qwen/Qwen3-ASR
    # GPU 配置由 vLLM 启动参数控制

  # Cloud 引擎 — 火山引擎
  doubao:
    api_key: ${DOUBAO_API_KEY}          # 支持环境变量引用
    app_id: your-app-id
    cluster: volcengine-asr-cluster

  # Cloud 引擎 — OpenAI
  openai:
    api_key: ${OPENAI_API_KEY}
    model: whisper-1
    # base_url: https://api.openai.com/v1  # 可选，自定义端点

  # Cloud 引擎 — Deepgram
  deepgram:
    api_key: ${DEEPGRAM_API_KEY}
    model: nova-2
    tier: enhanced

# VAD 参数（通常不需要调整）
vad:
  silence_threshold: 0.01
  silence_duration_ms: 500
  max_segment_sec: 15
  min_segment_ms: 300
```

### 9.3 API Key 优先级

API Key 的读取按以下优先级，高优先级覆盖低优先级：

1. **环境变量**（最高）：`DOUBAO_API_KEY`、`OPENAI_API_KEY`、`DEEPGRAM_API_KEY`
2. **config.yaml**：配置文件中的 `api_key` 字段（支持 `${ENV_VAR}` 语法引用环境变量）
3. **CLI flag**（退化方案）：`--api-key` flag（不推荐，会出现在进程列表中）

```bash
# 方式 1：环境变量（推荐）
export DOUBAO_API_KEY=your-key
asr-claw transcribe --file audio.wav --engine doubao

# 方式 2：config.yaml（见上方示例）

# 方式 3：CLI flag（不推荐）
asr-claw transcribe --file audio.wav --engine openai --api-key sk-xxx
```

---

## 10. 模型管理

### 10.1 引擎安装

```bash
# 安装 whisper.cpp 引擎（下载二进制 + 模型）
asr-claw engines install whisper

# 安装 qwen3-asr-rs（下载 Rust 二进制 + 模型）
asr-claw engines install qwen3-asr-rs

# 自定义模型大小
asr-claw engines install whisper --model large-v3
asr-claw engines install whisper --model base  # 更小更快
```

安装过程：
1. 检测当前平台（darwin-arm64 / linux-amd64 等）
2. 从 GitHub Releases 或镜像下载预编译二进制到 `~/.asr-claw/bin/`
3. 下载模型文件到 `~/.asr-claw/models/`
4. 验证完整性（SHA256 校验）
5. 自动更新 `config.yaml` 路径

### 10.2 引擎列表

```bash
$ asr-claw engines list
```

输出示例：

```json
{
  "ok": true,
  "command": "engines list",
  "data": {
    "engines": [
      {
        "name": "qwen3-asr-rs",
        "type": "cli",
        "installed": true,
        "version": "0.2.1",
        "model": "qwen3-asr (2.1 GB)",
        "native_stream": false
      },
      {
        "name": "whisper",
        "type": "cli",
        "installed": true,
        "version": "1.7.3",
        "model": "ggml-large-v3 (1.5 GB)",
        "native_stream": false
      },
      {
        "name": "qwen3-asr",
        "type": "service",
        "installed": false,
        "status": "not_configured",
        "native_stream": true,
        "note": "需要 GPU + vLLM，配置 endpoint 后可用"
      },
      {
        "name": "doubao",
        "type": "cloud",
        "installed": false,
        "status": "no_api_key",
        "native_stream": false,
        "note": "需要设置 DOUBAO_API_KEY"
      },
      {
        "name": "openai",
        "type": "cloud",
        "installed": false,
        "status": "no_api_key",
        "native_stream": false
      },
      {
        "name": "deepgram",
        "type": "cloud",
        "installed": false,
        "status": "no_api_key",
        "native_stream": true
      }
    ]
  },
  "duration_ms": 12,
  "timestamp": "2026-03-13T10:00:00Z"
}
```

### 10.3 Service 引擎生命周期

Service 类型引擎（如 qwen3-asr via vLLM）需要后台运行：

```bash
# 启动引擎服务
asr-claw engines start qwen3-asr
# → 启动 vLLM 进程，加载模型到 GPU
# → 记录 PID 到 ~/.asr-claw/run/qwen3-asr.pid
# → 等待 health check 通过

# 查看运行状态
asr-claw engines status
# → qwen3-asr: running (PID 12345, GPU 0, 4.2 GB VRAM, uptime 2h30m)

# 停止引擎服务
asr-claw engines stop qwen3-asr
# → 发送 SIGTERM，等待优雅退出

# 查看引擎详细信息
asr-claw engines info qwen3-asr
# → 能力、配置、模型路径、API 端点、资源占用
```

---

## 11. 与 adb-claw 的协作场景

### 11.1 实时采集 + 转写

最典型的场景 — 实时监听 Android 设备音频并转文字：

```bash
# 实时监听抖音直播间，转写主播语音（60 秒）
adb-claw audio capture --stream --duration 60000 | asr-claw transcribe --stream --lang zh

# 无限时长，直到 Ctrl+C
adb-claw audio capture --stream --duration 0 | asr-claw transcribe --stream --lang zh

# 指定引擎
adb-claw audio capture --stream --duration 0 | asr-claw transcribe --stream --engine whisper --lang zh
```

**注意**：adb-claw 默认只采集 10 秒。直播监控必须显式传 `--duration`。

### 11.2 tee 保存 + 同时转写

用 `tee` 同时保存原始音频和实时转写：

```bash
# 保存 WAV 文件的同时实时转写
adb-claw audio capture --stream --duration 0 | tee recording.wav | asr-claw transcribe --stream --lang zh
```

### 11.3 先录制，后转写

分步操作，适合长时间录制后批量处理：

```bash
# Step 1: 录制（JSON envelope 返回文件路径和字节数）
adb-claw audio capture --duration 60000 --file meeting.wav

# Step 2: 转写
asr-claw transcribe --file meeting.wav --lang zh --format srt > meeting.srt
```

### 11.4 Agent 工作流

AI Agent 通过两个 Skill 协作完成复杂任务。adb-claw 的抖音 App Profile 已将 asr-claw 列为"推荐能力"，Agent 会参考。

#### 场景 1：视频语音转文字（离线）

```
Agent 任务："帮我看看这个抖音视频在说什么"

1. Agent → adb-claw app current                              # 确认在抖音
2. Agent → 读取 douyin.md Profile → 看到推荐能力表 → 检查 asr-claw 是否可用
3. Agent → adb-claw tap {屏幕中心}                            # 暂停视频
4. Agent → 提醒用户"录音期间设备会静音"
5. Agent → adb-claw audio capture --duration 15000 --file /tmp/douyin.wav
6. Agent → asr-claw transcribe --file /tmp/douyin.wav --lang zh
7. Agent 获得文本内容，理解视频内容后回复用户
```

#### 场景 2：直播间语音 + 弹幕（实时双通道）

```
Agent 任务："帮我监控这个直播间在说什么和聊什么"

1. Agent → adb-claw open 'snssdk1128://live?room_id={id}'    # 打开直播间
2. Agent → adb-claw wait --text "说点什么" --timeout 10000     # 等待加载

# 并行启动两个采集通道：
3a. Agent → adb-claw audio capture --file live.wav --duration 60000    # 录制音频
3b. Agent → adb-claw monitor --stream --duration 60000                 # 采集弹幕

# 音频转写：
4. Agent → asr-claw transcribe --file live.wav --lang zh

# Agent 综合弹幕文本 + 主播语音，生成摘要
```

#### 场景 3：实时翻译（流式 pipe）

```
Agent 任务："实时翻译这个视频的内容"

1. Agent 启动 pipe（注意显式设置 duration）:
   adb-claw audio capture --stream --duration 60000 | asr-claw transcribe --stream --lang zh --format text

2. Agent 逐行读取 stdout，获得实时转写文本

3. Agent 对每段文本调用翻译：
   "今天给大家介绍一款好用的产品" → "Today I'll introduce a useful product"
```

#### Agent 前置检查清单

Agent 在启动音频采集前应检查：

1. **asr-claw 是否安装**：`which asr-claw` — 如果不可用，告知用户 `claw install asr-claw`
2. **Android 版本**：`adb-claw device info` → SDK >= 30（Android 11+）
3. **引擎是否可用**：`asr-claw engines list` — 至少一个引擎 installed=true
4. **提醒用户**：录音期间设备扬声器会静音

### 11.5 与 adb-claw monitor 的互补关系

`adb-claw monitor` 和 `adb-claw audio capture` + `asr-claw` 获取不同维度的信息：

| | adb-claw monitor | audio capture + asr-claw |
|---|---|---|
| **获取内容** | UI 上显示的文本（弹幕、标签、按钮） | 音频中的语音内容（主播说话、视频旁白） |
| **数据来源** | Accessibility 框架读取 UI 节点 | REMOTE_SUBMIX 采集系统混音 |
| **可靠性** | 只要 accessibility 连接正常就有数据 | 需要 Android 11+，且部分设备可能不支持 |
| **副作用** | 无 | 设备扬声器静音 |
| **适用场景** | 弹幕监控、UI 状态跟踪 | 语音转文字、内容理解 |

直播间场景建议两者结合：monitor 采集弹幕文本，audio capture + asr-claw 转写主播语音。

### 11.6 与其他音频源组合

asr-claw 不限于 adb-claw，任何能输出 WAV/PCM 的程序都能对接：

```bash
# 从麦克风实时转写（macOS）
ffmpeg -f avfoundation -i ":0" -ar 16000 -ac 1 -f wav pipe:1 | asr-claw transcribe --stream

# 从视频文件提取音频转写
ffmpeg -i video.mp4 -ar 16000 -ac 1 -f wav pipe:1 | asr-claw transcribe --stream

# 从网络流转写
curl -sN https://stream.example.com/audio | asr-claw transcribe --stream
```

---

## 12. 生命周期管理

### 12.1 Unix Pipe 信号处理

两个进程通过 pipe 连接时，需要正确处理各种终止场景：

```
adb-claw audio capture --stream  |  asr-claw transcribe --stream
         (writer)                          (reader)
         stdout ──────pipe──────── stdin
```

### 12.2 各场景处理

#### 正常结束（EOF）

```
adb-claw 停止采集 → 关闭 stdout → pipe 产生 EOF
                                    ↓
                        asr-claw 读到 EOF
                                    ↓
                        处理残余 buffer（VAD flush）
                                    ↓
                        输出最后结果，正常退出（exit 0）
```

asr-claw 代码处理：

```go
// io.ReadFull 返回 io.EOF 或 io.ErrUnexpectedEOF
if err == io.EOF || err == io.ErrUnexpectedEOF {
    // 处理最后残余
    if seg := vad.Flush(); seg != nil {
        processSegment(seg, ...)
    }
    break
}
```

#### 用户中断（Ctrl+C / SIGINT）

```
用户按 Ctrl+C
    ↓
Shell 向整个 pipeline 进程组发送 SIGINT
    ↓
adb-claw 收到 SIGINT → signal.NotifyContext 触发 context cancel
    → exec.CommandContext 终止 adb exec-out 子进程
    → Java 侧 shutdown hook 释放 AudioRecord
    → Go 侧 defer proc.Stop() 清理
    → 关闭 stdout → 退出
asr-claw 收到 SIGINT → 进入 graceful shutdown
    ↓
处理已接收的数据，输出结果，退出
```

asr-claw 的 graceful shutdown：

```go
func setupSignalHandler(cancel context.CancelFunc, vad *VADSegmenter, engine Engine) {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigCh
        // 通知主循环停止读取
        cancel()
        // 主循环会在 context.Done() 后 flush 残余并退出
    }()
}
```

#### 下游崩溃（asr-claw 意外退出）

```
asr-claw 崩溃 → pipe 读端关闭
    ↓
adb-claw 写 stdout 时收到 SIGPIPE
    ↓
adb-claw 默认行为：退出（exit 141）
```

adb-claw 已实现正确的 SIGPIPE 处理 — Java 侧 ADBClawAudio 捕获 "Broken pipe" 异常后释放 AudioRecord 资源并退出，Go 侧 `exec.CommandContext` 回收子进程。无需 asr-claw 做额外处理。

#### 上游崩溃（adb-claw 意外退出）

```
adb-claw 崩溃 → pipe 写端关闭 → asr-claw 读到 EOF
    ↓
与正常结束相同的逻辑：处理残余，输出结果，退出
```

由于 pipe 的 EOF 语义，上游崩溃和正常结束对下游来说是一样的。

#### 背压（Backpressure）

```
asr-claw 处理太慢（如引擎识别延迟高）
    ↓
pipe buffer 满（通常 64KB）
    ↓
adb-claw 写 stdout 时阻塞
    ↓
adb-claw 暂停音频采集
    ↓
asr-claw 处理完一段，pipe buffer 有空间
    ↓
adb-claw 恢复写入
```

Unix pipe 天然提供流控。pipe buffer 通常为 64KB（macOS）或 65536 bytes（Linux），足够缓冲约 2 秒的 16kHz 16-bit PCM 数据（32000 bytes/sec）。

需要注意的是：如果引擎延迟过高导致长时间阻塞，设备端的音频数据可能会丢失。对于这种场景，建议使用 `tee` 保存原始数据：

```bash
adb-claw audio capture --stream | tee backup.wav | asr-claw transcribe --stream
```

---

## 13. 项目结构

```
asr-claw/
├── CLAUDE.md                        # 项目说明（供 Claude Code 理解）
├── .claude-plugin/                  # Claude Code 插件配置
│   ├── plugin.json
│   └── marketplace.json
├── skills/
│   └── asr-claw/
│       └── SKILL.md                 # Skill 定义（Claude Code + OpenClaw 共用）
├── src/                             # Go 代码根目录
│   ├── go.mod
│   ├── go.sum
│   ├── main.go                      # 入口
│   ├── Makefile                     # 构建脚本
│   ├── cmd/                         # Cobra CLI 命令
│   │   ├── root.go                  # 根命令 + 全局 flags
│   │   ├── transcribe.go           # transcribe 命令（核心）
│   │   ├── engines.go              # engines list/install/start/stop/status/info
│   │   ├── doctor.go               # 环境检查
│   │   └── skill.go                # 输出 skill.json
│   └── pkg/
│       ├── engine/                  # 引擎抽象层
│       │   ├── engine.go           # Engine/StreamEngine/Session 接口定义
│       │   ├── registry.go         # 引擎注册表
│       │   ├── qwen3asr/           # Qwen3-ASR (vLLM service)
│       │   │   └── qwen3asr.go
│       │   ├── qwen3asrrs/         # Qwen3-ASR-RS (Rust CLI)
│       │   │   └── qwen3asrrs.go
│       │   ├── whisper/            # whisper.cpp (C++ CLI)
│       │   │   └── whisper.go
│       │   ├── doubao/             # 火山引擎 Seed-ASR 2.0
│       │   │   └── doubao.go
│       │   ├── openai/             # OpenAI Whisper API
│       │   │   └── openai.go
│       │   └── deepgram/           # Deepgram WebSocket
│       │       └── deepgram.go
│       ├── audio/                   # 音频处理
│       │   ├── wav.go              # WAV header 解析/生成
│       │   ├── vad.go              # VAD 状态机 + 语音段切分
│       │   ├── silence.go          # RMS 能量计算 + 静音检测
│       │   ├── resample.go         # 重采样（如 48kHz → 16kHz）
│       │   └── chunk.go            # 固定时间切分（退化方案）
│       └── output/                  # 输出格式化
│           └── envelope.go         # JSON envelope + Writer
├── scripts/                         # 辅助脚本
│   └── install-engine.sh           # 引擎安装脚本
└── docs/                            # 技术文档
    └── design.md                    # 本文档
```

### 代码约定

与 adb-claw 保持一致：

- 新命令放 `src/cmd/`，新包放 `src/pkg/`
- 所有引擎调用通过 `Engine` 接口，不直接 exec
- 命令输出使用 `output.Writer` 写 JSON envelope
- 测试文件与源码同目录，用 `_test.go` 后缀
- 错误码用大写下划线格式：`ENGINE_NOT_FOUND`、`TRANSCRIBE_FAILED`、`INVALID_AUDIO`
- Go 1.24，依赖 cobra v1.10.2
- 构建产物在项目根目录 `bin/`（已 gitignore）

---

## 14. 开发优先级

### 前置条件

**adb-claw `audio capture` 已实现** — 上游数据源就绪，输出标准 WAV 流（44-byte header + 16kHz mono 16-bit PCM）。asr-claw 可以立即开始开发，无需等待上游。

adb-claw 抖音 App Profile 已将 asr-claw 列为"推荐能力"，Agent 会在直播间场景主动提示用户安装。这意味着 Phase 1 完成后即可被真实用户使用。

### Phase 1 — MVP（最小可用版本）

**目标**：能从文件或 stdin 转写语音，单引擎可用。优先支持中文（直播监控主场景）。

| 模块 | 内容 |
|------|------|
| `cmd/root.go` | 根命令 + 全局 flags（-o, --timeout, --verbose） |
| `cmd/transcribe.go` | transcribe 命令，支持 `--file` 和 stdin |
| `pkg/audio/wav.go` | WAV header 解析（兼容 adb-claw 的 0x7FFFFFFF 流式标记） |
| `pkg/audio/vad.go` | VAD 状态机 + 语音段切分 |
| `pkg/audio/silence.go` | RMS 能量计算 |
| `pkg/engine/engine.go` | Engine 接口定义 |
| `pkg/engine/registry.go` | 引擎注册表 |
| `pkg/engine/qwen3asrrs/` | qwen3-asr-rs 引擎集成 |
| `pkg/output/envelope.go` | JSON envelope（复用 adb-claw 设计） |
| `cmd/doctor.go` | 环境检查（引擎是否安装、模型是否存在） |

**验收标准**：

```bash
# 文件转写（离线场景）
asr-claw transcribe --file test.wav --lang zh
# → 输出 JSON envelope with segments

# Pipe 转写（adb-claw 录制文件后）
cat test.wav | asr-claw transcribe --lang zh

# 端到端：adb-claw 录制 → asr-claw 转写
adb-claw audio capture --duration 15000 --file /tmp/test.wav
asr-claw transcribe --file /tmp/test.wav --lang zh
```

### Phase 2 — 流式 + 多引擎

**目标**：支持 `--stream` 流式模式（直播监控核心），增加 whisper.cpp 引擎和引擎管理命令。

| 模块 | 内容 |
|------|------|
| `cmd/transcribe.go` | 增加 `--stream` 流式模式 |
| `pkg/engine/whisper/` | whisper.cpp 引擎集成 |
| `cmd/engines.go` | engines list / install 命令 |
| 配置管理 | `~/.asr-claw/config.yaml` 读写 |
| `pkg/audio/resample.go` | 采样率转换 |

**验收标准**：

```bash
# 流式转写（直播监控主场景）— 注意 adb-claw 需要显式 --duration
adb-claw audio capture --stream --duration 60000 | asr-claw transcribe --stream --lang zh
# → 逐段输出 JSON Lines

# 引擎管理
asr-claw engines list
asr-claw engines install whisper
asr-claw transcribe --file test.wav --engine whisper
```

### Phase 3 — Service 引擎 + Cloud + 发布

**目标**：完整引擎生态，字幕输出，发布为 Claude Code Skill 和 ClawHub Skill。

| 模块 | 内容 |
|------|------|
| `pkg/engine/qwen3asr/` | Qwen3-ASR vLLM service 引擎（原生流式） |
| `pkg/engine/doubao/` | 火山引擎 Seed-ASR 2.0 |
| `pkg/engine/openai/` | OpenAI Whisper API |
| `pkg/engine/deepgram/` | Deepgram WebSocket（原生流式） |
| `cmd/engines.go` | 增加 start / stop / status |
| SRT/VTT 输出 | `--format srt` / `--format vtt` |
| `cmd/skill.go` | 输出 skill.json |
| `.claude-plugin/` | Claude Code 插件配置 |
| `skills/asr-claw/SKILL.md` | Skill 定义文件（Triggers 需覆盖"转写"、"语音识别"、"直播语音"等场景） |
| ClawHub 发布 | OpenClaw Skill 发布 |

**验收标准**：

```bash
# Service 引擎
asr-claw engines start qwen3-asr
adb-claw audio capture --stream --duration 0 | asr-claw transcribe --stream --engine qwen3-asr

# Cloud 引擎
DOUBAO_API_KEY=xxx asr-claw transcribe --file test.wav --engine doubao

# 字幕输出
asr-claw transcribe --file lecture.wav --format srt > lecture.srt

# 作为 Skill 运行
claude --plugin-dir /path/to/asr-claw

# Agent 端到端场景：打开抖音直播间 → 双通道采集
# （此时 adb-claw douyin profile 的"推荐能力"表会引导 Agent 使用 asr-claw）
```

---

## 15. Qwen3-ASR 原生流式细节

### 15.1 官方 Python API

Qwen3-ASR 是通义千问推出的语音识别模型，支持流式转写。以下是官方提供的流式 API 示例：

```python
from transformers import Qwen2AudioForConditionalGeneration, AutoProcessor

processor = AutoProcessor.from_pretrained("Qwen/Qwen3-ASR")
model = Qwen2AudioForConditionalGeneration.from_pretrained("Qwen/Qwen3-ASR")

asr = model.create_asr_pipeline(processor)

# 初始化流式状态
state = asr.init_streaming_state(
    chunk_size_sec=2.0,       # 每次送入的音频块大小（秒）
    unfixed_chunk_num=2,      # 未确认的块数量
    unfixed_token_num=5,      # 未确认的 token 数量
)

# 逐块送入音频
for audio_chunk in audio_stream:
    asr.streaming_transcribe(audio_chunk, state)
    print(state.text)  # 当前累计识别文本（含未确认部分）

# 结束流式，获取最终确认文本
asr.finish_streaming_transcribe(state)
print(state.text)  # 最终完整文本
```

### 15.2 关键参数解释

**`chunk_size_sec`（块大小）**：
- 每次送入引擎的音频长度。2.0 秒是推荐值
- 太小（<1s）：上下文不足，识别准确率下降
- 太大（>5s）：延迟增加，失去流式意义

**`unfixed_chunk_num`（未确认块数）**：
- 纠错机制的核心参数。值为 2 表示最近 2 个块的识别结果是"暂定的"，可能被后续输入纠正
- 例如：听到"今天天气"时暂时识别为"今天天气"，后续听到更多内容后可能纠正为"今天天气真不错"
- 值越大，纠错窗口越宽，但延迟越高

**`unfixed_token_num`（未确认 token 数）**：
- 控制最近 N 个 token 可被纠正
- 与 `unfixed_chunk_num` 共同决定纠错范围
- 值为 5 表示最后 5 个 token（大约 2-3 个汉字/词）可能被修改

**纠错机制示意**：

```
时间 →

块1 送入后:    "今天"
              ^^^^^ 未确认

块2 送入后:    "今天天气真"
               ^^^^^^^^^ 确认     ^^^^^ 未确认

块3 送入后:    "今天天气真不错我们"
                         ^^^^^^^^^ 确认     ^^^^^ 未确认

结束后:        "今天天气真不错我们去公园"
               ^^^^^^^^^^^^^^^^^^^^^^^^ 全部确认
```

### 15.3 部署架构

Qwen3-ASR 流式模式需要 GPU + vLLM 后端：

```
┌──────────────────────────────────────────────────────┐
│                    本地 GPU 服务器                     │
│                                                       │
│  ┌─────────────────────────────────────────────┐     │
│  │              vLLM Server                     │     │
│  │  model: Qwen/Qwen3-ASR                      │     │
│  │  GPU: NVIDIA A100/4090/etc                   │     │
│  │  端口: 8000                                   │     │
│  │                                               │     │
│  │  /v1/audio/transcriptions  (REST)            │     │
│  │  /v1/audio/stream          (WebSocket)       │     │
│  └──────────────────────┬──────────────────────┘     │
│                          │                            │
└──────────────────────────┼────────────────────────────┘
                           │ HTTP / WebSocket
                           │
              ┌────────────┴────────────┐
              │    asr-claw             │
              │    (qwen3-asr engine)   │
              └─────────────────────────┘
```

asr-claw 通过 HTTP/WebSocket 连接本地 vLLM 服务：

```go
// qwen3asr.go

type Qwen3ASR struct {
    endpoint string // http://localhost:8000
    model    string // Qwen/Qwen3-ASR
}

func (q *Qwen3ASR) Info() Capability {
    return Capability{
        Name:         "qwen3-asr",
        Type:         "service",
        NeedsModel:   true,
        NeedsAPIKey:  false,
        NativeStream: true,
        Connection:   "http",
        SampleRate:   16000,
    }
}

// TranscribeFile 非流式模式：整文件 POST 到 REST API
func (q *Qwen3ASR) TranscribeFile(path string, lang string) ([]Segment, error) {
    audioData, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    // POST /v1/audio/transcriptions
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    part, _ := writer.CreateFormFile("file", filepath.Base(path))
    part.Write(audioData)
    writer.WriteField("model", q.model)
    writer.WriteField("language", lang)
    writer.Close()

    resp, err := http.Post(q.endpoint+"/v1/audio/transcriptions", writer.FormDataContentType(), body)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // 解析响应...
    var result struct {
        Text     string `json:"text"`
        Segments []struct {
            Start float64 `json:"start"`
            End   float64 `json:"end"`
            Text  string  `json:"text"`
        } `json:"segments"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    segments := make([]Segment, len(result.Segments))
    for i, s := range result.Segments {
        segments[i] = Segment{
            Index: i,
            Start: s.Start,
            End:   s.End,
            Text:  s.Text,
        }
    }
    return segments, nil
}

// StreamSession 流式模式：通过 WebSocket 连接
func (q *Qwen3ASR) StreamSession(opts Options) (Session, error) {
    wsURL := strings.Replace(q.endpoint, "http", "ws", 1) + "/v1/audio/stream"

    conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
        "X-Model":       {q.model},
        "X-Language":    {opts.Lang},
        "X-Sample-Rate": {strconv.Itoa(opts.SampleRate)},
    })
    if err != nil {
        return nil, fmt.Errorf("连接 vLLM WebSocket 失败: %w", err)
    }

    return &qwen3Session{conn: conn}, nil
}

type qwen3Session struct {
    conn *websocket.Conn
}

func (s *qwen3Session) Feed(pcm []byte) (string, error) {
    // 发送二进制 PCM 数据
    err := s.conn.WriteMessage(websocket.BinaryMessage, pcm)
    if err != nil {
        return "", err
    }

    // 读取增量识别结果
    _, msg, err := s.conn.ReadMessage()
    if err != nil {
        return "", err
    }

    var result struct {
        Text  string `json:"text"`
        Final bool   `json:"is_final"`
    }
    json.Unmarshal(msg, &result)
    return result.Text, nil
}

func (s *qwen3Session) Finish() (string, error) {
    // 发送结束标记
    s.conn.WriteMessage(websocket.TextMessage, []byte(`{"action":"finish"}`))

    // 读取最终结果
    _, msg, err := s.conn.ReadMessage()
    if err != nil {
        return "", err
    }

    var result struct {
        Text string `json:"text"`
    }
    json.Unmarshal(msg, &result)
    return result.Text, nil
}

func (s *qwen3Session) Close() {
    s.conn.Close()
}
```

### 15.4 注意事项

- **流式模式必须 GPU**：Qwen3-ASR 流式推理需要 GPU，CPU 推理太慢无法实现实时流式
- **vLLM 版本要求**：需要支持 audio model 的 vLLM 版本（v0.8.0+）
- **非流式可用 Rust 版本**：qwen3-asr-rs 是 Rust 实现的纯 CPU 推理版本，适合不需要流式的场景
- **纠错导致文本"闪烁"**：流式模式下，`unfixed_chunk_num` 和 `unfixed_token_num` 会导致已输出的文本被修改。asr-claw 在 stream 输出时需要标记哪些文本是 `final`（已确认）、哪些是 `partial`（可能被纠正）：

```json
{"type": "partial", "text": "今天天气真不错我"}
{"type": "partial", "text": "今天天气真不错我们"}
{"type": "final",   "text": "今天天气真不错。", "start": 0.0, "end": 2.5}
{"type": "partial", "text": "我们去公园"}
{"type": "final",   "text": "我们去公园散步吧。", "start": 2.5, "end": 5.1}
```

---

## 附录 A：JSON Envelope 格式

与 adb-claw 保持一致的输出格式：

```json
{
  "ok": true,
  "command": "transcribe",
  "data": {
    "segments": [
      {
        "index": 0,
        "start": 0.0,
        "end": 2.5,
        "text": "今天天气真不错。",
        "lang": "zh"
      },
      {
        "index": 1,
        "start": 2.8,
        "end": 5.1,
        "text": "我们去公园散步吧。",
        "lang": "zh"
      }
    ],
    "full_text": "今天天气真不错。我们去公园散步吧。",
    "engine": "qwen3-asr-rs",
    "audio_duration_sec": 5.5
  },
  "duration_ms": 1230,
  "timestamp": "2026-03-13T10:00:00Z"
}
```

错误输出：

```json
{
  "ok": false,
  "command": "transcribe",
  "error": {
    "code": "ENGINE_NOT_FOUND",
    "message": "engine 'whisper' is not installed",
    "suggestion": "run 'asr-claw engines install whisper' to install"
  },
  "duration_ms": 5,
  "timestamp": "2026-03-13T10:00:00Z"
}
```

## 附录 B：SRT/VTT 输出示例

### SRT 格式

```
1
00:00:00,000 --> 00:00:02,500
今天天气真不错。

2
00:00:02,800 --> 00:00:05,100
我们去公园散步吧。

3
00:00:05,500 --> 00:00:08,200
公园里的花都开了。
```

### VTT 格式

```
WEBVTT

00:00:00.000 --> 00:00:02.500
今天天气真不错。

00:00:02.800 --> 00:00:05.100
我们去公园散步吧。

00:00:05.500 --> 00:00:08.200
公园里的花都开了。
```
