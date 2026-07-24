// Package cartridge loads NES cartridge images and exposes their contents
// to the mapper layer.
package cartridge

import "github.com/danielgatis/go-headless-nes/internal/region"

// Mirroring selects how the two physical nametables in console VRAM map
// onto the four logical nametable slots at PPU $2000-$2FFF. It is a
// property of the cartridge PCB: solder pads (or the mapper) choose it.
type Mirroring byte

const (
	// Horizontal mirroring: $2000=$2400 and $2800=$2C00 (vertical scrolling).
	Horizontal Mirroring = iota
	// Vertical mirroring: $2000=$2800 and $2400=$2C00 (horizontal scrolling).
	Vertical
	// SingleLow and SingleHigh map every slot to one physical table;
	// mappers with one-screen modes (MMC1, AxROM) switch between them.
	SingleLow
	// SingleHigh is the one-screen mode using the high nametable.
	SingleHigh
	// FourScreen boards carry extra VRAM so all four slots are distinct.
	FourScreen
)

func (m Mirroring) String() string {
	switch m {
	case Horizontal:
		return "horizontal"
	case Vertical:
		return "vertical"
	case SingleLow:
		return "single-screen low"
	case SingleHigh:
		return "single-screen high"
	case FourScreen:
		return "four-screen"
	}
	return "unknown"
}

// Cartridge is a parsed ROM image. PRG and CHR are immutable after load;
// mutable board state (PRG RAM, CHR RAM, bank registers) lives in the
// mapper, so a Cartridge can be shared freely by snapshots.
type Cartridge struct {
	PRG []byte // program ROM, a multiple of 16 KiB
	CHR []byte // character ROM; empty means the board provides CHR RAM

	MapperID   uint16 // 12 bits under NES 2.0, 8 under iNES 1
	Submapper  byte   // NES 2.0 only; 0 otherwise
	Mirroring  Mirroring
	HasBattery bool

	// Region is the TV system declared by the header, resolved to a
	// concrete system (never Auto). A console auto-detects from this at
	// power-on; the runtime override can ignore it.
	Region region.Region
}
