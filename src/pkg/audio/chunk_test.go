package audio

import (
	"math"
	"testing"
)

func TestChunkSegmenter_Basic(t *testing.T) {
	// 3-second chunks at 16kHz
	cs := NewChunkSegmenter(16000, 3.0)
	frameBytes := cs.FrameBytes()
	frame := make([]byte, frameBytes)

	// 3 seconds = 150 frames of 20ms
	var segment *AudioSegment
	for i := 0; i < 200; i++ {
		seg := cs.Feed(frame)
		if seg != nil {
			segment = seg
			break
		}
	}

	if segment == nil {
		t.Fatal("no segment produced after 200 frames")
	}

	// Chunk should be ~3 seconds
	if math.Abs(segment.Duration.Seconds()-3.0) > 0.1 {
		t.Errorf("chunk duration: got %v, want ~3s", segment.Duration)
	}
}

func TestChunkSegmenter_Flush(t *testing.T) {
	cs := NewChunkSegmenter(16000, 3.0)
	frameBytes := cs.FrameBytes()
	frame := make([]byte, frameBytes)

	// Feed 1 second (50 frames)
	for i := 0; i < 50; i++ {
		cs.Feed(frame)
	}

	// Flush should return remaining
	seg := cs.Flush()
	if seg == nil {
		t.Fatal("flush returned nil")
	}
	if math.Abs(seg.Duration.Seconds()-1.0) > 0.1 {
		t.Errorf("flush duration: got %v, want ~1s", seg.Duration)
	}
}

func TestChunkSegmenter_FlushEmpty(t *testing.T) {
	cs := NewChunkSegmenter(16000, 3.0)
	seg := cs.Flush()
	if seg != nil {
		t.Error("expected nil from empty flush")
	}
}
