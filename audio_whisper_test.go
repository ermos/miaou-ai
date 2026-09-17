package main

import (
	"encoding/binary"
	"testing"
)

func pcm(samples ...int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
}

func TestRMSLevel(t *testing.T) {
	if got := rmsLevel(nil); got != 0 {
		t.Errorf("rmsLevel(nil) = %d, want 0", got)
	}

	silence := pcm(0, 0, 0, 0, 0, 0)
	if got := rmsLevel(silence); got != 0 {
		t.Errorf("rmsLevel(silence) = %d, want 0", got)
	}

	// A single loud spike surrounded by silence (a keyboard click) should
	// read much lower on RMS than sustained loudness at the same peak -
	// that gap is what lets minVoiceDuration reject transient noise.
	click := pcm(0, 0, 30000, 0, 0, 0)
	sustained := pcm(30000, 30000, 30000, 30000, 30000, 30000)
	if rmsLevel(click) >= rmsLevel(sustained) {
		t.Errorf("rmsLevel(click)=%d should be well below rmsLevel(sustained)=%d",
			rmsLevel(click), rmsLevel(sustained))
	}
}
