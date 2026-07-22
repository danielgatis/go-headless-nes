package cartridge

import (
	"io"
	"os"

	"github.com/danielgatis/go-headless-nes/internal/errs"
)

// iNES file layout: 16-byte header, optional 512-byte trainer,
// PRG ROM in 16 KiB units, CHR ROM in 8 KiB units.
// Reference: https://www.nesdev.org/wiki/INES
const (
	headerSize  = 16
	trainerSize = 512
	prgUnit     = 16 * 1024
	chrUnit     = 8 * 1024
)

var magic = [4]byte{'N', 'E', 'S', 0x1a}

// ErrNotINES is returned when the input does not start with the iNES
// magic. Match it with errors.Is.
var ErrNotINES = errs.New("not an iNES file")

// LoadFile reads and parses an iNES ROM image from disk.
func LoadFile(path string) (*Cartridge, error) {
	// The caller chooses which ROM file to load; opening that path is the
	// whole job of this function.
	f, err := os.Open(path) //nolint:gosec // G304: opening the caller's ROM path is this loader's job
	if err != nil {
		return nil, errs.Wrap(err, "opening ROM")
	}
	defer func() { _ = f.Close() }()
	c, err := Load(f)
	if err != nil {
		return nil, errs.Wrapf(err, "%s", path)
	}
	return c, nil
}

// Load parses an iNES or NES 2.0 ROM image. NES 2.0 contributes the
// extended mapper bits, the submapper, and the larger bank-count field;
// the exotic size encodings and region fields are not needed for the
// boards this emulator emulates.
func Load(r io.Reader) (*Cartridge, error) {
	var h [headerSize]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return nil, errs.Wrap(err, "reading header")
	}
	if [4]byte(h[0:4]) != magic {
		return nil, errs.Wrap(ErrNotINES, "bad header magic")
	}

	prgBanks := int(h[4])
	chrBanks := int(h[5])
	flags6 := h[6]
	flags7 := h[7]
	nes2 := flags7&0x0C == 0x08

	// Archaic dumps carry ripper signatures ("DiskDude!") in bytes
	// 8-15, spilling garbage into byte 7. When the reserved tail is
	// dirty and the header is not NES 2.0, only byte 6's mapper nibble
	// can be trusted.
	if !nes2 && [4]byte(h[12:16]) != [4]byte{} {
		flags7 = 0
	}

	c := &Cartridge{
		MapperID:   uint16(flags7&0xf0) | uint16(flags6>>4),
		HasBattery: flags6&0x02 != 0,
	}
	if nes2 {
		c.MapperID |= uint16(h[8]&0x0F) << 8
		c.Submapper = h[8] >> 4
		// NES 2.0 extends the bank counts to 12 bits. The exponent
		// encoding (high nibble F) marks sizes beyond this scheme.
		if h[9]&0x0F == 0x0F || h[9]>>4 == 0x0F {
			return nil, errs.New("NES 2.0 exponent ROM sizes not supported")
		}
		prgBanks |= int(h[9]&0x0F) << 8
		chrBanks |= int(h[9]>>4) << 8
	}

	if prgBanks == 0 {
		return nil, errs.New("header declares no PRG ROM")
	}
	// Sanity cap far above any board this emulator supports, so a
	// malformed header cannot demand a giant allocation.
	if prgBanks > 256 || chrBanks > 256 {
		return nil, errs.Errorf("implausible ROM size: %d PRG + %d CHR banks", prgBanks, chrBanks)
	}

	switch {
	case flags6&0x08 != 0:
		c.Mirroring = FourScreen
	case flags6&0x01 != 0:
		c.Mirroring = Vertical
	default:
		c.Mirroring = Horizontal
	}

	// A trainer is a 512-byte code stub some old dumpers prepended.
	// It has no use on real hardware; skip it.
	if flags6&0x04 != 0 {
		if _, err := io.CopyN(io.Discard, r, trainerSize); err != nil {
			return nil, errs.Wrap(err, "skipping trainer")
		}
	}

	c.PRG = make([]byte, prgBanks*prgUnit)
	if _, err := io.ReadFull(r, c.PRG); err != nil {
		return nil, errs.Wrap(err, "reading PRG ROM")
	}
	// A 16 KiB PRG ROM on a board with a 32 KiB window repeats through
	// the unconnected address line. Pre-mirroring keeps every mapper's
	// bank arithmetic safe regardless of window size.
	if len(c.PRG) < 2*prgUnit {
		c.PRG = append(c.PRG, c.PRG...)
	}
	if chrBanks > 0 {
		c.CHR = make([]byte, chrBanks*chrUnit)
		if _, err := io.ReadFull(r, c.CHR); err != nil {
			return nil, errs.Wrap(err, "reading CHR ROM")
		}
	}
	return c, nil
}
