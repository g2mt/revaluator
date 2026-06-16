//go:build python

package lang

import (
	"context"
)

// #cgo LDFLAGS: -lpython3.12
// #include <Python.h>
import "C"

// pythonInterpreter embeds libpython via cgo.
//
// It maintains one Py_Initialize'd interpreter for the process lifetime.
// A single __main__ namespace is reused across Eval calls. State between
// calls is carried via a cached hash of the last evaluated prefix.
type pythonInterpreter struct{}

// Start initializes the embedded Python interpreter.
//
// Calls Py_Initialize once. The PyGILState / threading model is set up so
// that subsequent Eval calls can safely acquire the GIL.
func (p *pythonInterpreter) Start(ctx context.Context) error {
	// TODO: Py_Initialize, GIL setup
	return nil
}

// Eval evaluates the Python expression at offset within source.
//
// Algorithm:
//  1. Find the expression start via the Python AST (compile/ast module).
//  2. Split source into prefix (bytes [0, exprStart)) and the current
//     expression.
//  3. Compare prefix hash against the last evaluated prefix hash:
//     - If changed: reset the __main__ namespace and re-execute the
//       entire prefix to rebuild interpreter state.
//     - If unchanged: keep the current interpreter state.
//  4. Evaluate the current expression and return repr(result).
//  5. Statements that produce no value (assignments, defs) return "".
//
// Parsing uses libpython's own AST — no hand-written parser. Comment nodes
// are absent from Python ASTs.
func (p *pythonInterpreter) Eval(source string, offset int) (string, error) {
	// TODO: AST parse, prefix diff, eval, repr
	return "", nil
}

// Close finalizes the Python interpreter.
//
// Calls Py_FinalizeEx and releases any remaining resources.
func (p *pythonInterpreter) Close() error {
	// TODO: Py_FinalizeEx
	return nil
}

func init() {
	Register(&pythonInterpreter{})
}
