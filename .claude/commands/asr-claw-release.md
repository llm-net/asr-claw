---
description: "正式发布指定版本到 GitHub Releases + ClawHub"
allowed-tools: "Bash, Read, Edit, Grep, Glob, Write, Agent"
---

# 正式发布 $ARGUMENTS

按以下步骤**完整执行**发布流程。发布涉及两个平台：

- **GitHub Releases** — CI 自动构建 asr-claw（4 平台）+ qwen-asr（3 平台）预编译二进制
- **ClawHub** — OpenClaw 技能市场（https://clawhub.ai/dionren/asr-claw）

---

## Step 1: 同步版本号

将以下 5 处版本号更新为 `$ARGUMENTS`：

```
.claude-plugin/plugin.json      → "version": "$ARGUMENTS"
skills/asr-claw/SKILL.md        → version: $ARGUMENTS (frontmatter 第 3 行)
src/cmd/root.go                  → 注释中的 ldflags 示例版本
website/src/i18n.js              → hero.badge 中的版本号（EN + ZH 两处）
website/src/components/Install.jsx → machine-readable block 中的 version
```

## Step 2: 更新文档

检查以下文件是否需要更新（如有未提交的功能变更）：

- `skills/asr-claw/SKILL.md` — 命令文档、引擎矩阵、Getting Started
- `README.md` — 与 SKILL.md 对齐（Engines 表、Usage、Configuration）
- `CLAUDE.md` — 命令树、项目结构

## Step 3: ClawHub 安全审查

**发布前必须审查 `skills/asr-claw/SKILL.md`**，确保不会触发 ClawHub 安全扫描告警。

逐项检查以下规则：

### 3.1 Install Mechanism（最常触发告警）

- **禁止** `kind: "script"` + `curl | bash` 模式 — 这会被标记为 Suspicious
- **必须**使用 `kind: "download"` 直接下载二进制，或 `kind: "brew"` 包管理器安装
- 所有 download URL 必须指向 `github.com/llm-net/asr-claw`（不是其他旧 URL）
- 确认 frontmatter `homepage` 字段为 `https://github.com/llm-net/asr-claw`

```
# 合规示例
{ "kind": "download", "url": "https://github.com/llm-net/asr-claw/releases/latest/download/asr-claw-darwin-arm64" }

# 违规示例（会触发告警）
{ "kind": "script", "script": "curl -fsSL ... | bash" }
```

### 3.2 Purpose & Capability

- `name` / `description` 必须与运行时功能一致
- `requires.bins` 只列必需的二进制（asr-claw）

### 3.3 Instruction Scope

- SKILL.md 正文只包含 asr-claw 命令指引
- 不能有读取无关本地文件、网络请求、数据外传的指令

### 3.4 Credentials

- 不在 SKILL.md 中硬编码 API key
- 云引擎 API key 通过环境变量传递，属于用户自行配置，不违规

### 3.5 Persistence & Privilege

- 不设置 `always: true`
- 不请求系统级配置修改

用 Grep 检查：
```bash
# 必须无结果
grep -n "curl.*bash\|kind.*script\|always.*true\|credential\|secret\|api.key" skills/asr-claw/SKILL.md
```

如发现问题，修复后再继续。

## Step 4: 运行测试 & 构建

```bash
export PATH="/Users/dionren/go-sdk/go/bin:$PATH"
cd src && make test && make build
```

测试全部通过、构建成功后才能继续。

## Step 5: 提交 & 推送

```bash
git add <所有变更文件>
git commit -m "feat: v$ARGUMENTS — 简要描述"
git push origin main
```

提交信息使用 `feat: vX.Y.Z — 简要描述` 格式。

## Step 6: 打 tag 触发 GitHub Release

```bash
git tag v$ARGUMENTS
git push origin v$ARGUMENTS
```

推送 tag 后 GitHub Actions 自动执行 Release workflow：

- test → 交叉编译 asr-claw（4 平台）+ 原生编译 qwen-asr（3 平台）→ 创建 GitHub Release

Release 包含以下 assets：

```
asr-claw-darwin-arm64       # asr-claw 本体
asr-claw-darwin-amd64
asr-claw-linux-arm64
asr-claw-linux-amd64
qwen-asr-darwin-arm64       # 预编译 antirez/qwen-asr 引擎
qwen-asr-darwin-amd64
qwen-asr-linux-amd64
install.sh
checksums.txt
```

## Step 7: 发布到 ClawHub

**GitHub Release 只覆盖二进制分发，ClawHub 必须单独发布。**

### 7.1 同步 workspace（必须）

`clawhub publish` 从 `~/.openclaw/workspace/skills/asr-claw/` 读取文件，**不是**命令行指定的项目路径。如果 workspace 中有旧的 SKILL.md，发布的永远是旧内容，bump 再多版本都没用。

```bash
# 用项目中的最新 SKILL.md 覆盖 workspace 中的旧文件
cp skills/asr-claw/SKILL.md ~/.openclaw/workspace/skills/asr-claw/SKILL.md

# 验证两个文件一致
shasum -a 256 skills/asr-claw/SKILL.md ~/.openclaw/workspace/skills/asr-claw/SKILL.md
```

两个 hash 必须完全一致，不一致则说明复制失败，不要继续。

### 7.2 发布

```bash
clawhub publish skills/asr-claw --version $ARGUMENTS --changelog "变更摘要"
```

### 7.3 验证服务端文件

```bash
# 对比服务端文件 hash，确认上传的是最新内容
clawhub inspect asr-claw --files --version $ARGUMENTS
```

- ClawHub 上的技能路径：https://clawhub.ai/dionren/asr-claw
- 登录账号：`dionren`（`clawhub whoami` 验证）
- 发布后有安全扫描，通常几分钟后上线
- **同版本号不可重复发布**，如需重发必须 bump 版本

## Step 8: 验证

```bash
# GitHub CI 进度
gh run list --repo llm-net/asr-claw --limit 2

# GitHub Release assets（应有 9 个文件）
gh release view v$ARGUMENTS --repo llm-net/asr-claw

# ClawHub 版本确认（安全扫描中会暂时 hidden）
clawhub inspect asr-claw
```

向用户汇报两个平台的发布状态。

## 注意事项

- Git remote：`origin → llm-net/asr-claw`（主仓库，CI 和 Release 在此）
- 每步执行前确认上一步成功，不要跳步
- 如果 `clawhub publish` 报 "Version already exists"，说明该版本已发布过，需要 bump 版本号
- 如果 ClawHub 安全扫描标记为 Suspicious，检查 Step 3 的规则并修复后 bump 版本重发
