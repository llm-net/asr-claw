package qwenasr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/llm-net/asr-claw/pkg/config"
	"github.com/llm-net/asr-claw/pkg/engine"
)

func init() {
	engine.Register("qwen-asr", func() engine.Engine { return New() })
}

// QwenASR wraps the antirez/qwen-asr C binary for local Mac inference.
// Uses Qwen3-ASR-0.6B model with Apple Accelerate framework.
type QwenASR struct {
	binaryPath string
	modelPath  string
}

// New creates a QwenASR engine with paths from config or defaults.
func New() *QwenASR {
	base := config.BaseDir()
	cfg := config.Load()
	ec := cfg.GetEngine("qwen-asr")

	binaryPath := ec.Binary
	if binaryPath == "" {
		binaryPath = filepath.Join(base, "bin", "qwen-asr")
	}

	modelPath := ec.ModelPath
	if modelPath == "" {
		modelPath = filepath.Join(base, "models", "Qwen3-ASR-0.6B")
	}

	return &QwenASR{
		binaryPath: binaryPath,
		modelPath:  modelPath,
	}
}

func (q *QwenASR) Info() engine.Capability {
	return engine.Capability{
		Name:         "qwen-asr",
		Type:         "cli",
		Languages:    []string{"zh", "en", "ja", "ko", "fr", "de", "es", "pt", "ru", "ar", "it"},
		NeedsModel:   true,
		NeedsAPIKey:  false,
		NativeStream: false,
		Connection:   "subprocess",
		SampleRate:   16000,
		Installed:    q.isInstalled(),
	}
}

func (q *QwenASR) TranscribeFile(path string, lang string) ([]engine.Segment, error) {
	if !q.isInstalled() {
		return nil, fmt.Errorf("qwen-asr is not installed; run 'asr-claw engines install qwen-asr'")
	}

	args := []string{
		"-d", q.modelPath,
		"-i", path,
		"--silent",
	}

	// Map language code to qwen-asr language name
	if langName := mapLanguage(lang); langName != "" {
		args = append(args, "--language", langName)
	}

	cmd := exec.Command(q.binaryPath, args...)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("qwen-asr failed: %w", err)
	}

	text := strings.TrimSpace(string(output))
	if text == "" {
		return nil, nil
	}

	return []engine.Segment{
		{Index: 0, Text: text},
	}, nil
}

// BinaryPath returns the expected binary path (for install commands).
func (q *QwenASR) BinaryPath() string {
	return q.binaryPath
}

// ModelPath returns the expected model directory path.
func (q *QwenASR) ModelPath() string {
	return q.modelPath
}

func (q *QwenASR) isInstalled() bool {
	if _, err := os.Stat(q.binaryPath); err != nil {
		return false
	}
	// Check for model.safetensors in model dir
	safetensors := filepath.Join(q.modelPath, "model.safetensors")
	if _, err := os.Stat(safetensors); err != nil {
		return false
	}
	return true
}

// mapLanguage converts ISO 639-1 code to qwen-asr language name.
func mapLanguage(code string) string {
	m := map[string]string{
		"zh": "Chinese",
		"en": "English",
		"ja": "Japanese",
		"ko": "Korean",
		"fr": "French",
		"de": "German",
		"es": "Spanish",
		"pt": "Portuguese",
		"ru": "Russian",
		"ar": "Arabic",
		"it": "Italian",
		"nl": "Dutch",
		"pl": "Polish",
		"tr": "Turkish",
		"vi": "Vietnamese",
		"th": "Thai",
		"id": "Indonesian",
		"ms": "Malay",
		"hi": "Hindi",
		"uk": "Ukrainian",
	}
	if name, ok := m[code]; ok {
		return name
	}
	return ""
}
