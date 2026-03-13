package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/llm-net/asr-claw/pkg/audio"
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

var (
	filePath   string
	streamMode bool
	lang       string
	engineName string
	format     string
	chunkSec   float64
	inputRate  int
	inputBits  int
)

func init() {
	transcribeCmd.Flags().StringVar(&filePath, "file", "", "input audio file path")
	transcribeCmd.Flags().BoolVar(&streamMode, "stream", false, "streaming mode (process stdin in real-time)")
	transcribeCmd.Flags().StringVar(&lang, "lang", "zh", "language code (zh, en, ja, etc.)")
	transcribeCmd.Flags().StringVar(&engineName, "engine", "", "ASR engine name (auto-select if empty)")
	transcribeCmd.Flags().StringVar(&format, "format", "", "output format: json | text | srt | vtt (overrides -o for data)")
	transcribeCmd.Flags().Float64Var(&chunkSec, "chunk", 0, "fixed-time chunk seconds (fallback, disables VAD)")
	transcribeCmd.Flags().IntVar(&inputRate, "rate", 16000, "input sample rate for raw PCM")
	transcribeCmd.Flags().IntVar(&inputBits, "bits", 16, "input bits per sample for raw PCM")

	rootCmd.AddCommand(transcribeCmd)
}

var transcribeCmd = &cobra.Command{
	Use:   "transcribe",
	Short: "Transcribe audio to text",
	Long:  "Transcribe audio from file or stdin. Supports streaming mode for real-time transcription.",
	RunE:  runTranscribe,
}

func runTranscribe(cmd *cobra.Command, args []string) error {
	w := output.NewWriter(outputMode, "transcribe", verbose)

	// Select engine
	var eng engine.Engine
	var err error
	if engineName != "" {
		eng, err = engine.Get(engineName)
		if err != nil {
			w.WriteError("ENGINE_NOT_FOUND", err.Error(),
				fmt.Sprintf("run 'asr-claw engines list' to see available engines"))
			return nil
		}
	} else {
		eng, err = engine.AutoSelect()
		if err != nil {
			w.WriteError("NO_ENGINE", err.Error(),
				"run 'asr-claw engines install <engine>' to install one")
			return nil
		}
	}

	cap := eng.Info()
	if !cap.Installed {
		w.WriteError("ENGINE_NOT_INSTALLED",
			fmt.Sprintf("engine '%s' is not installed or not available", cap.Name),
			fmt.Sprintf("run 'asr-claw engines install %s' to install", cap.Name))
		return nil
	}

	w.Verbose("using engine: %s (%s)", cap.Name, cap.Type)

	// Determine input source
	var reader io.Reader
	if filePath != "" {
		f, err := os.Open(filePath)
		if err != nil {
			w.WriteError("INPUT_ERROR", fmt.Sprintf("cannot open file: %s", err.Error()), "")
			return nil
		}
		defer f.Close()
		reader = f
	} else {
		reader = os.Stdin
	}

	if streamMode {
		return runStreamTranscribe(reader, eng, w)
	}
	return runFileTranscribe(reader, eng, w)
}

// runFileTranscribe handles non-streaming transcription (file or stdin pipe).
func runFileTranscribe(r io.Reader, eng engine.Engine, w *output.Writer) error {
	// Read all input
	data, err := io.ReadAll(r)
	if err != nil {
		w.WriteError("INPUT_ERROR", fmt.Sprintf("read input failed: %s", err.Error()), "")
		return nil
	}

	if len(data) == 0 {
		w.WriteError("INPUT_ERROR", "no input data", "provide audio via --file or stdin")
		return nil
	}

	// Detect WAV header
	sampleRate := inputRate
	pcmData := data
	if len(data) >= 44 {
		header, _, _ := audio.DetectWAV(strings.NewReader(string(data[:44])))
		if header != nil {
			sampleRate = header.SampleRate
			pcmData = data[44:] // skip WAV header
			w.Verbose("WAV detected: %dHz %dbit %dch streaming=%v",
				header.SampleRate, header.BitsPerSample, header.Channels, header.IsStreaming)
		}
	}

	// Resample if needed
	engineRate := eng.Info().SampleRate
	if engineRate > 0 && sampleRate != engineRate {
		w.Verbose("resampling %dHz -> %dHz", sampleRate, engineRate)
		pcmData = audio.Resample(pcmData, sampleRate, engineRate)
		sampleRate = engineRate
	}

	// Write temp WAV file for engine
	tmpFile, err := os.CreateTemp("", "asr-claw-*.wav")
	if err != nil {
		w.WriteError("INTERNAL_ERROR", err.Error(), "")
		return nil
	}
	defer os.Remove(tmpFile.Name())

	if err := audio.WriteWAV(tmpFile, pcmData, sampleRate); err != nil {
		tmpFile.Close()
		w.WriteError("INTERNAL_ERROR", fmt.Sprintf("write temp WAV: %s", err.Error()), "")
		return nil
	}
	tmpFile.Close()

	// Transcribe
	segments, err := eng.TranscribeFile(tmpFile.Name(), lang)
	if err != nil {
		w.WriteError("TRANSCRIBE_FAILED", err.Error(), "")
		return nil
	}

	// Build output
	audioDuration := audio.PCMDuration(len(pcmData), sampleRate).Seconds()
	result := buildTranscribeResult(segments, eng.Info().Name, audioDuration)

	if format == "srt" {
		fmt.Print(formatSRT(segments))
	} else if format == "vtt" {
		fmt.Print(formatVTT(segments))
	} else if format == "text" {
		for _, seg := range segments {
			fmt.Println(seg.Text)
		}
	} else {
		w.WriteSuccess(result)
	}

	return nil
}

// runStreamTranscribe handles streaming transcription.
func runStreamTranscribe(r io.Reader, eng engine.Engine, w *output.Writer) error {
	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Check for native streaming engine
	if streamEng, ok := eng.(engine.StreamEngine); ok && eng.Info().NativeStream {
		return runNativeStreamTranscribe(ctx, r, streamEng, w)
	}

	// VAD or chunk-based streaming for CLI engines
	return runVADStreamTranscribe(ctx, r, eng, w)
}

// runVADStreamTranscribe implements VAD-segmented streaming for CLI engines.
func runVADStreamTranscribe(ctx context.Context, r io.Reader, eng engine.Engine, w *output.Writer) error {
	// Detect WAV header
	header, consumed, err := audio.DetectWAV(r)
	if err != nil {
		w.WriteError("INPUT_ERROR", fmt.Sprintf("read input failed: %s", err.Error()), "")
		return nil
	}

	sampleRate := inputRate
	if header != nil {
		sampleRate = header.SampleRate
		w.Verbose("WAV detected: %dHz streaming=%v", header.SampleRate, header.IsStreaming)
	}

	// Choose segmenter: VAD (default) or fixed chunk (--chunk N)
	type segmenter interface {
		Feed(frame []byte) *audio.AudioSegment
		Flush() *audio.AudioSegment
		FrameBytes() int
	}

	var seg segmenter
	if chunkSec > 0 {
		w.Verbose("using fixed-time chunking: %.1fs", chunkSec)
		seg = audio.NewChunkSegmenter(sampleRate, chunkSec)
	} else {
		w.Verbose("using VAD segmentation")
		seg = audio.NewVADSegmenter(sampleRate)
	}

	frameBytes := seg.FrameBytes()
	frame := make([]byte, frameBytes)
	segIndex := 0

	// Process any consumed bytes that weren't WAV header
	if consumed != nil && header == nil {
		// Not WAV, need to process consumed bytes as PCM
		for i := 0; i+frameBytes <= len(consumed); i += frameBytes {
			copy(frame, consumed[i:i+frameBytes])
			if audioSeg := seg.Feed(frame); audioSeg != nil {
				processAndOutputSegment(audioSeg, segIndex, eng, w)
				segIndex++
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown: flush remaining
			if audioSeg := seg.Flush(); audioSeg != nil {
				processAndOutputSegment(audioSeg, segIndex, eng, w)
			}
			return nil
		default:
		}

		_, err := io.ReadFull(r, frame)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			// Stream ended, flush remaining
			if audioSeg := seg.Flush(); audioSeg != nil {
				processAndOutputSegment(audioSeg, segIndex, eng, w)
			}
			return nil
		}
		if err != nil {
			w.WriteError("INPUT_ERROR", fmt.Sprintf("read PCM frame failed: %s", err.Error()), "")
			return nil
		}

		if audioSeg := seg.Feed(frame); audioSeg != nil {
			processAndOutputSegment(audioSeg, segIndex, eng, w)
			segIndex++
		}
	}
}

// runNativeStreamTranscribe uses the engine's native streaming via Session.
func runNativeStreamTranscribe(ctx context.Context, r io.Reader, eng engine.StreamEngine, w *output.Writer) error {
	header, _, err := audio.DetectWAV(r)
	if err != nil {
		w.WriteError("INPUT_ERROR", fmt.Sprintf("read input failed: %s", err.Error()), "")
		return nil
	}

	sampleRate := inputRate
	if header != nil {
		sampleRate = header.SampleRate
	}

	session, err := eng.StreamSession(engine.Options{
		Lang:          lang,
		SampleRate:    sampleRate,
		Channels:      1,
		BitsPerSample: 16,
	})
	if err != nil {
		w.WriteError("STREAM_ERROR", fmt.Sprintf("create stream session failed: %s", err.Error()), "")
		return nil
	}
	defer session.Close()

	// Read in 500ms chunks and feed to session
	chunkBytes := sampleRate * 2 / 2 // 500ms @ 16bit mono
	buf := make([]byte, chunkBytes)

	for {
		select {
		case <-ctx.Done():
			finalText, _ := session.Finish()
			if finalText != "" {
				w.WriteStreamText(finalText)
			}
			return nil
		default:
		}

		n, err := io.ReadFull(r, buf)
		if n > 0 {
			text, ferr := session.Feed(buf[:n])
			if ferr != nil {
				w.WriteError("STREAM_ERROR", ferr.Error(), "")
				return nil
			}
			if text != "" {
				w.WriteStreamText(text)
			}
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			w.WriteError("INPUT_ERROR", err.Error(), "")
			return nil
		}
	}

	finalText, err := session.Finish()
	if err != nil {
		w.WriteError("STREAM_ERROR", err.Error(), "")
		return nil
	}
	if finalText != "" {
		w.WriteStreamText(finalText)
	}

	return nil
}

// processAndOutputSegment writes a segment to temp WAV, transcribes, and outputs.
func processAndOutputSegment(seg *audio.AudioSegment, index int, eng engine.Engine, w *output.Writer) {
	sampleRate := eng.Info().SampleRate
	if sampleRate == 0 {
		sampleRate = 16000
	}

	// Write temp WAV
	tmpFile, err := os.CreateTemp("", "asr-claw-seg-*.wav")
	if err != nil {
		w.WriteError("INTERNAL_ERROR", err.Error(), "")
		return
	}
	defer os.Remove(tmpFile.Name())

	if err := audio.WriteWAV(tmpFile, seg.PCM, sampleRate); err != nil {
		tmpFile.Close()
		w.WriteError("INTERNAL_ERROR", err.Error(), "")
		return
	}
	tmpFile.Close()

	// Transcribe
	segments, err := eng.TranscribeFile(tmpFile.Name(), lang)
	if err != nil {
		w.WriteError("TRANSCRIBE_ERROR", err.Error(), "")
		return
	}

	// Adjust timestamps and output
	for i, s := range segments {
		s.Index = index
		s.Start += seg.Start.Seconds()
		s.End += seg.Start.Seconds()
		segments[i] = s
	}

	if format == "text" {
		for _, s := range segments {
			fmt.Println(s.Text)
		}
	} else if format == "srt" {
		for _, s := range segments {
			fmt.Printf("%d\n%s --> %s\n%s\n\n",
				s.Index+1,
				formatSRTTime(s.Start),
				formatSRTTime(s.End),
				s.Text)
		}
	} else if format == "vtt" {
		for _, s := range segments {
			fmt.Printf("%s --> %s\n%s\n\n",
				formatVTTTime(s.Start),
				formatVTTTime(s.End),
				s.Text)
		}
	} else {
		// JSON Lines (default for streaming)
		for _, s := range segments {
			w.WriteStreamSegment(s)
		}
	}
}

// buildTranscribeResult builds the data payload for non-streaming output.
func buildTranscribeResult(segments []engine.Segment, engineName string, audioDuration float64) map[string]interface{} {
	var fullText strings.Builder
	for _, s := range segments {
		fullText.WriteString(s.Text)
	}

	return map[string]interface{}{
		"segments":          segments,
		"full_text":         fullText.String(),
		"engine":            engineName,
		"audio_duration_sec": audioDuration,
	}
}

// formatSRT converts segments to SRT format.
func formatSRT(segments []engine.Segment) string {
	var b strings.Builder
	for _, s := range segments {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n",
			s.Index+1,
			formatSRTTime(s.Start),
			formatSRTTime(s.End),
			s.Text)
	}
	return b.String()
}

// formatVTT converts segments to WebVTT format.
func formatVTT(segments []engine.Segment) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for _, s := range segments {
		fmt.Fprintf(&b, "%s --> %s\n%s\n\n",
			formatVTTTime(s.Start),
			formatVTTTime(s.End),
			s.Text)
	}
	return b.String()
}

func formatSRTTime(sec float64) string {
	d := time.Duration(sec * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func formatVTTTime(sec float64) string {
	d := time.Duration(sec * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
