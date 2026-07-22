// Package errs is this emulator's error-handling convention: every error
// created in production code carries a stack trace captured at its
// origin. It is a small standard-library replacement for the handful
// of features the project used from a third-party error library.
//
// New and Errorf create leaf errors with a fresh stack. Wrap and Wrapf
// add a message layer while preserving the original stack, so the trace
// always points at where the failure began, not where it was wrapped.
// Errors implement Unwrap, so errors.Is and errors.As work as usual.
// Formatting an error with %+v prints the message chain followed by the
// origin stack, in the familiar file:line layout.
package errs

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
)

// Error is a message layer over an optional cause, carrying the stack
// trace of the failure's origin.
type Error struct {
	msg   string
	cause error
	stack []uintptr
}

// New returns an error with the given message and a fresh stack trace.
func New(msg string) error {
	return &Error{msg: msg, stack: callers()}
}

// Errorf returns a formatted error with a fresh stack trace. It does not
// interpret %w; use Wrap to attach a cause.
func Errorf(format string, args ...any) error {
	return &Error{msg: fmt.Sprintf(format, args...), stack: callers()}
}

// Wrap adds a message layer to err. If err already carries a stack, that
// stack is inherited so the trace keeps pointing at the origin; a plain
// standard-library error contributes a fresh stack captured here.
// Wrapping nil returns nil.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &Error{msg: msg, cause: err, stack: inheritStack(err)}
}

// Wrapf is Wrap with a formatted message.
func Wrapf(err error, format string, args ...any) error {
	return Wrap(err, fmt.Sprintf(format, args...))
}

// Error renders the message chain, outermost first: "wrap: inner: leaf".
func (e *Error) Error() string {
	if e.cause == nil {
		return e.msg
	}
	return e.msg + ": " + e.cause.Error()
}

// Unwrap exposes the cause so errors.Is and errors.As traverse the chain.
func (e *Error) Unwrap() error { return e.cause }

// Format implements fmt.Formatter. %v and %s print the message chain;
// %+v appends the origin stack in "func\n\tfile:line" form per frame.
func (e *Error) Format(s fmt.State, verb rune) {
	if verb == 'v' && s.Flag('+') {
		_, _ = io.WriteString(s, e.Error())
		writeStack(s, e.stack)
		return
	}
	_, _ = io.WriteString(s, e.Error())
}

// callers captures the stack, skipping runtime.Callers, this function,
// and the errs constructor that called it.
func callers() []uintptr {
	var pcs [32]uintptr
	n := runtime.Callers(3, pcs[:])
	return pcs[:n]
}

// inheritStack returns the origin stack already carried by err, or a
// fresh one captured at the wrap site if err has none. Every *Error this
// package builds already carries a non-nil stack (its own or inherited),
// so the topmost *Error's stack is the origin: errors.As finds it, and a
// foreign leaf falls through to a fresh capture.
func inheritStack(err error) []uintptr {
	var e *Error
	if errors.As(err, &e) {
		return e.stack
	}
	// The frames to skip mirror callers(), plus Wrap and its caller
	// (Wrapf or the wrap site); one extra Callers skip keeps the trace
	// starting at the code that wrapped a stackless standard error.
	var pcs [32]uintptr
	n := runtime.Callers(4, pcs[:])
	return pcs[:n]
}

// writeStack prints frames in pkg/errors' layout: a blank line, then
// "func\n\tfile:line" for each frame down to (but not through) the test
// or runtime bottom.
func writeStack(w io.Writer, stack []uintptr) {
	if len(stack) == 0 {
		return
	}
	_, _ = io.WriteString(w, "\n")
	frames := runtime.CallersFrames(stack)
	for {
		frame, more := frames.Next()
		_, _ = fmt.Fprintf(w, "\n%s\n\t%s:%d", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
}

// String helps when an *Error is printed via %v on the concrete type.
func (e *Error) String() string { return strings.TrimSpace(e.Error()) }
