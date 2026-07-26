package nes

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/testrom"
)

func TestStepperDotProgress(t *testing.T) {
	c := newConsole(t)
	s := NewStepper(c)
	defer s.Close()

	prev := c.State()
	for i := 0; i < 20; i++ {
		s.StepDot()
		cur := c.State()
		if cur.Dot == prev.Dot && cur.Scanline == prev.Scanline && cur.Frame == prev.Frame {
			t.Fatalf("StepDot %d made no progress at scanline %d dot %d", i, cur.Scanline, cur.Dot)
		}
		prev = cur
	}
}

func TestStepperScanline(t *testing.T) {
	c := newConsole(t)
	s := NewStepper(c)
	defer s.Close()

	start := c.State().Scanline
	s.StepScanline()
	if c.State().Scanline == start {
		t.Fatalf("StepScanline left scanline at %d", start)
	}
}

func TestStepperRunToDot(t *testing.T) {
	c := newConsole(t)
	s := NewStepper(c)
	defer s.Close()

	if !s.RunToDot(50, 100) {
		t.Fatal("RunToDot(50,100) never reached the target")
	}
	st := c.State()
	if st.Scanline != 50 || st.Dot != 100 {
		t.Fatalf("stopped at scanline %d dot %d, want 50,100", st.Scanline, st.Dot)
	}
}

func TestStepperCloseRestoresNormalRun(t *testing.T) {
	c := newConsole(t)
	s := NewStepper(c)
	s.StepDot()
	s.Close()

	// After Close the synchronous API must work again (and not deadlock).
	before := c.State().Frame
	c.RunFrame()
	if c.State().Frame == before {
		t.Error("RunFrame did not advance a frame after Stepper.Close")
	}
}

func BenchmarkRunFrame(b *testing.B) {
	c, err := NewConsole(testrom.Image(b))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RunFrame()
	}
}
