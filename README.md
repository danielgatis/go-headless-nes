# go-headless-nes

[![License MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/danielgatis/go-headless-nes/main/LICENSE)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/danielgatis/go-headless-nes)
[![Release](https://img.shields.io/github/release/danielgatis/go-headless-nes.svg?style=flat-square)](https://github.com/danielgatis/go-headless-nes/releases/latest)

<p align="center">
  <img src="examples/nes/nes.png" alt="The example frontend running Super Mario Bros." width="600">
</p>

A headless NES emulator core in Go. Zero dependencies, deterministic,
cycle-accurate. You embed it as a Go library and drive it in-process. The
UI, rewind and tooling are yours to build on top.

There is no window, no audio device, no scripting engine. Just the core
and its primitives.

## Features

- **Frame and instruction stepping**: run a frame, single-step an
  instruction, reset.
- **Video and audio every frame**: a 256x240 buffer of NES color indices
  and a block of `float32` samples, plus ready-made RGBA and PCM
  conversion for frontends.
- **Memory access**: peek (no side effects), poke, block reads.
- **Save states**: save and restore the whole console as a blob. That's
  all you need to build rewind and save-slots.
- **Debugger**: breakpoints, watchpoints, disassembly, nestest-format
  trace.
- **Live patching**: edit RAM, ROM and mapper registers while the game
  runs.
- **Regions**: NTSC, PAL and Dendy, auto-detected from the header and a
  built-in cartridge database (which corrects dumps that misreport their
  region), and overridable at runtime.

Window, audio output, key mapping, rewind and scripting are left out on
purpose. They're policy, and they all fall out of the primitives above.

## Installation

```bash
go get github.com/danielgatis/go-headless-nes
```

## Quick Start

As a library:

```go
import "github.com/danielgatis/go-headless-nes/nes"

console, _ := nes.NewConsole(rom)
console.RunFrame()
video := console.Video()   // 256*240 NES color indices
audio := console.Audio()   // float32 samples
```

Each `Console` is one independent instance, so N emulators are just N
values. A `Console` is not safe for concurrent use; drive each from a
single goroutine. A full runnable client lives in
[docs/CLIENT.md](docs/CLIENT.md).

To run the tests:

```bash
git submodule update --init   # fetch the test ROMs
go test ./...                 # everything runs, no external deps
```

## Example

A complete desktop frontend on [Ebitengine](https://ebitengine.org),
with video, audio and keyboard, lives in
[examples/nes](examples/nes/main.go). It is its own Go module, so the
Ebitengine dependency never touches the core's `go.mod`:

```bash
cd examples/nes && go run . game.nes
```

## Docs

- [Go client](docs/CLIENT.md): a minimal consumer, end to end.
- [Architecture](docs/ARCHITECTURE.md): layers, timing model,
  determinism.
- [Mappers](docs/MAPPERS.md): the full supported list.

### License

Copyright (c) 2026-present [Daniel Gatis](https://github.com/danielgatis)

Licensed under [MIT License](./LICENSE)

### Buy me a coffee

Liked some of my work? Buy me a coffee (or more likely a beer)

<a href="https://www.buymeacoffee.com/danielgatis" target="_blank"><img src="https://bmc-cdn.nyc3.digitaloceanspaces.com/BMC-button-images/custom_images/orange_img.png" alt="Buy Me A Coffee" style="height: auto !important;width: auto !important;"></a>
