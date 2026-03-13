package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// Envelope is the standard JSON response wrapper.
type Envelope struct {
	OK         bool        `json:"ok"`
	Command    string      `json:"command"`
	Data       interface{} `json:"data,omitempty"`
	Error      *ErrorInfo  `json:"error,omitempty"`
	DurationMs int64       `json:"duration_ms"`
	Timestamp  string      `json:"timestamp"`
}

// ErrorInfo describes an error with code and suggestion.
type ErrorInfo struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// Writer handles output in json/text/quiet modes.
type Writer struct {
	out     io.Writer
	errOut  io.Writer
	mode    string // "json", "text", "quiet"
	command string
	start   time.Time
	verbose bool
}

// NewWriter creates a Writer for the given output mode.
func NewWriter(mode, command string, verbose bool) *Writer {
	return &Writer{
		out:     os.Stdout,
		errOut:  os.Stderr,
		mode:    mode,
		command: command,
		start:   time.Now(),
		verbose: verbose,
	}
}

// WriteSuccess outputs a success envelope.
func (w *Writer) WriteSuccess(data interface{}) {
	switch w.mode {
	case "text":
		w.writeText(data)
	case "quiet":
		// no output
	default:
		w.writeJSON(&Envelope{
			OK:         true,
			Command:    w.command,
			Data:       data,
			DurationMs: time.Since(w.start).Milliseconds(),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// WriteError outputs an error envelope.
func (w *Writer) WriteError(code, message, suggestion string) {
	switch w.mode {
	case "text":
		fmt.Fprintf(w.errOut, "error: %s — %s\n", code, message)
		if suggestion != "" {
			fmt.Fprintf(w.errOut, "suggestion: %s\n", suggestion)
		}
	case "quiet":
		// no output
	default:
		w.writeJSON(&Envelope{
			OK:      false,
			Command: w.command,
			Error: &ErrorInfo{
				Code:       code,
				Message:    message,
				Suggestion: suggestion,
			},
			DurationMs: time.Since(w.start).Milliseconds(),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// WriteStreamSegment outputs a single segment in JSON Lines format (streaming).
func (w *Writer) WriteStreamSegment(data interface{}) {
	switch w.mode {
	case "text":
		w.writeText(data)
	case "quiet":
		// no output
	default:
		b, _ := json.Marshal(data)
		fmt.Fprintln(w.out, string(b))
	}
}

// WriteStreamText outputs incremental text (for native streaming engines).
func (w *Writer) WriteStreamText(text string) {
	switch w.mode {
	case "text":
		fmt.Fprintln(w.out, text)
	case "quiet":
		// no output
	default:
		b, _ := json.Marshal(map[string]string{"text": text})
		fmt.Fprintln(w.out, string(b))
	}
}

// Verbose writes debug output to stderr if verbose mode is enabled.
func (w *Writer) Verbose(format string, args ...interface{}) {
	if w.verbose {
		fmt.Fprintf(w.errOut, "[DEBUG] "+format+"\n", args...)
	}
}

func (w *Writer) writeJSON(env *Envelope) {
	enc := json.NewEncoder(w.out)
	enc.SetIndent("", "  ")
	enc.Encode(env)
}

func (w *Writer) writeText(data interface{}) {
	switch v := data.(type) {
	case string:
		fmt.Fprintln(w.out, v)
	case map[string]interface{}:
		if text, ok := v["full_text"]; ok {
			fmt.Fprintln(w.out, text)
		} else if segments, ok := v["segments"]; ok {
			if segs, ok := segments.([]interface{}); ok {
				for _, seg := range segs {
					if m, ok := seg.(map[string]interface{}); ok {
						fmt.Fprintln(w.out, m["text"])
					}
				}
			}
		} else {
			b, _ := json.Marshal(v)
			fmt.Fprintln(w.out, string(b))
		}
	default:
		b, _ := json.Marshal(v)
		fmt.Fprintln(w.out, string(b))
	}
}
