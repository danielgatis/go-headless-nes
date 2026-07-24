// Package region holds the per-TV-system timing parameters that separate
// NTSC, PAL and Dendy consoles: master-clock dividers, PPU frame
// geometry, the odd-frame dot skip, and the audio rates. The values are
// hardware facts (NESdev Wiki); each console resolves a Params once at
// configuration time and copies the scalars into the CPU, PPU and APU so
// nothing does a per-cycle region lookup.
package region

// Region identifies a console's TV system. The zero value is Auto, which
// is only meaningful in the runtime override API (it re-detects from the
// cartridge); a resolved Params is never built from Auto.
type Region uint8

// The regions. The numeric values are wire-stable: they are the payload
// byte of the OpSetRegion protocol command.
const (
	Auto Region = iota
	NTSC
	PAL
	Dendy
)

// String returns the region's short name.
func (r Region) String() string {
	switch r {
	case Auto:
		return "auto"
	case NTSC:
		return "ntsc"
	case PAL:
		return "pal"
	case Dendy:
		return "dendy"
	default:
		return "unknown"
	}
}

// Params is one console's resolved timing. Every field is a fact of the
// TV system; a unit copies the ones it needs when a region is applied.
type Params struct {
	Region Region

	// CPU master-clock split. A CPU cycle advances the shared master clock
	// by CPUStartClock+CPUEndClock ticks; the read/write access point sits
	// one tick either side of the midpoint (handled in the CPU).
	CPUStartClock uint64
	CPUEndClock   uint64

	// PPUDivider is how many master-clock ticks make one PPU dot.
	PPUDivider uint64

	// NMIScanline is the line whose dot 1 raises the vblank flag (Dendy
	// delays it to 291). VBlankEnd is the last vblank line; the pre-render
	// line is the next one and wraps back to -1. DotSkip enables the
	// odd-frame skipped dot (NTSC only).
	NMIScanline int
	VBlankEnd   int
	DotSkip     bool

	// CPUHz is the CPU clock in Hz (master / divider). EmitRate is how many
	// audio samples the APU produces per emulated second, chosen so that a
	// frontend running one emulated frame per display refresh delivers
	// ~44100 samples per real second.
	CPUHz    int
	EmitRate int

	// FPS is the frame rate: a frontend should run this many emulated
	// frames per second (round it for a fixed-tick loop), or PAL content
	// plays at NTSC's 60 and runs ~20% fast.
	FPS float64
}

// ntsc, pal and dendy are the three resolved parameter sets. Master
// clocks: NTSC 21477272 Hz, PAL/Dendy 26601712 Hz.
var (
	ntsc = Params{
		Region:        NTSC,
		CPUStartClock: 6, CPUEndClock: 6, // divider 12
		PPUDivider:  4,
		NMIScanline: 241,
		VBlankEnd:   260, // 262 scanlines total
		DotSkip:     true,
		CPUHz:       21477272 / 12, // 1789772
		EmitRate:    44173,         // 44100 * 60.0988/60
		FPS:         60.0988,
	}
	pal = Params{
		Region:        PAL,
		CPUStartClock: 8, CPUEndClock: 8, // divider 16
		PPUDivider:  5,
		NMIScanline: 241,
		VBlankEnd:   310, // 312 scanlines total
		DotSkip:     false,
		CPUHz:       26601712 / 16, // 1662607
		EmitRate:    44106,         // 44100 * 50.007/50
		FPS:         50.0070,
	}
	dendy = Params{
		Region:        Dendy,
		CPUStartClock: 7, CPUEndClock: 8, // divider 15
		PPUDivider:  5,
		NMIScanline: 291, // the one geometry difference from PAL
		VBlankEnd:   310,
		DotSkip:     false,
		CPUHz:       26601712 / 15, // 1773447
		EmitRate:    44106,         // same fps as PAL
		FPS:         50.0070,
	}
)

// ParamsFor resolves a region to its parameter set. Auto and any unknown
// value fall back to NTSC; callers that want auto-detection resolve the
// cartridge's region first.
func ParamsFor(r Region) Params {
	switch r {
	case PAL:
		return pal
	case Dendy:
		return dendy
	default:
		return ntsc
	}
}
