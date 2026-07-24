package nes

import (
	"github.com/danielgatis/go-headless-nes/internal/apu"
	"github.com/danielgatis/go-headless-nes/internal/bus"
	"github.com/danielgatis/go-headless-nes/internal/controller"
	"github.com/danielgatis/go-headless-nes/internal/cpu"
	"github.com/danielgatis/go-headless-nes/internal/mapper"
	"github.com/danielgatis/go-headless-nes/internal/ppu"
)

// Snapshot is the console's complete emulated state as one plain value:
// no pointers, no slices. Save and Restore are struct assignments, so
// they never allocate, serialize or reflect, which is what makes rewind
// cheap enough to run every frame. ROM contents are immutable and stay
// in the Cartridge, outside snapshots.
type Snapshot struct {
	CPU    cpu.State
	PPU    ppu.State
	APU    apu.State
	Bus    bus.State
	Mapper mapper.State
	Pads   [2]controller.Controller
	Ctrl   controller.ManagerState
}

// Save copies the console's state into s.
func (c *NES) Save(s *Snapshot) {
	s.CPU = c.CPU.State
	s.PPU = c.PPU.State
	s.APU = c.APU.State
	s.Bus = c.mem.State()
	c.Mapper.Save(&s.Mapper)
	s.Pads = c.Controllers
	s.Ctrl = c.ctrl.State()
}

// Restore rewinds the console to the state in s. The PPU framebuffer is
// derived state and repaints on the next rendered frame.
func (c *NES) Restore(s *Snapshot) {
	c.CPU.State = s.CPU
	c.PPU.State = s.PPU
	c.PPU.SyncRestored()
	c.APU.State = s.APU
	// The DMC's DMA hooks and the channels' region-table pointers travel
	// inside the APU state; re-install the console's own so a snapshot can
	// never carry stale wiring or a nil table pointer.
	c.APU.SetDMAHooks(c.dmcRequest, c.dmcStop)
	c.APU.ReinstallRegionTables()
	c.mem.SetState(s.Bus)
	c.Mapper.Restore(&s.Mapper)
	c.Controllers = s.Pads
	c.ctrl.SetState(s.Ctrl)
}
