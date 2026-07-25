# Go client

You drive the core from package `nes`
(`github.com/danielgatis/go-headless-nes/nes`): `nes.Console` embeds the
emulator in your process, no subprocess and no wire in between.

```go
package main

import (
	"os"

	"github.com/danielgatis/go-headless-nes/nes"
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

The rest of the surface: `Step`, `Reset`, `Peek`/`Poke`/`ReadMem`,
`SaveState`/`LoadState`, `State`, breakpoints and watchpoints, `Disasm`,
`SetTrace`, `PatchPRG`/`PatchCHR`/`ReadPRG`/`ReadCHR`, mapper state, and
`SetRegion`.

`NewConsole` auto-detects the TV system from the ROM header, corrected
against a built-in cartridge database (many dumps misreport the region),
so most consumers never touch it. `SetRegion(RegionPAL)` (or `RegionNTSC`,
`RegionDendy`, or `RegionAuto` to re-detect) overrides it at runtime: the
master-clock dividers, PPU frame geometry and APU rates change on the next
cycle, live and without a reset. That is how you fix a PAL game whose
header wrongly claims NTSC, which otherwise runs about 20% too fast. The
region is a fixed property of the machine, so it does not travel in a save
state.

Your frontend must also drive frames at the region's rate: `FrameRate`
returns ~60 on NTSC and ~50 on PAL/Dendy. Pinning the loop to 60 (a fixed
60 Hz tick, say) plays PAL content ~20% fast even with the timing correct
inside the core, so switch the loop's rate whenever the region changes.
The example does this with `ebiten.SetTPS`.

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
kept in a separate Go module so its dependencies stay out of the core. The
[examples/wasm](../examples/wasm/main.go) page drives the same `Console`
API from the browser through `syscall/js`.

## Testing

`go test ./...` runs everything, with no external dependencies:

- nestest compares every logged instruction exactly (registers, flags,
  cycles, PPU dot clock) through the debugger's trace.
- The blargg CPU, PPU, APU, OAM and MMC3 suites and AccuracyCoin (141/141)
  all run, and a smoke pass boots and runs every bundled test ROM.
- The snapshot codec gets a 200k-step round-trip that fails if any field
  goes missing from the serialized form.
- Per-package unit tests cover CPU quirks, PPU timing and rendering, APU
  channels, and every mapper.

The test ROMs live under `test/assets/` as git submodules of freely
redistributable suites, so fetch them before running the tests:

```
git submodule update --init
```
