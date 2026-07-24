// Package ppu emulates the 2C02 picture processing unit as a faithful
// sprite shifters, pixel output and the delayed-state machinery exactly
// as hardware does, so all observable timing (vblank/NMI, sprite 0 hit,
// overflow, the odd-frame skipped dot, $2007 buffering) matches.
//
// Timing model: scanline runs -1 (pre-render) .. 260 (last vblank line),
// cycle runs 0 .. 340. The machine ticks the PPU three times per CPU
// cycle. The NMI line is pushed to the CPU's edge detector on the exact
// dot it changes.
package ppu

import (
	"github.com/danielgatis/go-headless-nes/internal/cartridge"
	"github.com/danielgatis/go-headless-nes/internal/region"
)

// Board is the PPU's view of the cartridge: pattern tables, nametable
// mirroring, and the A12 watch line MMC3-style boards clock IRQs from.
type Board interface {
	ReadCHR(addr uint16) byte
	WriteCHR(addr uint16, v byte)
	Mirroring() cartridge.Mirroring

	// Scanline is called on each filtered rising edge of PPU address line
	// 12 (the MMC3 IRQ clock). The low-time filter lives in the PPU.
	Scanline()
}

// ntPager is an optional Board capability: boards that select the CIRAM
// page of each logical nametable individually (VRC6, Sunsoft-4, ...)
// return the page (0 or 1) per table, overriding Mirroring().
type ntPager interface {
	NametablePage(table byte) byte
}

// ntSource is an optional Board capability: boards that can serve
// nametable fetches from their own memory (CHR-ROM nametables) handle
// the access and report ok; !ok falls back to CIRAM.
type ntSource interface {
	ReadNT(addr uint16) (v byte, ok bool)
	WriteNT(addr uint16, v byte) (ok bool)
}

// vramSniffer is an optional Board capability: boards that watch the raw
// PPU bus address (a VRAM-address change notification) to latch CHR banks on
// nametable fetches (mappers 96, 218, 518, OekaKids-style tablets). The
// PPU calls this with every address it drives onto the PPU bus, before
// the fetch, so the board can react to the previous vs. current address.
type vramSniffer interface {
	NotifyVramAddr(addr uint16)
}

// a12FilterDots is how long A12 must stay low for the next rise to count
// (the MMC3 edge filter, ~3 CPU cycles).
const a12FilterDots = 10

// Frame dimensions are the same on every TV system; only the number of
// vblank lines below the picture changes.
const (
	Width  = 256
	Height = 240
)

// NTSC frame-geometry defaults. Configure overwrites the PPU's fields per
// region: PAL/Dendy run 312 scanlines (vblankEnd 310) with no dot skip,
// and Dendy delays the vblank flag to scanline 291.
const (
	defaultNMIScanline = 241 // vblank flag rises at (241, 1)
	defaultVBlankEnd   = 260 // last vblank scanline; pre-render is -1
)

// TileInfo is one background tile's fetched data.
type TileInfo struct {
	TileAddr      uint16
	LowByte       byte
	HighByte      byte
	PaletteOffset byte
}

// SpriteInfo is one loaded sprite's shifter state.
type SpriteInfo struct {
	BackgroundPriority bool
	SpriteX            byte
	LowByte            byte
	HighByte           byte
	PaletteOffset      byte
}

// spriteShifterDone marks an unused sprite shifter slot.
const spriteShifterDone = 0x8000

// State is the PPU's complete mutable state, copyable by assignment for
// rewind/save-state (no pointers or slices).
type State struct {
	Cycle    int32 // 0..340
	Scanline int16 // -1..260
	Frame    uint64

	VideoRAMAddr    uint16 // v
	TmpVideoRAMAddr uint16 // t
	HighBitShift    uint16
	LowBitShift     uint16
	SpriteRAMAddr   byte // OAMADDR
	OpenBus         byte
	XScroll         byte // fine x
	WriteToggle     bool // $2005/$2006 shared toggle

	NeedStateUpdate      bool
	RenderingEnabled     bool
	PrevRenderingEnabled bool

	Sprite0Visible     bool
	SpriteCount        byte
	SecondaryOamAddr   byte
	OamCopybuffer      byte
	SpriteInRange      bool
	Sprite0Added       bool
	OverflowBugCounter byte
	OamCopyDone        bool
	PpuBusAddress      uint16

	// Sprite evaluation can start on any OAM byte (per OAMADDR); these track
	// the aligned span of the first/last visible sprite for the extra-sprite
	// pass, without disturbing the (possibly misaligned) OAM address itself.
	FirstVisibleSpriteAddr byte
	LastVisibleSpriteAddr  byte

	Tile                TileInfo
	CurrentTilePalette  byte
	PreviousTilePalette byte
	PaletteRAMMask      byte

	SpriteIndex         uint32
	UpdateVramAddr      uint16
	UpdateVramAddrDelay byte
	PreventVblFlag      bool

	Control ControlFlags
	Mask    MaskFlags
	Status  StatusFlags

	SpriteTiles [64]SpriteInfo

	SpriteShifterList      [9]uint16
	NextSpriteShifter      byte
	NextSpriteShifterCycle uint16
	ActiveSpriteShifters   byte
	CountingSpriteShifters byte
	ExpiredSpriteShifters  byte
	DotSkipped             byte
	ProcessSprites         bool

	MinimumDrawBgCycle             uint16
	MinimumDrawSpriteCycle         uint16
	MinimumDrawSpriteStandardCycle uint16

	NeedVideoRAMIncrement bool
	AllowFullPpuAccess    bool

	MemoryDataReadStateMachine  byte
	MemoryDataWriteStateMachine byte
	MemoryDataWriteLatch        byte
	MemoryReadBuffer            byte
	IgnoreVramRead              uint32

	OpenBusDecayStamp [8]uint32

	PaletteRAM         [0x20]byte
	SpriteRAM          [0x100]byte
	SecondarySpriteRAM [0x20]byte
	VRAM               [4096]byte // 2 KiB CIRAM + 2 KiB for four-screen

	// A12 edge detection for mapper IRQ counters.
	A12        bool
	A12LowDots byte

	// BusDataPins is the byte most recently driven onto the shared AD0-7
	// pins by a VRAM read. When a $2007 buffer fill collides with an ALE
	// dot, the address latch keeps these pins instead of the new low byte.
	BusDataPins byte

	// MasterClock is the PPU's position on the shared master clock. It
	// advances one PPU dot per masterClockDivider ticks, so the machine can run the PPU to an exact sub-cycle boundary (see Run).
	MasterClock uint64
}

// defaultMasterClockDivider is how many master-clock ticks make one PPU
// dot (NTSC: 4; PAL/Dendy: 5). Configure overwrites the field.
const defaultMasterClockDivider = 4

// ControlFlags mirrors the decoded $2000 (ControlFlags).
type ControlFlags struct {
	BackgroundPatternAddr uint16 // 0x0000 or 0x1000
	SpritePatternAddr     uint16
	VerticalWrite         bool
	LargeSprites          bool
	NmiOnVerticalBlank    bool
}

// MaskFlags mirrors the decoded $2001 (MaskFlags).
type MaskFlags struct {
	Grayscale         bool
	BackgroundMask    bool
	SpriteMask        bool
	BackgroundEnabled bool
	SpritesEnabled    bool
	IntensifyRed      bool
	IntensifyGreen    bool
	IntensifyBlue     bool
}

// StatusFlags mirrors the $2002 flags (StatusFlags).
type StatusFlags struct {
	SpriteOverflow bool
	Sprite0Hit     bool
	VerticalBlank  bool
}

// PPU couples the copyable State with wiring and the framebuffer.
type PPU struct {
	State
	board Board

	// Per-region frame geometry (see the default constants). Set by
	// Configure; not part of State, since the region is fixed for a machine
	// and never enters a snapshot.
	masterClockDivider uint64
	nmiScanline        int16
	vblankEnd          int16
	dotSkip            bool

	// ntPager/ntSource cache the board's optional nametable capabilities
	// (derived from board at construction, not snapshotted).
	ntPager ntPager
	ntSrc   ntSource
	vramSn  vramSniffer

	// framebuffer holds one NES color index (0-63) per pixel — the value
	// DrawPixel reads out of palette RAM. Derived state: rewind repaints
	// it, so it is not snapshotted.
	framebuffer [Width * Height]byte

	frameComplete bool
	prevNMIOut    bool

	// onNMI pushes the /NMI output level to the CPU's edge detector the
	// instant it changes. Nil in bare-PPU
	// unit tests.
	onNMI func(bool)
}

// New returns a PPU in power-up state attached to a cartridge board.
func New(board Board) *PPU {
	p := &PPU{board: board}
	p.masterClockDivider = defaultMasterClockDivider
	p.nmiScanline = defaultNMIScanline
	p.vblankEnd = defaultVBlankEnd
	p.dotSkip = true
	p.ntPager, _ = board.(ntPager)
	p.ntSrc, _ = board.(ntSource)
	p.vramSn, _ = board.(vramSniffer)
	p.SpriteShifterList[8] = spriteShifterDone
	p.Reset(false)
	// The PPU parks at (scanline -1, cycle 340); its first Exec (the wrap to
	// (0,0)) is a real dot, run by the machine's reset-cycle PPU.Run calls via
	// the shared master clock — no priming here.
	return p
}

// Reset models power-on / the reset line.
func (p *PPU) Reset(softReset bool) {
	// The master-clock position restarts on every reset, unconditionally —
	// the CPU restarts its own too, so the two clocks stay aligned through
	// the eight clocked reset cycles.
	p.MasterClock = 0

	p.PreventVblFlag = false
	p.NeedStateUpdate = false
	p.PrevRenderingEnabled = false
	p.RenderingEnabled = false
	p.IgnoreVramRead = 0
	p.OpenBus = 0
	p.OpenBusDecayStamp = [8]uint32{}

	p.TmpVideoRAMAddr = 0
	p.HighBitShift = 0
	p.LowBitShift = 0
	p.SpriteRAMAddr = 0
	p.XScroll = 0
	p.WriteToggle = false

	p.Control = ControlFlags{}
	p.Mask = MaskFlags{}

	if !softReset {
		p.Status = StatusFlags{}
		p.VideoRAMAddr = 0
		// Static power-on palette.
		p.PaletteRAM = [0x20]byte{
			0x09, 0x01, 0x00, 0x01, 0x00, 0x02, 0x02, 0x0D,
			0x08, 0x10, 0x08, 0x24, 0x00, 0x00, 0x04, 0x2C,
			0x09, 0x01, 0x34, 0x03, 0x00, 0x04, 0x00, 0x14,
			0x08, 0x3A, 0x00, 0x02, 0x00, 0x20, 0x2C, 0x08,
		}
	}

	p.Tile = TileInfo{}
	p.CurrentTilePalette = 0
	p.PreviousTilePalette = 0
	p.PpuBusAddress = 0
	p.PaletteRAMMask = 0x3F
	p.OamCopybuffer = 0
	p.SpriteInRange = false
	p.Sprite0Added = false
	p.OamCopyDone = false

	p.SpriteTiles = [64]SpriteInfo{}
	p.SpriteCount = 0
	p.SecondaryOamAddr = 0
	p.Sprite0Visible = false
	p.SpriteIndex = 0

	// First execution will be cycle 0, scanline 0.
	p.Scanline = -1
	p.Cycle = 340

	p.Frame = 1
	p.MemoryReadBuffer = 0
	p.OverflowBugCounter = 0
	p.UpdateVramAddrDelay = 0
	p.UpdateVramAddr = 0
	// PPU register access is allowed from power-on rather than being gated
	// until the first pre-render line (the first-frame access restriction is
	// not modeled).
	p.AllowFullPpuAccess = true

	for i := range p.SpriteShifterList {
		p.SpriteShifterList[i] = spriteShifterDone
	}

	// The cached /NMI output level must follow the cleared control register,
	// or the dedup in pushNMI would swallow the next genuine rising edge.
	p.prevNMIOut = p.NMILine()

	p.updateMinimumDrawCycles()
}

// Configure applies a region's frame geometry. Call it before the reset
// cycles run.
func (p *PPU) Configure(pr region.Params) {
	p.masterClockDivider = pr.PPUDivider
	p.nmiScanline = int16(pr.NMIScanline)
	p.vblankEnd = int16(pr.VBlankEnd)
	p.dotSkip = pr.DotSkip
}

// SetNMICallback installs the CPU's /NMI-line sink.
func (p *PPU) SetNMICallback(f func(bool)) {
	p.onNMI = f
	if f != nil {
		f(p.NMILine())
	}
}

// Framebuffer is the last completed frame as NES color indices.
func (p *PPU) Framebuffer() *[Width * Height]byte { return &p.framebuffer }

// TakeFrame reports whether a frame completed since the last call.
func (p *PPU) TakeFrame() bool {
	done := p.frameComplete
	p.frameComplete = false
	return done
}

// NMILine reports the current /NMI output level: vblank flag AND enable.
func (p *PPU) NMILine() bool {
	return p.Status.VerticalBlank && p.Control.NmiOnVerticalBlank
}

// setNmiFlag / clearNmiFlag push the /NMI output-level change to the CPU
// . The CPU's own edge detector
// turns a rising edge into a serviced interrupt.
func (p *PPU) setNmiFlag()   { p.pushNMI(true) }
func (p *PPU) clearNmiFlag() { p.pushNMI(false) }

func (p *PPU) pushNMI(vblankEnabledEdge bool) {
	// The NMI line is (vblank flag AND enable). hardware pulls /NMI when
	// both are true and ClearNmiFlag otherwise; recompute the level.
	out := p.NMILine()
	if out != p.prevNMIOut && p.onNMI != nil {
		p.onNMI(out)
	}
	p.prevNMIOut = out
	_ = vblankEnabledEdge
}

// SyncRestored recomputes derived interrupt-edge state after State was
// replaced wholesale (save-state load, rewind).
func (p *PPU) SyncRestored() {
	p.prevNMIOut = p.NMILine()
	p.frameComplete = false
}

// isRenderingEnabled reports the delayed rendering-enabled flag.
func (p *PPU) isRenderingEnabled() bool { return p.RenderingEnabled }

// Tick advances the PPU by one dot, also advancing
// its master clock so a future master-clock-driven Run stays aligned.
func (p *PPU) Tick() {
	p.decayOpenBus()
	if !p.A12 && p.A12LowDots < 255 {
		p.A12LowDots++
	}
	p.exec()
	p.MasterClock += p.masterClockDivider
}

// Run advances the PPU until its master clock reaches runTo, executing at
// least one dot. Reserved for a future master-clock-
// driven machine glue for the per-sub-cycle PPU
// sync (which requires also porting the reset-cycle interleaving); the
// machine currently ticks the PPU with a fixed dots-before/after split.
func (p *PPU) Run(runTo uint64) {
	for {
		p.Tick()
		if p.MasterClock+p.masterClockDivider > runTo {
			break
		}
	}
}

func (p *PPU) exec() {
	if p.Cycle < 340 {
		// Cycles 1..340.
		p.Cycle++
		if p.Scanline < 240 {
			p.processScanline()
		} else if p.Cycle == 1 && p.Scanline == p.nmiScanline {
			if !p.PreventVblFlag {
				p.Status.VerticalBlank = true
				p.beginVBlank()
			}
			p.PreventVblFlag = false
		}
	} else {
		// Cycle 0.
		p.processScanlineFirstCycle()
	}

	if p.NeedStateUpdate {
		p.updateState()
	}
}

// processScanlineFirstCycle runs at cycle 0.
func (p *PPU) processScanlineFirstCycle() {
	p.Cycle = 0
	p.Scanline++
	if p.Scanline > p.vblankEnd {
		p.Scanline = -1
		// Force pre-render sprite fetches to load the dummy $FF tiles.
		p.SpriteCount = 0
		p.updateMinimumDrawCycles()
	}

	if p.Scanline < 240 {
		sortShifters(&p.SpriteShifterList)
		p.NextSpriteShifter = 0
		p.NextSpriteShifterCycle = p.SpriteShifterList[0] >> 4

		if p.Scanline == -1 {
			p.Status.SpriteOverflow = false
			p.Status.Sprite0Hit = false
			p.AllowFullPpuAccess = true
			// The OAM address bug: if the OAM row changes at the start of the
			// pre-render line while rendering, that row is corrupted.
			if p.isRenderingEnabled() {
				p.corruptOamRow(p.SpriteRAMAddr>>3, p.SecondaryOamAddr&0x1F)
			}
		} else if p.PrevRenderingEnabled {
			p.SecondaryOamAddr = 0
			if p.Scanline > 0 || p.Frame&1 == 0 {
				// Set bus address from the unused NT fetches at the end of
				// the previous scanline (skipped on line 0 after a skipped dot).
				p.setBusAddress((p.Tile.TileAddr << 4) | (p.VideoRAMAddr >> 12) | p.Control.BackgroundPatternAddr)
			}
		}
	} else if p.Scanline == 240 {
		if p.PrevRenderingEnabled {
			p.SecondaryOamAddr = 0
		}
		// At the start of vblank the bus address returns to v.
		p.setBusAddress(p.VideoRAMAddr & 0x3FFF)
		p.sendFrame()
		p.Frame++
	}
}

// beginVBlank triggers the NMI at the start of vblank.
func (p *PPU) beginVBlank() {
	if p.Control.NmiOnVerticalBlank {
		p.setNmiFlag()
	}
}

// sendFrame publishes the completed frame.
func (p *PPU) sendFrame() {
	p.frameComplete = true
}

// setBusAddress records the PPU bus address, notifies any VRAM sniffer,
// and drives A12 to the board.
func (p *PPU) setBusAddress(addr uint16) {
	p.PpuBusAddress = addr
	if p.vramSn != nil {
		p.vramSn.NotifyVramAddr(addr)
	}
	p.driveA12(addr&0x1000 != 0)
}

// driveA12 notifies the board on filtered rising edges of A12.
func (p *PPU) driveA12(high bool) {
	if high == p.A12 {
		return
	}
	if high && p.A12LowDots >= a12FilterDots {
		p.board.Scanline()
	}
	p.A12 = high
	if !high {
		p.A12LowDots = 0
	}
}

// updateMinimumDrawCycles recomputes the left-edge clip cycles. The emulator "force first column" options are
// off, so background/sprites are drawn from cycle 8 unless their mask bit
// is set (then cycle 0).
func (p *PPU) updateMinimumDrawCycles() {
	if p.Mask.BackgroundEnabled {
		if p.Mask.BackgroundMask {
			p.MinimumDrawBgCycle = 0
		} else {
			p.MinimumDrawBgCycle = 8
		}
	} else {
		p.MinimumDrawBgCycle = 300
	}
	if p.Mask.SpritesEnabled {
		if p.Mask.SpriteMask {
			p.MinimumDrawSpriteCycle = 0
		} else {
			p.MinimumDrawSpriteCycle = 8
		}
	} else {
		p.MinimumDrawSpriteCycle = 300
	}
	if p.Mask.SpritesEnabled {
		if p.Mask.SpriteMask {
			p.MinimumDrawSpriteStandardCycle = 0
		} else {
			p.MinimumDrawSpriteStandardCycle = 8
		}
	} else {
		p.MinimumDrawSpriteStandardCycle = 300
	}
}

// updateColorBitMasks recomputes the grayscale mask.
func (p *PPU) updateColorBitMasks() {
	if p.Mask.Grayscale {
		p.PaletteRAMMask = 0x30
	} else {
		p.PaletteRAMMask = 0x3F
	}
}

// sortShifters sorts the 8 active sprite shifter slots by X (slot 8 stays
// the sentinel). a sort over the shifter slots.
func sortShifters(list *[9]uint16) {
	// Insertion sort over the first 8 entries; small fixed size.
	for i := 1; i < 8; i++ {
		v := list[i]
		j := i - 1
		for j >= 0 && list[j] > v {
			list[j+1] = list[j]
			j--
		}
		list[j+1] = v
	}
}
