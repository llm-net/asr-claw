package audio

import (
	"math"
	"time"
)

const (
	// silenceThresholdRMS — below this RMS value, audio is considered silence.
	// 16-bit PCM normalized to [-1, 1].
	silenceThresholdRMS = 0.01

	// defaultSilenceDuration — consecutive silence to trigger segment boundary.
	defaultSilenceDuration = 500 * time.Millisecond

	// defaultMaxSegmentDuration — safety valve: force-cut if speaking non-stop.
	defaultMaxSegmentDuration = 15 * time.Second

	// defaultMinSegmentDuration — discard segments shorter than this (noise/clicks).
	defaultMinSegmentDuration = 300 * time.Millisecond

	// FrameSize — 20ms @ 16kHz × 2 bytes = 640 bytes.
	FrameSize = 640
)

// VADState represents the state machine state.
type VADState int

const (
	StateIdle         VADState = iota
	StateSpeaking
	StateSegmentReady
)

// AudioSegment represents a complete speech segment extracted by VAD.
type AudioSegment struct {
	PCM      []byte        // raw PCM data
	Start    time.Duration // start time in overall audio
	End      time.Duration // end time in overall audio
	Duration time.Duration // segment duration
}

// VADSegmenter implements VAD-based intelligent speech segmentation.
// Time tracking is based on PCM sample counts, not wall clock, so it
// works correctly regardless of processing speed.
type VADSegmenter struct {
	state            VADState
	sampleRate       int
	silenceThreshold float64
	silenceDuration  time.Duration
	maxDuration      time.Duration
	minDuration      time.Duration

	buffer           []byte        // current segment PCM accumulator
	silenceBytes     int           // consecutive silence bytes count
	segmentBytes     int           // total bytes in current segment (including silence)
	totalOffset      time.Duration // global time offset
}

// NewVADSegmenter creates a VAD segmenter for the given sample rate.
func NewVADSegmenter(sampleRate int) *VADSegmenter {
	return &VADSegmenter{
		state:            StateIdle,
		sampleRate:       sampleRate,
		silenceThreshold: silenceThresholdRMS,
		silenceDuration:  defaultSilenceDuration,
		maxDuration:      defaultMaxSegmentDuration,
		minDuration:      defaultMinSegmentDuration,
	}
}

// FrameBytes returns the frame size in bytes for this sample rate (20ms).
func (v *VADSegmenter) FrameBytes() int {
	return v.sampleRate * 2 * 20 / 1000 // 20ms @ 16bit mono
}

// bytesToDuration converts PCM byte count to duration.
func (v *VADSegmenter) bytesToDuration(bytes int) time.Duration {
	return PCMDuration(bytes, v.sampleRate)
}

// Feed inputs a single PCM frame (20ms). Returns a complete speech segment when
// a sentence boundary is detected, or nil if still accumulating.
func (v *VADSegmenter) Feed(frame []byte) *AudioSegment {
	rms := CalcRMS(frame)
	isSpeech := rms > v.silenceThreshold

	switch v.state {
	case StateIdle:
		if isSpeech {
			v.state = StateSpeaking
			v.buffer = make([]byte, 0, v.sampleRate*2*15) // pre-allocate 15s
			v.buffer = append(v.buffer, frame...)
			v.segmentBytes = len(frame)
			v.silenceBytes = 0
		}

	case StateSpeaking:
		v.buffer = append(v.buffer, frame...)
		v.segmentBytes += len(frame)

		// Check max segment duration (safety valve)
		if v.bytesToDuration(v.segmentBytes) >= v.maxDuration {
			return v.flushAtBestCutPoint()
		}

		if !isSpeech {
			v.silenceBytes += len(frame)
			// Silence long enough → sentence boundary
			if v.bytesToDuration(v.silenceBytes) >= v.silenceDuration {
				return v.flushSegment()
			}
		} else {
			// Speech resumed, reset silence counter
			v.silenceBytes = 0
		}
	}

	return nil
}

// Flush forces output of any accumulated PCM as a final segment.
// Call this when the input stream ends (EOF).
func (v *VADSegmenter) Flush() *AudioSegment {
	if len(v.buffer) == 0 {
		return nil
	}
	return v.flushSegment()
}

// flushSegment outputs current accumulated segment and resets state.
func (v *VADSegmenter) flushSegment() *AudioSegment {
	duration := PCMDuration(len(v.buffer), v.sampleRate)

	// Discard too-short segments (noise/breathing/clicks)
	if duration < v.minDuration {
		v.reset()
		return nil
	}

	seg := &AudioSegment{
		PCM:      v.buffer,
		Start:    v.totalOffset,
		End:      v.totalOffset + duration,
		Duration: duration,
	}
	v.totalOffset += duration
	v.reset()
	return seg
}

// flushAtBestCutPoint scans the last 2 seconds for the quietest 20ms window
// and cuts there, keeping the remainder for the next segment.
func (v *VADSegmenter) flushAtBestCutPoint() *AudioSegment {
	frameBytes := v.FrameBytes()
	scanWindow := 2 * time.Second
	scanBytes := int(scanWindow.Seconds()) * v.sampleRate * 2

	if len(v.buffer) < scanBytes {
		return v.flushSegment()
	}

	searchStart := len(v.buffer) - scanBytes
	bestPos := len(v.buffer)
	bestRMS := math.MaxFloat64

	for pos := searchStart; pos+frameBytes <= len(v.buffer); pos += frameBytes {
		window := v.buffer[pos : pos+frameBytes]
		rms := CalcRMS(window)
		if rms < bestRMS {
			bestRMS = rms
			bestPos = pos
		}
	}

	// Split at best cut point
	segData := make([]byte, bestPos)
	copy(segData, v.buffer[:bestPos])
	remaining := make([]byte, len(v.buffer)-bestPos)
	copy(remaining, v.buffer[bestPos:])

	duration := PCMDuration(len(segData), v.sampleRate)

	seg := &AudioSegment{
		PCM:      segData,
		Start:    v.totalOffset,
		End:      v.totalOffset + duration,
		Duration: duration,
	}
	v.totalOffset += duration

	// Keep remainder, stay in StateSpeaking
	v.buffer = remaining
	v.segmentBytes = len(remaining)
	v.silenceBytes = 0

	return seg
}

// reset clears state back to IDLE.
func (v *VADSegmenter) reset() {
	v.state = StateIdle
	v.buffer = nil
	v.segmentBytes = 0
	v.silenceBytes = 0
}

// CalcRMS computes the RMS energy of a 16-bit PCM frame, normalized to [0, 1].
func CalcRMS(frame []byte) float64 {
	samples := len(frame) / 2
	if samples == 0 {
		return 0
	}
	var sumSq float64
	for i := 0; i < len(frame)-1; i += 2 {
		sample := int16(frame[i]) | int16(frame[i+1])<<8 // little-endian
		normalized := float64(sample) / 32768.0
		sumSq += normalized * normalized
	}
	return math.Sqrt(sumSq / float64(samples))
}

// PCMDuration calculates duration from PCM byte count and sample rate (16-bit mono).
func PCMDuration(bytes int, sampleRate int) time.Duration {
	samples := bytes / 2 // 16-bit = 2 bytes per sample
	return time.Duration(float64(samples) / float64(sampleRate) * float64(time.Second))
}
