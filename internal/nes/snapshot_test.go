package nes

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/controller"
	"github.com/danielgatis/go-headless-nes/internal/ppu"
)

// TestSnapshotRestoresExactly is the completeness proof for Snapshot:
// run, save, run on, restore, run the same stretch again — every
// observable output must repeat exactly. Any emulated state missing
// from the snapshot shows up here as divergence.
func TestSnapshotRestoresExactly(t *testing.T) {
	console := newConsole(t)
	// Get past boot, with rendering enabled and buttons involved so
	// controller and PPU state matter.
	for range 20 {
		console.RunFrame()
	}
	console.Controllers[0].SetButtons(controller.Start)

	var snap Snapshot
	console.Save(&snap)

	runStretch := func() (uint64, uint64, [ppu.Width * ppu.Height]byte) {
		for range 10 {
			console.RunFrame()
		}
		return console.CPU.Cycles, console.MasterClock(), *console.PPU.Framebuffer()
	}

	cyclesA, masterA, fbA := runStretch()
	console.Restore(&snap)
	cyclesB, masterB, fbB := runStretch()

	if cyclesA != cyclesB {
		t.Errorf("CPU cycles diverged: %d vs %d", cyclesA, cyclesB)
	}
	if masterA != masterB {
		t.Errorf("master clock diverged: %d vs %d", masterA, masterB)
	}
	if fbA != fbB {
		t.Error("framebuffer diverged after restore")
	}
}

// TestSaveRestoreAllocates ensures snapshots stay allocation-free, the
// property rewind depends on to run every frame.
func TestSaveRestoreAllocates(t *testing.T) {
	console := newConsole(t)
	var snap Snapshot
	allocs := testing.AllocsPerRun(50, func() {
		console.Save(&snap)
		console.Restore(&snap)
	})
	if allocs != 0 {
		t.Errorf("Save+Restore allocated %v times per run, want 0", allocs)
	}
}
