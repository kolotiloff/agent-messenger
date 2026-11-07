//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// startAgain starts the updated program with the same arguments in a new
// console window; this one closes when the old program exits.
func startAgain(program string) error {
	cmd := exec.Command(program, os.Args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
