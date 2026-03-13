package audio

// Resample converts 16-bit mono PCM from srcRate to dstRate using linear interpolation.
// Returns the resampled PCM data.
func Resample(pcm []byte, srcRate, dstRate int) []byte {
	if srcRate == dstRate {
		return pcm
	}

	srcSamples := len(pcm) / 2
	if srcSamples == 0 {
		return pcm
	}

	ratio := float64(srcRate) / float64(dstRate)
	dstSamples := int(float64(srcSamples) / ratio)
	out := make([]byte, dstSamples*2)

	for i := 0; i < dstSamples; i++ {
		srcPos := float64(i) * ratio
		srcIdx := int(srcPos)
		frac := srcPos - float64(srcIdx)

		// Read source sample
		s0 := readSample(pcm, srcIdx)
		s1 := s0
		if srcIdx+1 < srcSamples {
			s1 = readSample(pcm, srcIdx+1)
		}

		// Linear interpolation
		sample := int16(float64(s0)*(1-frac) + float64(s1)*frac)

		// Write output sample (little-endian)
		out[i*2] = byte(sample)
		out[i*2+1] = byte(sample >> 8)
	}

	return out
}

func readSample(pcm []byte, idx int) int16 {
	offset := idx * 2
	if offset+1 >= len(pcm) {
		return 0
	}
	return int16(pcm[offset]) | int16(pcm[offset+1])<<8
}
