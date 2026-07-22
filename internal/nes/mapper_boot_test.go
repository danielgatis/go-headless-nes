package nes

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/cartridge"
)

// bootCart builds a synthetic cartridge for any mapper: RTI-filled PRG
// with vectors in every 8 KiB bank tail pointing at $8000, plus CHR ROM.
func bootCart(id uint16) *cartridge.Cartridge {
	prg := make([]byte, 512*1024)
	for i := range prg {
		prg[i] = 0x40 // RTI
	}
	for i := 0; i < len(prg); i += 0x2000 {
		tail := prg[i+0x2000-16 : i+0x2000]
		for j := 0; j < 16; j += 2 {
			tail[j] = 0x00
			tail[j+1] = 0x80
		}
	}
	chr := make([]byte, 256*1024)
	return &cartridge.Cartridge{
		PRG:       prg,
		CHR:       chr,
		MapperID:  id,
		Mirroring: cartridge.Vertical,
	}
}

// TestReferencePortBoardsBoot powers a whole console on each board that
// uses the optional wiring hooks (raster position, CPU write cycles, CPU
// peeks, board nametables, expansion audio) and runs frames with
// rendering forced on, so the hooks are exercised end to end: IRQ
// sampling, PPU fetch interception and snapshot round-trips included.
func TestReferencePortBoardsBoot(t *testing.T) {
	for _, id := range []uint16{90, 209, 211, 111, 163, 168, 176, 188, 284, 292, 513} {
		c, err := New(bootCart(id))
		if err != nil {
			t.Fatalf("mapper %d: %v", id, err)
		}
		c.PPU.WriteRegister(0x2001, 0x1E) // enable rendering
		for range 5 {
			c.RunFrame()
		}
		var s Snapshot
		c.Save(&s)
		c.Restore(&s)
		c.RunFrame()
	}
}
