package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// base carries the storage and default behavior every board shares:
// ROM references, 8 KiB of PRG RAM at $6000, 8 KiB of CHR RAM when the
// cartridge has no CHR ROM, and inert defaults for the IRQ and timing
// hooks. Boards embed it and override what their hardware actually has.
type base struct {
	prg       []byte
	chr       []byte // CHR ROM; nil means the board uses chrRAM
	mirroring cartridge.Mirroring

	prgRAM [8192]byte
	chrRAM [8192]byte

	// openBusVal is the last value the CPU data bus drove, pushed in by
	// the bus before each cartridge-port read so an undecoded read returns
	// the real floating value rather than a fabricated one.
	openBusVal byte
}

func makeBase(c *cartridge.Cartridge) base {
	return base{
		prg:       c.PRG,
		chr:       c.CHR,
		mirroring: c.Mirroring,
	}
}

// Mirroring reports the board's current nametable mirroring.
func (b *base) Mirroring() cartridge.Mirroring { return b.mirroring }

// Tick advances the board by one cycle.
func (b *base) Tick() {}

// Scanline clocks the board's per-scanline logic.
func (b *base) Scanline() {}

// IRQ reports whether the board is asserting the IRQ line.
func (b *base) IRQ() bool { return false }

// FiltersConsecutiveWrites reports whether the board drops the second of two consecutive writes.
func (b *base) FiltersConsecutiveWrites() bool { return false }

// SetOpenBus records the current data-bus value so a subsequent undecoded
// cartridge-port read returns it. The bus calls this before every read.
func (b *base) SetOpenBus(v byte) { b.openBusVal = v }

// readPRGRAM and writePRGRAM implement the common $6000-$7FFF window.
func (b *base) readPRGRAM(addr uint16) byte {
	return b.prgRAM[addr&0x1FFF]
}

func (b *base) writePRGRAM(addr uint16, v byte) {
	b.prgRAM[addr&0x1FFF] = v
}

// chrSize is the size of the board's CHR memory, ROM or RAM.
func (b *base) chrSize() int {
	if b.chr != nil {
		return len(b.chr)
	}
	return len(b.chrRAM)
}

// chrRead reads CHR ROM or RAM through the given bank window.
func (b *base) chrRead(bank, size int, addr uint16) byte {
	if b.chr != nil {
		return window(b.chr, bank, size)[int(addr)%size]
	}
	return window(b.chrRAM[:], bank, size)[int(addr)%size]
}

// chrWrite writes CHR RAM (CHR ROM ignores writes, as on hardware).
func (b *base) chrWrite(bank, size int, addr uint16, v byte) {
	if b.chr == nil {
		window(b.chrRAM[:], bank, size)[int(addr)%size] = v
	}
}

// saveRAM and restoreRAM handle the storage every board has; boards
// with registers pack them into s.Regs on top.
func (b *base) saveRAM(s *State) {
	copy(s.PRGRAM[:], b.prgRAM[:])
	s.CHRRAM = b.chrRAM
}

func (b *base) restoreRAM(s *State) {
	copy(b.prgRAM[:], s.PRGRAM[:len(b.prgRAM)])
	b.chrRAM = s.CHRRAM
}

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// openBus returns the value of an undriven cartridge-port read: the last
// value the CPU drove onto the data bus, pushed in via SetOpenBus. This
// matters when the last bus cycle was a dummy read at a different address
// than the final one (indexed page-cross), where the floating value is
// the dummy read's, not the corrected address's high byte.
func (b *base) openBus() byte {
	return b.openBusVal
}

// window returns the bank'th size-byte view of mem. Banks count from
// the end when negative (-1 is the last bank) and wrap modulo the
// number of banks, matching unconnected high address lines on smaller
// ROMs. Cartridge loading mirrors PRG up to 32 KiB, so mem always
// holds at least one full window.
func window(mem []byte, bank, size int) []byte {
	n := len(mem) / size
	bank %= n
	if bank < 0 {
		bank += n
	}
	return mem[bank*size : (bank+1)*size]
}
