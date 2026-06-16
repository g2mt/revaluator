//go:build javascript

package lang

import (
	"context"
	"errors"
)

// jsInterpreter is a stub placeholder for a future JavaScript runtime.
// When fully implemented it will embed a JS engine (e.g., goja or v8go).
type jsInterpreter struct{}

// Start is a no-op stub. A real implementation would initialize the JS runtime.
func (j *jsInterpreter) Start(ctx context.Context) error {
	return nil
}

// Eval returns an error indicating JavaScript evaluation is not yet implemented.
func (j *jsInterpreter) Eval(source string, offset int) (string, error) {
	return "", errors.New("javascript interpreter not yet implemented")
}

// Close is a no-op stub.
func (j *jsInterpreter) Close() error {
	return nil
}

func init() {
	Register(&jsInterpreter{})
}
