// internal/secret/sealkey/main.go

// Command sealkey prints the sealed form of an API key for -ldflags. It
// reads the key from the environment variable it's given, never from its
// arguments, so the key stays out of the process list and build logs:
//
//	go run ./internal/secret/sealkey GEMINI_API_KEY
//
// An unset or empty variable prints nothing, which builds without that key.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ksauraj/shellrecap/internal/secret"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: sealkey ENV_VAR")
		os.Exit(2)
	}
	sealed, err := secret.Seal(strings.TrimSpace(os.Getenv(os.Args[1])))
	if err != nil {
		fmt.Fprintln(os.Stderr, "sealkey:", err)
		os.Exit(1)
	}
	fmt.Print(sealed)
}
