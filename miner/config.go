package main

import (
	"encoding/json"
	"math/big"
	"os"
	"runtime"
)

const configFile = "auryx-miner.json"

// Config persists the miner's settings (server, contract, key, CPU usage) and
// all-time totals (blocks won + AURYX earned across every run).
type Config struct {
	RPC            string `json:"rpc"`
	Contract       string `json:"contract"`
	PrivKey        string `json:"private_key"`
	Name           string `json:"name"`
	CoresMode      string `json:"cores_mode"` // "all" | "all_but_one" | "single" | "custom"
	Cores          int    `json:"cores"`      // used when CoresMode == "custom"
	LifetimeBlocks int    `json:"lifetime_blocks"`
	LifetimeWei    string `json:"lifetime_wei"` // total AURYX earned ever, in wei (decimal string)
}

// addLifetime records a won block + its reward into the all-time totals.
func (c *Config) addLifetime(reward *big.Int) {
	c.LifetimeBlocks++
	c.LifetimeWei = new(big.Int).Add(lifetimeWei(c), reward).String()
}

// lifetimeWei parses the stored all-time earnings back into a big.Int (0 if unset).
func lifetimeWei(c *Config) *big.Int {
	v := new(big.Int)
	if c.LifetimeWei != "" {
		v.SetString(c.LifetimeWei, 10)
	}
	return v
}

func loadConfig() Config {
	cfg := Config{CoresMode: "all_but_one"}
	if data, err := os.ReadFile(configFile); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	return cfg
}

func saveConfig(cfg Config) {
	if data, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		_ = os.WriteFile(configFile, data, 0o600)
	}
}

// resolveCores maps the CoresMode setting to an actual core count (1..NumCPU).
func (c Config) resolveCores() int {
	total := runtime.NumCPU()
	switch c.CoresMode {
	case "all":
		return total
	case "single":
		return 1
	case "custom":
		if c.Cores < 1 {
			return 1
		}
		if c.Cores > total {
			return total
		}
		return c.Cores
	default: // all_but_one
		if total <= 1 {
			return 1
		}
		return total - 1
	}
}

func (c Config) coresLabel() string {
	switch c.CoresMode {
	case "all":
		return "all cores"
	case "single":
		return "single core"
	case "custom":
		return "custom"
	default:
		return "all but one (recommended)"
	}
}
