package main

import (
	"crypto/md5"
	"encoding/binary"
	"sort"
)

// hashMD5 calculates a hash of the sample so it can be recognized.
// Should take all elements other than value under consideration.
func hashMD5(s *sample) []byte {
	// TODO(szpakas): hash histogramDef
	hash := md5.New()

	hash.Write([]byte(s.kind))
	hash.Write([]byte("|"))

	hash.Write([]byte(s.name))

	// labels
	if len(s.labels) > 0 {
		hash.Write([]byte("|"))

		// get all keys sorted so hash is repeatable
		var keys []string
		for k := range s.labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, k := range keys {
			hash.Write([]byte(k))
			hash.Write([]byte("="))
			hash.Write([]byte(s.labels[k]))
			// separator between labels
			if i < len(keys)-1 {
				hash.Write([]byte(";"))
			}
		}
	}

	return hash.Sum([]byte{})
}

// hashProm calculates a hash based on Prometheus hashing algorithm.
func hashProm(s *sample) []byte {
	// TODO(szpakas): hash histogramDef
	h := hashPromNew()

	h = hashPromAdd(h, string(s.kind))
	h = hashPromAdd(h, "|")

	h = hashPromAdd(h, s.name)

	// labels
	if len(s.labels) > 0 {
		h = hashPromAdd(h, "|")

		// get all keys sorted so hash is repeatable
		var keys []string
		for k := range s.labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, k := range keys {
			h = hashPromAdd(h, k)
			h = hashPromAdd(h, "=")
			h = hashPromAdd(h, s.labels[k])
			// separator between labels
			if i < len(keys)-1 {
				h = hashPromAdd(h, ";")
			}
		}
	}

	bs := make([]byte, 8) // 64bit
	binary.LittleEndian.PutUint64(bs, h)

	return bs
}
