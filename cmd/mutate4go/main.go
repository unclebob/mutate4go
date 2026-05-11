package main

import (
	"fmt"
	"os"

	"github.com/unclebob/mutate4go/internal/cli"
	"github.com/unclebob/mutate4go/internal/runner"
)

func main() {
	err := runner.Run(cli.ValidateArgs(os.Args[1:]))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(runner.StatusCode(err))
	}
}
