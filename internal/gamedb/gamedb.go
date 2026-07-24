// Package gamedb is a CRC-indexed cartridge database that corrects ROM
// headers, the way the reference emulator does. Many iNES headers carry
// the wrong TV system, mirroring or mapper. The loader looks a ROM
// up by the CRC32 of its PRG+CHR data and, when found, applies the
// database's fields over the header.
//
// The table in table.go holds the data: a Go map keyed by CRC32, compiled
// from public NES cartridge databases.
package gamedb

import "github.com/danielgatis/go-headless-nes/internal/region"

// Entry is the subset of a database row this emulator models. The columns
// this core does not act on (input device, bus conflicts, RAM sizes, VS
// System details) are omitted from the table.
type Entry struct {
	Region region.Region

	// HasMapper is false for UNIF placeholder rows, whose board this
	// iNES-only loader cannot resolve; those keep the header's mapper.
	// Otherwise MapperID/Submapper override the header.
	HasMapper bool
	MapperID  uint16
	Submapper byte

	// Mirroring is the database's nametable arrangement as its letter code
	// (h, v, 4, 0, 1), or 0 when the row leaves it unspecified (then the
	// header's stands).
	Mirroring byte

	HasBattery bool

	// Validated marks a hand-checked row (it carried a submapper field).
	// The reference trusts such a row's battery flag outright; for others it
	// only adds a battery, never removes one.
	Validated bool
}

// Lookup returns the database entry for a ROM's PRG+CHR CRC32, if any.
func Lookup(crc uint32) (Entry, bool) {
	e, ok := db[crc]
	return e, ok
}
