package main

import (
	"encoding/binary"
	"testing"

	"golang.org/x/crypto/sha3"
)

// BenchmarkHashHot measures the optimized per-hash cost (one core).
// Run: go test -bench=HashHot -benchtime=2s
func BenchmarkHashHot(b *testing.B) {
	h := sha3.NewLegacyKeccak256()
	var input [84]byte
	var digest [32]byte
	for i := 0; i < b.N; i++ {
		binary.BigEndian.PutUint64(input[76:84], uint64(i))
		h.Reset()
		h.Write(input[:])
		h.Sum(digest[:0])
	}
}
