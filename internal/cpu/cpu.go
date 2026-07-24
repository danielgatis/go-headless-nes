// Package cpu implements the Ricoh 2A03's 6502 core: the opcode and
// addressing tables, the master-clock cycle split (startCycle/endCycle),
// the two-stage NMI edge detector and IRQ pipeline, the end-of-instruction
// interrupt dispatch, and the cycle-accurate DMA unit driving OAM and DMC
// transfers with RDY-halt / dummy-read behavior.
package cpu

import (
	"github.com/danielgatis/go-headless-nes/internal/bus"
	"github.com/danielgatis/go-headless-nes/internal/region"
)

// sysHooks is what the CPU calls into the rest of the machine on each bus
// cycle: clockStart runs the other chips up to the access point of a cycle,
// clockEnd runs the remainder, and sample latches the interrupt-line inputs.
// The console installs these; they drive the picture/audio units and the
// cartridge board off the CPU's shared master clock.
type sysHooks struct {
	clockStart func()
	clockEnd   func()
	sample     func()
}

// Status flag bits of the P register.
const (
	FlagC byte = 1 << iota // carry
	FlagZ                  // zero
	FlagI                  // interrupt disable
	FlagD                  // decimal (settable but ignored by the 2A03)
	FlagB                  // "break": only exists on pushed copies of P
	FlagU                  // unused/reserved, reads as 1
	FlagV                  // overflow
	FlagN                  // negative
)

// Interrupt vectors.
const (
	VecNMI   = 0xFFFA
	VecReset = 0xFFFC
	VecIRQ   = 0xFFFE
)

// Master-clock split, NTSC defaults. A CPU cycle advances the master
// clock by startClockCount+endClockCount ticks (12 on NTSC, 16 on PAL,
// 15 on Dendy); the read/write access point sits one tick either side of
// the midpoint, so a read lands at masterClock+5 and a write at +7 on
// NTSC. Configure overwrites these per region.
const (
	defaultStartClockCount = 6
	defaultEndClockCount   = 6
)

// Registers is the programmer-visible CPU state. It is a plain value so
// snapshots can copy it with an assignment.
type Registers struct {
	A, X, Y byte
	SP      byte
	P       byte
	PC      uint16
}

// State is the CPU's complete mutable state, separated from its bus
// wiring so a snapshot can copy it by value.
type State struct {
	Reg    Registers
	Cycles uint64 // CPU cycles since power-on

	// Interrupt recognition, : The NMI edge detector polls the
	// /NMI line during φ2 of each cycle (endCycle); the IRQ pipeline keeps
	// the previous cycle's value because "it's the status of the interrupt
	// lines at the end of the second-to-last cycle that matters."
	NMIFlag bool // the raw /NMI input level
	IRQFlag byte // wired-OR of the IRQ sources

	needNMI     bool
	prevNeedNMI bool
	prevNMIFlag bool
	runIRQ      bool
	prevRunIRQ  bool

	irqMask byte // 0xFF disables IRQ

	// DMA scratch.
	spriteDMATransfer bool
	spriteDMAOffset   byte
	dmcDMARunning     bool
	abortDMCDma       bool
	needHalt          bool
	needDummyRead     bool

	crashed bool // a HLT opcode was executed; only reset recovers

	masterClock uint64 // shared master clock position
	ppuOffset   uint64 // _ppuOffset (CPU/PPU alignment)

	// Stall and Jammed exist for the debugger's benefit. In the hardware model
	// DMA runs inline within an instruction's read cycles (no separate stall
	// steps), so Stall is always 0; Jammed mirrors crashed.
	Stall  uint16
	Jammed bool
}

// CPU is a 6502 core wired directly to the address space.
type CPU struct {
	State
	mem *bus.Memory
	sysHooks

	// Per-region master-clock split (see the default constants). Set by
	// Configure; not part of State, since the region is fixed for a machine
	// and never enters a snapshot.
	startClockCount uint64
	endClockCount   uint64

	instAddrMode addrMode // current instruction's addressing mode
	operand      uint16   // resolved operand (address or value)
	cpuWrite     bool     // true during a write access

	// dmcAddr/dmcDeliver wire the audio unit's DMC DMA into the CPU's DMA
	// unit: dmcAddr returns the sample address to read, dmcDeliver hands the
	// fetched byte back.
	dmcAddr    func() uint16
	dmcDeliver func(byte)
}

// New returns a 6502 wired to the given address space, in power-on state.
func New(mem *bus.Memory) *CPU {
	c := &CPU{mem: mem}
	c.startClockCount = defaultStartClockCount
	c.endClockCount = defaultEndClockCount
	c.ppuOffset = 1
	c.Reg.SP = 0
	c.Reg.P = FlagU
	c.Reset()
	return c
}

// Configure applies a region's master-clock split. It changes only the
// per-cycle increment, so it is safe to call live: the very next cycle
// advances the master clock by the new divider, with no reset and no jump
// in the clock position (matching how a real region switch behaves). Use
// SeedMasterClock once at power-on to place the clock origin.
func (c *CPU) Configure(p region.Params) {
	c.startClockCount = p.CPUStartClock
	c.endClockCount = p.CPUEndClock
}

// SeedMasterClock places the master clock at one divider's worth of
// ticks, the power-on origin. Reset does the same; New calls this after
// Configure so the origin reflects the region's divider without re-running
// the reset sequence's stack adjustment.
func (c *CPU) SeedMasterClock() { c.masterClock = c.cpuDivider() }

// cpuDivider is the master-clock ticks per CPU cycle (start+end).
func (c *CPU) cpuDivider() uint64 { return c.startClockCount + c.endClockCount }

// SetTicker installs the per-cycle machine callbacks. clockStart runs the
// pre-access portion of a cycle, clockEnd the post-access portion; sample
// latches interrupt inputs each cycle.
func (c *CPU) SetTicker(clockStart, clockEnd, sample func()) {
	c.clockStart = clockStart
	c.clockEnd = clockEnd
	c.sample = sample
}

// WriteCycle reports whether the current bus cycle is a CPU write. Boards
// whose IRQ counter ticks on CPU write cycles (JY Company) sample this from
// the per-cycle Tick hook.
func (c *CPU) WriteCycle() bool { return c.cpuWrite }

// SetDMCHooks wires the APU's DMC DMA into the CPU's DMA unit.
func (c *CPU) SetDMCHooks(addr func() uint16, deliver func(byte)) {
	c.dmcAddr = addr
	c.dmcDeliver = deliver
}

// SetPPUOffset sets the CPU/PPU alignment. Default 1.
func (c *CPU) SetPPUOffset(off uint64) { c.ppuOffset = off }

// MasterClock is the CPU's position on the shared master clock.
func (c *CPU) MasterClock() uint64 { return c.masterClock }

// PPURunTarget is the master-clock position the PPU must be advanced to at
// the current point in a CPU cycle (the masterClock - ppuOffset). The
// nes package reads it in the clockStart/clockEnd callbacks to run the PPU
// to the exact sub-cycle boundary, rather than a fixed dots-per-phase split.
func (c *CPU) PPURunTarget() uint64 { return c.masterClock - c.ppuOffset }

// Reset performs the hardware reset sequence: I is set, SP decrements by
// three (writes suppressed), PC loads from $FFFC. The sequence occupies
// eight cycles (run by RunResetCycles once the machine callbacks are wired).
func (c *CPU) Reset() {
	c.NMIFlag = false
	c.IRQFlag = 0
	c.spriteDMATransfer = false
	c.dmcDMARunning = false
	c.abortDMCDma = false
	c.needHalt = false
	c.needDummyRead = false
	c.crashed = false
	c.Jammed = false
	c.Stall = 0

	// Read the reset vector directly, without clocking the machine (so the
	// picture/audio units do not tick here).
	lo := uint16(c.mem.Read(VecReset))
	hi := uint16(c.mem.Read(VecReset + 1))
	c.Reg.PC = hi<<8 | lo

	c.Reg.P |= FlagI
	c.Reg.SP -= 3

	c.runIRQ = false
	c.prevRunIRQ = false
	c.needNMI = false
	c.prevNeedNMI = false
	c.prevNMIFlag = false

	// Establish the master-clock origin:
	// CycleCount starts at -1 and the master clock at cpuDivider, so the first
	// of the eight reset cycles (run by RunResetCycles once the machine
	// callbacks are wired) lands CycleCount at 0 and keeps the CPU/PPU clocks
	// aligned on the shared master clock.
	c.Cycles = ^uint64(0) // (uint64)-1
	c.masterClock = c.cpuDivider()
}

// RunResetCycles runs the eight CPU cycles the 2A03 spends before it fetches
// the first opcode after a reset/power-up, clocking the machine through the installed
// callbacks. Must be called after SetTicker; the console calls it in
// place of any lumped PPU catch-up.
func (c *CPU) RunResetCycles() {
	for i := 0; i < 8; i++ {
		c.startCycle(true)
		c.endCycle(true)
	}
}

// --- Interrupt line inputs (driven by the machine) ---

// SetNMILine drives the raw /NMI input level.
func (c *CPU) SetNMILine(level bool) { c.NMIFlag = level }

// IRQ sources. The IRQ line is the OR of the individual sources, each of
// which sets and clears its own bit independently.
const (
	IRQExternal     = 1 // cartridge board (mapper)
	IRQFrameCounter = 2 // APU frame counter
	IRQDMC          = 4 // APU delta-modulation channel
)

// These track the wired-OR IRQ line by source, exactly as hardware does.

// SetIrqSource raises one interrupt source.
func (c *CPU) SetIrqSource(source byte) { c.IRQFlag |= source }

// ClearIrqSource lowers one interrupt source.
func (c *CPU) ClearIrqSource(source byte) { c.IRQFlag &^= source }

// HasIrqSource reports whether the given interrupt source is asserted.
func (c *CPU) HasIrqSource(source byte) bool { return c.IRQFlag&source != 0 }

// --- DMA entry points ---

// StartOAMDMA queues an OAM DMA copying page << 8 into $2004. The transfer
// runs on the CPU's next read cycle.
func (c *CPU) StartOAMDMA(page byte) {
	c.spriteDMATransfer = true
	c.spriteDMAOffset = page
	c.needHalt = true
}

// StartDmcTransfer queues a DMC DMA fetch.
func (c *CPU) StartDmcTransfer() {
	c.dmcDMARunning = true
	c.needDummyRead = true
	c.needHalt = true
}

// StopDmcTransfer cancels or aborts a queued DMC DMA.
func (c *CPU) StopDmcTransfer() {
	if c.dmcDMARunning {
		if c.needHalt {
			c.dmcDMARunning = false
			c.needDummyRead = false
			c.needHalt = false
		} else {
			c.abortDMCDma = true
		}
	}
}

// oamWrite writes a byte to $2004 during OAM DMA.
func (c *CPU) oamWrite(v byte) { c.mem.Write(0x2004, v) }

// --- Cycle machinery ---

// startCycle opens a CPU cycle: advance the master clock to the access
// point and run the machine up to it. forRead places the access at
// masterClock+5 (read) vs +7 (write) on hardware; at whole-dot resolution
// the nes ticker handles the placement, so we simply run the pre-access
// portion here and bump the cycle counter.
func (c *CPU) startCycle(forRead bool) {
	if forRead {
		c.masterClock += c.startClockCount - 1
	} else {
		c.masterClock += c.startClockCount + 1
	}
	// The cycle count is bumped before the per-cycle device clock so the audio
	// unit and controller see the new count (as hardware does: the PPU is run
	// to the master clock first, then CycleCount++ and the CPU-clock of the
	// other chips). The PPU, run inside clockStart, keys off the master clock,
	// not this count, so the ordering does not disturb it.
	c.Cycles++
	if c.clockStart != nil {
		c.clockStart()
	}
}

// endCycle closes a CPU cycle: run the machine's remaining portion, then
// poll the interrupt lines in φ2. The NMI edge detector
// and IRQ pipeline shift one cycle here.
func (c *CPU) endCycle(forRead bool) {
	if forRead {
		c.masterClock += c.endClockCount + 1
	} else {
		c.masterClock += c.endClockCount - 1
	}
	if c.clockEnd != nil {
		c.clockEnd()
	}
	if c.sample != nil {
		c.sample()
	}

	// "The internal signal goes high during φ1 of the cycle that follows
	// the one where the edge is detected, and stays high until the NMI has
	// been handled."
	c.prevNeedNMI = c.needNMI

	// "This edge detector polls the status of the NMI line during φ2 of each
	// CPU cycle and raises an internal signal if the input goes from being
	// high during one cycle to being low during the next" (active-low line,
	// modeled here as a low→high level rise of NMIFlag).
	if !c.prevNMIFlag && c.NMIFlag {
		c.needNMI = true
	}
	c.prevNMIFlag = c.NMIFlag

	// "it's really the status of the interrupt lines at the end of the
	// second-to-last cycle that matters."
	c.prevRunIRQ = c.runIRQ
	c.runIRQ = (c.IRQFlag&^c.irqMask) != 0 && c.Reg.P&FlagI == 0
}

// --- Memory access ---

func (c *CPU) read(addr uint16) byte {
	c.processPendingDMA(addr)
	c.startCycle(true)
	v := c.mem.Read(addr)
	c.endCycle(true)
	return v
}

func (c *CPU) write(addr uint16, v byte) {
	c.cpuWrite = true
	c.startCycle(false)
	c.mem.Write(addr, v)
	c.endCycle(false)
	c.cpuWrite = false
}

// writeRMWSecond performs the second (modified) write of a read-modify-write
// pair. It dispatches through the memory unit like any write, so RAM and I/O
// receive it; a board that filters consecutive writes (MMC1) drops it only
// when the write lands in its own address range. Same cycle timing as a
// normal write.
func (c *CPU) writeRMWSecond(addr uint16, v byte) {
	c.cpuWrite = true
	c.startCycle(false)
	c.mem.WriteRMW(addr, v)
	c.endCycle(false)
	c.cpuWrite = false
}

func (c *CPU) readWord(addr uint16) uint16 {
	lo := uint16(c.read(addr))
	hi := uint16(c.read(addr + 1))
	return hi<<8 | lo
}

// dummyPCRead / dummyStackRead reproduce the discarded reads hardware does.
func (c *CPU) dummyPCRead()    { c.read(c.Reg.PC) }
func (c *CPU) dummyStackRead() { c.read(0x100 + uint16(c.Reg.SP)) }

func (c *CPU) getOPCode() byte {
	v := c.read(c.Reg.PC)
	c.Reg.PC++
	return v
}

func (c *CPU) readByte() byte {
	v := c.read(c.Reg.PC)
	c.Reg.PC++
	return v
}

func (c *CPU) readPCWord() uint16 {
	lo := uint16(c.readByte())
	hi := uint16(c.readByte())
	return hi<<8 | lo
}

// --- Stack ---

func (c *CPU) push(v byte) {
	c.write(0x100+uint16(c.Reg.SP), v)
	c.Reg.SP--
}

func (c *CPU) push16(v uint16) {
	c.push(byte(v >> 8))
	c.push(byte(v))
}

func (c *CPU) pop() byte {
	c.Reg.SP++
	return c.read(0x100 + uint16(c.Reg.SP))
}

func (c *CPU) popWord() uint16 {
	lo := uint16(c.pop())
	hi := uint16(c.pop())
	return hi<<8 | lo
}

// --- Flags ---

func (c *CPU) setFlags(f byte)       { c.Reg.P |= f }
func (c *CPU) clearFlags(f byte)     { c.Reg.P &^= f }
func (c *CPU) checkFlag(f byte) bool { return c.Reg.P&f == f }

func (c *CPU) setZeroNeg(v byte) {
	if v == 0 {
		c.setFlags(FlagZ)
	} else if v&0x80 != 0 {
		c.setFlags(FlagN)
	}
}

func (c *CPU) setRegister(reg *byte, v byte) {
	c.clearFlags(FlagZ | FlagN)
	c.setZeroNeg(v)
	*reg = v
}

func (c *CPU) setA(v byte) { c.setRegister(&c.Reg.A, v) }
func (c *CPU) setX(v byte) { c.setRegister(&c.Reg.X, v) }
func (c *CPU) setY(v byte) { c.setRegister(&c.Reg.Y, v) }

func pageCrossed(a uint16, b byte) bool {
	return (a+uint16(b))&0xFF00 != a&0xFF00
}

func pageCrossedSigned(a uint16, b int8) bool {
	return (a+uint16(int16(b)))&0xFF00 != a&0xFF00
}

// --- Execution ---

// Step executes one instruction, then services a pending interrupt at the
// boundary if the second-to-last-cycle poll asked for one. Returns the
// number of CPU cycles consumed.
func (c *CPU) Step() int {
	start := c.Cycles

	opCode := c.getOPCode()
	c.instAddrMode = addrModes[opCode]
	c.operand = c.fetchOperand()
	opTable[opCode](c)

	if c.prevRunIRQ || c.prevNeedNMI {
		c.irq()
	}
	return int(c.Cycles - start)
}

// irq runs the interrupt sequence: two dummy PC reads, push PC
// and P (B clear), set I, jump through the NMI or IRQ vector. The vector is
// chosen at the last moment: a pending NMI edge (needNMI) hijacks the IRQ.
func (c *CPU) irq() {
	c.dummyPCRead()
	c.dummyPCRead()
	c.push16(c.Reg.PC)

	if c.needNMI {
		c.needNMI = false
		c.push(c.Reg.P&^FlagB | FlagU) // interrupts push with B clear
		c.setFlags(FlagI)
		c.Reg.PC = c.readWord(VecNMI)
	} else {
		c.push(c.Reg.P | FlagU)
		c.setFlags(FlagI)
		c.Reg.PC = c.readWord(VecIRQ)
	}
}
