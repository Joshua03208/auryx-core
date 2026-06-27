package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Baked-in defaults so a user only needs to supply their private key.
const (
	minerVersion     = "0.1.2"
	defaultRPC       = "https://sepolia.base.org"
	defaultContract  = "0x619Ab437232f58fd0FC7606b98BB2D4948734750"
	latestReleaseAPI = "https://api.github.com/repos/Joshua03208/auryx-core/releases/latest"
	releasesPage     = "github.com/Joshua03208/auryx-core/releases"
)

// updateTo holds a newer available version (e.g. "0.1.2"), or "" if up to date.
var updateTo string

// fetchLatestVersion returns the latest released version (e.g. "0.1.2"), or "".
func fetchLatestVersion() string {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(latestReleaseAPI)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var data struct {
		TagName string `json:"tag_name"`
	}
	if json.NewDecoder(resp.Body).Decode(&data) != nil {
		return ""
	}
	return strings.TrimPrefix(data.TagName, "v")
}

// isNewer reports whether dotted version a is newer than b (e.g. "0.2.0" > "0.1.1").
func isNewer(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		na, _ := strconv.Atoi(pa[i])
		nb, _ := strconv.Atoi(pb[i])
		if na != nb {
			return na > nb
		}
	}
	return len(pa) > len(pb)
}

func formatAURYX(wei *big.Int) string {
	if wei == nil {
		return "0"
	}
	return new(big.Int).Div(wei, big.NewInt(1_000_000_000_000_000_000)).String()
}

func formatRate(hps float64) string {
	switch {
	case hps >= 1e9:
		return fmt.Sprintf("%.2f GH/s", hps/1e9)
	case hps >= 1e6:
		return fmt.Sprintf("%.1f MH/s", hps/1e6)
	case hps >= 1e3:
		return fmt.Sprintf("%.0f KH/s", hps/1e3)
	default:
		return fmt.Sprintf("%.0f H/s", hps)
	}
}

// expectedHashes returns the average number of hashes needed to solve a block at
// the given target (= 2^256 / target). PoW is memoryless, so this is a statistical
// average, not a countdown — an individual block can take far more or far fewer.
func expectedHashes(target *big.Int) float64 {
	if target == nil || target.Sign() <= 0 {
		return 0
	}
	maxVal := new(big.Float).SetInt(new(big.Int).Lsh(big.NewInt(1), 256))
	f, _ := new(big.Float).Quo(maxVal, new(big.Float).SetInt(target)).Float64()
	return f
}

// runMining mines round after round. If maxBlocks > 0 it stops after that many
// wins (used by tests / scripted runs); 0 means run until interrupted.
func runMining(chain *Chain, cfg *Config, maxBlocks int) {
	ctx := context.Background()
	cores := cfg.resolveCores()
	var miner20 [20]byte
	copy(miner20[:], chain.From().Bytes())
	won := 0

	// count time actually spent mining toward this session's + all-time stats
	runStart := time.Now()
	defer func() {
		d := time.Since(runStart)
		session.addMining(d)
		cfg.LifetimeSeconds += d.Seconds()
		saveConfig(*cfg)
	}()

	// Ctrl-C stops mining and returns to the menu (signal handling is scoped to
	// here via defer signal.Stop, so Ctrl-C at the menu still quits normally).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	userStop := make(chan struct{})
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-sigCh:
			close(userStop)
		case <-done:
		}
	}()

	// Reward is constant until an era boundary (a long way off), so fetch once.
	rewardBig := new(big.Int).Mul(big.NewInt(50), big.NewInt(1_000_000_000_000_000_000))
	if r, err := chain.Reward(ctx); err == nil {
		rewardBig = r
	}
	rewardStr := formatAURYX(rewardBig)
	rewardAURYX := weiToAURYX(rewardBig)

	// Per-block stats shared with the live readout below. blockExpected is the
	// average hashes this block should take (from its difficulty); blockStartHash
	// is the hash count when the block began, so cur-start = hashes into this block.
	var rmu sync.Mutex
	var blockStartHash int64
	var blockExpected float64

	// live progress readout: how far through the *average* effort this block is
	// (can pass 100% — just an unlucky-long block), plus a live earnings + network
	// share estimate from your hashrate and the current difficulty.
	go func() {
		t := time.NewTicker(4 * time.Second)
		defer t.Stop()
		last, lastT := hashesDone(), time.Now()
		for {
			select {
			case <-done:
				return
			case <-userStop:
				return // Ctrl-C: stop printing immediately
			case now := <-t.C:
				cur := hashesDone()
				dt := now.Sub(lastT).Seconds()
				if dt > 0 && cur > last {
					rate := float64(cur-last) / dt
					rmu.Lock()
					start, exp := blockStartHash, blockExpected
					rmu.Unlock()
					if exp > 0 && rate > 0 {
						pct := float64(cur-start) / exp * 100
						earnPerHr := rate * 3600 / exp * rewardAURYX
						nb := session.networkBlock()
						if nb <= 0 {
							nb = 60 // until we've measured, use the contract's target
						}
						share := rate * nb / exp * 100
						if share > 100 {
							share = 100
						}
						line := fmt.Sprintf("mining . %.0f%% of avg block . %s . ~%s AURYX/hr . ~%.0f%% of net",
							pct, formatRate(rate), fmtAmt(earnPerHr), share)
						if pct >= 100 {
							line += "  (longer than usual - normal)"
						}
						uiInfo(line)
					} else {
						uiInfo("hashing at " + formatRate(rate))
					}
				}
				last, lastT = cur, now
			}
		}
	}()

	uiInfo(fmt.Sprintf("Mining with %d core(s). Press Ctrl-C to stop and return to the menu.", cores))
	fmt.Println()

	var lastChallenge [32]byte
	var lastBlockAt time.Time

	for {
		select {
		case <-userStop:
			fmt.Println()
			uiInfo(fmt.Sprintf("Stopped - won %d block(s) this run. Back to the menu.", won))
			return
		default:
		}

		challenge, err := chain.Challenge(ctx)
		if err != nil {
			uiErr("Can't reach the network - retrying in 3s...")
			time.Sleep(3 * time.Second)
			continue
		}

		// a changed challenge means a block was found (by anyone) — time it for the
		// network-block-time estimate that drives the network-share calculation.
		if challenge != lastChallenge {
			if !lastBlockAt.IsZero() {
				session.recordBlockTime(time.Since(lastBlockAt).Seconds())
			}
			lastBlockAt = time.Now()
			lastChallenge = challenge
		}

		target, err := chain.Target(ctx)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		// reset progress for this block: where the hash counter is now, and how
		// many hashes this difficulty should take on average.
		rmu.Lock()
		blockStartHash = hashesDone()
		blockExpected = expectedHashes(target)
		rmu.Unlock()

		var once sync.Once
		stop := make(chan struct{})
		closeStop := func() { once.Do(func() { close(stop) }) }

		// abandon the grind if someone else mines the block (challenge changes)
		go func() {
			t := time.NewTicker(2 * time.Second)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					if cur, e := chain.Challenge(ctx); e == nil && cur != challenge {
						closeStop()
						return
					}
				}
			}
		}()
		// stop the grind immediately if the user pressed Ctrl-C
		go func() {
			select {
			case <-userStop:
				closeStop()
			case <-stop:
			}
		}()

		sol := MineMultiCore(challenge, miner20, target, cores, stop)
		closeStop()
		if sol == nil {
			continue // stale, or user stopped (handled at the top of the loop)
		}

		tx, err := chain.SubmitMint(ctx, sol.Nonce, sol.Digest)
		if err != nil {
			uiErr("Submit failed - trying again.")
			continue
		}
		ok, err := chain.WaitMined(ctx, tx)
		if err != nil {
			uiErr("Transaction error - trying again.")
			continue
		}
		if !ok {
			uiInfo("Beaten to that block - trying again.")
			continue
		}
		won++
		session.recordWin(rewardBig)
		cfg.addLifetime(rewardBig)
		saveConfig(*cfg)
		bal, _ := chain.Balance(ctx, chain.From())
		uiWon(won, rewardStr, formatAURYX(bal))
		if maxBlocks > 0 && won >= maxBlocks {
			return
		}
	}
}

var stdin = bufio.NewReader(os.Stdin)

func ask(prompt string) string {
	fmt.Print(prompt)
	line, _ := stdin.ReadString('\n')
	return strings.TrimSpace(line)
}

func interactiveSetup(cfg Config) Config {
	fmt.Println("First-time setup:")
	fmt.Printf("  Server:   %s\n", cfg.RPC)
	fmt.Printf("  Contract: %s\n", cfg.Contract)
	fmt.Println("  Just paste your mining wallet's private key (from MetaMask). It's")
	fmt.Println("  saved locally in auryx-miner.json and never leaves your machine.")
	if cfg.PrivKey == "" {
		cfg.PrivKey = ask("  Private key: ")
	}
	saveConfig(cfg)
	return cfg
}

// settingsMenu lets the user pick how many CPU cores to mine with.
func settingsMenu(cfg *Config) {
	total := runtime.NumCPU()
	for {
		uiTitle("CPU SETTINGS")
		uiRow("Detected", fmt.Sprintf("%d cores", total))
		uiRow("Current", fmt.Sprintf("%s (%d cores)", cfg.coresLabel(), cfg.resolveCores()))
		fmt.Println()
		uiOption("1", fmt.Sprintf("Use all cores (%d)", total))
		uiOption("2", fmt.Sprintf("Use all but one (%d) - recommended", max(1, total-1)))
		uiOption("3", "Single core")
		uiOption("4", "Custom number of cores")
		uiOption("0", "Back")
		switch ask("\n   > ") {
		case "1":
			cfg.CoresMode = "all"
			saveConfig(*cfg)
		case "2":
			cfg.CoresMode = "all_but_one"
			saveConfig(*cfg)
		case "3":
			cfg.CoresMode = "single"
			saveConfig(*cfg)
		case "4":
			if n, err := strconv.Atoi(ask("  How many cores? ")); err == nil {
				cfg.CoresMode = "custom"
				cfg.Cores = n
				saveConfig(*cfg)
			}
		case "0":
			return
		}
	}
}

// menu returns an action for runMenu to handle: "quit", "changekey", or "logout".
func menu(chain *Chain, cfg *Config) string {
	go refreshRank(chain.From().Hex()) // fetch leaderboard position in the background
	for {
		bal := "(checking...)"
		if b, err := chain.Balance(context.Background(), chain.From()); err == nil {
			bal = formatAURYX(b) + " AURYX"
		}
		uiTitle("A U R Y X   M I N E R")
		rank := currentRank()
		if rank == "" {
			rank = "checking..."
		}
		uiRow("Leaderboard", rank)
		uiRow("All-time", fmt.Sprintf("%d blocks . %s AURYX . %s mined",
			cfg.LifetimeBlocks, formatAURYX(lifetimeWei(cfg)), humanDur(cfg.LifetimeSeconds)))
		fmt.Println()
		uiRow("Wallet", shortAddr(chain.From().Hex()))
		uiRow("Balance", bal)
		uiRow("Network", "Base Sepolia")
		uiRow("CPU", fmt.Sprintf("%s (%d of %d cores)", cfg.coresLabel(), cfg.resolveCores(), runtime.NumCPU()))
		uiRow("Version", "v"+minerVersion)
		if updateTo != "" {
			fmt.Printf("\n   %s%sUpdate available: v%s%s  %s%s%s\n",
				cGold, cBold, updateTo, cReset, cGray, releasesPage, cReset)
		}
		fmt.Println()
		uiOption("1", "Start mining")
		uiOption("2", "Stats (session + all-time)")
		uiOption("3", "Leaderboard")
		uiOption("4", "Settings (CPU cores)")
		uiOption("5", "Change wallet")
		uiOption("6", "Log out")
		uiOption("0", "Quit")
		switch ask("\n   > ") {
		case "1":
			runMining(chain, cfg, 0)
			go refreshRank(chain.From().Hex()) // position likely changed
		case "2":
			showStats(chain, cfg)
		case "3":
			showLeaderboard(chain)
		case "4":
			settingsMenu(cfg)
		case "5":
			return "changekey"
		case "6":
			return "logout"
		case "0":
			return "quit"
		}
	}
}

func main() {
	enableColors()
	rpc := flag.String("rpc", "", "RPC URL override")
	contract := flag.String("contract", "", "contract address override")
	key := flag.String("key", "", "private key override")
	blocks := flag.Int("blocks", 0, "mine N blocks then exit (0 = interactive menu)")
	coresFlag := flag.Int("cores", 0, "cores override (0 = use config)")
	flag.Parse()

	cfg := loadConfig()
	if *key != "" {
		cfg.PrivKey = *key
	}
	if *coresFlag > 0 {
		cfg.CoresMode = "custom"
		cfg.Cores = *coresFlag
	}
	// The coin + network are baked into the binary (overridable only by an explicit
	// --rpc/--contract flag). We force them here, ignoring any saved values, so a
	// redeploy is picked up on rebuild and a stale saved address can't mislead.
	cfg.RPC = defaultRPC
	if *rpc != "" {
		cfg.RPC = *rpc
	}
	cfg.Contract = defaultContract
	if *contract != "" {
		cfg.Contract = *contract
	}

	if cfg.PrivKey == "" {
		if *blocks > 0 {
			fmt.Println("missing --key for non-interactive run")
			os.Exit(1)
		}
		cfg = interactiveSetup(cfg)
	}

	if *blocks > 0 {
		chain, err := NewChain(cfg.RPC, cfg.Contract, cfg.PrivKey)
		if err != nil {
			fmt.Println("connect error:", err)
			os.Exit(1)
		}
		runMining(chain, &cfg, *blocks)
		return
	}
	// One-off check for a newer release, so the menu can flag it. Non-fatal.
	if latest := fetchLatestVersion(); latest != "" && isNewer(latest, minerVersion) {
		updateTo = latest
	}
	runMenu(&cfg)
}

// runMenu connects with the current key and runs the menu, rebuilding the
// connection when the user changes wallet, and clearing the key on log out.
func runMenu(cfg *Config) {
	for {
		chain, err := NewChain(cfg.RPC, cfg.Contract, cfg.PrivKey)
		if err != nil {
			fmt.Println("connect error:", err)
			ask("  Press Enter to close.") // so a double-clicked window doesn't vanish
			return
		}
		switch menu(chain, cfg) {
		case "quit":
			return
		case "changekey":
			fmt.Println("\n  Switch wallet — paste a different private key (or leave blank to cancel).")
			k := ask("  Private key: ")
			if k != "" {
				cfg.PrivKey = k
				saveConfig(*cfg)
				fmt.Println("  Wallet changed.")
			}
			// loop reconnects with the new key
		case "logout":
			cfg.PrivKey = ""
			saveConfig(*cfg)
			fmt.Println("\n  Logged out — your key has been removed from this computer.")
			fmt.Println("  Next launch will ask for a key again.")
			ask("  Press Enter to close.")
			return
		}
	}
}
