package cpu

import (
	"os"
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/bus"
)

// nromCart is a minimal NROM cartridge handler for the CPU test driver: it
// maps the PRG ROM into $8000-$FFFF (16 KiB banks mirror). It is only used
// to exercise the CPU; the full board model arrives later.
type nromCart struct {
	prg []byte
}

func (c *nromCart) Ranges() *bus.Ranges {
	r := bus.NewRanges()
	r.Add(bus.OpRead, 0x8000, 0xFFFF)
	return r
}
func (c *nromCart) ReadReg(addr uint16) byte {
	return c.prg[int(addr-0x8000)%len(c.prg)]
}
func (c *nromCart) PeekReg(addr uint16) byte  { return c.ReadReg(addr) }
func (c *nromCart) WriteReg(_ uint16, _ byte) {}

func loadNestestPRG(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../test/roms/other/nestest.nes")
	if err != nil {
		t.Skipf("nestest ROM not found (%v)", err)
	}
	if len(data) < 16 || string(data[:4]) != "NES\x1a" {
		t.Fatalf("not an iNES image")
	}
	return data[16 : 16+int(data[4])*0x4000]
}

// TestNestestRuns drives the new CPU through nestest in automated mode
// ($C000). It is a smoke test that the core CPU executes the full official +
// unofficial opcode suite without desync: nestest writes 0 to $02/$03 when
// every opcode behaved correctly. Cycle-exact trace comparison against the
// canonical log is done at the integration level once the console is wired.
func TestNestestRuns(t *testing.T) {
	prg := loadNestestPRG(t)

	mem := bus.New()
	mem.Register(&nromCart{prg: prg})
	cpu := New(mem)
	cpu.SetTicker(func() {}, func() {}, func() {})
	cpu.Reg.PC = 0xC000

	// nestest's automated run is ~26k instructions; cap generously.
	for i := 0; i < 60000; i++ {
		cpu.Step()
		// The run ends by looping at a fixed address once the result codes
		// are written; stop when both result bytes are populated and PC has
		// left the test body ($C000-$FFFF into the RAM result loop is not
		// used by nestest, so we just run a fixed budget then check).
	}

	if v := mem.Peek(0x02); v != 0 {
		t.Errorf("official opcode failure code $02 = %02X (want 00)", v)
	}
	if v := mem.Peek(0x03); v != 0 {
		t.Errorf("unofficial opcode failure code $03 = %02X (want 00)", v)
	}
}
