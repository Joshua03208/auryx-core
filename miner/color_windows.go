//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableColors turns on ANSI/VT processing so colors render in the Windows
// console (PowerShell, Windows Terminal, conhost on Win10+).
func enableColors() {
	h := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err == nil {
		_ = windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
}
