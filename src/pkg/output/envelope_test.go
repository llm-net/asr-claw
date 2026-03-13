package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriter_WriteSuccess_JSON(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter("json", "test", false)
	w.out = &buf

	w.WriteSuccess(map[string]string{"key": "value"})

	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !env.OK {
		t.Error("expected ok=true")
	}
	if env.Command != "test" {
		t.Errorf("command: got %q, want %q", env.Command, "test")
	}
	if env.Error != nil {
		t.Error("expected nil error")
	}
}

func TestWriter_WriteError_JSON(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter("json", "test", false)
	w.out = &buf

	w.WriteError("TEST_ERROR", "something failed", "try again")

	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.OK {
		t.Error("expected ok=false")
	}
	if env.Error == nil {
		t.Fatal("expected non-nil error")
	}
	if env.Error.Code != "TEST_ERROR" {
		t.Errorf("error code: got %q, want %q", env.Error.Code, "TEST_ERROR")
	}
	if env.Error.Suggestion != "try again" {
		t.Errorf("suggestion: got %q, want %q", env.Error.Suggestion, "try again")
	}
}

func TestWriter_WriteSuccess_Text(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter("text", "test", false)
	w.out = &buf

	w.WriteSuccess("hello world")

	if strings.TrimSpace(buf.String()) != "hello world" {
		t.Errorf("text output: got %q, want %q", buf.String(), "hello world\n")
	}
}

func TestWriter_WriteSuccess_Quiet(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter("quiet", "test", false)
	w.out = &buf

	w.WriteSuccess("should not appear")

	if buf.Len() != 0 {
		t.Errorf("quiet mode produced output: %q", buf.String())
	}
}

func TestWriter_Verbose(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter("json", "test", true)
	w.errOut = &buf

	w.Verbose("debug %s", "info")

	if !strings.Contains(buf.String(), "[DEBUG]") {
		t.Error("verbose output missing [DEBUG] prefix")
	}
	if !strings.Contains(buf.String(), "debug info") {
		t.Error("verbose output missing message")
	}
}

func TestWriter_Verbose_Disabled(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter("json", "test", false)
	w.errOut = &buf

	w.Verbose("should not appear")

	if buf.Len() != 0 {
		t.Errorf("non-verbose mode produced debug output: %q", buf.String())
	}
}
