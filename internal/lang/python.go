//go:build python

package lang

/*
#cgo pkg-config: python3-embed
#define PY_SSIZE_T_CLEAN
#include <Python.h>
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

// splitHelper is executed once at Start. It defines _reval_split(source, offset)
// which uses Python's own AST to locate the top-level statement whose start lies
// at or after the given byte offset. It returns (exprStart, exprText) where
// exprStart is the byte offset of that statement's start and exprText is the
// statement's source span. Everything before exprStart is the prefix.
//
// All offsets are computed in UTF-8 bytes to match what the Neovim plugin sends
// (a byte offset) and what CPython stores in col_offset (UTF-8 byte columns).
const splitHelper = `
import ast as _ast

def _reval_split(_source, _offset):
    _sb = _source.encode('utf-8')
    _starts = [0]
    for _i in range(len(_sb)):
        if _sb[_i] == 0x0a:
            _starts.append(_i + 1)
    try:
        _tree = _ast.parse(_source)
    except SyntaxError:
        return (_offset, '')
    _target = None
    for _node in _tree.body:
        _s = _starts[_node.lineno - 1] + _node.col_offset
        if _s >= _offset:
            _target = _node
            break
    if _target is None:
        if _tree.body:
            _target = _tree.body[-1]
        else:
            return (_offset, '')
    _start = _starts[_target.lineno - 1] + _target.col_offset
    _end = _starts[_target.end_lineno - 1] + _target.end_col_offset
    _expr = _sb[_start:_end].decode('utf-8')
    return (_start, _expr)
`

// pythonInterpreter embeds libpython via cgo.
//
// It maintains one Py_Initialize'd interpreter for the process lifetime.
// A single __main__ namespace dict (ns) is reused across Eval calls. The
// last evaluated prefix is cached so the namespace is only rebuilt when
// the prefix changes.
type pythonInterpreter struct {
	mu sync.Mutex

	// ns is the namespace dict used as globals/locals for the current
	// expression and prefix execution. Rebuilt when the prefix changes.
	ns *C.PyObject

	// helper holds the globals of the AST-splitting helper module.
	helper *C.PyObject

	// split is a borrowed-ref-turned-owned reference to _reval_split.
	split *C.PyObject

	// lastPrefix is the last successfully evaluated prefix text.
	lastPrefix string

	// tstate is the saved main thread state captured after initialization so
	// the GIL can be re-acquired per Eval via PyGILState_Ensure.
	tstate *C.PyThreadState

	started bool
}

// Start initializes the embedded Python interpreter.
//
// It calls Py_Initialize once, installs the AST-splitting helper, creates the
// initial (empty) namespace, then releases the GIL so subsequent Eval calls can
// acquire it from any OS thread via PyGILState_Ensure.
func (p *pythonInterpreter) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return nil
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	C.Py_Initialize()
	if C.Py_IsInitialized() == 0 {
		return errors.New("python: Py_Initialize failed")
	}

	// Install the helper that performs AST-based expression splitting.
	p.helper = C.PyDict_New()
	if p.helper == nil {
		return errors.New("python: failed to allocate helper namespace")
	}
	if err := runCode(splitHelper, C.int(C.Py_file_input), p.helper); err != nil {
		return fmt.Errorf("python: failed to install AST helper: %w", err)
	}

	cname := C.CString("_reval_split")
	p.split = C.PyDict_GetItemString(p.helper, cname) // borrowed
	C.free(unsafe.Pointer(cname))
	if p.split == nil {
		return errors.New("python: _reval_split not defined")
	}
	C.Py_IncRef(p.split) // take an owned reference

	// Fresh, empty namespace.
	p.resetNamespace()
	p.lastPrefix = ""

	// Release the GIL; Eval re-acquires it per call.
	p.tstate = C.PyEval_SaveThread()
	p.started = true
	return nil
}

// Eval evaluates the Python expression at byte offset within source.
//
// It locates the current expression via the AST helper, then incrementally
// maintains interpreter state: if the prefix is unchanged the namespace is
// kept; if the prefix grew by one or more statements, only the delta is
// evaluated; otherwise the namespace is rebuilt from scratch. It then
// evaluates the current expression returning repr(result).
// Statements with no value (assignments, defs) return "".
func (p *pythonInterpreter) Eval(source string, offset int) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return "", errors.New("python: interpreter not started")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	gil := C.PyGILState_Ensure()
	defer C.PyGILState_Release(gil)

	// 1. Split into prefix + current expression using the AST helper.
	exprStart, expr, err := p.splitSource(source, offset)
	if err != nil {
		return "", err
	}
	if exprStart < 0 || exprStart > len(source) {
		exprStart = len(source)
	}
	prefix := source[:exprStart]

	// 2. Maintain interpreter state based on prefix changes.
	if prefix != p.lastPrefix {
		if p.lastPrefix != "" && strings.HasPrefix(prefix, p.lastPrefix) {
			// Prefix grew: evaluate only the new portion.
			delta := prefix[len(p.lastPrefix):]
			if len(delta) > 0 && (delta[0] == '\n' || delta[0] == ';') {
				if err := runCode(delta, C.int(C.Py_file_input), p.ns); err != nil {
					p.lastPrefix = ""
					return "", err
				}
			} else {
				// Not a clean continuation; rebuild from scratch.
				p.resetNamespace()
				if err := runCode(prefix, C.int(C.Py_file_input), p.ns); err != nil {
					p.lastPrefix = ""
					return "", err
				}
			}
		} else {
			// Different prefix; rebuild from scratch.
			p.resetNamespace()
			if err := runCode(prefix, C.int(C.Py_file_input), p.ns); err != nil {
				p.lastPrefix = ""
				return "", err
			}
		}
		p.lastPrefix = prefix
	}

	if expr == "" {
		return "", nil
	}

	// 3. Evaluate the current expression. Try expression mode first; on a
	//    SyntaxError it is a statement, which we exec for its side effects.
	cexpr := C.CString(expr)
	defer C.free(unsafe.Pointer(cexpr))

	result := C.PyRun_String(cexpr, C.int(C.Py_eval_input), p.ns, p.ns)
	if result == nil {
		if C.PyErr_ExceptionMatches(C.PyExc_SyntaxError) != 0 {
			// Not an expression: run as a statement, no value to report.
			C.PyErr_Clear()
			if err := runCode(expr, C.int(C.Py_file_input), p.ns); err != nil {
				return "", err
			}
			return "", nil
		}
		return "", pyFetchError()
	}
	defer C.Py_DecRef(result)

	repr := C.PyObject_Repr(result)
	if repr == nil {
		return "", pyFetchError()
	}
	defer C.Py_DecRef(repr)

	return goStringFromPy(repr), nil
}

// Close finalizes the Python interpreter.
func (p *pythonInterpreter) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return nil
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Re-acquire the GIL on this thread before finalizing.
	C.PyEval_RestoreThread(p.tstate)

	if p.split != nil {
		C.Py_DecRef(p.split)
		p.split = nil
	}
	if p.ns != nil {
		C.Py_DecRef(p.ns)
		p.ns = nil
	}
	if p.helper != nil {
		C.Py_DecRef(p.helper)
		p.helper = nil
	}

	if C.Py_FinalizeEx() < 0 {
		p.started = false
		return errors.New("python: Py_FinalizeEx reported an error")
	}
	p.started = false
	return nil
}

// splitSource calls the helper _reval_split(source, offset) and returns the byte
// offset of the current expression and its source text. Must be called with the
// GIL held.
func (p *pythonInterpreter) splitSource(source string, offset int) (int, string, error) {
	srcObj := pyUnicode(source)
	if srcObj == nil {
		return 0, "", pyFetchError()
	}
	offObj := C.PyLong_FromLongLong(C.longlong(offset))
	if offObj == nil {
		C.Py_DecRef(srcObj)
		return 0, "", pyFetchError()
	}

	args := C.PyTuple_New(2)
	// PyTuple_SetItem steals references to srcObj and offObj.
	C.PyTuple_SetItem(args, 0, srcObj)
	C.PyTuple_SetItem(args, 1, offObj)

	res := C.PyObject_CallObject(p.split, args)
	C.Py_DecRef(args)
	if res == nil {
		return 0, "", pyFetchError()
	}
	defer C.Py_DecRef(res)

	startObj := C.PyTuple_GetItem(res, 0) // borrowed
	exprObj := C.PyTuple_GetItem(res, 1)  // borrowed
	if startObj == nil || exprObj == nil {
		return 0, "", errors.New("python: malformed split result")
	}

	start := int(C.PyLong_AsLongLong(startObj))
	expr := goStringFromPy(exprObj)
	return start, expr, nil
}

// resetNamespace discards the current namespace and creates a fresh one with
// __builtins__ installed. Must be called with the GIL held.
func (p *pythonInterpreter) resetNamespace() {
	if p.ns != nil {
		C.Py_DecRef(p.ns)
	}
	p.ns = C.PyDict_New()

	cbuiltins := C.CString("builtins")
	mod := C.PyImport_ImportModule(cbuiltins)
	C.free(unsafe.Pointer(cbuiltins))
	if mod != nil {
		ckey := C.CString("__builtins__")
		C.PyDict_SetItemString(p.ns, ckey, mod)
		C.free(unsafe.Pointer(ckey))
		C.Py_DecRef(mod)
	}
}

// runCode executes src in the given namespace as both globals and locals. Must
// be called with the GIL held.
func runCode(src string, start C.int, ns *C.PyObject) error {
	csrc := C.CString(src)
	defer C.free(unsafe.Pointer(csrc))

	res := C.PyRun_String(csrc, start, ns, ns)
	if res == nil {
		return pyFetchError()
	}
	C.Py_DecRef(res)
	return nil
}

// pyUnicode builds a Python str from a Go string. Must be called with the GIL
// held. Returns a new reference (or nil on error).
func pyUnicode(s string) *C.PyObject {
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	return C.PyUnicode_FromStringAndSize(cs, C.Py_ssize_t(len(s)))
}

// goStringFromPy converts a Python str object to a Go string. Must be called
// with the GIL held.
func goStringFromPy(o *C.PyObject) string {
	if o == nil {
		return ""
	}
	var size C.Py_ssize_t
	cs := C.PyUnicode_AsUTF8AndSize(o, &size)
	if cs == nil {
		C.PyErr_Clear()
		return ""
	}
	return C.GoStringN(cs, C.int(size))
}

// pyFetchError fetches the current Python exception and converts it to a Go
// error of the form "ExceptionType: message". Must be called with the GIL held.
func pyFetchError() error {
	if C.PyErr_Occurred() == nil {
		return errors.New("python: unknown error")
	}

	var ptype, pvalue, ptb *C.PyObject
	C.PyErr_Fetch(&ptype, &pvalue, &ptb)
	C.PyErr_NormalizeException(&ptype, &pvalue, &ptb)

	msg := "python error"
	if pvalue != nil {
		s := C.PyObject_Str(pvalue)
		if s != nil {
			msg = goStringFromPy(s)
			C.Py_DecRef(s)
		}
	}

	name := ""
	if ptype != nil {
		cattr := C.CString("__name__")
		n := C.PyObject_GetAttrString(ptype, cattr)
		C.free(unsafe.Pointer(cattr))
		if n != nil {
			name = goStringFromPy(n)
			C.Py_DecRef(n)
		}
	}

	if ptype != nil {
		C.Py_DecRef(ptype)
	}
	if pvalue != nil {
		C.Py_DecRef(pvalue)
	}
	if ptb != nil {
		C.Py_DecRef(ptb)
	}

	if name != "" {
		return fmt.Errorf("%s: %s", name, msg)
	}
	return errors.New(msg)
}

func init() {
	Register(&pythonInterpreter{})
}
