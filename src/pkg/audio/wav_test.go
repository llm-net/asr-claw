package audio

import (
	"bytes"
	"testing"
)

func TestDetectWAV_ValidHeader(t *testing.T) {
	// Build a valid 44-byte WAV header
	pcm := make([]byte, 100)
	var buf bytes.Buffer
	if err := WriteWAV(&buf, pcm, 16000); err != nil {
		t.Fatalf("WriteWAV failed: %v", err)
	}

	header, consumed, err := DetectWAV(&buf)
	if err != nil {
		t.Fatalf("DetectWAV error: %v", err)
	}
	if consumed != nil {
		t.Fatalf("expected nil consumed bytes for valid WAV")
	}
	if header == nil {
		t.Fatal("expected non-nil header")
	}
	if header.SampleRate != 16000 {
		t.Errorf("sample rate: got %d, want 16000", header.SampleRate)
	}
	if header.BitsPerSample != 16 {
		t.Errorf("bits per sample: got %d, want 16", header.BitsPerSample)
	}
	if header.Channels != 1 {
		t.Errorf("channels: got %d, want 1", header.Channels)
	}
	if header.IsStreaming {
		t.Error("expected non-streaming header")
	}
}

func TestDetectWAV_StreamingHeader(t *testing.T) {
	// Build streaming WAV header (data_size = 0x7FFFFFFF)
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	header[4] = 0xFF
	header[5] = 0xFF
	header[6] = 0xFF
	header[7] = 0x7F
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	header[16] = 16 // subchunk size
	header[20] = 1  // PCM
	header[22] = 1  // mono
	// 16000 sample rate
	header[24] = 0x80
	header[25] = 0x3E
	// byte rate = 32000
	header[28] = 0x00
	header[29] = 0x7D
	// block align = 2
	header[32] = 2
	// bits per sample = 16
	header[34] = 16
	copy(header[36:40], "data")
	header[40] = 0xFF
	header[41] = 0xFF
	header[42] = 0xFF
	header[43] = 0x7F

	r := bytes.NewReader(header)
	h, _, err := DetectWAV(r)
	if err != nil {
		t.Fatalf("DetectWAV error: %v", err)
	}
	if h == nil {
		t.Fatal("expected non-nil header")
	}
	if !h.IsStreaming {
		t.Error("expected streaming header")
	}
}

func TestDetectWAV_NotWAV(t *testing.T) {
	data := make([]byte, 44)
	copy(data, "NOT_A_WAV_FILE_AT_ALL_REALLY_NOT_WAV_HEADER")

	r := bytes.NewReader(data)
	h, consumed, err := DetectWAV(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h != nil {
		t.Error("expected nil header for non-WAV data")
	}
	if consumed == nil {
		t.Error("expected consumed bytes for non-WAV data")
	}
}

func TestWriteWAV(t *testing.T) {
	pcm := make([]byte, 32000) // 1 second of 16kHz 16-bit mono
	var buf bytes.Buffer

	err := WriteWAV(&buf, pcm, 16000)
	if err != nil {
		t.Fatalf("WriteWAV error: %v", err)
	}

	// Should be 44 header + PCM data
	expected := 44 + len(pcm)
	if buf.Len() != expected {
		t.Errorf("output size: got %d, want %d", buf.Len(), expected)
	}

	// Verify header markers
	data := buf.Bytes()
	if string(data[0:4]) != "RIFF" {
		t.Error("missing RIFF marker")
	}
	if string(data[8:12]) != "WAVE" {
		t.Error("missing WAVE marker")
	}
	if string(data[36:40]) != "data" {
		t.Error("missing data marker")
	}
}
