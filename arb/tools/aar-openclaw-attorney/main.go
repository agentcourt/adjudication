package main

import (
	"context"
	"fmt"
	"os"

	"adjudication/arb/tools/aar-openclaw-attorney/internal/openclawattorney"
)

type stdio struct{}

func (stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }

func main() {
	if err := openclawattorney.Run(context.Background(), stdio{}, os.Stderr, openclawattorney.ConfigFromEnv()); err != nil {
		fmt.Fprintf(os.Stderr, "aar-openclaw-attorney: %v\n", err)
		os.Exit(1)
	}
}
