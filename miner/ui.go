package main

import (
	"fmt"
	"strings"
)

// ANSI colors (enabled on Windows consoles via enableColors()).
const (
	cReset = "\033[0m"
	cBold  = "\033[1m"
	cDim   = "\033[2m"
	cGold  = "\033[38;5;178m"
	cGreen = "\033[38;5;78m"
	cRed   = "\033[38;5;203m"
	cGray  = "\033[38;5;245m"
)

func clearScreen() { fmt.Print("\033[H\033[2J") }

func shortAddr(a string) string {
	if len(a) < 12 {
		return a
	}
	return a[:6] + "..." + a[len(a)-4:]
}

// uiTitle clears the screen and draws a gold title with a rule under it.
func uiTitle(title string) {
	clearScreen()
	fmt.Printf("\n   %s%s%s%s\n", cGold, cBold, title, cReset)
	fmt.Printf("   %s%s%s\n\n", cGray, strings.Repeat("-", 40), cReset)
}

func uiRow(label, value string) {
	fmt.Printf("   %s%-9s%s %s\n", cGray, label, cReset, value)
}

func uiOption(key, label string) {
	fmt.Printf("   %s%s%s   %s\n", cGold, key, cReset, label)
}

func uiWon(count int, reward, balance string) {
	fmt.Printf("   %s%s[WON]%s block %d    %s+%s AURYX%s    balance: %s%s%s AURYX\n",
		cGreen, cBold, cReset, count, cGreen, reward, cReset, cBold, balance, cReset)
}

func uiInfo(s string) { fmt.Printf("   %s%s%s\n", cDim, s, cReset) }
func uiErr(s string)  { fmt.Printf("   %s%s%s\n", cRed, s, cReset) }
