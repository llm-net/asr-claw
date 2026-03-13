package audio

import (
	"encoding/binary"
	"fmt"
	"io"
)

// WAVHeader contains parsed WAV header information.
type WAVHeader struct {
	SampleRate    int
	BitsPerSample int
	Channels      int
	IsStreaming   bool // data_size == 0x7FFFFFFF
}

// DetectWAV reads and parses a 44-byte WAV header from the reader.
// Returns nil header (no error) if the data is not WAV format.
func DetectWAV(r io.Reader) (*WAVHeader, []byte, error) {
	var buf [44]byte
	n, err := io.ReadFull(r, buf[:])
	if err != nil {
		return nil, buf[:n], err
	}

	// Check RIFF + WAVE markers
	if string(buf[0:4]) != "RIFF" || string(buf[8:12]) != "WAVE" {
		return nil, buf[:], nil // not WAV, return consumed bytes for reprocessing
	}

	// Verify fmt chunk
	if string(buf[12:16]) != "fmt " {
		return nil, buf[:], nil
	}

	audioFormat := binary.LittleEndian.Uint16(buf[20:22])
	if audioFormat != 1 { // PCM only
		return nil, buf[:], fmt.Errorf("unsupported WAV format: %d (only PCM=1 supported)", audioFormat)
	}

	h := &WAVHeader{
		Channels:      int(binary.LittleEndian.Uint16(buf[22:24])),
		SampleRate:    int(binary.LittleEndian.Uint32(buf[24:28])),
		BitsPerSample: int(binary.LittleEndian.Uint16(buf[34:36])),
	}

	dataSize := binary.LittleEndian.Uint32(buf[40:44])
	h.IsStreaming = dataSize == 0x7FFFFFFF

	return h, nil, nil
}

// WriteWAV writes PCM data as a standard WAV file.
func WriteWAV(w io.Writer, pcm []byte, sampleRate int) error {
	channels := 1
	bitsPerSample := 16
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := len(pcm)
	chunkSize := 36 + dataSize

	header := make([]byte, 44)

	// RIFF header
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(chunkSize))
	copy(header[8:12], "WAVE")

	// fmt sub-chunk
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16) // sub-chunk size
	binary.LittleEndian.PutUint16(header[20:22], 1)  // PCM format
	binary.LittleEndian.PutUint16(header[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(header[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(header[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(header[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(header[34:36], uint16(bitsPerSample))

	// data sub-chunk
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(dataSize))

	if _, err := w.Write(header); err != nil {
		return err
	}
	if _, err := w.Write(pcm); err != nil {
		return err
	}
	return nil
}
