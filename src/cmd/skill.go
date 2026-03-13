package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(skillCmd)
}

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Output skill.json for Claude Code / OpenClaw integration",
	RunE:  runSkill,
}

func runSkill(cmd *cobra.Command, args []string) error {
	skill := map[string]interface{}{
		"name":        "asr-claw",
		"version":     Version,
		"description": "Speech recognition CLI for AI agent automation. Transcribe audio streams from stdin, files, or URLs with multiple ASR engines.",
		"commands": []map[string]interface{}{
			{
				"name":        "transcribe",
				"description": "Transcribe audio to text",
				"usage":       "asr-claw transcribe [--file <path>] [--stream] [--lang <code>] [--engine <name>] [--format <fmt>]",
			},
			{
				"name":        "engines list",
				"description": "List available ASR engines",
				"usage":       "asr-claw engines list",
			},
			{
				"name":        "engines install",
				"description": "Install an ASR engine",
				"usage":       "asr-claw engines install <engine>",
			},
			{
				"name":        "engines start",
				"description": "Start a service engine",
				"usage":       "asr-claw engines start <engine>",
			},
			{
				"name":        "engines stop",
				"description": "Stop a service engine",
				"usage":       "asr-claw engines stop <engine>",
			},
			{
				"name":        "engines status",
				"description": "Show running engine status",
				"usage":       "asr-claw engines status",
			},
			{
				"name":        "engines info",
				"description": "Show engine details",
				"usage":       "asr-claw engines info <engine>",
			},
			{
				"name":        "doctor",
				"description": "Check environment and engine availability",
				"usage":       "asr-claw doctor",
			},
		},
		"supported_engines": []string{
			"qwen-asr", "qwen3-asr", "whisper", "doubao", "openai", "deepgram",
		},
	}

	b, _ := json.MarshalIndent(skill, "", "  ")
	fmt.Println(string(b))
	return nil
}
