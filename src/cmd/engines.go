package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

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

var enginesCmd = &cobra.Command{
	Use:   "engines",
	Short: "Manage ASR engines",
}

func init() {
	rootCmd.AddCommand(enginesCmd)

	enginesCmd.AddCommand(enginesListCmd)
	enginesCmd.AddCommand(enginesInstallCmd)
	enginesCmd.AddCommand(enginesStartCmd)
	enginesCmd.AddCommand(enginesStopCmd)
	enginesCmd.AddCommand(enginesStatusCmd)
	enginesCmd.AddCommand(enginesInfoCmd)
}

// --- engines list ---

var enginesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available engines and their status",
	RunE:  runEnginesList,
}

func runEnginesList(cmd *cobra.Command, args []string) error {
	w := output.NewWriter(outputMode, "engines list", verbose)

	caps := engine.List()

	type engineEntry struct {
		Name         string `json:"name"`
		Type         string `json:"type"`
		Installed    bool   `json:"installed"`
		NativeStream bool   `json:"native_stream"`
		Status       string `json:"status,omitempty"`
		Note         string `json:"note,omitempty"`
	}

	entries := make([]engineEntry, len(caps))
	for i, cap := range caps {
		entry := engineEntry{
			Name:         cap.Name,
			Type:         cap.Type,
			Installed:    cap.Installed,
			NativeStream: cap.NativeStream,
		}

		if !cap.Installed {
			switch {
			case cap.NeedsAPIKey:
				entry.Status = "no_api_key"
				envVar := strings.ToUpper(strings.ReplaceAll(cap.Name, "-", "_")) + "_API_KEY"
				entry.Note = fmt.Sprintf("set %s environment variable", envVar)
			case cap.NeedsModel:
				entry.Status = "not_installed"
				entry.Note = fmt.Sprintf("run 'asr-claw engines install %s'", cap.Name)
			default:
				entry.Status = "not_configured"
			}
		} else {
			entry.Status = "ready"
		}

		entries[i] = entry
	}

	w.WriteSuccess(map[string]interface{}{
		"engines": entries,
	})
	return nil
}

// --- engines install ---

var installModel string

var enginesInstallCmd = &cobra.Command{
	Use:   "install <engine>",
	Short: "Install an engine (download binary and model)",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnginesInstall,
}

func init() {
	enginesInstallCmd.Flags().StringVar(&installModel, "model", "", "model variant (e.g., large-v3, base)")
}

func runEnginesInstall(cmd *cobra.Command, args []string) error {
	w := output.NewWriter(outputMode, "engines install", verbose)
	name := args[0]

	eng, err := engine.Get(name)
	if err != nil {
		w.WriteError("ENGINE_NOT_FOUND", err.Error(),
			fmt.Sprintf("available engines: %s", strings.Join(engine.Names(), ", ")))
		return nil
	}

	cap := eng.Info()
	if cap.Type == "cloud" {
		w.WriteError("INSTALL_NOT_NEEDED",
			fmt.Sprintf("engine '%s' is a cloud API — no installation needed", name),
			fmt.Sprintf("set the API key environment variable to use it"))
		return nil
	}

	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".asr-claw")

	// Create directories
	for _, dir := range []string{
		filepath.Join(baseDir, "bin"),
		filepath.Join(baseDir, "models", name),
		filepath.Join(baseDir, "cache", "segments"),
	} {
		os.MkdirAll(dir, 0755)
	}

	platform := runtime.GOOS + "-" + runtime.GOARCH
	w.Verbose("platform: %s", platform)
	w.Verbose("install dir: %s", baseDir)

	// Engine-specific install logic
	switch name {
	case "qwen-asr":
		return installQwenASR(w, baseDir, platform)
	case "whisper":
		w.WriteSuccess(map[string]interface{}{
			"engine":  name,
			"status":  "install_instructions",
			"message": "whisper.cpp requires manual installation",
			"steps": []string{
				"1. Download whisper.cpp binary from https://github.com/ggerganov/whisper.cpp/releases",
				fmt.Sprintf("2. Place binary at %s", filepath.Join(baseDir, "bin", "whisper-cpp")),
				fmt.Sprintf("3. Download model to %s", filepath.Join(baseDir, "models", "whisper")),
				"4. Run: asr-claw engines list  # verify installation",
			},
		})
	case "qwen3-asr":
		w.WriteSuccess(map[string]interface{}{
			"engine":  name,
			"status":  "install_instructions",
			"message": "qwen3-asr requires vLLM with GPU",
			"steps": []string{
				"1. Install vLLM: pip install vllm",
				"2. Download model: huggingface-cli download Qwen/Qwen3-ASR-0.6B",
				"3. Start vLLM: vllm serve Qwen/Qwen3-ASR-0.6B --port 8000",
				"4. Or: asr-claw engines start qwen3-asr",
			},
		})
	default:
		w.WriteError("INSTALL_NOT_SUPPORTED",
			fmt.Sprintf("automated install for '%s' is not yet supported", name),
			"check the documentation for manual installation steps")
	}

	return nil
}

// --- engines start ---

var enginesStartCmd = &cobra.Command{
	Use:   "start <engine>",
	Short: "Start a service-type engine",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnginesStart,
}

func runEnginesStart(cmd *cobra.Command, args []string) error {
	w := output.NewWriter(outputMode, "engines start", verbose)
	name := args[0]

	eng, err := engine.Get(name)
	if err != nil {
		w.WriteError("ENGINE_NOT_FOUND", err.Error(), "")
		return nil
	}

	cap := eng.Info()
	if cap.Type != "service" {
		w.WriteError("NOT_SERVICE_ENGINE",
			fmt.Sprintf("engine '%s' is type '%s', not a service engine", name, cap.Type),
			"only service engines can be started/stopped")
		return nil
	}

	home, _ := os.UserHomeDir()
	runDir := filepath.Join(home, ".asr-claw", "run")
	os.MkdirAll(runDir, 0755)
	pidFile := filepath.Join(runDir, name+".pid")

	// Check if already running
	if pid, err := readPIDFile(pidFile); err == nil {
		if processExists(pid) {
			w.WriteError("ALREADY_RUNNING",
				fmt.Sprintf("engine '%s' is already running (PID %d)", name, pid),
				fmt.Sprintf("run 'asr-claw engines stop %s' first", name))
			return nil
		}
	}

	// Start vLLM for qwen3-asr
	switch name {
	case "qwen3-asr":
		endpoint := os.Getenv("QWEN3_ASR_ENDPOINT")
		if endpoint == "" {
			endpoint = "http://localhost:8000"
		}

		vllmCmd := exec.Command("vllm", "serve", "Qwen/Qwen3-ASR", "--port", "8000")
		vllmCmd.Stdout = os.Stderr
		vllmCmd.Stderr = os.Stderr

		if err := vllmCmd.Start(); err != nil {
			w.WriteError("START_FAILED",
				fmt.Sprintf("failed to start vLLM: %s", err.Error()),
				"ensure vLLM is installed: pip install vllm")
			return nil
		}

		// Write PID file
		os.WriteFile(pidFile, []byte(strconv.Itoa(vllmCmd.Process.Pid)), 0644)

		w.WriteSuccess(map[string]interface{}{
			"engine":   name,
			"status":   "starting",
			"pid":      vllmCmd.Process.Pid,
			"endpoint": endpoint,
			"message":  "vLLM is starting, wait for model loading to complete",
		})

		// Detach process
		go vllmCmd.Wait()
	default:
		w.WriteError("START_NOT_SUPPORTED",
			fmt.Sprintf("starting engine '%s' is not supported", name), "")
	}

	return nil
}

// --- engines stop ---

var enginesStopCmd = &cobra.Command{
	Use:   "stop <engine>",
	Short: "Stop a running service engine",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnginesStop,
}

func runEnginesStop(cmd *cobra.Command, args []string) error {
	w := output.NewWriter(outputMode, "engines stop", verbose)
	name := args[0]

	home, _ := os.UserHomeDir()
	pidFile := filepath.Join(home, ".asr-claw", "run", name+".pid")

	pid, err := readPIDFile(pidFile)
	if err != nil {
		w.WriteError("NOT_RUNNING",
			fmt.Sprintf("engine '%s' is not running (no PID file)", name), "")
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil || !processExists(pid) {
		os.Remove(pidFile)
		w.WriteError("NOT_RUNNING",
			fmt.Sprintf("engine '%s' process (PID %d) is not running", name, pid), "")
		return nil
	}

	// Send SIGTERM for graceful shutdown
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		w.WriteError("STOP_FAILED",
			fmt.Sprintf("failed to stop engine '%s': %s", name, err.Error()), "")
		return nil
	}

	// Wait up to 10 seconds
	done := make(chan struct{})
	go func() {
		proc.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Stopped gracefully
	case <-time.After(10 * time.Second):
		proc.Kill()
	}

	os.Remove(pidFile)

	w.WriteSuccess(map[string]interface{}{
		"engine":  name,
		"status":  "stopped",
		"pid":     pid,
		"message": fmt.Sprintf("engine '%s' stopped", name),
	})
	return nil
}

// --- engines status ---

var enginesStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of running engines",
	RunE:  runEnginesStatus,
}

func runEnginesStatus(cmd *cobra.Command, args []string) error {
	w := output.NewWriter(outputMode, "engines status", verbose)

	home, _ := os.UserHomeDir()
	runDir := filepath.Join(home, ".asr-claw", "run")

	type engineStatus struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		PID    int    `json:"pid,omitempty"`
	}

	var statuses []engineStatus

	entries, _ := os.ReadDir(runDir)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".pid") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".pid")
		pidFile := filepath.Join(runDir, entry.Name())

		pid, err := readPIDFile(pidFile)
		if err != nil {
			continue
		}

		if processExists(pid) {
			statuses = append(statuses, engineStatus{
				Name:   name,
				Status: "running",
				PID:    pid,
			})
		} else {
			os.Remove(pidFile)
			statuses = append(statuses, engineStatus{
				Name:   name,
				Status: "stopped (stale PID file cleaned)",
			})
		}
	}

	if len(statuses) == 0 {
		statuses = []engineStatus{} // ensure empty array, not null
	}

	w.WriteSuccess(map[string]interface{}{
		"running_engines": statuses,
	})
	return nil
}

// --- engines info ---

var enginesInfoCmd = &cobra.Command{
	Use:   "info <engine>",
	Short: "Show detailed engine information",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnginesInfo,
}

func runEnginesInfo(cmd *cobra.Command, args []string) error {
	w := output.NewWriter(outputMode, "engines info", verbose)
	name := args[0]

	eng, err := engine.Get(name)
	if err != nil {
		w.WriteError("ENGINE_NOT_FOUND", err.Error(),
			fmt.Sprintf("available engines: %s", strings.Join(engine.Names(), ", ")))
		return nil
	}

	cap := eng.Info()

	home, _ := os.UserHomeDir()
	info := map[string]interface{}{
		"name":          cap.Name,
		"type":          cap.Type,
		"installed":     cap.Installed,
		"native_stream": cap.NativeStream,
		"connection":    cap.Connection,
		"sample_rate":   cap.SampleRate,
		"languages":     cap.Languages,
		"needs_model":   cap.NeedsModel,
		"needs_api_key": cap.NeedsAPIKey,
	}

	// Add paths for CLI engines
	if cap.Type == "cli" {
		switch name {
		case "qwen-asr":
			info["binary_path"] = filepath.Join(home, ".asr-claw", "bin", "qwen-asr")
			info["model_path"] = filepath.Join(home, ".asr-claw", "models", "Qwen3-ASR-0.6B")
			info["model"] = "Qwen/Qwen3-ASR-0.6B"
			info["backend"] = "antirez/qwen-asr (C, Accelerate)"
		default:
			info["binary_path"] = filepath.Join(home, ".asr-claw", "bin", name)
			info["model_path"] = filepath.Join(home, ".asr-claw", "models", name)
		}
	}

	// Check running status for service engines
	if cap.Type == "service" {
		pidFile := filepath.Join(home, ".asr-claw", "run", name+".pid")
		if pid, err := readPIDFile(pidFile); err == nil && processExists(pid) {
			info["running"] = true
			info["pid"] = pid
		} else {
			info["running"] = false
		}
	}

	w.WriteSuccess(info)
	return nil
}

// --- qwen-asr install ---

func installQwenASR(w *output.Writer, baseDir, platform string) error {
	binDir := filepath.Join(baseDir, "bin")
	modelDir := filepath.Join(baseDir, "models", "Qwen3-ASR-0.6B")
	binaryPath := filepath.Join(binDir, "qwen-asr")

	// Check prerequisites
	for _, tool := range []string{"git", "make", "cc"} {
		if _, err := exec.LookPath(tool); err != nil {
			w.WriteError("MISSING_PREREQUISITE",
				fmt.Sprintf("'%s' not found in PATH", tool),
				fmt.Sprintf("install with: brew install %s", tool))
			return nil
		}
	}

	os.MkdirAll(binDir, 0755)
	os.MkdirAll(modelDir, 0755)

	// Step 1: Build binary
	binaryReady := false
	if _, err := os.Stat(binaryPath); err == nil {
		w.Verbose("binary already exists at %s", binaryPath)
		binaryReady = true
	}

	if !binaryReady {
		w.Verbose("cloning antirez/qwen-asr...")
		tmpDir, err := os.MkdirTemp("", "asr-claw-build-*")
		if err != nil {
			w.WriteError("INTERNAL_ERROR", err.Error(), "")
			return nil
		}
		defer os.RemoveAll(tmpDir)

		cloneCmd := exec.Command("git", "clone", "--depth", "1",
			"https://github.com/antirez/qwen-asr.git", filepath.Join(tmpDir, "qwen-asr"))
		cloneCmd.Stderr = os.Stderr
		if err := cloneCmd.Run(); err != nil {
			w.WriteError("CLONE_FAILED",
				fmt.Sprintf("git clone failed: %s", err.Error()),
				"check network and try again")
			return nil
		}

		// Build — use 'make blas' on macOS for Accelerate framework
		buildDir := filepath.Join(tmpDir, "qwen-asr")
		makeTarget := "blas"
		if runtime.GOOS != "darwin" {
			makeTarget = ""
		}

		var buildCmd *exec.Cmd
		if makeTarget != "" {
			buildCmd = exec.Command("make", makeTarget)
		} else {
			buildCmd = exec.Command("make")
		}
		buildCmd.Dir = buildDir
		buildCmd.Stderr = os.Stderr
		buildCmd.Stdout = os.Stderr
		if err := buildCmd.Run(); err != nil {
			w.WriteError("BUILD_FAILED",
				fmt.Sprintf("make failed: %s", err.Error()),
				"ensure Xcode command line tools are installed: xcode-select --install")
			return nil
		}

		// Find built binary (may be named qwen_asr or qwen-asr)
		var builtBinary string
		for _, name := range []string{"qwen_asr", "qwen-asr"} {
			p := filepath.Join(buildDir, name)
			if _, err := os.Stat(p); err == nil {
				builtBinary = p
				break
			}
		}
		if builtBinary == "" {
			w.WriteError("BUILD_FAILED",
				"build succeeded but cannot find binary",
				"check build output for errors")
			return nil
		}

		// Copy to bin
		data, err := os.ReadFile(builtBinary)
		if err != nil {
			w.WriteError("INTERNAL_ERROR", err.Error(), "")
			return nil
		}
		if err := os.WriteFile(binaryPath, data, 0755); err != nil {
			w.WriteError("INTERNAL_ERROR", err.Error(), "")
			return nil
		}
		w.Verbose("binary installed to %s", binaryPath)
	}

	// Step 2: Download model
	safetensors := filepath.Join(modelDir, "model.safetensors")
	modelReady := false
	if info, err := os.Stat(safetensors); err == nil && info.Size() > 1_000_000_000 {
		w.Verbose("model already exists at %s", modelDir)
		modelReady = true
	}

	if !modelReady {
		// Try huggingface-cli first
		if hfCli, err := exec.LookPath("huggingface-cli"); err == nil {
			w.Verbose("downloading model via huggingface-cli...")
			dlCmd := exec.Command(hfCli, "download", "Qwen/Qwen3-ASR-0.6B", "--local-dir", modelDir)
			dlCmd.Stdout = os.Stderr
			dlCmd.Stderr = os.Stderr
			if err := dlCmd.Run(); err == nil {
				w.Verbose("model downloaded via huggingface-cli")
				modelReady = true
			} else {
				w.Verbose("huggingface-cli failed, falling back to direct download")
			}
		}
	}

	if !modelReady {
		// Direct HTTP download from HuggingFace
		hfBase := os.Getenv("HF_MIRROR")
		if hfBase == "" {
			hfBase = "https://huggingface.co"
		}

		files := []string{
			"config.json",
			"generation_config.json",
			"merges.txt",
			"model.safetensors",
			"preprocessor_config.json",
			"tokenizer_config.json",
			"vocab.json",
			"chat_template.json",
		}

		curlPath, _ := exec.LookPath("curl")
		wgetPath, _ := exec.LookPath("wget")

		for _, file := range files {
			dest := filepath.Join(modelDir, file)
			if _, err := os.Stat(dest); err == nil {
				w.Verbose("already exists: %s", file)
				continue
			}

			url := fmt.Sprintf("%s/Qwen/Qwen3-ASR-0.6B/resolve/main/%s", hfBase, file)
			w.Verbose("downloading %s...", file)

			var dlCmd *exec.Cmd
			if curlPath != "" {
				dlCmd = exec.Command(curlPath, "-fSL", "--progress-bar", "-o", dest, url)
			} else if wgetPath != "" {
				dlCmd = exec.Command(wgetPath, "-q", "--show-progress", "-O", dest, url)
			} else {
				w.WriteError("NO_DOWNLOAD_TOOL",
					"neither curl nor wget found",
					"install curl: brew install curl")
				return nil
			}
			dlCmd.Stderr = os.Stderr
			dlCmd.Stdout = os.Stderr
			if err := dlCmd.Run(); err != nil {
				// Clean up partial download
				os.Remove(dest)
				w.WriteError("DOWNLOAD_FAILED",
					fmt.Sprintf("failed to download %s: %s", file, err.Error()),
					"check network; for China, set HF_MIRROR=https://hf-mirror.com")
				return nil
			}
		}
	}

	// Step 3: Write config if needed
	cfgPath := filepath.Join(baseDir, "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		cfgContent := fmt.Sprintf(`# asr-claw configuration
default:
  engine: qwen-asr
  lang: zh
  format: json

engines:
  qwen-asr:
    binary: %s
    model_path: %s
`, binaryPath, modelDir)
		os.WriteFile(cfgPath, []byte(cfgContent), 0644)
	}

	w.WriteSuccess(map[string]interface{}{
		"engine":      "qwen-asr",
		"status":      "installed",
		"binary":      binaryPath,
		"model_dir":   modelDir,
		"model":       "Qwen/Qwen3-ASR-0.6B",
		"config":      cfgPath,
		"next_steps": []string{
			"asr-claw engines list              # verify",
			"asr-claw transcribe --file audio.wav  # test",
		},
	})
	return nil
}

// --- helpers ---

func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
