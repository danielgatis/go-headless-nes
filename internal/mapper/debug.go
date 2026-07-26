package mapper

// BankMap is a board's live bank layout for a debugger's mapper view. PRG
// has one 8 KiB bank index per CPU window from $8000 ($8000, $A000, $C000,
// $E000); CHR has one 1 KiB bank index per PPU window from $0000. An entry
// is -1 when the board does not resolve that window through a bank window
// (open bus, PRG RAM, or fixed direct-mapped ROM).
type BankMap struct {
	PRG [4]int
	CHR [8]int
}

// prober is the bank-map probe capability every board gets by embedding
// base. It is unexported: only ProbeBankMap uses it.
type prober interface {
	probeBegin()
	probeEnd()
	probeReset()
	probeResult() (offset, size int)
}

func unknownBankMap() BankMap {
	return BankMap{
		PRG: [4]int{-1, -1, -1, -1},
		CHR: [8]int{-1, -1, -1, -1, -1, -1, -1, -1},
	}
}

// ProbeBankMap reports which ROM or RAM bank m currently maps at each fixed
// CPU and PPU window. It works for any board that banks through the shared
// win helper (nearly all of them): it reads one byte per window and records
// the window that read resolved. The board is snapshotted and restored
// around the probe, so a read with a side effect (an MMC2/MMC4 CHR latch,
// read-triggered banking) leaves no net change. Windows a board maps without
// the helper come back as -1.
//
// It must run on the console's own goroutine while that console is not
// executing (a debugger inspecting a paused machine); the probe state lives
// on the board, so probing one console never disturbs another.
func ProbeBankMap(m Mapper) BankMap {
	bm := unknownBankMap()
	p, ok := m.(prober)
	if !ok {
		return bm
	}

	var st State
	m.Save(&st)
	defer m.Restore(&st)

	p.probeBegin()
	defer p.probeEnd()

	for w := 0; w < 4; w++ {
		addr := uint16(0x8000 + w*0x2000)
		p.probeReset()
		m.ReadPRG(addr)
		if off, size := p.probeResult(); size > 0 {
			bm.PRG[w] = (off + int(addr)%size) / 0x2000
		}
	}
	for j := 0; j < 8; j++ {
		addr := uint16(j * 0x400)
		p.probeReset()
		m.ReadCHR(addr)
		if off, size := p.probeResult(); size > 0 {
			bm.CHR[j] = (off + int(addr)%size) / 0x400
		}
	}
	return bm
}
