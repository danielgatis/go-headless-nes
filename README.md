# go-headless-nes

[![Go Report Card](https://goreportcard.com/badge/github.com/danielgatis/go-headless-nes?style=flat-square)](https://goreportcard.com/report/github.com/danielgatis/go-headless-nes)
[![License MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/danielgatis/go-headless-nes/main/LICENSE)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/danielgatis/go-headless-nes)
[![Release](https://img.shields.io/github/release/danielgatis/go-headless-nes.svg?style=flat-square)](https://github.com/danielgatis/go-headless-nes/releases/latest)

<p align="center">
  <img src="examples/nes/nes.png" alt="The example frontend running Super Mario Bros." width="600">
</p>

A headless NES emulator core in Go. Zero dependencies, deterministic,
cycle-accurate. Use it as a Go library, or run it as a standalone process
and drive it over a tiny binary protocol on stdin/stdout. The UI, rewind
and tooling are yours to build on top.

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
- **Binary protocol**: drive the core from any language over
  stdin/stdout or TCP.

Window, audio output, key mapping, rewind and scripting are left out on
purpose. They're policy, and they all fall out of the primitives above.

## Installation

```bash
go get github.com/danielgatis/go-headless-nes
```

## Quick Start

As a library:

```go
import nes "github.com/danielgatis/go-headless-nes"

console, _ := nes.NewConsole(rom)
console.RunFrame()
video := console.Video()   // 256*240 NES color indices
audio := console.Audio()   // float32 samples
```

Or as a standalone core process:

```bash
go build ./cmd/nesd   # builds the core binary (Go 1.25+)
git submodule update --init      # fetch the test ROMs (only needed for tests)
go test ./...                    # everything runs, no external deps
```

The binary isn't interactive. It reads command frames on stdin and writes
event frames on stdout, so something else drives it: a UI, a test harness,
a TAS tool, a WASM page. If you'd rather talk to it over TCP, start it
with `--listen 127.0.0.1:4444` and each connection gets its own
independent console.

```go
enc.Write(nes.OpLoadROM, rom)
enc.Write(nes.OpRunFrame, nil)   // yields a Video + Audio event
```

Full runnable clients for both modes live in
[docs/CLIENT.md](docs/CLIENT.md).

## Example

A complete desktop frontend on [Ebitengine](https://ebitengine.org),
with video, audio and keyboard, lives in
[examples/nes](examples/nes/main.go). It is its own Go module, so the
Ebitengine dependency never touches the core's `go.mod`:

```bash
cd examples/nes && go run . game.nes
```

## Docs

- [Protocol reference](docs/PROTOCOL.md): frames, opcodes, payloads,
  state block.
- [Go client](docs/CLIENT.md): minimal consumers, end to end.
- [Architecture](docs/ARCHITECTURE.md): layers, timing model,
  determinism.
- [Mappers](docs/MAPPERS.md): the full supported list.

### License

Copyright (c) 2026-present [Daniel Gatis](https://github.com/danielgatis)

Licensed under [MIT License](./LICENSE)

### Buy me a coffee

Liked some of my work? Buy me a coffee (or more likely a beer)

<a href="https://www.buymeacoffee.com/danielgatis" target="_blank"><img src="https://bmc-cdn.nyc3.digitaloceanspaces.com/BMC-button-images/custom_images/orange_img.png" alt="Buy Me A Coffee" style="height: auto !important;width: auto !important;"></a>
