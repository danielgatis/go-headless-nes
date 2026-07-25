package ppu

// PeekVRAM reads one byte of the PPU address space ($0000-$3FFF) the way a
// rendering fetch resolves it: pattern tables through the board, nametables
// through CIRAM mirroring, and palette RAM from the internal latch. It is
// for debuggers and the pattern/nametable/palette viewers.
//
// It reads through the live PPU bus, so on latch-based CHR boards (MMC2 and
// MMC4) fetching a pattern byte can flip the CHR latch, exactly as a real
// fetch would. Callers that must not perturb the board should read the raw
// ROM through the cartridge instead.
func (p *PPU) PeekVRAM(addr uint16) byte {
	addr &= 0x3FFF
	if addr >= 0x3F00 {
		return p.readPaletteRAM(addr)
	}
	return p.mapperReadVram(addr)
}

// PokeVRAM writes one byte of the PPU address space ($0000-$3FFF): pattern
// tables on a CHR-RAM board, nametables through CIRAM mirroring, or palette
// RAM. A write into pattern space on a CHR-ROM board is dropped by the
// board, as in hardware.
func (p *PPU) PokeVRAM(addr uint16, value byte) {
	addr &= 0x3FFF
	if addr >= 0x3F00 {
		p.writePaletteRAM(addr, value)
		return
	}
	p.mapperWriteVram(addr, value)
}
