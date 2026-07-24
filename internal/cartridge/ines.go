package cartridge

import (
	"hash/crc32"
	"io"
	"os"

	"github.com/danielgatis/go-headless-nes/internal/errs"
	"github.com/danielgatis/go-headless-nes/internal/gamedb"
	"github.com/danielgatis/go-headless-nes/internal/region"
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
// extended mapper bits, the submapper, the larger bank-count field, and
// the finer region encoding; the exotic size encodings are not needed
// for the boards this emulator emulates.
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
		// Byte 12 bits 0-1: 0=NTSC, 1=PAL, 2=multi-region, 3=Dendy. A
		// multi-region cartridge runs on NTSC hardware here.
		switch h[12] & 0x03 {
		case 1:
			c.Region = region.PAL
		case 3:
			c.Region = region.Dendy
		default:
			c.Region = region.NTSC
		}
	} else {
		// iNES 1.0 byte 9 bit 0: 0=NTSC, 1=PAL.
		if h[9]&0x01 != 0 {
			c.Region = region.PAL
		} else {
			c.Region = region.NTSC
		}
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

	// The cartridge database is keyed by the CRC32 of the raw PRG then CHR
	// data, exactly as dumped, so accumulate it before PRG is mirrored.
	crc := crc32.NewIEEE()

	c.PRG = make([]byte, prgBanks*prgUnit)
	if _, err := io.ReadFull(r, c.PRG); err != nil {
		return nil, errs.Wrap(err, "reading PRG ROM")
	}
	_, _ = crc.Write(c.PRG)
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
		_, _ = crc.Write(c.CHR)
	}

	// The database corrects headers that misreport the TV system, mirroring
	// or mapper. When a ROM is listed, its fields win over the header, the
	// same policy the reference emulator follows.
	applyDatabase(c, crc.Sum32())
	return c, nil
}

// applyDatabase overrides header-derived fields with the cartridge
// database entry for crc, when one exists. It mirrors the reference
// emulator's precedence: the database wins on the fields this core models
// (TV system, mapper and submapper, mirroring, battery).
func applyDatabase(c *Cartridge, crc uint32) {
	e, ok := gamedb.Lookup(crc)
	if !ok {
		return
	}
	c.Region = e.Region
	if e.HasMapper {
		c.MapperID = e.MapperID
		c.Submapper = e.Submapper
	}
	switch e.Mirroring {
	case 'h':
		c.Mirroring = Horizontal
	case 'v':
		c.Mirroring = Vertical
	case '4':
		c.Mirroring = FourScreen
	case '0':
		c.Mirroring = SingleLow
	case '1':
		c.Mirroring = SingleHigh
	}
	// A validated row's battery flag is authoritative; an unvalidated row
	// can only add a battery, never clear the header's.
	if e.Validated {
		c.HasBattery = e.HasBattery
	} else {
		c.HasBattery = c.HasBattery || e.HasBattery
	}
}
