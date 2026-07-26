package nes

import "runtime"

// Stepper drives a Console one PPU dot at a time, the finest step the
// hardware model exposes. It runs the emulation on its own goroutine and
// suspends it at each dot, deep inside the current CPU instruction, so a
// debugger can single-step below the instruction level and stop at an exact
// (scanline, dot).
//
// While a Stepper is engaged it OWNS the Console. Do not call the Console's
// own Step or RunFrame, and do not mutate it from another goroutine. Read
// state (State, Peek, PPUState, ...) only after a Step call has returned,
// when the emulation is suspended at a dot boundary; the handshake that
// suspends it also publishes that state to your goroutine. Call Close to end
// stepping and return the Console to normal synchronous use.
type Stepper struct {
	c      *Console
	step   chan struct{}
	synced chan struct{}
	done   chan struct{}
	closed bool
}

// NewStepper engages dot-stepping on c and suspends the machine at the next
// dot boundary. The Console must not be driven by anything else for the
// Stepper's lifetime.
func NewStepper(c *Console) *Stepper {
	s := &Stepper{
		c:      c,
		step:   make(chan struct{}),
		synced: make(chan struct{}),
		done:   make(chan struct{}),
	}
	c.core.SetDotHook(s.onDot)
	go s.run()
	<-s.synced // sync to the first dot boundary
	return s
}

// run is the emulation goroutine: it steps the core forever, suspending at
// each dot through onDot until Close stops it.
func (s *Stepper) run() {
	for {
		s.c.core.Step()
	}
}

// onDot runs on the emulation goroutine at the end of each dot: it announces
// arrival, then waits for permission to advance. Either half also observes
// Close and exits the goroutine cleanly.
func (s *Stepper) onDot() {
	select {
	case s.synced <- struct{}{}:
	case <-s.done:
		runtime.Goexit()
	}
	select {
	case <-s.step:
	case <-s.done:
		runtime.Goexit()
	}
}

// StepDot advances the machine by exactly one PPU dot.
func (s *Stepper) StepDot() {
	if s.closed {
		return
	}
	s.step <- struct{}{}
	<-s.synced
}

// StepScanline advances until the raster crosses into the next scanline.
func (s *Stepper) StepScanline() {
	if s.closed {
		return
	}
	start := s.c.State().Scanline
	for s.c.State().Scanline == start {
		s.StepDot()
	}
}

// RunToDot advances until the raster reaches (scanline, dot). It scans at
// most a little over one frame of dots, so a target that never occurs
// returns false instead of hanging. It returns true when the target is hit.
func (s *Stepper) RunToDot(scanline, dot int) bool {
	if s.closed {
		return false
	}
	const maxDots = 341*262 + 1 // an upper bound on one frame's dots
	for i := 0; i < maxDots; i++ {
		st := s.c.State()
		if st.Scanline == scanline && st.Dot == dot {
			return true
		}
		s.StepDot()
	}
	st := s.c.State()
	return st.Scanline == scanline && st.Dot == dot
}

// Close ends dot-stepping, stops the emulation goroutine and returns the
// Console to normal synchronous use. It is idempotent.
func (s *Stepper) Close() {
	if s.closed {
		return
	}
	s.closed = true
	close(s.done)
	s.c.core.SetDotHook(nil)
}
