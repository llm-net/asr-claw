# asr-claw

语音识别 CLI，供 AI agent 自动化调用。纯工具层，不含 LLM/Agent 逻辑。

## 发布渠道

asr-claw 同时作为两个平台的 Skill 发布，**共用一份 `skills/asr-claw/SKILL.md`**：

- **Claude Code**：通过插件市场安装（`.claude-plugin/`），按 `## Triggers` 触发，`## Binary` 指示二进制位置
- **OpenClaw**：通过 ClawHub 安装，读取 YAML frontmatter 中的 `metadata.openclaw`（OS 要求、依赖、安装脚本）

两个平台读同一个文件，Claude Code 忽略 frontmatter，OpenClaw 忽略 `## Binary` 段落。

```
.claude-plugin/              # Claude Code 插件配置
├── plugin.json              # 插件元数据
└── marketplace.json         # 市场发布配置
skills/
└── asr-claw/SKILL.md        # Skill 定义（两个平台共用）
```

## 项目结构

```
src/                    # Go 代码根目录（go.mod 在此）
├── main.go             # 入口
├── Makefile            # 构建脚本
├── cmd/                # Cobra CLI 命令
│   ├── root.go         # 根命令 + 全局 flags
│   ├── transcribe.go   # 核心转写命令
│   ├── engines.go      # 引擎管理（list/install/start/stop/status/info）
│   ├── doctor.go       # 环境检查
│   └── skill.go        # 输出 skill.json
└── pkg/
    ├── config/             # 配置管理
    │   └── config.go       # ~/.asr-claw/config.yaml 读写
    ├── engine/             # ASR 引擎抽象层
    │   ├── engine.go       # Engine/StreamEngine/Session 接口
    │   ├── registry.go     # 引擎注册表
    │   ├── qwenasr/        # antirez/qwen-asr（Mac 本地推理，推荐）
    │   ├── qwen3asr/       # Qwen3-ASR vLLM service（GPU 服务器）
    │   ├── whisper/        # whisper.cpp
    │   ├── doubao/         # 火山引擎 Seed-ASR 2.0
    │   ├── openai/         # OpenAI Whisper API
    │   └── deepgram/       # Deepgram WebSocket
    ├── audio/              # 音频处理
    │   ├── wav.go          # WAV header 解析
    │   ├── vad.go          # VAD 语音活动检测
    │   ├── resample.go     # 重采样
    │   └── chunk.go        # 分段管理
    └── output/
        └── envelope.go     # JSON 响应 envelope + Writer

scripts/
├── setup.sh                # SessionStart hook（下载 asr-claw 二进制）
├── install.sh              # 独立安装脚本
└── install-qwen-asr.sh     # qwen-asr 引擎安装（编译 + 模型下载）

docs/                   # 技术文档
└── asr-claw-design.md  # 完整设计文档
```

## 构建

```bash
cd src
make build     # 产物 → bin/asr-claw（项目根目录）
make test      # go test ./...
make lint      # go vet
make clean
```

Go 1.24，依赖 cobra v1.10.2。构建产物在项目根目录 `bin/`（已 gitignore）。

## 本地开发加载

开发完成后，编译并加载到 Claude Code：

```bash
cd src && make build   # 编译到 bin/asr-claw
claude --plugin-dir .  # 在项目根目录启动，加载当前目录为插件
```

- `make build` 产物输出到项目根目录 `bin/asr-claw`，与插件 SKILL.md 和 `setup.sh` 引用的路径一致
- SessionStart hook 检测到 `bin/asr-claw` 已存在会跳过下载，直接使用本地编译版本
- 已有会话中修改代码后，重新 `make build` + 重启 Claude Code 即可生效

## 架构要点

- **Engine 接口** (`pkg/engine/engine.go`) — 所有 ASR 引擎实现统一接口：`Engine`（基础）、`StreamEngine`（流式扩展）、`Session`（流式会话）
- **JSON Envelope** (`pkg/output/envelope.go`) — 统一 `{ok, command, data, error, duration_ms, timestamp}`。error 含 `{code, message, suggestion}`。支持 json/text/quiet 三种输出模式
- **VAD 分段** (`pkg/audio/vad.go`) — 基于 RMS 能量检测的语音活动检测，按自然句子边界分段，避免固定时间切分破坏语义
- **Unix pipe** — 默认从 stdin 读取 WAV 流，天然支持与 adb-claw 等工具 pipe 协作

## 命令树

```
asr-claw
├── transcribe                       # 核心：音频 → 文本
│   --file <path>                    # 本地文件输入
│   --stream                         # 流式模式（实时转写）
│   --lang <code>                    # 语言（默认 zh）
│   --engine <name>                  # 指定引擎
│   --format json|text|srt|vtt       # 输出格式
│   --chunk <seconds>                # 固定时间分段（fallback）
├── engines list                     # 列出可用引擎
├── engines install <engine>         # 安装引擎（下载模型）
├── engines start <engine>           # 启动引擎服务
├── engines stop <engine>            # 停止引擎服务
├── engines status                   # 引擎运行状态
├── engines info <engine>            # 引擎详情
├── skill                            # 输出 skill.json
└── doctor                           # 环境检查
```

## 全局 Flags

```
-o, --output <format>  # json（默认）| text | quiet
--timeout <ms>         # 命令超时（默认 30000）
--verbose              # 调试输出到 stderr
```

## 代码约定

- 新命令放 `src/cmd/`，新包放 `src/pkg/`
- 所有 ASR 调用必须通过 `Engine` 接口，不直接调用具体引擎
- 命令输出必须使用 `output.Writer` 写 JSON envelope
- 测试文件与源码同目录，用 `_test.go` 后缀
- 错误码用大写下划线格式，如 `ENGINE_NOT_FOUND`、`TRANSCRIBE_FAILED`
- `skills/asr-claw/SKILL.md` 同时服务 Claude Code 和 OpenClaw，修改时需兼顾两个平台的格式要求

## 与 adb-claw 协作

asr-claw 通过 Unix pipe 接收 adb-claw 的音频流：

```bash
adb-claw audio capture --stream --duration 0 | asr-claw transcribe --stream --lang zh
```

- adb-claw 只做音频采集（设备 → WAV 流），asr-claw 只做语音识别（WAV → 文本）
- 两者各自独立仓库、独立版本号、独立发布
- 流协议：44 字节 WAV header + 连续 raw PCM 16kHz mono 16-bit

## 开发工作流

- 每个 Phase 开始前先 plan，对齐实现方案后再编码
- 编码完成后运行 `cd src && make test && make build` 验证
- 每个 Phase 完成后更新 SKILL.md 命令文档和 CLAUDE.md 命令树
- 新命令遵循现有代码约定：Engine 接口、JSON envelope、顶级命令风格

## 发布流程

```
Git remote: origin → llm-net/asr-claw（主仓库，CI 和 Release 在此）
ClawHub:    https://clawhub.ai/dionren/asr-claw
```

发布步骤：

1. 更新 `.claude-plugin/plugin.json` 中的 version
2. 更新 `skills/asr-claw/SKILL.md` 中的 version
3. 提交并打 tag：`git tag v<版本号> && git push origin main --tags`
4. GitHub Actions 自动构建多平台二进制 + 创建 Release
5. 到 ClawHub 同步更新
