# Go client

There are two ways to drive the core, both from the root package
`github.com/danielgatis/go-headless-nes` (package `nes`):

- As a library: `nes.Console` embeds the emulator in your process.
  No subprocess, no protocol.
- Over the protocol: launch the `cmd/nesd` binary with
  `os/exec` and wire an `Encoder` to its stdin and a `Decoder` to its
  stdout, or start it with `--listen` and connect over TCP. Use this
  when you're crossing a process or language boundary.

## As a library

```go
package main

import (
	"os"

	nes "github.com/danielgatis/go-headless-nes"
)

func main() {
	rom, _ := os.ReadFile(os.Args[1])
	console, err := nes.NewConsole(rom)
	if err != nil {
		panic(err)
	}

	console.SetButtons(0, nes.ButtonStart)
	for i := 0; i < 60; i++ {
		console.RunFrame()
		video := console.Video() // 256*240 NES color indices (0-63)
		audio := console.Audio() // float32 samples since the last call
		_, _ = video, audio
	}

	state := console.SaveState() // whole console as a blob
	_ = console.LoadState(state) // ...restore it later, deterministically
}
```

The full surface mirrors the protocol one-to-one: `Step`, `Reset`,
`Peek`/`Poke`/`ReadMem`, `SaveState`/`LoadState`, `State`, breakpoints and
watchpoints, `Disasm`, `SetTrace`, `PatchPRG`/`PatchCHR`/`ReadPRG`/`ReadCHR`
and mapper state.

Each `Console` is a self-contained instance, so if you want N emulators
you just create N consoles. A `Console` is not safe for concurrent use;
drive each one from its own goroutine.

### Presentation helpers

Two conversions show up in every frontend, so the package ships them
ready to use.

`Console.VideoRGBA` renders the framebuffer as 8-bit RGBA through
`Palette` (the Nestopia NTSC colors), ready for a texture upload. Pass
the previous frame's slice back in and it allocates only once:

```go
pixels = console.VideoRGBA(pixels)
```

`AudioStream` sits between the console and a real-time audio API such as
Ebitengine, oto or SDL. The emulator side calls `Push` with each frame's
samples; the device side reads 16-bit little-endian stereo PCM at
`SampleRate` through the standard `io.Reader` interface. It is safe for
concurrent use, produces silence instead of blocking when the emulator
pauses, and drops the oldest samples past its latency cap (the core
emits slightly more than one display frame of audio per emulated frame,
so an unbounded buffer would accumulate latency forever):

```go
stream := nes.NewAudioStream(0) // 0 means the default 250ms cap
// each frame: stream.Push(console.Audio())
// hand stream to your audio library as an io.Reader
```

Neither helper is mandatory. `Video` still hands you raw color indices
and `Audio` raw float32 samples if you want your own palette or mixing.

For a complete frontend built this way, see
[examples/nes](../examples/nes/main.go): a desktop player on Ebitengine,
kept in a separate Go module so its dependencies stay out of the core.

## Over the protocol

```go
package main

import (
	"io"
	"os"
	"os/exec"

	nes "github.com/danielgatis/go-headless-nes"
)

func main() {
	rom, _ := os.ReadFile(os.Args[1])

	cmd := exec.Command("nesd")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		panic(err)
	}

	enc := nes.NewEncoder(stdin)
	dec := nes.NewDecoder(stdout)

	dec.ReadHandshake()                     // core sends its version
	enc.WriteHandshake(nes.ProtocolVersion) // we send ours

	enc.Write(nes.OpLoadROM, rom)
	for i := 0; i < 60; i++ {
		enc.Write(nes.OpRunFrame, nil) // each yields a Video + Audio event
	}
	stdin.Close() // no more commands; the core exits at EOF

	for {
		f, err := dec.Read()
		if err == io.EOF {
			break
		}
		switch f.Op {
		case nes.OpVideo: // f.Payload: 256*240 NES color indices (0-63)
		case nes.OpAudio: // f.Payload: little-endian float32 samples
		}
	}
	cmd.Wait()
}
```

One process is one instance. For N emulators, launch N processes, each
with its own encoder/decoder pair.

### Over TCP

`nesd --listen 127.0.0.1:4444` serves the same protocol on a
TCP socket instead of stdin/stdout. Each connection gets its own console,
so a single core process can host N independent instances, and clients in
any language (or on another machine) can connect. The client code is the
same as above, with the pipes swapped for a `net.Conn`:

```go
conn, _ := net.Dial("tcp", "127.0.0.1:4444")
enc := nes.NewEncoder(conn)
dec := nes.NewDecoder(conn)
// handshake and drive exactly as with pipes
```

With `--listen 127.0.0.1:0` the OS picks a free port. The core announces
the bound address on stderr (`listening on 127.0.0.1:53808`), so a parent
process can scrape it from there.

The full opcode table, payload layouts, the structured state block and the
error semantics are in [PROTOCOL.md](PROTOCOL.md).

## Testing

`go test ./...` runs everything, with no external dependencies:

- nestest compares every logged instruction exactly (registers, flags,
  cycles, PPU dot clock) through the debugger's trace.
- The blargg CPU, PPU, APU, OAM and MMC3 suites and AccuracyCoin (141/141)
  all run, and a smoke pass boots and runs every bundled test ROM.
- The snapshot codec gets a 200k-step round-trip that fails if any field
  goes missing from the wire form.
- The protocol has codec round-trips and framing edge cases, plus
  end-to-end server tests: video and audio emission, peek and poke, step
  and state, breakpoint stops, disassembly, PRG patch and read-back,
  mapper round-trip, and snapshot determinism across the wire.
- Per-package unit tests cover CPU quirks, PPU timing and rendering, APU
  channels, and every mapper.

The test ROMs live under `test/assets/` as git submodules of freely
redistributable suites, so fetch them before running the tests:

```
git submodule update --init
```
