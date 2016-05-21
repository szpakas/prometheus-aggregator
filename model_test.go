package main

import (
	"crypto/md5"
	"testing"

	a "github.com/stretchr/testify/assert"
)

func Test_Sample_Hash_MD5(t *testing.T) {
	// TODO(szpakas): add gauges and histograms
	testCases := map[string]struct {
		s  sample
		hD []byte
	}{
		"multiple labels": {
			sample{
				name: "name_of_1_metric_total", kind: sampleCounter,
				labels: map[string]string{"service": "srvA1", "host": "hostA", "phpVersion": "5.6", "labelA": "labelValueA", "label2": "labelValue2"},
				value:  12.345,
			},
			[]byte("c|name_of_1_metric_total|host=hostA;label2=labelValue2;labelA=labelValueA;phpVersion=5.6;service=srvA1"),
		},
		"single label": {
			sample{
				name: "name_of_1_metric_total", kind: sampleCounter,
				labels: map[string]string{"service": "srvA1"},
				value:  12.345,
			},
			[]byte("c|name_of_1_metric_total|service=srvA1"),
		},
		"no labels": {
			sample{
				name: "name_of_1_metric_total", kind: sampleCounter,
				labels: map[string]string{},
				value:  12.345,
			},
			[]byte("c|name_of_1_metric_total"),
		},
	}

	for k, tC := range testCases {
		h := md5.New()
		h.Write(tC.hD)
		a.Equal(t, h.Sum([]byte{}), hashMD5(&tC.s), "[%s] hash creation mismatch", k)
	}
}
