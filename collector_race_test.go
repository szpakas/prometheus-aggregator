package main

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// Test_Collector_ProcessVsCollect checks against the race between write, process and collect/describe.
// Test with:
//   go test ./ -run Test_Race_Collector_WriteVsProcessVsCollect -race -count 1000 -cpu 1,2,4,8,16
func Test_Race_Collector_WriteVsProcessVsCollect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test")
	}
	samples := []*sample{
		{
			name: "name_of_2_metric_total", kind: sampleCounter,
			labels: map[string]string{},
			value:  10000.0001,
		},
		{
			name: "name_of_3_metric", kind: sampleGauge,
			labels: map[string]string{"labelA": "labelValueA", "label2": "labelValue2"},
			value:  7.3,
		},
		{
			name: "name_of_3_metric", kind: sampleHistogramLinear,
			labels:       map[string]string{},
			histogramDef: []string{"3.3", "2.0", "5"},
			value:        17.3,
		},
	}

	defer thInitSampleHasher(hashMD5)()
	c := newCollector()
	c.shutdownTimeout = time.Millisecond * 100

	go c.process()

	wg := sync.WaitGroup{}
	wg.Add(2)
	wgDoneCh := make(chan struct{})

	errInWrite := make(chan error, 1)
	go func() {
		for i := 0; i < 100; i++ {
			for i, s := range samples {
				if err := c.Write(s); err != nil {
					errInWrite <- errors.Wrap(err, fmt.Sprintf("in sample %d", i))
					return
				}
			}
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 100; i++ {
			descCh := make(chan *prometheus.Desc, 100) // should be bigger than number of metrics described
			c.Describe(descCh)
			metricCh := make(chan prometheus.Metric, 100) // channel need enough capacity to get all metrics
			c.Collect(metricCh)
		}
		wg.Done()
	}()

	go func() {
		wg.Wait()
		close(wgDoneCh)
	}()

inTesting:
	for {
		select {
		case err := <-errInWrite:
			t.Fatal(errors.Wrap(err, "error on write"))
		case <-time.After(time.Millisecond * 10):
			t.Fatal("timeout on testing")
		case <-wgDoneCh:
			break inTesting
		}
		runtime.Gosched()
	}

	if err := c.stop(); err != nil {
		t.Fatal("timeout in shutdown")
	}
}
