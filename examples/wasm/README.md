# WebAssembly example

The go-headless-nes core running in the browser. The core is compiled to
WebAssembly and does exactly what it does everywhere else — emulate a
frame, hand back video and audio. The page (`index.html`) supplies the
policy: a `requestAnimationFrame` loop, a canvas, keyboard input, and an
AudioWorklet that plays sound from a ring buffer (`nes-audio-worklet.js`).
The worklet runs audio on its own thread and drains the ring at the
hardware clock, so jitter in the render loop never turns into crackle. The
loop itself paces emulation off a wall clock — one NES frame per 60.0988
fps of elapsed time — rather than one frame per rAF tick, so the game runs
at true speed on 60, 120, or 144Hz displays alike. (This is the same split
jsnes-web's FrameTimer uses; the difference is the AudioWorklet ring in
place of its deprecated ScriptProcessorNode.)

Like `examples/nes`, this is its own Go module, so nothing here touches
the core's `go.mod`.

## Build and run

```bash
cd examples/wasm
./build.sh          # produces nes.wasm and copies wasm_exec.js
go run ./serve      # static server with the right .wasm MIME type
```

Then open <http://localhost:8080>, pick a `.nes` ROM, and play.

You can serve the directory with any static HTTP server instead of
`go run ./serve` — the only requirement is that `.wasm` files are sent
with `Content-Type: application/wasm`, which `instantiateStreaming`
needs. Opening `index.html` from `file://` will not work.

## Controls

| Key | Button |
| --- | --- |
| Arrows | D-pad |
| Z | B |
| X | A |
| Enter | Start |
| Shift | Select |
| R | Reset |
| 1 / 2 / 3 | force NTSC / PAL / Dendy |
| 0 | auto (re-detect from the header) |

The loop paces itself at the core's frame rate (`nesFrameRate`), so a PAL
game plays at 50 fps instead of NTSC's 60. A ROM whose header wrongly
claims NTSC runs ~20% fast until you press `2`.

## The bridge

`main.go` exposes a handful of globals to JavaScript and then blocks
forever so they stay callable:

| Global | Purpose |
| --- | --- |
| `nesLoadROM(u8)` | build a Console from iNES bytes; returns `""` or an error string |
| `nesRunFrame()` | advance one frame |
| `nesSetButtons(pad, mask)` | latch controller state (`nesButtons` holds the bit layout) |
| `nesVideoRGBA(u8)` | fill a reused RGBA buffer with the frame |
| `nesAudio()` | return the frame's samples as a `Float32Array` |
| `nesReset()` | reset the console |
| `nesSetRegion(code)` | 0 auto, 1 NTSC, 2 PAL, 3 Dendy (live, no reset) |
| `nesRegion()` | resolved region name |
| `nesFrameRate()` | emulated frames per second, to pace the loop |

The page passes its canvas-backed pixel buffer into `nesVideoRGBA` so the
same allocation is reused every frame instead of copying a fresh slice
across the JS boundary.
