//go:build windows

package localui

import "golang.org/x/sys/windows"

// openURL asks Windows to open a fixed https page in the default browser.
func openURL(u string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(u)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}
