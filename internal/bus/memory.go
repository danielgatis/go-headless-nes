// Package bus is the CPU-side Memory fabric: a 64 KiB table of read and
// write handlers plus the console's work RAM and the two floating-bus
// latches. Devices register the addresses they serve; reads and writes
// dispatch through the table and keep open bus up to date.
package bus

// Type selects which open-bus latch a read drives. The audio/IO chip has
// two: the external bus (cartridge/RAM/picture-unit side) and the internal
// bus (the CPU's own $4015/$4016/$4017 read path). Most reads drive both; a
// $4015 read drives only the internal one, and the DMA internal-register
// glitch reads one or the other explicitly.
type Type int

// Which open-bus latch an address participates in.
const (
	Internal Type = iota
	External
	Both
)

// Op is which side of the bus a handler serves an address on.
type Op int

// The bus sides a handler can claim an address for.
const (
	OpRead Op = iota
	OpWrite
	OpAny
)

// Handler is a device mapped into the CPU address space. A device lists
// the addresses it serves via ranges() and answers reads/writes there.
type Handler interface {
	// Ranges reports the read/write addresses this handler claims.
	Ranges() *Ranges
	// ReadReg returns the byte at addr (with side effects).
	ReadReg(addr uint16) byte
	// WriteReg stores value at addr (with side effects).
	WriteReg(addr uint16, value byte)
	// PeekReg returns the byte at addr without side effects, for debuggers.
	PeekReg(addr uint16) byte
}

// Ranges collects the addresses a handler serves. A handler fills it in
// its ranges(); the Memory unit reads it to build the dispatch table.
type Ranges struct {
	readAddrs     []uint16
	writeAddrs    []uint16
	allowOverride bool
}

// NewRanges returns an empty address-range set for a handler to populate.
func NewRanges() *Ranges { return &Ranges{} }

// Override lets this handler's addresses replace ones already claimed (the
// work-RAM handler uses it to blanket $0000-$1FFF).
func (r *Ranges) Override() *Ranges { r.allowOverride = true; return r }

// Add claims [start, end] (end defaults to start when 0) for the operation.
func (r *Ranges) Add(op Op, start, end uint16) {
	if end == 0 {
		end = start
	}
	if op == OpRead || op == OpAny {
		for i := int(start); i <= int(end); i++ {
			r.readAddrs = append(r.readAddrs, uint16(i))
		}
	}
	if op == OpWrite || op == OpAny {
		for i := int(start); i <= int(end); i++ {
			r.writeAddrs = append(r.writeAddrs, uint16(i))
		}
	}
}

// AddOne claims a single address for the given operation.
func (r *Ranges) AddOne(op Op, addr uint16) { r.Add(op, addr, addr) }

// openBusUnit holds the two floating-bus latches. A read of an unmapped
// address returns the external latch (the value last driven onto the pins);
// it is also the default handler for every otherwise-unmapped address.
type openBusUnit struct {
	external byte
	internal byte
}

// setOpenBus records value on the selected latch(es). A $4015 read always
// drives only the internal latch, regardless of bt (forceInternal).
func (o *openBusUnit) setOpenBus(bt Type, value byte, forceInternal bool) {
	if forceInternal {
		o.internal = value
		return
	}
	switch bt {
	case Internal:
		o.internal = value
	case External:
		o.external = value
	default: // Both
		o.internal = value
		o.external = value
	}
}

func (o *openBusUnit) Ranges() *Ranges           { return NewRanges() }
func (o *openBusUnit) ReadReg(_ uint16) byte     { return o.external }
func (o *openBusUnit) PeekReg(addr uint16) byte  { return byte(addr >> 8) }
func (o *openBusUnit) WriteReg(_ uint16, _ byte) {}

// RMWWriter is implemented by handlers that recognize the second write
// of a read-modify-write pair (MMC1-style boards ignore the second of
// two writes on consecutive cycles).
type RMWWriter interface {
	WriteRMWSecond(addr uint16, v byte)
}

const cpuMemorySize = 0x10000

// workRAM is the console's 2 KiB work RAM, mirrored through $1FFF.
type workRAM struct {
	ram [0x800]byte
}

func (m *workRAM) Ranges() *Ranges {
	r := NewRanges().Override()
	r.Add(OpAny, 0x0000, 0x1FFF)
	return r
}
func (m *workRAM) ReadReg(addr uint16) byte     { return m.ram[addr&0x7FF] }
func (m *workRAM) PeekReg(addr uint16) byte     { return m.ram[addr&0x7FF] }
func (m *workRAM) WriteReg(addr uint16, v byte) { m.ram[addr&0x7FF] = v }

// Hooks are optional debug taps on the CPU bus. Every field is nil by
// default, so a non-debug console pays one nil check per access and
// nothing else; the byte-for-byte behaviour is unchanged when no hook is
// installed.
//
// FilterRead substitutes the value the CPU sees on a read (a Game Genie
// swaps a byte this way). FilterWrite returns the value to actually store
// and whether to perform the write at all (allow=false blocks it, for a
// value lock). OnRead and OnWrite observe the final access after any
// filter, and must not themselves touch the bus (no re-entrancy).
type Hooks struct {
	OnRead      func(addr uint16, value byte)
	OnWrite     func(addr uint16, value byte)
	FilterRead  func(addr uint16, value byte) byte
	FilterWrite func(addr uint16, value byte) (out byte, allow bool)
}

// Memory is the CPU's address space: a 64 KiB table of read and write
// handlers, plus the open-bus latches. Devices register their address
// ranges; a read or write dispatches to the handler at that address and
// updates open bus. This is the whole Memory fabric the CPU sees.
type Memory struct {
	openBus       openBusUnit
	work          workRAM
	readHandlers  [cpuMemorySize]Handler
	writeHandlers [cpuMemorySize]Handler
	hooks         Hooks
}

// SetHooks installs (or with the zero value clears) the debug bus taps.
func (m *Memory) SetHooks(h Hooks) { m.hooks = h }

// New builds the address space with every address defaulting to open
// bus, then maps the work RAM over $0000-$1FFF.
func New() *Memory {
	m := &Memory{}
	for i := 0; i < cpuMemorySize; i++ {
		m.readHandlers[i] = &m.openBus
		m.writeHandlers[i] = &m.openBus
	}
	m.Register(&m.work)
	return m
}

// Register maps a device onto the addresses it claims. An address that is
// already claimed by another handler cannot be silently overridden unless
// the incoming handler set Override().
func (m *Memory) Register(h Handler) {
	r := h.Ranges()
	for _, addr := range r.readAddrs {
		if !r.allowOverride && m.readHandlers[addr] != &m.openBus && m.readHandlers[addr] != h {
			panic("Memory: read handler override not allowed")
		}
		m.readHandlers[addr] = h
	}
	for _, addr := range r.writeAddrs {
		if !r.allowOverride && m.writeHandlers[addr] != &m.openBus && m.writeHandlers[addr] != h {
			panic("Memory: write handler override not allowed")
		}
		m.writeHandlers[addr] = h
	}
}

// Read returns the byte at addr on both open-bus latches (a normal CPU read).
func (m *Memory) Read(addr uint16) byte { return m.ReadBus(addr, Both) }

// ReadBus returns the byte at addr, driving the selected open-bus latch(es).
// A $4015 read forces the internal latch only.
func (m *Memory) ReadBus(addr uint16, bt Type) byte {
	v := m.readHandlers[addr].ReadReg(addr)
	if m.hooks.FilterRead != nil {
		v = m.hooks.FilterRead(addr, v)
	}
	m.openBus.setOpenBus(bt, v, addr == 0x4015)
	if m.hooks.OnRead != nil {
		m.hooks.OnRead(addr, v)
	}
	return v
}

// Write stores value at addr and drives both open-bus latches.
func (m *Memory) Write(addr uint16, value byte) {
	allow := true
	if m.hooks.FilterWrite != nil {
		value, allow = m.hooks.FilterWrite(addr, value)
	}
	if allow {
		m.writeHandlers[addr].WriteReg(addr, value)
	}
	m.openBus.setOpenBus(Both, value, false)
	if m.hooks.OnWrite != nil {
		m.hooks.OnWrite(addr, value)
	}
}

// WriteRMW performs the second (modified) write of a read-modify-write pair.
// It behaves like write except that when the target handler itself filters
// consecutive writes (the cartridge board's serial port), the handler decides
// whether to drop the write, matching boards that ignore the second of two
// back-to-back writes. RAM and I/O just take the write.
func (m *Memory) WriteRMW(addr uint16, value byte) {
	allow := true
	if m.hooks.FilterWrite != nil {
		value, allow = m.hooks.FilterWrite(addr, value)
	}
	if allow {
		if h, ok := m.writeHandlers[addr].(RMWWriter); ok {
			h.WriteRMWSecond(addr, value)
		} else {
			m.writeHandlers[addr].WriteReg(addr, value)
		}
	}
	m.openBus.setOpenBus(Both, value, false)
	if m.hooks.OnWrite != nil {
		m.hooks.OnWrite(addr, value)
	}
}

// Peek reads without side effects, for debuggers and tracing.
func (m *Memory) Peek(addr uint16) byte { return m.readHandlers[addr].PeekReg(addr) }

// OpenBus returns the external latch (for the DMA glitch).
func (m *Memory) OpenBus() byte { return m.openBus.external }

// InternalOpenBus returns the internal latch (for the DMA glitch).
func (m *Memory) InternalOpenBus() byte { return m.openBus.internal }

// SetOpenBus lets the DMA unit drive a latch directly.
func (m *Memory) SetOpenBus(bt Type, value byte) { m.openBus.setOpenBus(bt, value, false) }

// State is the fabric's copyable state for snapshots.
type State struct {
	RAM             [0x800]byte
	ExternalOpenBus byte
	InternalOpenBus byte
}

// State copies the work RAM and open-bus latches out.
func (m *Memory) State() State {
	return State{RAM: m.work.ram, ExternalOpenBus: m.openBus.external, InternalOpenBus: m.openBus.internal}
}

// SetState restores the work RAM and open-bus latches.
func (m *Memory) SetState(s State) {
	m.work.ram = s.RAM
	m.openBus.external = s.ExternalOpenBus
	m.openBus.internal = s.InternalOpenBus
}
