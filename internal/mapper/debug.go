package mapper

// BankMapper is an optional Mapper capability: it reports which ROM (or RAM)
// bank is currently visible at each fixed window of the CPU and PPU address
// spaces, for a debugger's mapper view. A board that does not implement it
// is reported as having no bank information.
//
// PRGBankMap has four entries, one per 8 KiB CPU window from $8000 ($8000,
// $A000, $C000, $E000), each an 8 KiB bank index into PRG ROM. CHRBankMap
// has eight entries, one per 1 KiB PPU window from $0000, each a 1 KiB bank
// index into CHR ROM or CHR RAM.
type BankMapper interface {
	PRGBankMap() [4]int
	CHRBankMap() [8]int
}

// normBank wraps bank into [0, n) the way window() does, so a reported bank
// matches the byte the board actually reads.
func normBank(bank, n int) int {
	if n <= 0 {
		return 0
	}
	bank %= n
	if bank < 0 {
		bank += n
	}
	return bank
}

// chrLinear is the CHR map of a board with a single fixed 8 KiB window (no
// CHR banking, or 8 KiB of CHR RAM): windows 0..7 map straight through.
func chrLinear() [8]int { return [8]int{0, 1, 2, 3, 4, 5, 6, 7} }

// prgFixed is the PRG map of a board that does not bank PRG (NROM, CNROM):
// the 32 KiB window maps straight through, and a 16 KiB ROM mirrors.
func prgFixed(prg []byte) [4]int {
	var out [4]int
	for w := range out {
		out[w] = ((w * 0x2000) % len(prg)) / 0x2000
	}
	return out
}

// chr1kAt normalizes a board's native (bank, size) CHR window to the 1 KiB
// bank index at the given PPU window address.
func chr1kAt(chrLen, bank, size int, addr uint16) int {
	off := normBank(bank, chrLen/size)*size + int(addr)%size
	return off / 0x400
}

// PRGBankMap implements BankMapper.
func (m *NROM) PRGBankMap() [4]int { return prgFixed(m.prg) }

// CHRBankMap implements BankMapper.
func (m *NROM) CHRBankMap() [8]int { return chrLinear() }

// PRGBankMap implements BankMapper.
func (m *CNROM) PRGBankMap() [4]int { return prgFixed(m.prg) }

// CHRBankMap implements BankMapper.
func (m *CNROM) CHRBankMap() [8]int {
	b := normBank(int(m.bank), m.chrSize()/8192)
	var out [8]int
	for j := range out {
		out[j] = b*8 + j
	}
	return out
}

// PRGBankMap implements BankMapper.
func (m *UxROM) PRGBankMap() [4]int {
	n16 := len(m.prg) / 0x4000
	lo := normBank(int(m.bank), n16)
	hi := n16 - 1 // $C000 is hardwired to the last 16 KiB bank
	return [4]int{lo * 2, lo*2 + 1, hi * 2, hi*2 + 1}
}

// CHRBankMap implements BankMapper.
func (m *UxROM) CHRBankMap() [8]int { return chrLinear() }

// PRGBankMap implements BankMapper.
func (m *AxROM) PRGBankMap() [4]int {
	b := normBank(int(m.reg&7), len(m.prg)/0x8000)
	return [4]int{b * 4, b*4 + 1, b*4 + 2, b*4 + 3}
}

// CHRBankMap implements BankMapper.
func (m *AxROM) CHRBankMap() [8]int { return chrLinear() }

// PRGBankMap implements BankMapper.
func (m *MMC1) PRGBankMap() [4]int {
	var out [4]int
	n16 := len(m.prg) / 0x4000
	for w := 0; w < 4; w++ {
		addr := uint16(0x8000 + w*0x2000)
		b16 := normBank(m.prgBankNum(addr), n16)
		out[w] = b16*2 + (w & 1)
	}
	return out
}

// CHRBankMap implements BankMapper.
func (m *MMC1) CHRBankMap() [8]int {
	var out [8]int
	for j := 0; j < 8; j++ {
		addr := uint16(j * 0x400)
		bank, size := m.chrWindow(addr)
		out[j] = chr1kAt(m.chrSize(), bank, size, addr)
	}
	return out
}

// PRGBankMap implements BankMapper.
func (m *MMC3) PRGBankMap() [4]int {
	var out [4]int
	n8 := len(m.prg) / 0x2000
	for w := 0; w < 4; w++ {
		addr := uint16(0x8000 + w*0x2000)
		out[w] = normBank(m.prgBank(addr), n8)
	}
	return out
}

// CHRBankMap implements BankMapper.
func (m *MMC3) CHRBankMap() [8]int {
	var out [8]int
	for j := 0; j < 8; j++ {
		addr := uint16(j * 0x400)
		bank, size := m.chrWindow(addr)
		out[j] = chr1kAt(m.chrSize(), bank, size, addr)
	}
	return out
}
