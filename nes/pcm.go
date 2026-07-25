package nes

import (
	"sync"
	"time"

	"github.com/danielgatis/go-headless-nes/internal/apu"
)

// SampleRate is the playback rate of the samples Audio produces, in Hz.
const SampleRate = apu.SampleRate

// AudioStream adapts Audio's float32 samples to the pull model of
// real-time audio APIs (Ebitengine, oto, SDL): the emulator goroutine
// pushes each frame's samples in, and the audio device drains them
// through Read as 16-bit little-endian stereo PCM at SampleRate. Unlike
// Console, it is safe for concurrent use.
//
// A bounded buffer is part of the contract. The core deliberately emits
// slightly more than one display frame of audio per emulated frame, so
// an unbounded buffer would turn that surplus into ever-growing latency;
// past the cap the oldest samples are dropped. In the other direction,
// Read never blocks: when the buffer runs dry it produces silence, so a
// paused emulator cannot stall the audio device.
type AudioStream struct {
	mu    sync.Mutex
	buf   []float32
	limit int
}

// NewAudioStream returns a stream that holds at most maxLatency of
// audio. Zero or negative means the default of 250ms.
func NewAudioStream(maxLatency time.Duration) *AudioStream {
	if maxLatency <= 0 {
		maxLatency = 250 * time.Millisecond
	}
	limit := int(maxLatency.Seconds()*SampleRate + 0.5)
	if limit < 1 {
		limit = 1
	}
	return &AudioStream{limit: limit}
}

// Push appends samples drained from Console.Audio, dropping the oldest
// ones once the latency cap is exceeded.
func (s *AudioStream) Push(samples []float32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = append(s.buf, samples...)
	if len(s.buf) > s.limit {
		s.buf = s.buf[len(s.buf)-s.limit:]
	}
}

// Read fills p with 16-bit little-endian stereo PCM, duplicating the
// mono NES signal onto both channels and clamping to [-1, 1]. It always
// fills whole frames (4 bytes each) and pads with silence when fewer
// samples are buffered than requested.
func (s *AudioStream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	frames := len(p) / 4
	for i := range frames {
		var v float32
		if i < len(s.buf) {
			v = s.buf[i]
		}
		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
		d := int16(v * 32767)
		p[4*i+0] = byte(d)
		p[4*i+1] = byte(d >> 8)
		p[4*i+2] = byte(d)
		p[4*i+3] = byte(d >> 8)
	}
	if frames >= len(s.buf) {
		s.buf = s.buf[:0]
	} else {
		s.buf = s.buf[frames:]
	}
	return frames * 4, nil
}
