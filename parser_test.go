package main

import (
	"strings"
	"testing"

	a "github.com/stretchr/testify/assert"
)

func Test_SampleParser_Parse_Success(t *testing.T) {
	cases := map[string]struct {
		in  string
		exp []sample
	}{
		"counters with shared labels": {
			`service=srvA1;host=hostA;phpVersion=5.6
name_of_1_metric_total|c|labelA=labelValueA;label2=labelValue2|12.345
name_of_2_metric_total|c|56
name_of_3_metric|g|7.3`,
			[]sample{
				{
					name: "name_of_1_metric_total", kind: sampleCounter,
					labels: map[string]string{"service": "srvA1", "host": "hostA", "phpVersion": "5.6", "labelA": "labelValueA", "label2": "labelValue2"},
					value:  12.345,
				},
				{
					name: "name_of_2_metric_total", kind: sampleCounter,
					labels: map[string]string{"service": "srvA1", "host": "hostA", "phpVersion": "5.6"},
					value:  56,
				},
				{
					name: "name_of_3_metric", kind: sampleGauge,
					labels: map[string]string{"service": "srvA1", "host": "hostA", "phpVersion": "5.6"},
					value:  7.3,
				},
			},
		},
		"counters withouth shared labels": {
			`name_of_1_metric_total|c|labelA=labelValueA;label2=labelValue2|12.345
name_of_2_metric_total|c|56
name_of_3_metric|g|labelA=labelValueA;label2=labelValue2|7.3
name_of_3_metric|g|17.3`,
			[]sample{
				{
					name: "name_of_1_metric_total", kind: sampleCounter,
					labels: map[string]string{"labelA": "labelValueA", "label2": "labelValue2"},
					value:  12.345,
				},
				{
					name: "name_of_2_metric_total", kind: sampleCounter,
					labels: map[string]string{},
					value:  56,
				},
				{
					name: "name_of_3_metric", kind: sampleGauge,
					labels: map[string]string{"labelA": "labelValueA", "label2": "labelValue2"},
					value:  7.3,
				},
				{
					name: "name_of_3_metric", kind: sampleGauge,
					labels: map[string]string{},
					value:  17.3,
				},
			},
		},
		"histogram, linear buckets": {
			`name_of_1_metric_seconds|hl|3.3;2.0;5|labelA=labelValueA;label2=labelValue2|12.345`,
			[]sample{
				{
					name: "name_of_1_metric_seconds", kind: sampleHistogramLinear,
					labels:       map[string]string{"labelA": "labelValueA", "label2": "labelValue2"},
					value:        12.345,
					histogramDef: []string{"3.3", "2.0", "5"},
				},
			},
		},
	}

	for k, tc := range cases {
		r := strings.NewReader(tc.in)
		got, err := parseSample(r)
		if !a.NoError(t, err, k) {
			continue
		}

		for i := 0; i < len(tc.exp); i++ {
			if len(got) < i+1 {
				t.Errorf("[%s] Missing sample no. %d", k, i)
				continue
			}

			a.Equal(t, tc.exp[i], *got[i], k)
		}
	}
}
