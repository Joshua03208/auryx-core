package main

import (
	"encoding/binary"
	"math/big"
	"sync"

	"golang.org/x/crypto/sha3"
)

// Solution is a winning nonce and the digest it produced.
type Solution struct {
	Nonce  *big.Int
	Digest [32]byte
}

// solutionDigest computes keccak256(abi.encodePacked(bytes32 challenge,
// address miner, uint256 nonce)) — byte-for-byte identical to the AuryxToken
// contract. Used by tests and to re-verify a found solution.
func solutionDigest(challenge [32]byte, miner [20]byte, nonce *big.Int) [32]byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(challenge[:])
	h.Write(miner[:])
	var nb [32]byte
	nonce.FillBytes(nb[:]) // big-endian, left-padded to 32 bytes (uint256)
	h.Write(nb[:])
	var out [32]byte
	h.Sum(out[:0])
	return out
}

// digestBelowTarget reports whether digest (as a 256-bit big-endian integer)
// is strictly less than target.
func digestBelowTarget(digest [32]byte, target *big.Int) bool {
	return new(big.Int).SetBytes(digest[:]).Cmp(target) < 0
}

// lessThanBytes reports whether a < b comparing them as big-endian 256-bit ints.
// Allocation-free — used in the hot mining loop.
func lessThanBytes(a, b *[32]byte) bool {
	for i := 0; i < 32; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// MineMultiCore grinds nonces across `cores` goroutines until one finds a digest
// below `target`, or `stop` is closed. Returns the winning Solution, or nil if
// stopped first (e.g. the challenge changed — someone else mined the block).
//
// Hot-loop is allocation-free: each worker reuses one keccak hasher and a single
// 84-byte input buffer, varying only the low 8 bytes of the nonce.
func MineMultiCore(challenge [32]byte, miner [20]byte, target *big.Int, cores int, stop <-chan struct{}) *Solution {
	if cores < 1 {
		cores = 1
	}
	var targetB [32]byte
	target.FillBytes(targetB[:])

	resultCh := make(chan *Solution, cores)
	internalStop := make(chan struct{})
	var once sync.Once
	stopAll := func() { once.Do(func() { close(internalStop) }) }

	var wg sync.WaitGroup
	for c := 0; c < cores; c++ {
		wg.Add(1)
		go func(start uint64) {
			defer wg.Done()
			h := sha3.NewLegacyKeccak256()
			var input [84]byte // challenge(32) || miner(20) || nonce(32)
			copy(input[0:32], challenge[:])
			copy(input[32:52], miner[:])
			// input[52:84] is the big-endian uint256 nonce; we vary only the low 8 bytes
			var digest [32]byte
			nonce := start
			i := 0
			for {
				if i&0x3FFF == 0 { // check stop every 16384 hashes
					select {
					case <-internalStop:
						return
					case <-stop:
						return
					default:
					}
				}
				i++
				binary.BigEndian.PutUint64(input[76:84], nonce)
				h.Reset()
				h.Write(input[:])
				h.Sum(digest[:0])
				if lessThanBytes(&digest, &targetB) {
					resultCh <- &Solution{Nonce: new(big.Int).SetUint64(nonce), Digest: digest}
					stopAll()
					return
				}
				nonce += uint64(cores)
			}
		}(uint64(c))
	}

	var sol *Solution
	select {
	case sol = <-resultCh:
	case <-stop:
		sol = nil
	}
	stopAll()
	wg.Wait()
	return sol
}
