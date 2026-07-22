package nes

import (
	"reflect"
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/testrom"
)

// TestSnapshotBinaryRoundTrip is the anti-fragility guard for the manual
// per-package serialization: it runs a console far enough to populate the
// deep render/audio pipeline state (including the unexported fields a
// reflection encoder would miss), snapshots it, marshals to bytes,
// unmarshals into a fresh Snapshot, and asserts the two are identical.
//
// A field a codec forgot to write comes back zero and trips the
// comparison here. The DMC's DMA hook function fields are excluded: they
// are re-installed on Restore and deliberately not serialized.
func TestSnapshotBinaryRoundTrip(t *testing.T) {
	c, err := New(testrom.New(t))
	if err != nil {
		t.Fatal(err)
	}
	// Run long enough to exercise CPU, PPU (multiple frames), APU channels
	// and the frame counter, so unexported latches hold non-zero values.
	for i := 0; i < 200_000; i++ {
		c.Step()
	}

	var want Snapshot
	c.Save(&want)

	b := want.MarshalBinary()

	var got Snapshot
	if err := got.UnmarshalBinary(b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The DMC DMA hooks are not serialized (re-installed on Restore); zero
	// them in the reference so the comparison ignores live wiring.
	want.APU.DMC.ClearDMAHooks()
	got.APU.DMC.ClearDMAHooks()

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("snapshot round-trip mismatch (a serialized field was dropped?)\nlen=%d", len(b))
	}
}

// TestSnapshotUnmarshalShort rejects a truncated payload.
func TestSnapshotUnmarshalShort(t *testing.T) {
	var s Snapshot
	if err := s.UnmarshalBinary([]byte{1, 2, 3}); err == nil {
		t.Error("expected error unmarshaling a truncated snapshot")
	}
}
