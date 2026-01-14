// Package main is the entry point for the tanuki CLI.
package main

import (
	"os"

	"github.com/bkonkle/tanuki/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
