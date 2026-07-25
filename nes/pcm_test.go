package nes

import (
	"testing"
	"time"

	"github.com/danielgatis/go-headless-nes/internal/testrom"
)

func TestAudioStreamPCM(t *testing.T) {
	s := NewAudioStream(0)
	s.Push([]float32{0, 0.5, -1, 2}) // 2 must clamp to 1

	p := make([]byte, 6*4) // ask for more frames than buffered
	n, err := s.Read(p)
	if err != nil || n != len(p) {
		t.Fatalf("Read = %d, %v; want %d, nil", n, err, len(p))
	}

	want := []int16{0, 16383, -32767, 32767, 0, 0} // trailing silence
	for i, w := range want {
		for ch := 0; ch < 2; ch++ {
			off := 4*i + 2*ch
			got := int16(uint16(p[off]) | uint16(p[off+1])<<8)
			if got != w {
				t.Errorf("frame %d ch %d = %d, want %d", i, ch, got, w)
			}
		}
	}
}

func TestAudioStreamLatencyCap(t *testing.T) {
	latency := time.Second * 10 / SampleRate // 10 samples
	s := NewAudioStream(latency)

	samples := make([]float32, 100)
	for i := range samples {
		samples[i] = float32(i) / 200
	}
	s.Push(samples)

	p := make([]byte, 4)
	if _, err := s.Read(p); err != nil {
		t.Fatal(err)
	}
	got := int16(uint16(p[0]) | uint16(p[1])<<8)
	want := int16(samples[90] * 32767) // oldest 90 dropped
	if got != want {
		t.Errorf("first sample after overflow = %d, want %d", got, want)
	}
}

func TestVideoRGBA(t *testing.T) {
	c, err := NewConsole(testrom.Image(t))
	if err != nil {
		t.Fatal(err)
	}
	c.RunFrame()

	px := c.VideoRGBA(nil)
	if len(px) != VideoWidth*VideoHeight*4 {
		t.Fatalf("len = %d, want %d", len(px), VideoWidth*VideoHeight*4)
	}
	for i, ci := range c.Video() {
		want := Palette[ci&0x3F]
		if px[4*i] != want.R || px[4*i+1] != want.G || px[4*i+2] != want.B || px[4*i+3] != 0xFF {
			t.Fatalf("pixel %d = %v, want %v", i, px[4*i:4*i+4], want)
		}
	}

	if again := c.VideoRGBA(px); &again[0] != &px[0] {
		t.Error("VideoRGBA did not reuse a large-enough dst")
	}
}
