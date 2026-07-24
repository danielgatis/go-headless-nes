// A one-file static server for the WASM example. Browsers refuse to
// instantiate a .wasm file served with the wrong MIME type, and
// WebAssembly.instantiateStreaming needs application/wasm specifically,
// so we set it explicitly rather than trusting the OS mime table.
//
//	go run ./serve        # serves the example on http://localhost:8080
//	go run ./serve :3000  # pick a port
package main

import (
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	addr := ":8080"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	// Serve the example directory itself, so http://localhost:8080/ lands on
	// index.html instead of a directory listing. `go run ./serve` runs with
	// the working dir at examples/wasm, so "." is exactly that directory.
	fs := http.FileServer(http.Dir("."))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		}
		fs.ServeHTTP(w, r)
	})

	log.Printf("serving the wasm example on http://localhost%s", addr) //nolint:gosec // G706: addr is a local dev flag, not untrusted input
	log.Fatal(http.ListenAndServe(addr, handler))                      //nolint:gosec // G114: a throwaway localhost static server needs no timeouts
}
