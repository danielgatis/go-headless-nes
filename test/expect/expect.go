// Package expect is the suite's tiny assertion helper: a standard-library
// stand-in for the handful of testify helpers the tests used. Assert
// reports a failure and continues; Require stops the test. Both accept an
// optional trailing message (a bare value, or a format string plus args).
package expect

import (
	"fmt"
	"reflect"
	"testing"
)

// Assert reports failures without halting the test.
var Assert AssertT

// Require halts the test on the first failure.
var Require RequireT

// AssertT groups the reporting (non-fatal) checks.
type AssertT struct{}

// RequireT groups the fatal checks.
type RequireT struct{}

func msg(args []any) string {
	switch len(args) {
	case 0:
		return ""
	case 1:
		return ": " + fmt.Sprint(args[0])
	default:
		return ": " + fmt.Sprintf(args[0].(string), args[1:]...)
	}
}

func equal(want, got any) bool { return reflect.DeepEqual(want, got) }

// equalValues compares ignoring exact type, as testify's EqualValues
// does: the suite only uses it for untyped constants against bytes.
func equalValues(want, got any) bool {
	return equal(want, got) || fmt.Sprint(want) == fmt.Sprint(got)
}

// Equal reports a non-fatal failure when want and got differ.
func (AssertT) Equal(t *testing.T, want, got any, args ...any) {
	t.Helper()
	if !equal(want, got) {
		t.Errorf("not equal:\n want: %#v\n  got: %#v%s", want, got, msg(args))
	}
}

// EqualValues reports a non-fatal failure when want and got differ,
// ignoring exact type.
func (AssertT) EqualValues(t *testing.T, want, got any, args ...any) {
	t.Helper()
	if !equalValues(want, got) {
		t.Errorf("not equal:\n want: %#v\n  got: %#v%s", want, got, msg(args))
	}
}

// Equal fails the test when want and got differ.
func (RequireT) Equal(t *testing.T, want, got any, args ...any) {
	t.Helper()
	if !equal(want, got) {
		t.Fatalf("not equal:\n want: %#v\n  got: %#v%s", want, got, msg(args))
	}
}

// NoError fails the test when err is non-nil.
func (RequireT) NoError(t *testing.T, err error, args ...any) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v%s", err, msg(args))
	}
}

// True fails the test when cond is false.
func (RequireT) True(t *testing.T, cond bool, args ...any) {
	t.Helper()
	if !cond {
		t.Fatalf("expected true%s", msg(args))
	}
}

// NotZero fails the test when v is nil or a zero value.
func (RequireT) NotZero(t *testing.T, v any, args ...any) {
	t.Helper()
	if v == nil || reflect.ValueOf(v).IsZero() {
		t.Fatalf("expected non-zero value%s", msg(args))
	}
}

// GreaterOrEqualf fails the test when a < b.
func (RequireT) GreaterOrEqualf(t *testing.T, a, b int, format string, args ...any) {
	t.Helper()
	if a < b {
		t.Fatalf("%d < %d: %s", a, b, fmt.Sprintf(format, args...))
	}
}
