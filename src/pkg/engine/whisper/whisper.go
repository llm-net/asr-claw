package whisper

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/llm-net/asr-claw/pkg/engine"
)

func init() {
	engine.Register("whisper", func() engine.Engine { return New() })
}

// Whisper is the whisper.cpp CLI engine.
type Whisper struct {
	binaryPath string
	modelPath  string
	threads    int
}

// New creates a Whisper engine instance.
func New() *Whisper {
	home, _ := os.UserHomeDir()
	return &Whisper{
		binaryPath: filepath.Join(home, ".asr-claw", "bin", "whisper-cpp"),
		modelPath:  filepath.Join(home, ".asr-claw", "models", "whisper", "ggml-large-v3.bin"),
		threads:    4,
	}
}

func (w *Whisper) Info() engine.Capability {
	return engine.Capability{
		Name:         "whisper",
		Type:         "cli",
		Languages:    []string{"zh", "en", "ja", "ko", "fr", "de", "es", "pt", "ru", "ar"},
		NeedsModel:   true,
		NeedsAPIKey:  false,
		NativeStream: false,
		Connection:   "subprocess",
		SampleRate:   16000,
		Installed:    w.isInstalled(),
	}
}

func (w *Whisper) TranscribeFile(path string, lang string) ([]engine.Segment, error) {
	if !w.isInstalled() {
		return nil, fmt.Errorf("whisper.cpp is not installed; run 'asr-claw engines install whisper'")
	}

	args := []string{
		"-m", w.modelPath,
		"-l", lang,
		"-t", fmt.Sprintf("%d", w.threads),
		"-oj",   // output JSON
		"-of", "/dev/stdout", // output to stdout
		"-f", path,
	}

	cmd := exec.Command(w.binaryPath, args...)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("whisper.cpp failed: %w", err)
	}

	return parseWhisperJSON(output)
}

func (w *Whisper) isInstalled() bool {
	if _, err := os.Stat(w.binaryPath); err != nil {
		return false
	}
	if _, err := os.Stat(w.modelPath); err != nil {
		return false
	}
	return true
}

func parseWhisperJSON(data []byte) ([]engine.Segment, error) {
	// whisper.cpp JSON output format
	var result struct {
		Transcription []struct {
			Timestamps struct {
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"timestamps"`
			Offsets struct {
				From int `json:"from"`
				To   int `json:"to"`
			} `json:"offsets"`
			Text string `json:"text"`
		} `json:"transcription"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		// Fallback: plain text
		text := strings.TrimSpace(string(data))
		if text == "" {
			return nil, nil
		}
		return []engine.Segment{{Index: 0, Text: text}}, nil
	}

	segments := make([]engine.Segment, len(result.Transcription))
	for i, t := range result.Transcription {
		segments[i] = engine.Segment{
			Index: i,
			Start: float64(t.Offsets.From) / 1000.0,
			End:   float64(t.Offsets.To) / 1000.0,
			Text:  strings.TrimSpace(t.Text),
		}
	}
	return segments, nil
}
