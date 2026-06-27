package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// leaderboardURL is the public stats server's mining leaderboard (read-only).
const leaderboardURL = "https://auryx.stackv.dev/api/leaderboard"

// sessionStats tracks this run (since the program launched). hashCounter resets
// each launch, so hashesDone() already gives "hashes this session".
type sessionStats struct {
	mu         sync.Mutex
	miningSecs float64 // total time actually spent mining this run
	blocks     int     // blocks won this run
	earnedWei  *big.Int
	netBlock   float64 // measured average network block time (EMA), 0 = unknown yet
}

var session = &sessionStats{earnedWei: big.NewInt(0)}

func (s *sessionStats) recordWin(reward *big.Int) {
	s.mu.Lock()
	s.blocks++
	s.earnedWei.Add(s.earnedWei, reward)
	s.mu.Unlock()
}

func (s *sessionStats) addMining(d time.Duration) {
	s.mu.Lock()
	s.miningSecs += d.Seconds()
	s.mu.Unlock()
}

// recordBlockTime folds an observed network block interval into an EMA, so the
// network-share estimate tracks reality rather than the contract's 60s target.
func (s *sessionStats) recordBlockTime(secs float64) {
	if secs <= 0 || secs > 3600 {
		return // ignore nonsense (clock jumps, very first sample, etc.)
	}
	s.mu.Lock()
	if s.netBlock == 0 {
		s.netBlock = secs
	} else {
		s.netBlock = 0.7*s.netBlock + 0.3*secs
	}
	s.mu.Unlock()
}

func (s *sessionStats) networkBlock() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.netBlock
}

func (s *sessionStats) snapshot() (blocks int, earned *big.Int, miningSecs, netBlock float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.blocks, new(big.Int).Set(s.earnedWei), s.miningSecs, s.netBlock
}

// --- small formatting helpers ---

// weiToAURYX converts a wei amount (18 decimals) to a float in whole AURYX.
func weiToAURYX(wei *big.Int) float64 {
	if wei == nil {
		return 0
	}
	f, _ := new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(1e18)).Float64()
	return f
}

// fmtAmt formats an AURYX amount with sensible precision (more decimals when small).
func fmtAmt(v float64) string {
	switch {
	case v >= 100:
		return fmt.Sprintf("%.0f", v)
	case v >= 1:
		return fmt.Sprintf("%.1f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}

// fmtCount formats a large count (e.g. hashes) as 36.2 G, 1.4 T, etc.
func fmtCount(n int64) string {
	f := float64(n)
	switch {
	case f >= 1e12:
		return fmt.Sprintf("%.1f T", f/1e12)
	case f >= 1e9:
		return fmt.Sprintf("%.1f G", f/1e9)
	case f >= 1e6:
		return fmt.Sprintf("%.1f M", f/1e6)
	case f >= 1e3:
		return fmt.Sprintf("%.1f K", f/1e3)
	default:
		return strconv.FormatInt(n, 10)
	}
}

// humanDur formats a duration in seconds as "1h 4m", "12m 30s", or "45s".
func humanDur(secs float64) string {
	s := int(secs)
	h, m, sec := s/3600, (s%3600)/60, s%60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, sec)
	default:
		return fmt.Sprintf("%ds", sec)
	}
}

// --- Stats screen ---

// showStats prints this-session and all-time totals, plus live earnings/share
// estimates derived from the current difficulty and your average hashrate.
func showStats(chain *Chain, cfg *Config) {
	ctx := context.Background()
	blocks, earned, miningSecs, netBlock := session.snapshot()
	hashes := hashesDone()

	var avgRate, earnPerHr, sharePct float64
	if miningSecs > 0 {
		avgRate = float64(hashes) / miningSecs
	}
	if avgRate > 0 {
		if target, err := chain.Target(ctx); err == nil {
			if exp := expectedHashes(target); exp > 0 {
				reward := new(big.Int).Mul(big.NewInt(50), big.NewInt(1_000_000_000_000_000_000))
				if r, e := chain.Reward(ctx); e == nil {
					reward = r
				}
				earnPerHr = avgRate * 3600 / exp * weiToAURYX(reward)
				nb := netBlock
				if nb <= 0 {
					nb = 60 // fall back to the contract's target block time
				}
				sharePct = avgRate * nb / exp * 100
				if sharePct > 100 {
					sharePct = 100
				}
			}
		}
	}

	uiTitle("STATS")
	uiRow("This session", "")
	uiRow("  Mining time", humanDur(miningSecs))
	uiRow("  Blocks won", strconv.Itoa(blocks))
	uiRow("  AURYX earned", formatAURYX(earned))
	uiRow("  Hashes tried", fmtCount(hashes))
	if blocks > 0 {
		uiRow("  Avg per block", humanDur(miningSecs/float64(blocks)))
	}
	if avgRate > 0 {
		uiRow("  Avg hashrate", formatRate(avgRate))
	}
	if earnPerHr > 0 {
		uiRow("  Est. earnings", "~"+fmtAmt(earnPerHr)+" AURYX/hr")
	}
	if sharePct > 0 {
		uiRow("  Network share", fmt.Sprintf("~%.0f%%", sharePct))
	}
	fmt.Println()
	uiRow("All time", "")
	uiRow("  Blocks won", strconv.Itoa(cfg.LifetimeBlocks))
	uiRow("  AURYX earned", formatAURYX(lifetimeWei(cfg)))
	fmt.Println()
	ask("   Press Enter to go back.")
}

// --- Leaderboard screen ---

type lbEntry struct {
	Address string `json:"address"`
	Mined   string `json:"mined"`
}

func fetchLeaderboard() ([]lbEntry, error) {
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Get(leaderboardURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out []lbEntry
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// showLeaderboard fetches the top miners from the stats server and marks your row.
func showLeaderboard(chain *Chain) {
	uiTitle("LEADERBOARD")
	uiInfo("Fetching from auryx.stackv.dev ...")
	lb, err := fetchLeaderboard()
	if err != nil {
		uiErr("Couldn't reach the leaderboard right now - try again in a moment.")
		ask("   Press Enter to go back.")
		return
	}
	me := strings.ToLower(chain.From().Hex())
	uiTitle("LEADERBOARD  (mined in the last ~9000 blocks)")
	if len(lb) == 0 {
		uiInfo("No miners on the board yet - mine a block to appear!")
		fmt.Println()
		ask("   Press Enter to go back.")
		return
	}
	youRanked := false
	for i, e := range lb {
		marker := ""
		if strings.ToLower(e.Address) == me {
			marker = "   <- you"
			youRanked = true
		}
		amt := e.Mined
		if f, perr := strconv.ParseFloat(e.Mined, 64); perr == nil {
			amt = fmtAmt(f)
		}
		uiRow(fmt.Sprintf("#%-2d %s", i+1, shortAddr(e.Address)), amt+" AURYX"+marker)
	}
	if !youRanked {
		fmt.Println()
		uiInfo("You're not in the top " + strconv.Itoa(len(lb)) + " yet - keep mining.")
	}
	fmt.Println()
	ask("   Press Enter to go back.")
}
