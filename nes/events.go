package nes

import core "github.com/danielgatis/go-headless-nes/internal/nes"

// EventKind identifies a discrete machine event delivered to an EventSink.
type EventKind uint8

// The event kinds. Instruction/Frame/Scanline/SpriteZeroHit are polled at
// instruction granularity; NMI/IRQ and the two DMA kinds are reported exactly
// at their hardware moment.
const (
	// EventInstruction fires before each instruction executes (PC, Opcode).
	EventInstruction EventKind = iota
	// EventNMI fires when the CPU services a non-maskable interrupt.
	EventNMI
	// EventIRQ fires when the CPU services a maskable interrupt.
	EventIRQ
	// EventFrame fires when the PPU completes a frame (Frame).
	EventFrame
	// EventScanline fires when the raster crosses to a new scanline (Scanline).
	EventScanline
	// EventSpriteZeroHit fires when sprite-0 hit is first set (Scanline, Dot).
	EventSpriteZeroHit
	// EventOAMDMA fires when an OAM DMA starts (Page = source page).
	EventOAMDMA
	// EventDMCDMA fires when the DMC starts a sample fetch.
	EventDMCDMA
)

// Event is one machine event. Only the fields relevant to Kind are set.
type Event struct {
	Kind     EventKind
	PC       uint16 // EventInstruction
	Opcode   byte   // EventInstruction
	Frame    uint64 // EventFrame
	Scanline int    // EventScanline, EventSpriteZeroHit
	Dot      int    // EventSpriteZeroHit
	Page     byte   // EventOAMDMA (source page, high byte of the source address)
}

// EventSink receives machine events. It runs on the emulation goroutine, in
// the middle of a step, so it must not drive the Console (reading state with
// Peek/State is fine; Step, RunFrame and Poke are not).
type EventSink interface {
	OnEvent(Event)
}

// SetEventSink routes machine events to s. With no kinds it subscribes to
// every event; naming kinds subscribes to only those, so an unwanted event
// (the per-instruction firehose in particular) costs nothing. A nil sink
// removes the subscription.
func (c *Console) SetEventSink(s EventSink, kinds ...EventKind) {
	c.eventSink = s
	if s == nil {
		c.core.SetEventHooks(core.EventHooks{})
		return
	}
	want := func(k EventKind) bool {
		if len(kinds) == 0 {
			return true
		}
		for _, x := range kinds {
			if x == k {
				return true
			}
		}
		return false
	}
	var h core.EventHooks
	if want(EventInstruction) {
		h.OnInstruction = func(pc uint16, op byte) {
			s.OnEvent(Event{Kind: EventInstruction, PC: pc, Opcode: op})
		}
	}
	if want(EventNMI) {
		h.OnNMI = func() { s.OnEvent(Event{Kind: EventNMI}) }
	}
	if want(EventIRQ) {
		h.OnIRQ = func() { s.OnEvent(Event{Kind: EventIRQ}) }
	}
	if want(EventFrame) {
		h.OnFrame = func(f uint64) { s.OnEvent(Event{Kind: EventFrame, Frame: f}) }
	}
	if want(EventScanline) {
		h.OnScanline = func(sl int) { s.OnEvent(Event{Kind: EventScanline, Scanline: sl}) }
	}
	if want(EventSpriteZeroHit) {
		h.OnSpriteZeroHit = func(sl, dot int) {
			s.OnEvent(Event{Kind: EventSpriteZeroHit, Scanline: sl, Dot: dot})
		}
	}
	if want(EventOAMDMA) {
		h.OnOAMDMA = func(page byte) { s.OnEvent(Event{Kind: EventOAMDMA, Page: page}) }
	}
	if want(EventDMCDMA) {
		h.OnDMCDMA = func() { s.OnEvent(Event{Kind: EventDMCDMA}) }
	}
	c.core.SetEventHooks(h)
}
