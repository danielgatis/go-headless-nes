package nes

import "github.com/danielgatis/go-headless-nes/internal/region"

// Region selects a console's TV system. NewConsole auto-detects it from
// the ROM header; SetRegion overrides it at runtime, where RegionAuto
// means "re-detect from the cartridge".
type Region uint8

// The regions. The values are wire-stable: they are the payload byte of
// the OpSetRegion command.
const (
	RegionAuto Region = iota
	RegionNTSC
	RegionPAL
	RegionDendy
)

// String returns the region's short name.
func (r Region) String() string { return region.Region(r).String() }
