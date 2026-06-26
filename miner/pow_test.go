package main

import (
	"math/big"
	"testing"
)

var maxTarget = new(big.Int).Lsh(big.NewInt(1), 252) // matches contract MAX_TARGET

func TestSolutionDigestDeterministic(t *testing.T) {
	var ch [32]byte
	var miner [20]byte
	ch[0] = 0xab
	miner[0] = 0xcd
	a := solutionDigest(ch, miner, big.NewInt(42))
	b := solutionDigest(ch, miner, big.NewInt(42))
	if a != b {
		t.Fatal("digest not deterministic")
	}
}

func TestAddressBinding(t *testing.T) {
	var ch [32]byte
	var alice, bob [20]byte
	alice[19] = 1
	bob[19] = 2
	if solutionDigest(ch, alice, big.NewInt(7)) == solutionDigest(ch, bob, big.NewInt(7)) {
		t.Fatal("digest must differ by miner address (front-run binding)")
	}
}

func TestMineFindsValidSolution(t *testing.T) {
	var ch [32]byte
	var miner [20]byte
	ch[0] = 0x01
	miner[19] = 0x09
	stop := make(chan struct{})
	sol := MineMultiCore(ch, miner, maxTarget, 4, stop)
	if sol == nil {
		t.Fatal("expected a solution")
	}
	// re-derive and confirm it really is below target
	d := solutionDigest(ch, miner, sol.Nonce)
	if d != sol.Digest {
		t.Fatal("returned digest does not match recomputation")
	}
	if !digestBelowTarget(d, maxTarget) {
		t.Fatal("returned digest is not below target")
	}
}

func TestStopReturnsNil(t *testing.T) {
	var ch [32]byte
	var miner [20]byte
	// target = 1 is effectively unsolvable; closing stop should yield nil promptly
	stop := make(chan struct{})
	close(stop)
	sol := MineMultiCore(ch, miner, big.NewInt(1), 2, stop)
	if sol != nil {
		t.Fatal("expected nil when stopped before finding a solution")
	}
}
