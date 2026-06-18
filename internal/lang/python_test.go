//go:build python

package lang

import (
	"context"
	"strings"
	"testing"
)

// newStartedInterpreter starts a fresh interpreter and registers cleanup.
//
// Note: CPython can only be initialized once per process, so a single
// interpreter is shared across all tests via the package-level helper.
func newStartedInterpreter(t *testing.T) *pythonInterpreter {
	t.Helper()
	if sharedInterp == nil {
		t.Fatalf("shared interpreter not initialized")
	}
	return sharedInterp
}

var sharedInterp *pythonInterpreter

func TestMain(m *testing.M) {
	p := &pythonInterpreter{}
	if err := p.Start(context.Background()); err != nil {
		panic("Start failed: " + err.Error())
	}
	sharedInterp = p
	code := m.Run()
	_ = p.Close()
	// os.Exit so deferred funcs don't matter; use the run code.
	if code != 0 {
		panic("tests failed")
	}
}

// evalLine evaluates source treating the final line as the current expression.
func evalLine(t *testing.T, p *pythonInterpreter, source string) (string, error) {
	t.Helper()
	// line = 0-based index of the last line.
	line := strings.Count(source, "\n")
	return p.Eval(source, line)
}

func TestEvalSimpleExpression(t *testing.T) {
	p := newStartedInterpreter(t)

	got, err := p.Eval("1 + 2", 0)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	if got != "3" {
		t.Fatalf("got %q, want %q", got, "3")
	}
}

func TestEvalStringRepr(t *testing.T) {
	p := newStartedInterpreter(t)

	got, err := p.Eval(`"hello"`, 0)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	if got != `'hello'` {
		t.Fatalf("got %q, want %q", got, `'hello'`)
	}
}

func TestEvalWithPrefixState(t *testing.T) {
	p := newStartedInterpreter(t)

	source := "x = 10\ny = 20\nx + y"
	got, err := evalLine(t, p, source)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	if got != "30" {
		t.Fatalf("got %q, want %q", got, "30")
	}
}

func TestEvalStatementReturnsEmpty(t *testing.T) {
	p := newStartedInterpreter(t)

	source := "a = 5"
	got, err := p.Eval(source, 0)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
}

func TestEvalFunctionDef(t *testing.T) {
	p := newStartedInterpreter(t)

	source := "def f(n):\n    return n * n\nf(6)"
	got, err := evalLine(t, p, source)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	if got != "36" {
		t.Fatalf("got %q, want %q", got, "36")
	}
}

func TestEvalPrefixChangeRebuildsState(t *testing.T) {
	p := newStartedInterpreter(t)

	// First evaluation establishes z = 1.
	got, err := evalLine(t, p, "z = 1\nz")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	if got != "1" {
		t.Fatalf("got %q, want %q", got, "1")
	}

	// Changed prefix should rebuild namespace and reflect z = 99.
	got, err = evalLine(t, p, "z = 99\nz")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	if got != "99" {
		t.Fatalf("got %q, want %q", got, "99")
	}
}

func TestEvalUnchangedPrefixKeepsState(t *testing.T) {
	p := newStartedInterpreter(t)

	source := "k = 7\nk * 2"
	if _, err := evalLine(t, p, source); err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	firstPrefix := p.lastPrefix

	if _, err := evalLine(t, p, source); err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	if p.lastPrefix != firstPrefix {
		t.Fatalf("prefix changed on identical source: %q -> %q", firstPrefix, p.lastPrefix)
	}
}

func TestEvalRuntimeError(t *testing.T) {
	p := newStartedInterpreter(t)

	_, err := p.Eval("1 / 0", 0)
	if err == nil {
		t.Fatalf("expected error for division by zero")
	}
	if !strings.Contains(err.Error(), "ZeroDivisionError") {
		t.Fatalf("expected ZeroDivisionError, got: %v", err)
	}
}

func TestEvalNameError(t *testing.T) {
	p := newStartedInterpreter(t)

	_, err := p.Eval("undefined_name_xyz", 0)
	if err == nil {
		t.Fatalf("expected NameError")
	}
	if !strings.Contains(err.Error(), "NameError") {
		t.Fatalf("expected NameError, got: %v", err)
	}
}

func TestEvalPrefixSyntaxError(t *testing.T) {
	p := newStartedInterpreter(t)

	// A syntactically invalid prefix should surface an error.
	source := "def broken(:\n1"
	_, err := evalLine(t, p, source)
	if err == nil {
		t.Fatalf("expected error for invalid prefix")
	}
}

func TestEvalListRepr(t *testing.T) {
	p := newStartedInterpreter(t)

	got, err := p.Eval("[1, 2, 3]", 0)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	if got != "[1, 2, 3]" {
		t.Fatalf("got %q, want %q", got, "[1, 2, 3]")
	}
}

func TestEvalImportInPrefix(t *testing.T) {
	p := newStartedInterpreter(t)

	source := "import math\nmath.sqrt(16)"
	got, err := evalLine(t, p, source)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	if got != "4.0" {
		t.Fatalf("got %q, want %q", got, "4.0")
	}
}
