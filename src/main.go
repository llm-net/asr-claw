package main

import (
	"os"

	"github.com/llm-net/asr-claw/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
