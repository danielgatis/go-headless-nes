package ppu

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/bus"
	"github.com/danielgatis/go-headless-nes/internal/cartridge"
)

// The PPU must satisfy the memory-handler interface so it can register into
// the CPU address space at $2000-$3FFF.
var _ bus.Handler = (*PPU)(nil)

// stubBoard is a minimal cartridge board for exercising the PPU alone: CHR
// RAM and fixed mirroring, no bank switching or IRQ.
type stubBoard struct {
	chr       [0x2000]byte
	mirroring cartridge.Mirroring
}

func (b *stubBoard) ReadCHR(addr uint16) byte       { return b.chr[addr&0x1FFF] }
func (b *stubBoard) WriteCHR(addr uint16, v byte)   { b.chr[addr&0x1FFF] = v }
func (b *stubBoard) Mirroring() cartridge.Mirroring { return b.mirroring }
func (b *stubBoard) Scanline()                      {}

// TestPPURunsFrame drives the PPU by its master clock through a full frame
// and checks the frame-complete flag and scanline wrap — a smoke test that
// the per-dot pipeline runs without panicking. Cycle-exact behavior is
// verified against blargg's ROMs once the console is wired (integration).
func TestPPURunsFrame(t *testing.T) {
	p := New(&stubBoard{mirroring: cartridge.Horizontal})
	// One NTSC frame is 341*262 dots = 89342; the master clock advances 4
	// per dot. Run past one frame boundary.
	target := uint64(89342 * masterClockDivider * 2)
	p.Run(target)
	if p.Frame == 0 {
		t.Error("no frame completed after two frames of dots")
	}
	if p.Scanline < -1 || p.Scanline > vblankEnd {
		t.Errorf("scanline out of range: %d", p.Scanline)
	}
}
