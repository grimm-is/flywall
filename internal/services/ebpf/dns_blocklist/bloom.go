// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package dns_blocklist

import (
	"hash/fnv"
)

// BloomFilter represents a bloom filter for DNS domains
type BloomFilter struct {
	bitset []byte
	size   uint32
	hashes uint32
}

// NewBloomFilter creates a new bloom filter
func NewBloomFilter(size uint32, hashCount uint32) *BloomFilter {
	return &BloomFilter{
		bitset: make([]byte, (size+7)/8),
		size:   size,
		hashes: hashCount,
	}
}

// Add adds a domain to the bloom filter
func (bf *BloomFilter) Add(domain string) {
	for i := uint32(0); i < bf.hashes; i++ {
		hash := bf.hash(domain, i)
		index := hash % bf.size
		byteIndex := index / 8
		bitIndex := index % 8
		bf.bitset[byteIndex] |= 1 << bitIndex
	}
}

// Test checks if a domain might be in the bloom filter
func (bf *BloomFilter) Test(domain string) bool {
	for i := uint32(0); i < bf.hashes; i++ {
		hash := bf.hash(domain, i)
		index := hash % bf.size
		byteIndex := index / 8
		bitIndex := index % 8
		if bf.bitset[byteIndex]&(1<<bitIndex) == 0 {
			return false
		}
	}
	return true
}

// Clear clears the bloom filter
func (bf *BloomFilter) Clear() {
	for i := range bf.bitset {
		bf.bitset[i] = 0
	}
}

// GetBitset returns the underlying bitset
func (bf *BloomFilter) GetBitset() []byte {
	return bf.bitset
}

// SetBitset sets the underlying bitset
func (bf *BloomFilter) SetBitset(bitset []byte) {
	if len(bitset) == len(bf.bitset) {
		copy(bf.bitset, bitset)
	}
}

// hash computes a hash for the domain with the given seed
func (bf *BloomFilter) hash(domain string, seed uint32) uint32 {
	h := fnv.New32a()
	h.Write([]byte{byte(seed)})
	h.Write([]byte(domain))
	return h.Sum32()
}

// EstimateFalsePositiveRate estimates the false positive rate
func (bf *BloomFilter) EstimateFalsePositiveRate(items uint32) float64 {
	if bf.size == 0 || bf.hashes == 0 {
		return 1.0
	}

	// Formula: (1 - e^(-k*n/m))^k
	// where k = number of hashes, n = number of items, m = size of filter
	k := float64(bf.hashes)
	n := float64(items)
	m := float64(bf.size)

	exponent := -k * n / m
	if exponent < -700 { // Prevent underflow
		exponent = -700
	}

	pow := 1.0 - exp(exponent)
	result := pow

	for i := uint32(1); i < bf.hashes; i++ {
		result *= pow
	}

	return result
}

// exp is a simple exponential function approximation
func exp(x float64) float64 {
	// Simple Taylor series approximation for small x
	if x > -1.0 {
		return 1.0 + x + x*x/2 + x*x*x/6
	}

	// For larger negative x, use continued fraction
	// This is a very rough approximation
	return 0.0
}
