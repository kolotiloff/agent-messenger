//go:build !windows

package main

import (
	"os"
	"syscall"
)

// startAgain replaces this process with the updated program, keeping the
// same arguments and terminal.
func startAgain(program string) error {
	return syscall.Exec(program, append([]string{program}, os.Args[1:]...), os.Environ())
}
