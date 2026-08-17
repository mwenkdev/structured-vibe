// Command svibe is the Structured Vibe CLI.
//
// Structured Vibe resolves, validates, materializes, and advises. The host
// loads skills, runs models, and executes tools.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/mwenkdev/structured-vibe/internal/cli"
)

func main() {
	e := &cli.Env{Stdout: os.Stdout, Stderr: os.Stderr}

	err := cli.Run(e, os.Args[1:])
	if err == nil {
		return
	}

	var exit *cli.ExitError
	if errors.As(err, &exit) {
		os.Exit(exit.Code)
	}

	fmt.Fprintln(os.Stderr, "svibe:", err)
	os.Exit(1)
}
