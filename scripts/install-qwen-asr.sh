#!/usr/bin/env bash
#
# install-qwen-asr.sh — Build antirez/qwen-asr binary and download Qwen3-ASR-0.6B model
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
BASE_DIR="${ASR_CLAW_HOME:-$HOME/.asr-claw}"
BIN_DIR="$BASE_DIR/bin"
MODEL_SIZE="${MODEL_SIZE:-0.6B}"
MODEL_NAME="Qwen3-ASR-${MODEL_SIZE}"
MODEL_DIR="$BASE_DIR/models/$MODEL_NAME"
HF_BASE="${HF_MIRROR:-https://huggingface.co}"
HF_REPO="Qwen/$MODEL_NAME"
REPO_URL="https://github.com/antirez/qwen-asr.git"
BUILD_DIR="$(mktemp -d)"

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

cleanup() {
    rm -rf "$BUILD_DIR"
}
trap cleanup EXIT

# --- Check prerequisites ---
check_prereqs() {
    local missing=()
    command -v git  >/dev/null 2>&1 || missing+=(git)
    command -v make >/dev/null 2>&1 || missing+=(make)
    command -v cc   >/dev/null 2>&1 || missing+=(cc)

    if [[ ${#missing[@]} -gt 0 ]]; then
        die "Missing prerequisites: ${missing[*]}. Install with: brew install ${missing[*]}"
    fi

    # Check for curl or wget
    if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
        die "Need curl or wget for model download"
    fi
}

# --- Build binary ---
build_binary() {
    if [[ "$MODEL_ONLY" == "true" ]]; then
        info "Skipping binary build (--model-only)"
        return 0
    fi

    # Check if binary already exists and is functional
    if [[ -x "$BIN_DIR/qwen-asr" ]]; then
        ok "Binary already exists at $BIN_DIR/qwen-asr"
        return 0
    fi

    info "Cloning antirez/qwen-asr..."
    git clone --depth 1 "$REPO_URL" "$BUILD_DIR/qwen-asr" 2>&1 | tail -1

    info "Building with Accelerate framework (this may take a minute)..."
    cd "$BUILD_DIR/qwen-asr"

    # Use 'make blas' for macOS Accelerate, fallback to 'make'
    if [[ "$(uname -s)" == "Darwin" ]]; then
        make blas 2>&1 | tail -3
    else
        make 2>&1 | tail -3
    fi

    # Find the built binary
    local binary=""
    for name in qwen_asr qwen-asr; do
        if [[ -x "$BUILD_DIR/qwen-asr/$name" ]]; then
            binary="$BUILD_DIR/qwen-asr/$name"
            break
        fi
    done

    if [[ -z "$binary" ]]; then
        die "Build succeeded but cannot find binary in $BUILD_DIR/qwen-asr/"
    fi

    mkdir -p "$BIN_DIR"
    cp "$binary" "$BIN_DIR/qwen-asr"
    chmod +x "$BIN_DIR/qwen-asr"
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
    build_binary
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
