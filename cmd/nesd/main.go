// Command go-headless-nes is the standalone NES emulator core. It speaks
// the binary control protocol (the root nes package) over stdin/stdout: a
// consumer sends command frames (load ROM, run frame, input, debug, patch)
// and receives event frames (video, audio, state, ...). It has no window,
// no audio device, no scripting, those are the consumer's to build on top
// of the core's primitives.
//
// Stream discipline: stdout carries ONLY protocol frames. Every diagnostic
// (the --trace log, fatal errors, usage/flag errors) goes to stderr. A
// single stray byte on stdout would desync the framing and break the
// client, so nothing here (and nothing in the nes package) may print to it.
//
// Usage:
//
//	go-headless-nes [--trace] [--listen addr]
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"

	nes "github.com/danielgatis/go-headless-nes"
)

func main() {
	fs := flag.NewFlagSet("nesd", flag.ContinueOnError)
	trace := fs.Bool("trace", false, "Log protocol opcodes as text to stderr.")
	listen := fs.String("listen", "", "Serve the protocol on a TCP address (e.g. 127.0.0.1:4444) instead of stdin/stdout. Each connection gets its own console.")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(fs.Output(), "usage: go-headless-nes [--trace] [--listen addr]")
		_, _ = fmt.Fprintln(fs.Output(), "\nNES emulator core speaking the binary protocol over stdin/stdout (or TCP with --listen).")
		_, _ = fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		os.Exit(2)
	}

	if *listen != "" {
		if err := serveTCP(*listen, *trace); err != nil {
			fmt.Fprintf(os.Stderr, "go-headless-nes: %+v\n", err)
			os.Exit(1)
		}
		return
	}

	srv := nes.NewServer(bufio.NewReader(os.Stdin), os.Stdout)
	if *trace {
		srv.SetTrace(os.Stderr)
	}
	if err := srv.Serve(); err != nil {
		// Errors carry an errs stack trace from their origin.
		fmt.Fprintf(os.Stderr, "go-headless-nes: %+v\n", err)
		os.Exit(1)
	}
}

// serveTCP accepts protocol connections on addr forever. Every connection
// is served by its own Server (its own console) so N clients are N
// independent emulator instances. The bound address is announced on stderr
// (stdout stays reserved for protocol frames), so a parent process using
// port 0 can scrape the port the OS picked.
func serveTCP(addr string, trace bool) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "go-headless-nes: listening on %s\n", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go func() {
			defer func() { _ = conn.Close() }()
			srv := nes.NewServer(bufio.NewReader(conn), conn)
			if trace {
				srv.SetTrace(os.Stderr)
			}
			if err := srv.Serve(); err != nil {
				fmt.Fprintf(os.Stderr, "go-headless-nes: %s: %+v\n", conn.RemoteAddr(), err)
			}
		}()
	}
}
