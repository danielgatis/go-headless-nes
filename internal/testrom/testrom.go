// Package testrom builds synthetic NROM cartridges for unit tests, so a
// test that only needs "a console that runs" does not have to load a real
// ROM. The image mirrors nestest's board — 16 KiB PRG, 8 KiB CHR,
// horizontal mirroring, mapper 0 — and reproduces the handful of program
// landmarks the emulator's unit tests lean on. Tests that genuinely
// depend on a real program's behaviour (nestest's menu, blargg's beep)
// stay in the ROM-driven integration suite under test/.
package testrom

import (
	"bytes"
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/cartridge"
)

const (
	prgBank = 16 * 1024
	chrBank = 8 * 1024
)

// New returns a synthetic mapper-0 (NROM) cartridge with these program
// landmarks, chosen to match the ones nestest happens to have so tests
// need no changes beyond swapping the ROM source:
//
//	reset vector -> $C004
//	$C000: JMP $C5F5   (4C F5 C5)  nestest's automated-entry jump
//	$C003: NOP         (EA)        a second decodable instruction
//	$C004: JMP $C004   (4C 04 C0)  reset spins here forever
//	$C5F5: JSR $C5FB   (20 FB C5)  first JSR; pushes $C5 to $01FD
//	$C5FB: JMP $C5FB   (4C FB C5)  the subroutine spins forever
//
// Unwritten PRG is NOP. Booting from the reset vector simply spins, which
// is all a session/audio/determinism test needs. A test that wants the
// JSR side effect or the $C5F5 landmark sets PC to $C000 itself, exactly
// as it did against nestest.
func New(tb testing.TB) *cartridge.Cartridge {
	tb.Helper()
	cart, err := cartridge.Load(bytes.NewReader(Image(tb)))
	if err != nil {
		tb.Fatalf("building synthetic cartridge: %v", err)
	}
	return cart
}

// Image returns the raw iNES bytes of the synthetic NROM, for callers that
// need the on-disk form (e.g. driving the protocol server's LoadROM).
func Image(tb testing.TB) []byte {
	tb.Helper()

	prg := bytes.Repeat([]byte{0xEA}, prgBank) // NOP fill
	// The bank maps at $C000, so addr-0xC000 indexes into it.
	put := func(addr uint16, b ...byte) { copy(prg[addr-0xC000:], b) }
	put(0xC000, 0x4C, 0xF5, 0xC5) // JMP $C5F5
	put(0xC003, 0xEA)             // NOP
	put(0xC004, 0x4C, 0x04, 0xC0) // JMP $C004
	put(0xC5F5, 0x20, 0xFB, 0xC5) // JSR $C5FB
	put(0xC5FB, 0x4C, 0xFB, 0xC5) // JMP $C5FB
	put(0xFFFA, 0x04, 0xC0)       // NMI vector
	put(0xFFFC, 0x04, 0xC0)       // reset vector
	put(0xFFFE, 0x04, 0xC0)       // IRQ vector

	img := append(inesHeader(), prg...)
	img = append(img, make([]byte, chrBank)...) // blank CHR
	return img
}

// inesHeader is a 16-byte iNES header: one 16 KiB PRG bank, one 8 KiB CHR
// bank, mapper 0, horizontal mirroring.
func inesHeader() []byte {
	h := make([]byte, 16)
	copy(h, []byte{'N', 'E', 'S', 0x1a})
	h[4] = 1 // PRG banks
	h[5] = 1 // CHR banks
	return h
}
