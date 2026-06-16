package main

import (
	"context"
	"fmt"
	"os"

	"revaluator/internal/lang"
	"revaluator/internal/rpc"
)

func main() {
	ctx := context.Background()

	if lang.Active == nil {
		fmt.Fprintln(os.Stderr, "revaluator: no interpreter registered (build tag missing?)")
		os.Exit(1)
	}

	if err := lang.Active.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "revaluator: interpreter start failed: %v\n", err)
		os.Exit(1)
	}
	defer lang.Active.Close()

	if err := rpc.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "revaluator: rpc serve error: %v\n", err)
		os.Exit(1)
	}
}
