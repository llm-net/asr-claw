package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/llm-net/asr-claw/pkg/engine"
	"github.com/llm-net/asr-claw/pkg/output"

	// Register all engines
	_ "github.com/llm-net/asr-claw/pkg/engine/deepgram"
	_ "github.com/llm-net/asr-claw/pkg/engine/doubao"
	_ "github.com/llm-net/asr-claw/pkg/engine/openai"
	_ "github.com/llm-net/asr-claw/pkg/engine/qwen3asr"
	_ "github.com/llm-net/asr-claw/pkg/engine/qwenasr"
	_ "github.com/llm-net/asr-claw/pkg/engine/whisper"
)

func init() {
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check environment and engine availability",
	RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	w := output.NewWriter(outputMode, "doctor", verbose)

	type check struct {
		Name   string `json:"name"`
		Status string `json:"status"` // "ok", "warning", "error"
		Detail string `json:"detail,omitempty"`
	}

	var checks []check

	// 1. Platform
	checks = append(checks, check{
		Name:   "platform",
		Status: "ok",
		Detail: runtime.GOOS + "/" + runtime.GOARCH,
	})

	// 2. Config directory
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".asr-claw")
	if _, err := os.Stat(configDir); err != nil {
		checks = append(checks, check{
			Name:   "config_dir",
			Status: "warning",
			Detail: configDir + " does not exist (will be created on first use)",
		})
	} else {
		checks = append(checks, check{
			Name:   "config_dir",
			Status: "ok",
			Detail: configDir,
		})
	}

	// 3. ffmpeg (useful for audio conversion)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		checks = append(checks, check{
			Name:   "ffmpeg",
			Status: "warning",
			Detail: "ffmpeg not found (optional, for audio format conversion)",
		})
	} else {
		checks = append(checks, check{
			Name:   "ffmpeg",
			Status: "ok",
			Detail: "available",
		})
	}

	// 4. Check each engine
	caps := engine.List()
	hasAvailable := false
	for _, cap := range caps {
		status := "not_available"
		detail := ""

		if cap.Installed {
			status = "ok"
			detail = "installed and ready"
			hasAvailable = true
		} else if cap.NeedsAPIKey {
			detail = "API key not set"
		} else if cap.NeedsModel {
			detail = "not installed"
		} else {
			detail = "not configured"
		}

		checks = append(checks, check{
			Name:   "engine:" + cap.Name,
			Status: status,
			Detail: detail,
		})
	}

	// 5. Overall readiness
	if hasAvailable {
		checks = append(checks, check{
			Name:   "overall",
			Status: "ok",
			Detail: "at least one engine is available",
		})
	} else {
		checks = append(checks, check{
			Name:   "overall",
			Status: "error",
			Detail: "no engine available; run 'asr-claw engines install <engine>' or set API keys",
		})
	}

	w.WriteSuccess(map[string]interface{}{
		"version": Version,
		"checks":  checks,
	})
	return nil
}
