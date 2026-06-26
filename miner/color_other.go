//go:build !windows

package main

// On Linux/macOS terminals ANSI works out of the box.
func enableColors() {}
