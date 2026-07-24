# go-headless-nes binary protocol

You drive the `nesd` core over a small binary protocol. The
native binary speaks it on stdin for commands in and stdout for events
out, or on TCP with `--listen addr`, where each connection is served by
its own console, so N clients are N independent emulator instances.
The same `nes.Server` runs over any `io.Reader` and `io.Writer`, so
other transports like WebAssembly JS callbacks reuse it unchanged. And if
you're embedding the emulator in a Go program, you don't need the protocol
at all: use `nes.Console` directly, see [CLIENT.md](CLIENT.md).

Multi-byte integers are big-endian unless noted. The one exception is
audio samples, which are little-endian float32 to match typical audio
APIs.

The current protocol version is 1 (`nes.ProtocolVersion`).

---

## Framing

Every message, in both directions, is one frame:

```
+--------+------------------------+------------------+
| 1 byte | 4 bytes                | length bytes     |
| opcode | payload length (u32 BE)| payload          |
+--------+------------------------+------------------+
```

A payload length above `MaxPayload` (1 MiB) is rejected as a transport
error, which guards against a corrupt length header forcing a huge
allocation. End-of-stream at a frame boundary is a clean shutdown rather
than an error.

## Handshake

Immediately on connect the core sends a `Handshake` frame whose 1-byte
payload is its protocol version. The consumer replies with its own
`Handshake` + version. If the versions differ, the core emits an `Error`
event and closes.

```
core     -> Handshake [0x01]
consumer -> Handshake [0x01]
```

## Opcode ranges

Opcodes are partitioned so new ones slot into reserved space without
renumbering existing ones:

| Range        | Meaning                       |
| ------------ | ----------------------------- |
| `0x00`       | Handshake                     |
| `0x01`-`0x3F`| Core commands (consumer→core) |
| `0x40`-`0x7F`| Extension commands (reserved) |
| `0x81`-`0xBF`| Core events (core→consumer)   |
| `0xC0`-`0xFE`| Extension events (reserved)   |
| `0xFF`       | Error                         |

An opcode outside the known set produces an `Error` event and does not
break the stream, so the consumer and core can differ in features without
desyncing.

---

## Commands (consumer → core)

### Execution / memory / state

| Op          | Code | Payload                          | Emits           |
| ----------- | ---- | -------------------------------- | --------------- |
| `LoadROM`   | 0x01 | raw iNES / NES 2.0 bytes         | none |
| `RunFrame`  | 0x02 | *(empty)*                        | `Video`,`Audio` (+`Stop`) |
| `Step`      | 0x03 | *(empty)*                        | `State` (+`Stop`) |
| `SetInput`  | 0x04 | 2 bytes: pad0, pad1 (button bits)| none |
| `Reset`     | 0x05 | *(empty)*                        | none |
| `SaveState` | 0x06 | *(empty)*                        | `Snapshot`      |
| `LoadState` | 0x07 | snapshot bytes (from `Snapshot`) | none |
| `Peek`      | 0x08 | 2 bytes: addr                    | `Value`         |
| `Poke`      | 0x09 | 3 bytes: addr(2) + value(1)      | none |
| `GetState`  | 0x0A | *(empty)*                        | `State`         |
| `SetRegion` | 0x0B | 1 byte: 0 auto, 1 NTSC, 2 PAL, 3 Dendy | none |

`RunFrame` applies the current input to both controllers, runs to the end
of the video frame (or until a breakpoint or watchpoint halts it), and
emits a `Video` then an `Audio` event. If it halted early it also emits
`Stop`. `Step` runs exactly one CPU instruction and emits the `State`
block.

`LoadROM` auto-detects the TV system from the header (iNES byte 9 or NES
2.0 byte 12), corrected against a built-in cartridge database keyed by the
ROM's CRC, since many dumps misreport their region. `SetRegion` overrides
it afterward: `0` re-detects from the cartridge, `1`/`2`/`3` force
NTSC/PAL/Dendy. The switch is live and does
not reset the console: the master-clock dividers, PPU frame geometry and
APU rate tables take effect on the next cycle, so a PAL game whose header
wrongly says NTSC (and so runs ~20% fast) is corrected mid-play with one
command. Region is a fixed property of the machine, so it is not carried
in a snapshot. The opcode is additive and does not change the protocol
version; a client that never sends it keeps the auto-detected region.

Controller button bits (`SetInput` payload, one byte per pad):

```
bit 0 A   bit 1 B   bit 2 Select   bit 3 Start
bit 4 Up  bit 5 Down bit 6 Left    bit 7 Right
```

### Debug (drive the built-in debugger)

| Op         | Code | Payload                        | Emits        |
| ---------- | ---- | ------------------------------ | ------------ |
| `AddBreak` | 0x10 | 2 bytes: addr                  | none |
| `DelBreak` | 0x11 | 2 bytes: addr                  | none |
| `AddWatch` | 0x12 | 2 bytes: addr                  | none |
| `DelWatch` | 0x13 | 2 bytes: addr                  | none |
| `Disasm`   | 0x14 | 3 bytes: addr(2) + count(1)    | `DisasmText` |
| `ReadMem`  | 0x15 | 4 bytes: addr(2) + len(2)      | `MemBlock`   |
| `SetTrace` | 0x16 | 1 byte: 0=off, 1=on            | `TraceLine`* |

A breakpoint halts `RunFrame` when PC reaches `addr`. A watchpoint halts
when the byte at `addr` changes value. With tracing on, every executed
instruction produces a `TraceLine` event (nestest format).

### Live patch (romhack / trainer)

| Op            | Code | Payload                         | Emits        |
| ------------- | ---- | ------------------------------- | ------------ |
| `PatchPRG`    | 0x20 | offset(4) + bytes               | none |
| `PatchCHR`    | 0x21 | offset(4) + bytes               | none |
| `ReadPRG`     | 0x22 | offset(4) + len(4)              | `MemBlock`   |
| `ReadCHR`     | 0x23 | offset(4) + len(4)              | `MemBlock`   |
| `MapperWrite` | 0x24 | 3 bytes: addr(2) + value(1)     | none |
| `GetMapper`   | 0x25 | *(empty)*                       | `MapperState`|
| `SetMapper`   | 0x26 | mapper-state bytes              | none |

`PatchPRG` and `PatchCHR` write straight into the cartridge's PRG or
CHR-ROM buffer, bypassing the mapper, so it's a real code or graphics
patch. An offset plus length past the end of the buffer, or a patch to CHR
on a CHR-RAM board, comes back as an `Error`. `MapperWrite` drives the
mapper's PRG write path exactly as a running game would, doing banking and
register writes. `GetMapper` and `SetMapper` snapshot and restore the
mapper's raw register file plus PRG and CHR RAM.

`Peek`, `Poke` and `ReadMem` address the CPU bus (RAM, registers, mapped
ROM). `PatchPRG`, `ReadPRG`, `PatchCHR` and `ReadCHR` address the
cartridge ROM buffers by absolute offset.

---

## Events (core → consumer)

| Op            | Code | Payload                                       |
| ------------- | ---- | --------------------------------------------- |
| `Video`       | 0x81 | 61440 bytes: 256×240 NES color indices (0-63) |
| `Audio`       | 0x82 | N × 4 bytes: little-endian float32, mono      |
| `Snapshot`    | 0x83 | console snapshot bytes (`SaveState` reply)    |
| `Value`       | 0x84 | 1 byte: `Peek` result                         |
| `State`       | 0x85 | structured state block (see below)            |
| `Stop`        | 0x86 | 5 bytes: reason(1) + addr(2) + old(1) + new(1)|
| `DisasmText`  | 0x87 | UTF-8 text, one instruction per line          |
| `MemBlock`    | 0x88 | raw bytes (`ReadMem`/`ReadPRG`/`ReadCHR`)     |
| `TraceLine`   | 0x89 | UTF-8 nestest-format line (no trailing `\n`)  |
| `MapperState` | 0x8A | mapper-state bytes (`GetMapper` reply)        |
| `Error`       | 0xFF | UTF-8 error message                           |

`Video` is row-major NES color indices, and the consumer maps them to
RGBA with its own palette. `Audio` samples are mono, unipolar `[0,1]`, at
44100 Hz.

`Stop` reason values: `1` = breakpoint (`addr` = PC), `2` = watchpoint
(`addr` = watched address, `old`/`new` = the value change).

### The `State` block (`0x85`)

Emitted by `GetState` and `Step`. It's a version byte followed by
fixed-order, big-endian fields. Future versions append fields at the
end, so a consumer reads what it knows by length and ignores any tail.

```
offset  size  field
   0      1    state version (currently 1)
   1      1    A         (CPU register)
   2      1    X
   3      1    Y
   4      1    SP
   5      1    P         (status flags)
   6      2    PC
   8      8    CPU cycles since power-on
  16      2    CPU stall
  18      2    PPU scanline  (int16, -1..260)
  20      4    PPU dot/cycle (int32, 0..340)
  24      8    PPU frame counter
  32      8    master clock  (21.477 MHz ticks since power-on)
        ----
  total  40 bytes (version 1)
```

---

## Error handling

There are two tiers.

- Command errors (no ROM loaded, a malformed ROM, a wrong payload size,
  an out-of-range patch, an unknown opcode) produce an `Error` event and
  the loop keeps going. The framing stays intact, so the consumer stays
  in sync.
- Transport errors (a malformed frame header, an oversize length, a
  truncated payload, a write failure) are fatal and end the session.

## Building rewind, save-slots and a debugger UI on top

These are consumer policy rather than core features, and they all reduce
to the primitives above:

- Rewind and time-machine: `SaveState` every frame into a ring buffer the
  consumer keeps, then `LoadState` an earlier snapshot to rewind.
- Numbered save-slots: snapshots the consumer stores by name or index.
- Debugger front-end: `AddBreak` and `AddWatch` plus `RunFrame` and
  `Step`, reading `Stop` and `State`. `Disasm` gives the code view, and
  `ReadMem` with `Poke` gives the memory view.
