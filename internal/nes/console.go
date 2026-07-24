// Package nes assembles the chips into a machine on one shared master
// clock: everything advances from inside the CPU's bus cycles. NES is
// the single root of all emulated state.
package nes

import (
	"github.com/danielgatis/go-headless-nes/internal/apu"
	"github.com/danielgatis/go-headless-nes/internal/bus"
	"github.com/danielgatis/go-headless-nes/internal/cartridge"
	"github.com/danielgatis/go-headless-nes/internal/controller"
	"github.com/danielgatis/go-headless-nes/internal/cpu"
	"github.com/danielgatis/go-headless-nes/internal/mapper"
	"github.com/danielgatis/go-headless-nes/internal/ppu"
	"github.com/danielgatis/go-headless-nes/internal/region"
)

// NES is a whole console: the CPU, picture unit, audio unit, cartridge
// board, controllers and the memory fabric that binds them.
type NES struct {
	CPU         *cpu.CPU
	PPU         *ppu.PPU
	APU         *apu.APU
	Mapper      mapper.Mapper
	Controllers [2]controller.Controller
	Cart        *cartridge.Cartridge

	mem  *bus.Memory
	ctrl *controller.Manager

	// region is the resolved TV system currently in effect (never Auto).
	region region.Params

	// dmcRequest/dmcStop are the console's canonical DMC-DMA hooks, created
	// once here so snapshot Restore can re-install them without allocating
	// new method values.
	dmcRequest func()
	dmcStop    func()
}

// prgHandler maps the cartridge board's PRG side ($4020-$FFFF) into the CPU
// address space and forwards read-modify-write second writes so boards that
// filter consecutive writes (MMC1) can drop them. It hands the board the
// current external open-bus value before each read, so an address the board
// does not decode ($4020-$5FFF on a simple board) returns the real floating
// bus value rather than a fabricated one.
type prgHandler struct {
	m       mapper.Mapper
	openBus func() byte
}

func (h *prgHandler) Ranges() *bus.Ranges {
	r := bus.NewRanges()
	r.Add(bus.OpRead, 0x4020, 0xFFFF)
	r.Add(bus.OpWrite, 0x4020, 0xFFFF)
	return r
}
func (h *prgHandler) ReadReg(addr uint16) byte {
	h.m.SetOpenBus(h.openBus())
	return h.m.ReadPRG(addr)
}
func (h *prgHandler) WriteReg(addr uint16, v byte) { h.m.WritePRG(addr, v) }
func (h *prgHandler) PeekReg(addr uint16) byte     { return h.m.ReadPRG(addr) }
func (h *prgHandler) WriteRMWSecond(addr uint16, v byte) {
	if h.m.FiltersConsecutiveWrites() {
		return
	}
	h.m.WritePRG(addr, v)
}

// ciramUser is an optional board capability: boards that can put the
// console's own VRAM on their CHR bus (Namco 163, MMC5) receive direct
// CIRAM accessors when the console is wired.
type ciramUser interface {
	SetCIRAM(read func(idx uint16) byte, write func(idx uint16, v byte))
}

// audioSource is an optional board capability: expansion audio mixed
// into the APU output.
type audioSource interface {
	AudioLevel() float32
}

// ppuRegSniffer is an optional Board capability (MMC5): observe the
// CPU's writes to the PPU registers ($2000-$2007, not their mirrors).
type ppuRegSniffer interface {
	SniffPPUReg(addr uint16, v byte)
}

// ppuPosUser is an optional board capability: boards whose behavior keys
// off the live raster position (Nanjing's automatic CHR switch) receive an
// accessor for the PPU's current scanline and dot.
type ppuPosUser interface {
	SetPPUPos(pos func() (scanline, dot int))
}

// cpuWriteChecker is an optional board capability: boards whose IRQ
// counter can tick on CPU write cycles (JY Company) receive an accessor
// reporting whether the current bus cycle is a write.
type cpuWriteChecker interface {
	SetCPUWriteCheck(isWrite func() bool)
}

// cpuPeeker is an optional board capability: boards that observe CPU
// memory beyond the cartridge port (Dragon Fighter's protection reads
// zero-page RAM) receive a side-effect-free bus peek.
type cpuPeeker interface {
	SetCPUPeek(peek func(addr uint16) byte)
}

// resettable is an optional board capability: boards whose bank mode is
// cycled by the console RESET line (multicarts like mappers 60/230/233/
// 313) observe the reset here. soft is true for a soft reset (the reset
// button), false for a power cycle.
type resettable interface {
	Reset(soft bool)
}

// ppuSniffHandler overrides the PPU register writes to forward each one
// to both the PPU and the board.
type ppuSniffHandler struct {
	ppu     *ppu.PPU
	sniffer ppuRegSniffer
}

func (h *ppuSniffHandler) Ranges() *bus.Ranges {
	r := bus.NewRanges().Override()
	r.Add(bus.OpWrite, 0x2000, 0x2007)
	return r
}
func (h *ppuSniffHandler) ReadReg(_ uint16) byte { return 0 }
func (h *ppuSniffHandler) PeekReg(_ uint16) byte { return 0 }
func (h *ppuSniffHandler) WriteReg(addr uint16, v byte) {
	h.ppu.WriteRegister(addr, v)
	h.sniffer.SniffPPUReg(addr, v)
}

// oamDMAHandler catches the $4014 write that starts an OAM DMA.
type oamDMAHandler struct {
	start func(page byte)
}

func (h *oamDMAHandler) Ranges() *bus.Ranges {
	r := bus.NewRanges()
	r.AddOne(bus.OpWrite, 0x4014)
	return r
}
func (h *oamDMAHandler) ReadReg(_ uint16) byte     { return 0 }
func (h *oamDMAHandler) PeekReg(_ uint16) byte     { return 0 }
func (h *oamDMAHandler) WriteReg(_ uint16, v byte) { h.start(v) }

// New builds and powers on a console around a cartridge.
func New(cart *cartridge.Cartridge) (*NES, error) {
	m, err := mapper.New(cart)
	if err != nil {
		return nil, err
	}
	c := &NES{Mapper: m, Cart: cart}

	c.mem = bus.New()
	c.PPU = ppu.New(m)
	c.APU = apu.New(nil, nil)

	// The control manager reads open bus for the undriven joypad bits and the
	// CPU cycle count for its read-clock cache.
	c.ctrl = controller.NewManager(&c.Controllers, c.mem.OpenBus)

	// Boards that can route console VRAM onto their CHR or nametable
	// buses (Namco 163, MMC5) get direct CIRAM accessors.
	if cu, ok := m.(ciramUser); ok {
		cu.SetCIRAM(
			func(idx uint16) byte { return c.PPU.VRAM[idx&0x7FF] },
			func(idx uint16, v byte) { c.PPU.VRAM[idx&0x7FF] = v },
		)
	}

	// Map every device into the address space BEFORE the CPU is created, so
	// the CPU's reset can read the reset vector from the board (not open bus).
	prg := &prgHandler{m: m, openBus: c.mem.OpenBus}
	c.mem.Register(c.PPU)
	if sniff, ok := m.(ppuRegSniffer); ok {
		c.mem.Register(&ppuSniffHandler{ppu: c.PPU, sniffer: sniff})
	}
	c.mem.Register(c.APU)
	c.mem.Register(c.ctrl)
	c.mem.Register(prg)

	c.CPU = cpu.New(c.mem)
	if pu, ok := m.(ppuPosUser); ok {
		pu.SetPPUPos(func() (int, int) {
			return int(c.PPU.Scanline), int(c.PPU.Cycle)
		})
	}
	if wc, ok := m.(cpuWriteChecker); ok {
		wc.SetCPUWriteCheck(c.CPU.WriteCycle)
	}
	if pk, ok := m.(cpuPeeker); ok {
		pk.SetCPUPeek(c.mem.Peek)
	}
	c.ctrl.SetCycleCount(func() uint64 { return c.CPU.Cycles })
	c.mem.Register(&oamDMAHandler{start: c.CPU.StartOAMDMA})

	// The DMC fetches sample bytes through the CPU's DMA unit: it requests a
	// transfer, and the DMA delivers the byte back via the hooks below.
	c.dmcRequest = c.CPU.StartDmcTransfer
	c.dmcStop = c.CPU.StopDmcTransfer
	c.APU.SetDMAHooks(c.dmcRequest, c.dmcStop)
	c.CPU.SetDMCHooks(c.APU.DMCReadAddr, c.APU.DMCDeliver)
	// The frame-counter IRQ self-clear is timed by the CPU cycle count (the
	// "master clock" the frame counter observes advances one per CPU cycle,
	// so its parity picks out get/put cycles).
	c.APU.SetClock(func() uint64 { return c.CPU.Cycles })
	// Bit 5 of a $4015 read comes from the internal open-bus latch.
	c.APU.SetInternalOpenBus(c.mem.InternalOpenBus)
	// Boards with expansion audio feed the mixer directly.
	if src, ok := m.(audioSource); ok {
		c.APU.SetExternalAudio(src.AudioLevel)
	}

	// Apply the cartridge's TV system to every timed unit, then seed the
	// master clock at the region's divider before the reset cycles run.
	c.configureRegion(region.ParamsFor(cart.Region))
	c.CPU.SeedMasterClock()

	// Per-cycle machine callbacks and interrupt wiring.
	c.CPU.SetTicker(c.clockBefore, c.clockAfter, c.sample)
	c.PPU.SetNMICallback(c.CPU.SetNMILine)

	// Run the eight reset cycles now that every callback is wired: they clock
	// the picture/audio units and the board through the per-cycle callbacks,
	// bringing the whole machine to the post-reset master-clock position.
	c.CPU.RunResetCycles()
	return c, nil
}

// configureRegion applies a resolved TV system to the CPU, PPU and APU.
func (c *NES) configureRegion(p region.Params) {
	c.region = p
	c.CPU.Configure(p)
	c.PPU.Configure(p)
	c.APU.Configure(p)
}

// Region reports the TV system currently in effect (never Auto).
func (c *NES) Region() region.Region { return c.region.Region }

// FrameRate is how many emulated frames make one second on the current
// TV system (NTSC ~60.0988, PAL/Dendy ~50.0070). A frontend should drive
// this many frames per second.
func (c *NES) FrameRate() float64 { return c.region.FPS }

// SetRegion switches the console's TV system at runtime. Auto re-detects
// from the cartridge header. The switch is live and does not reset the
// console: the timed units simply pick up the new dividers, frame
// geometry and audio tables on their next cycle, the same way real
// hardware keeps running when its region is changed. The region is a
// fixed property of the machine, so it is not part of a save state.
func (c *NES) SetRegion(r region.Region) {
	if r == region.Auto {
		r = c.Cart.Region
	}
	c.configureRegion(region.ParamsFor(r))
}

// clockBefore opens a CPU cycle: run the PPU to the master-clock position of
// the bus access, then clock the rest of the machine for the cycle in
// hardware order, board, audio unit, then any pending controller strobe.
func (c *NES) clockBefore() {
	c.PPU.Run(c.CPU.PPURunTarget())
	c.Mapper.Tick()
	c.APU.Tick()
	if c.ctrl.HasPendingWrites() {
		c.ctrl.ProcessWrites()
	}
}

// clockAfter closes a CPU cycle: run the PPU to the end-of-cycle position.
func (c *NES) clockAfter() {
	c.PPU.Run(c.CPU.PPURunTarget())
}

// sample latches the interrupt-line inputs after each bus access. Each IRQ
// source drives its own bit of the CPU's wired-OR IRQ line independently
// (frame counter, DMC, and the cartridge board's external IRQ), matching how
// hardware tracks the sources separately.
func (c *NES) sample() {
	if c.APU.FrameIRQ {
		c.CPU.SetIrqSource(cpu.IRQFrameCounter)
	} else {
		c.CPU.ClearIrqSource(cpu.IRQFrameCounter)
	}
	if c.APU.DMC.IRQ {
		c.CPU.SetIrqSource(cpu.IRQDMC)
	} else {
		c.CPU.ClearIrqSource(cpu.IRQDMC)
	}
	if c.Mapper.IRQ() {
		c.CPU.SetIrqSource(cpu.IRQExternal)
	} else {
		c.CPU.ClearIrqSource(cpu.IRQExternal)
	}
}

// Step executes one CPU instruction; the rest of the machine advances inside
// its bus cycles.
func (c *NES) Step() int { return c.CPU.Step() }

// RunFrame steps the machine until the picture unit completes a frame.
func (c *NES) RunFrame() {
	for {
		c.Step()
		if c.PPU.TakeFrame() {
			return
		}
	}
}

// Reset presses the console's reset button. Every unit resets in hardware
// order, picture, audio, then the CPU, whose eight clocked reset cycles run
// last so they advance the freshly restarted master clocks together. RAM,
// VRAM, OAM and the vblank flag keep their contents, as on the real
// front-panel button.
func (c *NES) Reset() {
	if r, ok := c.Mapper.(resettable); ok {
		r.Reset(true)
	}
	c.PPU.Reset(true)
	c.APU.Reset()
	c.CPU.Reset()
	c.CPU.RunResetCycles()
}

// Peek reads a byte without side effects (debuggers, trace, tests).
// The APU/IO block ($4000-$401F, minus the joypad ports) reads back as
// $FF, matching the debugger convention of the emulator that produced
// the canonical nestest.log, the same convention the previous bus kept.
func (c *NES) Peek(addr uint16) byte {
	if addr >= 0x4000 && addr < 0x4020 && addr != 0x4016 && addr != 0x4017 {
		return 0xFF
	}
	return c.mem.Peek(addr)
}

// Write performs a side-effecting CPU-visible write (for tests and tooling).
func (c *NES) Write(addr uint16, v byte) { c.mem.Write(addr, v) }

// MasterClock is the shared master-clock position in ticks since
// power-on (21.477272 MHz on NTSC, 26.601712 MHz on PAL and Dendy).
func (c *NES) MasterClock() uint64 { return c.CPU.MasterClock() }

// Framebuffer is the last completed frame as NES color indices.
func (c *NES) Framebuffer() *[ppu.Width * ppu.Height]byte { return c.PPU.Framebuffer() }

// DrainSamples copies buffered audio samples to dst.
func (c *NES) DrainSamples(dst []float32) int { return c.APU.DrainSamples(dst) }
