package audio

import (
	"math"
	"testing"
)

func TestCalcRMS_Silence(t *testing.T) {
	frame := make([]byte, 640) // all zeros = silence
	rms := CalcRMS(frame)
	if rms != 0 {
		t.Errorf("silence RMS: got %f, want 0", rms)
	}
}

func TestCalcRMS_Signal(t *testing.T) {
	// Generate a sine wave frame
	frame := make([]byte, 640)
	for i := 0; i < 320; i++ {
		sample := int16(10000 * math.Sin(2*math.Pi*float64(i)/32))
		frame[i*2] = byte(sample)
		frame[i*2+1] = byte(sample >> 8)
	}
	rms := CalcRMS(frame)
	if rms < 0.1 {
		t.Errorf("signal RMS too low: %f", rms)
	}
}

func TestCalcRMS_EmptyFrame(t *testing.T) {
	rms := CalcRMS(nil)
	if rms != 0 {
		t.Errorf("empty frame RMS: got %f, want 0", rms)
	}
}

func TestPCMDuration(t *testing.T) {
	// 32000 bytes at 16kHz = 1 second
	d := PCMDuration(32000, 16000)
	if d.Seconds() != 1.0 {
		t.Errorf("duration: got %v, want 1s", d)
	}

	// 640 bytes at 16kHz = 20ms
	d = PCMDuration(640, 16000)
	expected := 0.02
	if math.Abs(d.Seconds()-expected) > 0.001 {
		t.Errorf("duration: got %v, want %vs", d, expected)
	}
}

func TestVADSegmenter_SilenceOnly(t *testing.T) {
	vad := NewVADSegmenter(16000)
	frame := make([]byte, vad.FrameBytes()) // silence

	// Feed many silence frames — should never produce a segment
	for i := 0; i < 100; i++ {
		seg := vad.Feed(frame)
		if seg != nil {
			t.Fatal("got segment from pure silence")
		}
	}

	// Flush should return nil
	seg := vad.Flush()
	if seg != nil {
		t.Fatal("flush produced segment from silence")
	}
}

func TestVADSegmenter_SpeechThenSilence(t *testing.T) {
	vad := NewVADSegmenter(16000)
	frameBytes := vad.FrameBytes()

	// Generate speech frames (loud signal)
	speechFrame := generateSpeechFrame(frameBytes)

	// Feed 500ms of speech (25 frames at 20ms each)
	for i := 0; i < 25; i++ {
		seg := vad.Feed(speechFrame)
		if seg != nil {
			t.Fatal("got segment too early during speech")
		}
	}

	// Feed silence until segment is produced
	silenceFrame := make([]byte, frameBytes)
	var segment *AudioSegment
	for i := 0; i < 50; i++ { // up to 1s of silence
		seg := vad.Feed(silenceFrame)
		if seg != nil {
			segment = seg
			break
		}
	}

	if segment == nil {
		t.Fatal("no segment produced after speech + silence")
	}

	if segment.Duration.Seconds() < 0.3 {
		t.Errorf("segment too short: %v", segment.Duration)
	}
}

func TestVADSegmenter_Flush(t *testing.T) {
	vad := NewVADSegmenter(16000)
	frameBytes := vad.FrameBytes()

	speechFrame := generateSpeechFrame(frameBytes)

	// Feed some speech
	for i := 0; i < 25; i++ {
		vad.Feed(speechFrame)
	}

	// Flush should produce the accumulated segment
	seg := vad.Flush()
	if seg == nil {
		t.Fatal("flush returned nil after speech")
	}
	if len(seg.PCM) == 0 {
		t.Fatal("segment has no PCM data")
	}
}

func generateSpeechFrame(size int) []byte {
	frame := make([]byte, size)
	for i := 0; i < size/2; i++ {
		sample := int16(5000 * math.Sin(2*math.Pi*float64(i)/32))
		frame[i*2] = byte(sample)
		frame[i*2+1] = byte(sample >> 8)
	}
	return frame
}
