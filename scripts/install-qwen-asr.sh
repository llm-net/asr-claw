#!/usr/bin/env bash
#
# install-qwen-asr.sh — Download pre-built qwen-asr binary and Qwen3-ASR model
#
# The qwen-asr binary is pre-compiled by asr-claw CI from antirez/qwen-asr
# and distributed alongside asr-claw in GitHub Releases.
#
# Usage:
#   bash install-qwen-asr.sh [--model-only] [--binary-only] [--model-size 0.6B|1.7B]
#
# Environment:
#   ASR_CLAW_HOME    Override base directory (default: ~/.asr-claw)
#   HF_MIRROR        HuggingFace mirror URL (for China: https://hf-mirror.com)
#

set -euo pipefail

# --- Configuration ---
REPO="llm-net/asr-claw"
BASE_DIR="${ASR_CLAW_HOME:-$HOME/.asr-claw}"
BIN_DIR="$BASE_DIR/bin"
MODEL_SIZE="${MODEL_SIZE:-0.6B}"
MODEL_NAME="Qwen3-ASR-${MODEL_SIZE}"
MODEL_DIR="$BASE_DIR/models/$MODEL_NAME"
HF_BASE="${HF_MIRROR:-https://huggingface.co}"
HF_REPO="Qwen/$MODEL_NAME"

BINARY_ONLY=false
MODEL_ONLY=false

# --- Parse args ---
while [[ $# -gt 0 ]]; do
    case "$1" in
        --binary-only) BINARY_ONLY=true; shift ;;
        --model-only)  MODEL_ONLY=true; shift ;;
        --model-size)  MODEL_SIZE="$2"; MODEL_NAME="Qwen3-ASR-${MODEL_SIZE}"; MODEL_DIR="$BASE_DIR/models/$MODEL_NAME"; HF_REPO="Qwen/$MODEL_NAME"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# --- Helpers ---
info()  { echo -e "\033[0;34m[info]\033[0m  $*"; }
ok()    { echo -e "\033[0;32m[ok]\033[0m    $*"; }
warn()  { echo -e "\033[0;33m[warn]\033[0m  $*"; }
error() { echo -e "\033[0;31m[error]\033[0m $*" >&2; }
die()   { error "$@"; exit 1; }

# --- Check prerequisites ---
check_prereqs() {
    if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
        die "Need curl or wget to download files"
    fi
}

# --- Detect platform ---
detect_platform() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        darwin) os="darwin" ;;
        linux)  os="linux" ;;
        *)      die "Unsupported OS: $os" ;;
    esac

    case "$arch" in
        x86_64|amd64)  arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)             die "Unsupported architecture: $arch" ;;
    esac

    echo "${os}-${arch}"
}

# --- Get latest release tag ---
get_latest_version() {
    local url="https://api.github.com/repos/${REPO}/releases/latest"
    local tag

    if command -v curl &>/dev/null; then
        tag="$(curl -fsSL "$url" 2>/dev/null | grep '"tag_name"' | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/')"
    else
        tag="$(wget -qO- "$url" 2>/dev/null | grep '"tag_name"' | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/')"
    fi

    if [ -z "$tag" ]; then
        die "Failed to get latest release version from GitHub"
    fi

    echo "$tag"
}

# --- Verify checksum ---
verify_checksum() {
    local file="$1"
    local filename="$2"
    local version="$3"
    local checksums_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"
    local expected actual checksums

    if command -v curl &>/dev/null; then
        checksums="$(curl -fsSL "$checksums_url" 2>/dev/null)" || { warn "Checksum file not available, skipping verification."; return 0; }
    else
        checksums="$(wget -qO- "$checksums_url" 2>/dev/null)" || { warn "Checksum file not available, skipping verification."; return 0; }
    fi

    expected="$(echo "$checksums" | grep "$filename" | awk '{print $1}')"
    if [ -z "$expected" ]; then return 0; fi

    if command -v sha256sum &>/dev/null; then
        actual="$(sha256sum "$file" | awk '{print $1}')"
    elif command -v shasum &>/dev/null; then
        actual="$(shasum -a 256 "$file" | awk '{print $1}')"
    else
        return 0
    fi

    if [ "$expected" != "$actual" ]; then
        error "Checksum mismatch! Removing downloaded file."
        rm -f "$file"
        return 1
    fi
    ok "Checksum verified."
}

# --- Download pre-built binary ---
download_binary() {
    if [[ "$MODEL_ONLY" == "true" ]]; then
        info "Skipping binary download (--model-only)"
        return 0
    fi

    if [[ -x "$BIN_DIR/qwen-asr" ]]; then
        ok "Binary already exists at $BIN_DIR/qwen-asr"
        return 0
    fi

    local platform version asset_name download_url
    platform="$(detect_platform)"
    asset_name="qwen-asr-${platform}"

    # Check if this platform has a pre-built binary
    if [[ "$platform" == "linux-arm64" ]]; then
        die "Pre-built qwen-asr binary is not available for linux-arm64 yet. Please build from source: https://github.com/antirez/qwen-asr"
    fi

    info "Fetching latest release version..."
    version="$(get_latest_version)"

    download_url="https://github.com/${REPO}/releases/download/${version}/${asset_name}"

    info "Downloading qwen-asr ${version} for ${platform}..."
    mkdir -p "$BIN_DIR"

    if command -v curl >/dev/null 2>&1; then
        curl -fSL --progress-bar -o "$BIN_DIR/qwen-asr" "$download_url" || die "Download failed. Check your network or visit https://github.com/${REPO}/releases"
    else
        wget -q --show-progress -O "$BIN_DIR/qwen-asr" "$download_url" || die "Download failed. Check your network or visit https://github.com/${REPO}/releases"
    fi

    chmod +x "$BIN_DIR/qwen-asr"

    verify_checksum "$BIN_DIR/qwen-asr" "$asset_name" "$version" || die "Checksum verification failed"

    ok "Binary installed to $BIN_DIR/qwen-asr"
}

# --- Download model ---
download_file() {
    local url="$1"
    local dest="$2"

    if [[ -f "$dest" ]]; then
        ok "Already exists: $(basename "$dest")"
        return 0
    fi

    info "Downloading $(basename "$dest")..."
    if command -v curl >/dev/null 2>&1; then
        curl -fSL --progress-bar -o "$dest" "$url"
    else
        wget -q --show-progress -O "$dest" "$url"
    fi
}

download_model() {
    if [[ "$BINARY_ONLY" == "true" ]]; then
        info "Skipping model download (--binary-only)"
        return 0
    fi

    # Check if model already exists
    if [[ -f "$MODEL_DIR/model.safetensors" ]]; then
        ok "Model already exists at $MODEL_DIR/"
        return 0
    fi

    # Try huggingface-cli first (handles auth, resume, etc.)
    if command -v huggingface-cli >/dev/null 2>&1; then
        info "Downloading model via huggingface-cli..."
        huggingface-cli download "$HF_REPO" --local-dir "$MODEL_DIR" && {
            ok "Model downloaded to $MODEL_DIR/"
            return 0
        }
        warn "huggingface-cli failed, falling back to direct download"
    fi

    # Direct HTTP download
    info "Downloading $MODEL_NAME model files (~1.9 GB total)..."
    mkdir -p "$MODEL_DIR"

    local files=(
        "config.json"
        "generation_config.json"
        "merges.txt"
        "model.safetensors"
        "preprocessor_config.json"
        "tokenizer_config.json"
        "vocab.json"
        "chat_template.json"
    )

    local base_url="$HF_BASE/$HF_REPO/resolve/main"

    for file in "${files[@]}"; do
        download_file "$base_url/$file" "$MODEL_DIR/$file"
    done

    # Verify model.safetensors exists and has reasonable size
    local size
    size=$(stat -f%z "$MODEL_DIR/model.safetensors" 2>/dev/null || stat -c%s "$MODEL_DIR/model.safetensors" 2>/dev/null || echo 0)
    if [[ "$size" -lt 1000000000 ]]; then
        die "model.safetensors seems too small ($size bytes). Download may have failed."
    fi

    ok "Model downloaded to $MODEL_DIR/"
}

# --- Write/update config ---
update_config() {
    local config_file="$BASE_DIR/config.yaml"

    # Only create config if it doesn't exist
    if [[ ! -f "$config_file" ]]; then
        info "Creating default config at $config_file"
        cat > "$config_file" << EOF
# asr-claw configuration
# See: https://github.com/llm-net/asr-claw

default:
  engine: qwen-asr
  lang: zh
  format: json

engines:
  qwen-asr:
    binary: $BIN_DIR/qwen-asr
    model_path: $MODEL_DIR
EOF
        ok "Config written to $config_file"
    else
        ok "Config already exists at $config_file"
    fi
}

# --- Main ---
main() {
    info "Installing qwen-asr engine ($MODEL_NAME) for asr-claw"
    info "Base directory: $BASE_DIR"
    echo ""

    check_prereqs
    download_binary
    echo ""
    download_model
    echo ""
    update_config

    echo ""
    ok "Installation complete!"
    echo ""
    echo "  Binary:  $BIN_DIR/qwen-asr"
    echo "  Model:   $MODEL_DIR/"
    echo "  Config:  $BASE_DIR/config.yaml"
    echo ""
    echo "  Test: asr-claw engines list"
    echo "  Use:  asr-claw transcribe --file audio.wav --engine qwen-asr"
}

main "$@"
