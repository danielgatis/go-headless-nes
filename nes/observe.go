package nes

import "github.com/danielgatis/go-headless-nes/internal/bus"

// Observer taps every CPU-bus access the machine makes while it runs:
// instruction fetches, operand reads, writes and the reads and writes a
// DMA performs. It is the primitive an out-of-process-free debugger builds
// read and write breakpoints, access heatmaps and event views on, without
// the core knowing anything about them.
//
// The callbacks fire from inside Step and RunFrame, on the same goroutine,
// after any MemFilter has been applied, so value is the byte that actually
// crossed the bus. An Observer must not call back into the Console (Peek is
// fine, Poke and Step are not): it runs in the middle of a bus cycle.
//
// Install one with SetObserver; SetObserver(nil) removes it and restores
// the zero-overhead path.
type Observer interface {
	OnRead(addr uint16, value byte)
	OnWrite(addr uint16, value byte)
}

// FilterAction is a MemFilter's verdict on one access.
type FilterAction uint8

// The filter verdicts. Block is meaningless on a read (you cannot stop the
// CPU from reading) and is treated as Pass there.
const (
	// Pass leaves the access untouched.
	Pass FilterAction = iota
	// Replace substitutes the returned value.
	Replace
	// Block drops a write (the store never happens); Pass on a read.
	Block
)

// MemFilter intercepts bus accesses and can change what happens. On a read
// it substitutes the value the CPU sees, which is exactly how a Game Genie
// code works. On a write it can replace the stored value or block the
// store entirely, which is how a value lock or trainer freezes a byte.
// Return Pass to leave the access alone.
//
// A MemFilter runs before the Observer and must not touch the bus. Install
// one with SetMemFilter; SetMemFilter(nil) removes it.
type MemFilter interface {
	FilterRead(addr uint16, value byte) (byte, FilterAction)
	FilterWrite(addr uint16, value byte) (byte, FilterAction)
}

// SetObserver installs o as the bus observer, or removes the current one
// when o is nil.
func (c *Console) SetObserver(o Observer) {
	c.observer = o
	c.wireHooks()
}

// SetMemFilter installs f as the bus filter, or removes the current one
// when f is nil.
func (c *Console) SetMemFilter(f MemFilter) {
	c.filter = f
	c.wireHooks()
}

// wireHooks rebuilds the core's bus taps from the current observer and
// filter. A nil observer or filter leaves its callbacks nil, so the core's
// hot path stays a single nil check per access.
func (c *Console) wireHooks() {
	var h bus.Hooks
	if o := c.observer; o != nil {
		h.OnRead = o.OnRead
		h.OnWrite = o.OnWrite
	}
	if f := c.filter; f != nil {
		h.FilterRead = func(addr uint16, value byte) byte {
			if v, act := f.FilterRead(addr, value); act == Replace {
				return v
			}
			return value
		}
		h.FilterWrite = func(addr uint16, value byte) (byte, bool) {
			switch v, act := f.FilterWrite(addr, value); act {
			case Replace:
				return v, true
			case Block:
				return value, false
			default:
				return value, true
			}
		}
	}
	c.core.SetBusHooks(h)
}
