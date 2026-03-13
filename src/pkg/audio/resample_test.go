package audio

import (
	"testing"
)

func TestResample_SameRate(t *testing.T) {
	pcm := []byte{0x01, 0x02, 0x03, 0x04}
	out := Resample(pcm, 16000, 16000)
	if len(out) != len(pcm) {
		t.Errorf("same-rate resample changed length: got %d, want %d", len(out), len(pcm))
	}
}

func TestResample_Downsample(t *testing.T) {
	// 48kHz → 16kHz should produce ~1/3 the samples
	srcSamples := 4800 // 100ms at 48kHz
	pcm := make([]byte, srcSamples*2)
	for i := 0; i < srcSamples; i++ {
		sample := int16(i % 1000)
		pcm[i*2] = byte(sample)
		pcm[i*2+1] = byte(sample >> 8)
	}

	out := Resample(pcm, 48000, 16000)
	expectedSamples := 1600 // 100ms at 16kHz
	gotSamples := len(out) / 2

	if gotSamples != expectedSamples {
		t.Errorf("downsample: got %d samples, want %d", gotSamples, expectedSamples)
	}
}

func TestResample_Upsample(t *testing.T) {
	// 8kHz → 16kHz should produce ~2x the samples
	srcSamples := 800 // 100ms at 8kHz
	pcm := make([]byte, srcSamples*2)
	for i := 0; i < srcSamples; i++ {
		sample := int16(i % 500)
		pcm[i*2] = byte(sample)
		pcm[i*2+1] = byte(sample >> 8)
	}

	out := Resample(pcm, 8000, 16000)
	expectedSamples := 1600
	gotSamples := len(out) / 2

	if gotSamples != expectedSamples {
		t.Errorf("upsample: got %d samples, want %d", gotSamples, expectedSamples)
	}
}

func TestResample_Empty(t *testing.T) {
	out := Resample(nil, 16000, 8000)
	if out != nil {
		t.Error("expected nil for nil input")
	}
}
