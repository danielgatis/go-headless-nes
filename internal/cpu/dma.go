package cpu

import "github.com/danielgatis/go-headless-nes/internal/bus"

// Cycle-accurate DMA, a direct translation of the ProcessPendingDma
// and ProcessDmaRead. The DMA unit halts the CPU on
// a read cycle and steals the bus to run OAM and/or DMC transfers, with
// the RDY-halt, dummy-read, get/put-cycle alignment and the internal-
// register read glitch all reproduced. This is cycle-accurate.

// processPendingDMA runs any queued OAM/DMC DMA before a CPU read. It is a
// no-op unless a transfer has been requested (needHalt).
func (c *CPU) processPendingDMA(readAddress uint16) {
	if !c.needHalt {
		return
	}

	prevReadAddress := readAddress
	enableInternalRegReads := readAddress&0xFFE0 == 0x4000
	c.needHalt = false

	// Halt cycle: a dummy read of the CPU's current address.
	c.startCycle(true)
	c.mem.Read(readAddress)
	c.endCycle(true)

	if c.abortDMCDma {
		c.dmcDMARunning = false
		c.abortDMCDma = false
		if !c.spriteDMATransfer {
			c.needDummyRead = false
			return
		}
	}

	var spriteDMACounter uint16
	var spriteReadAddr byte
	var readValue byte

	// processCycle consumes the halt/dummy/sprite alignment cycle before a
	// real DMA read, mirroring the lambda.
	processCycle := func() {
		switch {
		case c.abortDMCDma:
			c.dmcDMARunning = false
			c.abortDMCDma = false
			c.needDummyRead = false
			c.needHalt = false
		case c.needHalt:
			c.needHalt = false
		case c.needDummyRead:
			c.needDummyRead = false
		}
		c.startCycle(true)
	}

	for c.dmcDMARunning || c.spriteDMATransfer {
		getCycle := c.Cycles&0x01 == 0
		if getCycle {
			switch {
			case c.dmcDMARunning && !c.needHalt && !c.needDummyRead:
				// DMC DMA is ready to read a byte.
				processCycle()
				readValue = c.processDMARead(c.dmcReadAddr(), &prevReadAddress, enableInternalRegReads)
				c.endCycle(true)
				c.dmcDMARunning = false
				c.abortDMCDma = false
				c.deliverDMC(readValue)
				if c.needHalt {
					c.processPendingDMA(readAddress)
				}
			case c.spriteDMATransfer:
				// DMC not ready; run a sprite DMA read.
				processCycle()
				readValue = c.processDMARead(uint16(c.spriteDMAOffset)*0x100+uint16(spriteReadAddr), &prevReadAddress, enableInternalRegReads)
				c.endCycle(true)
				spriteReadAddr++
				spriteDMACounter++
			default:
				// DMC running but not ready and no sprite DMA: dummy read.
				processCycle()
				c.mem.Read(readAddress)
				c.endCycle(true)
			}
		} else {
			if c.spriteDMATransfer && spriteDMACounter&0x01 != 0 {
				// Sprite DMA write cycle (only after a sprite read).
				processCycle()
				c.oamWrite(readValue)
				c.endCycle(true)
				spriteDMACounter++
				if spriteDMACounter == 0x200 {
					c.spriteDMATransfer = false
				}
			} else {
				// Align to a read cycle (or perform a DMC read next).
				processCycle()
				c.mem.Read(readAddress)
				c.endCycle(true)
			}
		}
	}
}

// processDMARead reproduces the 2A03 DMA read, including the internal-
// register glitch. When the CPU is halted while reading $4000-$401F, the DMA
// read can hit the CPU's internal registers ($4015/$4016/$4017) instead of,
// or alongside, the address the DMA meant to read.
func (c *CPU) processDMARead(addr uint16, prevReadAddress *uint16, enableInternalRegReads bool) byte {
	if !enableInternalRegReads {
		var val byte
		if addr >= 0x4015 && addr <= 0x401A {
			// The readable audio/IO registers can't be seen by DMA in this case.
			val = c.mem.OpenBus()
		} else {
			val = c.mem.Read(addr)
		}
		*prevReadAddress = addr
		return val
	}

	// The glitch: the CPU reads from its internal audio/input registers
	// regardless of the address the DMA unit meant to read.
	internalAddr := uint16(0x4000) | (addr & 0x1F)
	isSameAddress := internalAddr == addr

	var val byte
	switch internalAddr {
	case 0x4015:
		// $4015 reads drive only the internal bus, not the external one.
		val = c.mem.ReadBus(internalAddr, bus.Internal)
		if !isSameAddress {
			// Also trigger the external-bus read at the intended address.
			c.mem.ReadBus(addr, bus.External)
		}
	case 0x4016, 0x4017:
		val = c.mem.Read(internalAddr)
		if !isSameAddress {
			// Bus conflict: the joypad drives its open-bus bits (top three,
			// mask 0xE0) while the DMA read drives the rest. The joypad bits
			// win on those pins; the others AND together.
			const obMask = 0xE0
			externalValue := c.mem.Read(addr)
			c.mem.SetOpenBus(bus.External, (externalValue&obMask)|(val&^obMask))
			val = (externalValue & obMask) | ((val &^ obMask) & (externalValue &^ obMask))
		}
	default:
		val = c.mem.Read(addr)
	}
	*prevReadAddress = internalAddr
	return val
}

// dmcReadAddr returns the DMC sample address to fetch.
func (c *CPU) dmcReadAddr() uint16 {
	if c.dmcAddr != nil {
		return c.dmcAddr()
	}
	return 0
}

// deliverDMC hands a fetched DMC byte back to the APU.
func (c *CPU) deliverDMC(v byte) {
	if c.dmcDeliver != nil {
		c.dmcDeliver(v)
	}
}
