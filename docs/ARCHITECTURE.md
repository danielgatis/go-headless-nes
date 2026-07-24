# Architecture

```
Hardware simulation   internal/{cpu,ppu,apu,bus,cartridge,mapper,controller}
        |
Console               internal/nes         one nes.NES value: the whole machine
        |
Debug layer           internal/debugger    breakpoints/watchpoints/disasm/trace
        |
Public API            . (package nes)      Console (in-process library) +
        |                                  protocol codec & server
Binary                cmd/nesd  binds the server to os.Stdin/os.Stdout
```

Everything emulated lives in one internal `nes.NES` value, with no globals
and no hidden state. The public root package wraps it two ways. `Console`
exposes it directly as a Go library, and each `Console` is one independent
emulator instance. The protocol `Server` maps command frames onto a
`Console`, reading and writing over plain `io.Reader` and `io.Writer`.
That's why the same server binds to stdio (or a TCP socket) in
`cmd/nesd` today and could bind to JS callbacks for a
WebAssembly build without touching the core or the codec.

## Timing

Everything advances from inside the CPU's bus cycles on one shared master
clock (21.477272 MHz on NTSC, 26.601712 MHz on PAL and Dendy). Each read
or write runs the PPU to the exact sub-cycle position of the access (a
read lands at +5 master ticks and a write at +7 on NTSC), then clocks the
board and the APU. The PPU renders per dot, background fetches take their
real two-cycle ALE/data shape on a modeled address bus, and DMA is
cycle-exact, including OAM and DMC and the collisions and aborts between
them.

The TV system is one small set of parameters resolved once when the
console powers on (or when the region is overridden): the CPU and PPU
master-clock dividers, the number of scanlines, the vblank/NMI line, the
odd-frame dot skip, and the APU's rate tables. Each unit copies the
scalars it needs into fields, so no per-cycle code branches on the region;
NTSC stays byte-for-byte what it was.

None of that is a claim on faith. AccuracyCoin passes 141/141, nestest
matches the canonical log line by line, and blargg's CPU, PPU, APU, OAM and
MMC3 suites pass, all held in place by the test suite as regression floors.

## Determinism

State is built for determinism. Every chip keeps its mutable state in a
plain, value-copyable `State` struct, and `nes.Snapshot` is the whole
console as a single value, so in-process save and restore are just struct
assignments with no allocation (a test enforces that).

For the wire, each `State` has a hand-written field-by-field binary codec
in `internal/serial`. A round-trip test runs the console 200k steps and
compares every field, which catches a codec that ever drops one. The usual
reflection encoders (gob, `binary.Write`) won't work here, because the
`State` structs carry load-bearing unexported fields that those encoders
quietly skip.
