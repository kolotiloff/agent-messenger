//go:build windows

package main

import "golang.org/x/sys/windows"

// openInBrowser asks Windows to open the file with its default program for
// .html, normally the browser.
func openInBrowser(path string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}
