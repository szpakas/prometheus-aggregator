package main

// Inline and byte-free variant of hash/fnv's fnv64a.
// Taken from Prometheus, licence MIT

const (
	offset64 = 14695981039346656037
	prime64  = 1099511628211
)

// hashNew initializes a new fnv64a hash value.
func hashPromNew() uint64 {
	return offset64
}

// hashAdd adds a string to a fnv64a hash value, returning the updated hash.
func hashPromAdd(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}

// hashAddByte adds a byte to a fnv64a hash value, returning the updated hash.
func hashPromAddByte(h uint64, b byte) uint64 {
	h ^= uint64(b)
	h *= prime64
	return h
}
