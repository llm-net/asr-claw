package audio

import "time"

// ChunkSegmenter implements fixed-time chunking as a fallback strategy.
// Unlike VAD, it splits audio at fixed intervals regardless of speech content.
type ChunkSegmenter struct {
	sampleRate  int
	chunkSec    float64
	buffer      []byte
	totalOffset time.Duration
}

// NewChunkSegmenter creates a fixed-time chunk segmenter.
// chunkSec: chunk duration in seconds (e.g., 3.0).
func NewChunkSegmenter(sampleRate int, chunkSec float64) *ChunkSegmenter {
	return &ChunkSegmenter{
		sampleRate: sampleRate,
		chunkSec:   chunkSec,
	}
}

// FrameBytes returns the frame size for 20ms.
func (c *ChunkSegmenter) FrameBytes() int {
	return c.sampleRate * 2 * 20 / 1000
}

// Feed inputs a PCM frame. Returns a segment when the chunk duration is reached.
func (c *ChunkSegmenter) Feed(frame []byte) *AudioSegment {
	c.buffer = append(c.buffer, frame...)

	chunkBytes := int(c.chunkSec * float64(c.sampleRate) * 2)
	if len(c.buffer) >= chunkBytes {
		segData := make([]byte, chunkBytes)
		copy(segData, c.buffer[:chunkBytes])
		c.buffer = c.buffer[chunkBytes:]

		duration := PCMDuration(len(segData), c.sampleRate)
		seg := &AudioSegment{
			PCM:      segData,
			Start:    c.totalOffset,
			End:      c.totalOffset + duration,
			Duration: duration,
		}
		c.totalOffset += duration
		return seg
	}
	return nil
}

// Flush outputs any remaining buffered PCM as a final segment.
func (c *ChunkSegmenter) Flush() *AudioSegment {
	if len(c.buffer) == 0 {
		return nil
	}

	duration := PCMDuration(len(c.buffer), c.sampleRate)
	seg := &AudioSegment{
		PCM:      c.buffer,
		Start:    c.totalOffset,
		End:      c.totalOffset + duration,
		Duration: duration,
	}
	c.totalOffset += duration
	c.buffer = nil
	return seg
}
