package mapper

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/cartridge"
)

// Focused tests for the simple discrete-logic and single-latch boards.

func TestUxROMBanking(t *testing.T) {
	c := cart(2, 8, 0)
	m := NewUxROM(c)
	if got := m.ReadPRG(0xC000); got != 7 {
		t.Errorf("$C000 bank = %d, want 7 (hardwired last)", got)
	}
	// The board has bus conflicts, so land the write on a ROM byte
	// that agrees with it — exactly what real games do.
	c.PRG[0x0200] = 0xFF
	m.WritePRG(0x8200, 3)
	if got := m.ReadPRG(0x8000); got != 3 {
		t.Errorf("$8000 bank = %d, want 3", got)
	}
	if got := m.ReadPRG(0xC000); got != 7 {
		t.Error("bank write moved the fixed bank")
	}
	// CHR RAM board.
	m.WriteCHR(0x0123, 0x77)
	if got := m.ReadCHR(0x0123); got != 0x77 {
		t.Errorf("CHR RAM = %02X, want 77", got)
	}
}

func TestCNROMBankingAndBusConflict(t *testing.T) {
	c := cart(3, 1, 4)
	m := NewCNROM(c)
	m.WritePRG(0x8000, 2)
	// PRG at $8000 is bank 0 -> byte 0x00: the AND masks the write.
	if m.bank != 0 {
		t.Errorf("bus conflict not applied: bank = %d, want 0", m.bank)
	}
	// Give the write a conflict-free ROM byte to land on.
	c.PRG[0x0100] = 0xFF
	m.WritePRG(0x8100, 2)
	if got := m.ReadCHR(0x0000); got != 2 {
		t.Errorf("CHR bank = %d, want 2", got)
	}
}

func TestAxROMBankingAndMirroring(t *testing.T) {
	c := cart(7, 8, 0) // 4 x 32K banks
	m := NewAxROM(c)
	m.WritePRG(0x8000, 0x02)                // bank 2, single-screen low
	if got := m.ReadPRG(0x8000); got != 4 { // 32K bank 2 starts at 16K bank 4
		t.Errorf("PRG read = %d, want 4", got)
	}
	if m.Mirroring() != cartridge.SingleLow {
		t.Errorf("mirroring = %v, want single-low", m.Mirroring())
	}
	m.WritePRG(0x8000, 0x12)
	if m.Mirroring() != cartridge.SingleHigh {
		t.Errorf("mirroring = %v, want single-high", m.Mirroring())
	}
}

func TestColorDreams(t *testing.T) {
	c := cart(11, 8, 4)
	m := NewColorDreams(c)
	c.PRG[0x0100] = 0xFF // conflict-free write site
	m.WritePRG(0x8100, 0x12)
	if got := m.ReadPRG(0x8000); got != 4 { // 32K bank 2 -> 16K bank 4
		t.Errorf("PRG read = %d, want 4", got)
	}
	if got := m.ReadCHR(0x0000); got != 1 {
		t.Errorf("CHR bank = %d, want 1", got)
	}
}

func TestGxROM(t *testing.T) {
	c := cart(66, 8, 4)
	m := NewGxROM(c)
	c.PRG[0x0100] = 0xFF     // conflict-free write site
	m.WritePRG(0x8100, 0x12) // PRG block 1, CHR bank 2
	if got := m.ReadPRG(0x8000); got != 2 {
		t.Errorf("PRG read = %d, want 2", got)
	}
	if got := m.ReadCHR(0x0000); got != 2 {
		t.Errorf("CHR bank = %d, want 2", got)
	}
}

func TestCamerica(t *testing.T) {
	c := cart(71, 8, 0)
	c.Submapper = 1 // BF9097 (Fire Hawk): has the mirroring register
	m := NewCamerica(c)
	m.WritePRG(0xC000, 5)
	if got := m.ReadPRG(0x8000); got != 5 {
		t.Errorf("switchable bank = %d, want 5", got)
	}
	if got := m.ReadPRG(0xC000); got != 7 {
		t.Errorf("fixed bank = %d, want 7", got)
	}
	if m.Mirroring() != cartridge.Vertical {
		t.Error("header mirroring lost before any $8000 write")
	}
	m.WritePRG(0x9000, 0x10) // Fire Hawk one-screen select
	if m.Mirroring() != cartridge.SingleHigh {
		t.Errorf("mirroring = %v, want single-high", m.Mirroring())
	}

	// The common BF9093 has no register at $8000-$9FFF: mirroring
	// stays whatever the header says.
	plain := NewCamerica(cart(71, 8, 0))
	plain.WritePRG(0x9000, 0x10)
	if plain.Mirroring() != cartridge.Vertical {
		t.Errorf("BF9093 mirroring = %v, want vertical", plain.Mirroring())
	}
}

func TestNINA03RegisterDecode(t *testing.T) {
	m := NewNINA03(cart(79, 4, 8))
	m.WritePRG(0x4100, 0x0B) // PRG bank 1, CHR bank 3
	if got := m.ReadPRG(0x8000); got != 2 {
		t.Errorf("PRG read = %d, want 2 (32K bank 1)", got)
	}
	if got := m.ReadCHR(0x0000); got != 3 {
		t.Errorf("CHR bank = %d, want 3", got)
	}
	// Addresses without bit 8 set are not the register.
	m.WritePRG(0x4200, 0x00)
	if got := m.ReadCHR(0x0000); got != 3 {
		t.Error("write without A8 changed the register")
	}
}

func TestJaleco87SwappedBits(t *testing.T) {
	m := NewJaleco87(cart(87, 1, 4))
	m.WritePRG(0x6000, 0x01) // bit 0 -> CHR A14: bank 2
	if got := m.ReadCHR(0x0000); got != 2 {
		t.Errorf("CHR bank = %d, want 2 (bits swapped)", got)
	}
	m.WritePRG(0x6000, 0x02)
	if got := m.ReadCHR(0x0000); got != 1 {
		t.Errorf("CHR bank = %d, want 1 (bits swapped)", got)
	}
}

func TestUN1ROM(t *testing.T) {
	c := cart(94, 8, 0)
	m := NewUN1ROM(c)
	c.PRG[0x0100] = 0xFF // conflict-free write site
	m.WritePRG(0x8100, 3<<2)
	if got := m.ReadPRG(0x8000); got != 3 {
		t.Errorf("bank = %d, want 3", got)
	}
	if got := m.ReadPRG(0xC000); got != 7 {
		t.Errorf("fixed bank = %d, want 7", got)
	}
}

func TestJaleco140(t *testing.T) {
	m := NewJaleco140(cart(140, 8, 4))
	m.WritePRG(0x6000, 0x13) // PRG block 1, CHR 3
	if got := m.ReadPRG(0x8000); got != 2 {
		t.Errorf("PRG read = %d, want 2", got)
	}
	if got := m.ReadCHR(0x0000); got != 3 {
		t.Errorf("CHR bank = %d, want 3", got)
	}
	m.WritePRG(0x8000, 0x25) // ROM area is not the register
	if got := m.ReadCHR(0x0000); got != 3 {
		t.Error("$8000 write changed the $6000 latch")
	}
}

func TestUNROM180Inverted(t *testing.T) {
	c := cart(180, 8, 0)
	m := NewUNROM180(c)
	c.PRG[0x0100] = 0xFF // conflict-free write site
	m.WritePRG(0x8100, 5)
	if got := m.ReadPRG(0x8000); got != 0 {
		t.Errorf("$8000 bank = %d, want 0 (fixed first)", got)
	}
	if got := m.ReadPRG(0xC000); got != 5 {
		t.Errorf("$C000 bank = %d, want 5", got)
	}
}

func TestQuattroBlocks(t *testing.T) {
	m := NewQuattro(cart(232, 16, 0))       // 4 blocks of 4 x 16K
	m.WritePRG(0x8000, 2<<3)                // block 2
	m.WritePRG(0xC000, 1)                   // inner bank 1
	if got := m.ReadPRG(0x8000); got != 9 { // 2*4 + 1
		t.Errorf("$8000 bank = %d, want 9", got)
	}
	if got := m.ReadPRG(0xC000); got != 11 { // block's last: 2*4 + 3
		t.Errorf("$C000 bank = %d, want 11", got)
	}
}

func TestMapper34BNROM(t *testing.T) {
	c := cart(34, 8, 0) // CHR RAM -> BNROM
	m := NewMapper34(c)
	c.PRG[0x0300] = 0xFF
	m.WritePRG(0x8300, 0x01)
	if got := m.ReadPRG(0x8000); got != 2 {
		t.Errorf("PRG read = %d, want 2 (32K bank 1)", got)
	}
	m.WriteCHR(0x0042, 0x99)
	if got := m.ReadCHR(0x0042); got != 0x99 {
		t.Error("BNROM CHR RAM not writable")
	}
}

func TestMapper34NINA001(t *testing.T) {
	c := chrCart(34, 4, 8) // CHR ROM -> NINA-001; bytes encode 1K bank
	m := NewMapper34(c)
	m.WritePRG(0x7FFD, 1)
	m.WritePRG(0x7FFE, 3) // low window: 4K bank 3 -> 1K bank 12
	m.WritePRG(0x7FFF, 5) // high window: 4K bank 5 -> 1K bank 20
	if got := m.ReadPRG(0x8000); got != 2 {
		t.Errorf("PRG read = %d, want 2", got)
	}
	if got := m.ReadCHR(0x0000); got != 12 {
		t.Errorf("CHR low = %d, want 12", got)
	}
	if got := m.ReadCHR(0x1000); got != 20 {
		t.Errorf("CHR high = %d, want 20", got)
	}
	// The registers live inside PRG RAM and must read back as RAM.
	if got := m.ReadPRG(0x7FFE); got != 3 {
		t.Errorf("register readback = %d, want 3", got)
	}
}

func TestDxROM(t *testing.T) {
	c := chrCart(206, 4, 8)
	for i := range c.PRG {
		c.PRG[i] = byte(i / 0x2000)
	}
	m := NewDxROM(c)
	set := func(reg, v byte) {
		m.WritePRG(0x8000, reg)
		m.WritePRG(0x8001, v)
	}
	set(6, 2)
	set(7, 3)
	set(0, 4)
	set(2, 9)

	if got := m.ReadPRG(0x8000); got != 2 {
		t.Errorf("$8000 bank = %d, want 2", got)
	}
	if got := m.ReadPRG(0xC000); got != 6 {
		t.Errorf("$C000 bank = %d, want 6 (second-last)", got)
	}
	if got := m.ReadCHR(0x0000); got != 4 {
		t.Errorf("CHR $0000 bank = %d, want 4", got)
	}
	if got := m.ReadCHR(0x1000); got != 9 {
		t.Errorf("CHR $1000 bank = %d, want 9", got)
	}
	// No MMC3 features: bit 7 of bank select must not invert CHR.
	m.WritePRG(0x8000, 0x80)
	if got := m.ReadCHR(0x0000); got != 4 {
		t.Error("DxROM must not implement CHR inversion")
	}
}
