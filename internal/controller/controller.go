// Package controller emulates the standard NES joypad: a 4021 shift
// register that latches all eight buttons while strobed and shifts one bit
// out per $4016/$4017 read, plus the control manager that maps the two
// ports into the CPU address space and reproduces the read-clock cache (a
// same-address read on the same or an adjacent cycle does not shift the
// register twice).
package controller

import "github.com/danielgatis/go-headless-nes/internal/bus"

// Button bits, in the order the shift register reports them.
const (
	A byte = 1 << iota
	B
	Select
	Start
	Up
	Down
	Left
	Right
)

// Controller is one joypad. It is a plain value — three bytes of
// state — so snapshots copy it by assignment.
type Controller struct {
	Buttons byte // current physical button state, set by the UI driver
	Strobe  bool
	Shift   byte // latched shift register
}

// SetButtons updates the physical button state from the UI driver.
func (c *Controller) SetButtons(buttons byte) {
	c.Buttons = buttons
}

// WriteStrobe handles a $4016 write. While the strobe bit is high the
// shift register continuously follows the buttons, so it latches on
// the high write and again on the 1->0 transition: buttons that change
// between the two writes are captured by the falling edge.
func (c *Controller) WriteStrobe(v byte) {
	strobe := v&1 != 0
	if strobe || c.Strobe {
		c.Shift = c.Buttons
	}
	c.Strobe = strobe
}

// Read shifts one bit out of the register. While strobed, it always
// reports the A button. After all eight bits, official controllers
// report 1.
func (c *Controller) Read() byte {
	if c.Strobe {
		return c.Buttons & 1
	}
	v := c.Shift & 1
	c.Shift = c.Shift>>1 | 0x80
	return v
}

// Peek is Read without shifting, for debuggers.
func (c *Controller) Peek() byte {
	if c.Strobe {
		return c.Buttons & 1
	}
	return c.Shift & 1
}

// Manager maps the two joypad ports into the CPU address space:
// $4016/$4017 reads and the $4016 strobe write. It reproduces the read-clock
// cache — a genuinely new read (different address, or more than one cycle
// since the last read) shifts the register; an immediate re-read of the same
// address returns the cached value without shifting, matching hardware and
// the DMA internal-register glitch.
type Manager struct {
	pads         *[2]Controller
	openBus      func() byte   // external open bus for the undriven bits
	cycleCount   func() uint64 // CPU cycle count, drives the read cache and strobe timing
	prevReadAddr uint16
	padPrevCycle [2]uint64
	padPrevValue [2]byte

	// A $4016 write does not strobe the ports immediately: the OUT pins are
	// only updated at the start of a put cycle. The write is scheduled and
	// applied by processWrites 1-2 cycles later, so a one-cycle strobe lands
	// only when it falls on a put cycle.
	writeValue   byte
	writePending byte
}

// Ranges reports the joypad register addresses in the CPU address space.
func (m *Manager) Ranges() *bus.Ranges {
	r := bus.NewRanges()
	r.AddOne(bus.OpRead, 0x4016)
	r.AddOne(bus.OpRead, 0x4017)
	r.AddOne(bus.OpWrite, 0x4016)
	return r
}

// WriteReg handles a write to $4016, scheduling the strobe.
func (m *Manager) WriteReg(_ uint16, v byte) {
	// Schedule the strobe for the start of the next put cycle (1 cycle away
	// if the write is on an odd cycle, 2 if even).
	m.writeValue = v
	if m.cycleCount != nil && m.cycleCount()&1 != 0 {
		m.writePending = 1
	} else {
		m.writePending = 2
	}
}

// HasPendingWrites reports whether a scheduled strobe is waiting.
func (m *Manager) HasPendingWrites() bool { return m.writePending > 0 }

// ProcessWrites is clocked once per CPU cycle. When the scheduled strobe's
// countdown reaches zero it drives the OUT pins (strobes both ports).
func (m *Manager) ProcessWrites() {
	if m.writePending > 0 {
		m.writePending--
		if m.writePending == 0 {
			m.pads[0].WriteStrobe(m.writeValue)
			m.pads[1].WriteStrobe(m.writeValue)
		}
	}
}

// ReadReg returns a port read. Only bits 0-2 are driven; the top three are
// open bus (for LDA $4016 that is the operand high byte the CPU just
// fetched). The read cache prevents double-shifting on adjacent same-address
// reads.
func (m *Manager) ReadReg(addr uint16) byte {
	port := int(addr - 0x4016)
	return m.openBus()&0xE0 | m.readPad(port, addr)
}

// PeekReg reads a port without side effects (bit 6 driven by convention).
func (m *Manager) PeekReg(addr uint16) byte {
	port := int(addr - 0x4016)
	return 0x40 | m.pads[port].Peek()
}

// readPad clocks the joypad's shift register only on a genuinely new read.
func (m *Manager) readPad(port int, addr uint16) byte {
	if m.cycleCount == nil {
		v := m.pads[port].Read()
		m.prevReadAddr = addr
		return v
	}
	cyc := m.cycleCount()
	v := m.padPrevValue[port]
	if m.prevReadAddr != addr || m.padPrevCycle[port] < cyc-1 {
		v = m.pads[port].Read()
	}
	m.padPrevCycle[port] = cyc
	m.padPrevValue[port] = v
	m.prevReadAddr = addr
	return v
}

// NewManager wires the two joypad ports; openBus supplies the floating
// value for the undriven bits.
func NewManager(pads *[2]Controller, openBus func() byte) *Manager {
	return &Manager{pads: pads, openBus: openBus}
}

// SetCycleCount installs the CPU cycle counter that drives the
// read-clock cache and the strobe scheduling.
func (m *Manager) SetCycleCount(f func() uint64) { m.cycleCount = f }

// ManagerState is the manager's copyable state for snapshots.
type ManagerState struct {
	PrevReadAddr uint16
	PadPrevCycle [2]uint64
	PadPrevValue [2]byte
	WriteValue   byte
	WritePending byte
}

// State copies the strobe scheduling and read-clock cache out.
func (m *Manager) State() ManagerState {
	return ManagerState{
		PrevReadAddr: m.prevReadAddr,
		PadPrevCycle: m.padPrevCycle,
		PadPrevValue: m.padPrevValue,
		WriteValue:   m.writeValue,
		WritePending: m.writePending,
	}
}

// SetState restores the strobe scheduling and read-clock cache.
func (m *Manager) SetState(s ManagerState) {
	m.prevReadAddr = s.PrevReadAddr
	m.padPrevCycle = s.PadPrevCycle
	m.padPrevValue = s.PadPrevValue
	m.writeValue = s.WriteValue
	m.writePending = s.WritePending
}
