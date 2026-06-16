package rpc

import (
	"io"
)

// Serve runs the JSON-RPC read→dispatch→write loop over stdio.
//
// It reads newline-delimited JSON Request messages from stdin, dispatches to
// the active language interpreter on "eval", and writes JSON Response messages
// to stdout. On "shutdown" it closes the interpreter and exits the loop.
//
// Serve blocks until stdin closes or a shutdown request is received.
func Serve(stdin io.Reader, stdout io.Writer) error {
	// TODO: implement stdio JSON-RPC loop
	return nil
}
