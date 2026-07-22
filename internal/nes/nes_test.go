package nes

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/testrom"
)

func newConsole(t *testing.T) *NES {
	t.Helper()
	console, err := New(testrom.New(t))
	if err != nil {
		t.Fatal(err)
	}
	return console
}

// TestDeterminism runs two consoles from the same ROM in lockstep and
// requires identical state: same framebuffer, same cycle counts, same
// master clock. Any hidden or global state would show up here. It needs
// no real program — determinism is a property of the core, so a synthetic
// ROM suffices. Behaviour that depends on the actual nestest program
// (menu rendering, controller-driven runs) lives in the integration suite
// under test/.
func TestDeterminism(t *testing.T) {
	a := newConsole(t)
	b := newConsole(t)
	for range 20 {
		a.RunFrame()
		b.RunFrame()
	}
	if a.CPU.Cycles != b.CPU.Cycles {
		t.Errorf("cycle divergence: %d vs %d", a.CPU.Cycles, b.CPU.Cycles)
	}
	if *a.PPU.Framebuffer() != *b.PPU.Framebuffer() {
		t.Error("framebuffer divergence")
	}
	if a.MasterClock() != b.MasterClock() {
		t.Errorf("master clock divergence: %d vs %d", a.MasterClock(), b.MasterClock())
	}
}
