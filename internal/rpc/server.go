package rpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"revaluator/internal/lang"
)

// flusher is a subset of bufio.Writer / os.File that supports explicit flush.
type flusher interface {
	Flush() error
}

// writeResponse marshals resp as JSON, writes a newline, and flushes if the
// writer supports it.
func writeResponse(w io.Writer, resp Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	if _, err := fmt.Fprintf(w, "%s\n", data); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	if f, ok := w.(flusher); ok {
		if err := f.Flush(); err != nil {
			return fmt.Errorf("flush response: %w", err)
		}
	}
	return nil
}

// Serve runs the JSON-RPC read→dispatch→write loop over stdio.
//
// It reads newline-delimited JSON Request messages from stdin, dispatches to
// the active language interpreter on "eval", and writes JSON Response messages
// to stdout. On "shutdown" it closes the interpreter and exits the loop.
//
// Serve blocks until stdin closes or a shutdown request is received.
func Serve(stdin io.Reader, stdout io.Writer) error {
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := scanner.Bytes()

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			// Malformed line; respond with an error (no ID to echo).
			_ = writeResponse(stdout, Response{
				Error: fmt.Sprintf("invalid request: %v", err),
			})
			continue
		}

		switch req.Method {
		case "eval":
			source, ok := req.Params["source"].(string)
			if !ok {
				_ = writeResponse(stdout, Response{
					ID:    req.ID,
					Error: "missing or invalid 'source' parameter (expected string)",
				})
				continue
			}

			lineFloat, ok := req.Params["line"].(float64)
			if !ok {
				_ = writeResponse(stdout, Response{
					ID:    req.ID,
					Error: "missing or invalid 'line' parameter (expected number)",
				})
				continue
			}
			line := int(lineFloat)

			if lang.Active == nil {
				_ = writeResponse(stdout, Response{
					ID:    req.ID,
					Error: "no language interpreter registered",
				})
				continue
			}

			value, err := lang.Active.Eval(source, line)
			if err != nil {
				_ = writeResponse(stdout, Response{
					ID:    req.ID,
					Error: err.Error(),
				})
				continue
			}

			_ = writeResponse(stdout, Response{
				ID:    req.ID,
				Value: value,
			})

		case "shutdown":
			if lang.Active != nil {
				_ = lang.Active.Close()
			}
			return nil

		default:
			_ = writeResponse(stdout, Response{
				ID:    req.ID,
				Error: fmt.Sprintf("unknown method: %q", req.Method),
			})
		}
	}

	// Scanner stopped (EOF or error). Clean up the interpreter.
	if lang.Active != nil {
		_ = lang.Active.Close()
	}
	return scanner.Err()
}
