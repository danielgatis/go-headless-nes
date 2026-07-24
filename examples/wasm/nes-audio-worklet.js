// AudioWorkletProcessor that plays the emulator's audio from an internal
// ring buffer fed over the message port. This is the robust way to do
// emulator audio in the browser: the main thread posts NES samples at
// frame boundaries (bursty, jittery), and the audio thread drains them
// here at a steady rate driven by the hardware clock. Nothing is scheduled
// by hand, so there are no per-buffer gaps to hear as crackle.
//
// Samples arrive at the core's fixed rate (44100). This processor runs at
// the context's real rate (often 48000), so it resamples on the fly with
// linear interpolation using a fractional read cursor.
//
// Using the port (rather than a SharedArrayBuffer) keeps the page servable
// without COOP/COEP headers; postMessage from main→worklet is cheap enough
// for the ~735 samples per frame we push.

class NESAudioProcessor extends AudioWorkletProcessor {
  constructor(options) {
    super();
    const { srcRate, capacity } = options.processorOptions;
    // Fixed-size Float32 ring. capacity is in samples and holds a good
    // fraction of a second so the queue never starves under normal jitter.
    this.ring = new Float32Array(capacity);
    this.cap = capacity;
    this.head = 0;   // next write slot
    this.tail = 0;   // next read slot
    this.size = 0;   // samples currently queued
    this.readPos = 0; // fractional offset into the sample we're on
    this.step = srcRate / sampleRate; // src samples consumed per output sample
    this.last = 0;

    this.port.onmessage = (e) => {
      const chunk = e.data;
      for (let i = 0; i < chunk.length; i++) {
        if (this.size < this.cap) {
          this.ring[this.head] = chunk[i];
          this.head = (this.head + 1) % this.cap;
          this.size++;
        } else {
          // Overflow: drop the oldest to bound latency (rare — only if the
          // main thread ran far ahead of playback).
          this.ring[this.head] = chunk[i];
          this.head = (this.head + 1) % this.cap;
          this.tail = (this.tail + 1) % this.cap;
        }
      }
    };
  }

  peek(offset) {
    // Sample `offset` positions ahead of tail, without consuming.
    return this.ring[(this.tail + offset) % this.cap];
  }

  process(_inputs, outputs) {
    const out = outputs[0][0];
    if (!out) return true;

    let pos = this.readPos;
    for (let i = 0; i < out.length; i++) {
      const idx = pos | 0;
      // Need samples idx and idx+1 available to interpolate. If not, hold
      // the last value instead of clicking; the ring refills next frame.
      if (idx + 1 >= this.size) {
        out[i] = this.last;
        continue;
      }
      const frac = pos - idx;
      const a = this.peek(idx);
      const b = this.peek(idx + 1);
      this.last = a + (b - a) * frac;
      out[i] = this.last;
      pos += this.step;
    }

    // Consume the whole source samples we advanced past.
    const consumed = pos | 0;
    const take = Math.min(consumed, this.size);
    this.tail = (this.tail + take) % this.cap;
    this.size -= take;
    this.readPos = pos - consumed;
    return true;
  }
}

registerProcessor("nes-audio", NESAudioProcessor);
