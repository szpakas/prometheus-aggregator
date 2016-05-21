package main

import (
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	a "github.com/stretchr/testify/assert"
)

func Test_Collector_New(t *testing.T) {
	c := newCollector()
	a.IsType(t, &collector{}, c)
	a.Equal(t, ingressQueueSize, cap(c.ingressCh))
	a.NotNil(t, c.counters)
	a.NotNil(t, c.gauges)
	a.NotNil(t, c.histograms)
}

var tfCollectorSamples = []*sample{
	{
		name: "name_of_1_metric_total", kind: sampleCounter,
		labels: map[string]string{"service": "srvA1", "host": "hostA", "phpVersion": "5.6", "labelA": "labelValueA", "label2": "labelValue2"},
		value:  1.1,
	},
	{
		name: "name_of_2_metric_total", kind: sampleCounter,
		labels: map[string]string{"service": "srvA1", "host": "hostA", "phpVersion": "5.6"},
		value:  100.01,
	},
	{
		name: "name_of_1_metric_total", kind: sampleCounter,
		labels: map[string]string{"labelA": "labelValueA", "label2": "labelValue2"},
		value:  1000.001,
	},
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
		name: "name_of_3_metric", kind: sampleGauge,
		labels: map[string]string{},
		value:  17.3,
	},
}

func Test_Collector_Write_Success(t *testing.T) {
	c := newCollector()
	for _, s := range tfCollectorSamples {
		c.Write(s)
	}
	if !a.Len(t, c.ingressCh, len(tfCollectorSamples)) {
		t.FailNow()
	}
	for i := 0; i < len(tfCollectorSamples); i++ {
		a.Equal(t, tfCollectorSamples[i], <-c.ingressCh)
	}
}

func Test_Collector_Write_ChannelFull(t *testing.T) {
	c := &collector{}
	// size of buffer is smaller than number of samples to store
	bufLen := 2
	c.ingressCh = make(chan *sample, bufLen)
	errGot := make(chan error, len(tfCollectorSamples))

	for _, s := range tfCollectorSamples {
		errGot <- c.Write(s)
	}

	if !a.Len(t, c.ingressCh, bufLen) {
		t.FailNow()
	}

	// check on calls which should add samples to buffer
	for i := 0; i < bufLen; i++ {
		a.Equal(t, tfCollectorSamples[i], <-c.ingressCh)
		a.Nil(t, <-errGot)
	}

	// check on calls which should result in errors
	for i := bufLen; i < len(tfCollectorSamples); i++ {
		a.Equal(t, ErrIngressQueueFull, <-errGot)
	}
}

func thInitSampleHasher(h sampleHasherFunc) func() {
	sampleHasherOld := sampleHasher
	sampleHasher = h
	return func() {
		sampleHasher = sampleHasherOld
	}
}

func thCollectorProcessPopulate(c *collector, samples []*sample) {
	for _, s := range samples {
		c.ingressCh <- s
	}
}

func thCollectorProcessSynchronise(t *testing.T, c *collector) {
	c.shutdownTimeout = time.Millisecond * 100
	sampleProcessingDoneCh := make(chan struct{})

	// failInTestHook is used to pass failures from "process" goroutine
	failInTestHook := make(chan struct{}, 1)
	c.testHookProcessSampleDone = func() {
		select {
		case sampleProcessingDoneCh <- struct{}{}:
		case <-time.After(time.Millisecond * 10):
			failInTestHook <- struct{}{}
		}
		runtime.Gosched()
	}

	go c.process()

inProcessing:
	for {
		select {
		case <-sampleProcessingDoneCh:
			if len(c.ingressCh) == 0 {
				break inProcessing
			}
		case <-failInTestHook:
			t.Fatal("timeout in testHookProcessSampleDone")
		case <-time.After(time.Millisecond * 10):
			t.Fatal("timeout in synchronise")
		}
		runtime.Gosched()
	}

	if err := c.stop(); err != nil {
		t.Fatal("timeout in shutdown")
	}
}

func Test_Collector_Process_Success_NewHashes(t *testing.T) {
	tests := map[string]struct {
		h sampleHasherFunc
	}{
		"md5":  {hashMD5},
		"prom": {hashProm},
	}

	sampleHasherOld := sampleHasher
	defer func() {
		sampleHasher = sampleHasherOld
	}()

	for sym, tc := range tests {
		sampleHasher = tc.h

		c := newCollector()
		thCollectorProcessPopulate(c, tfCollectorSamples)
		thCollectorProcessSynchronise(t, c)

		// check if the samples are converted to metrics
		var hashesGot []string
		for h := range c.counters {
			hashesGot = append(hashesGot, h)
		}
		for h := range c.gauges {
			hashesGot = append(hashesGot, h)
		}
		sort.Strings(hashesGot)

		var hashesExp []string
		for _, s := range tfCollectorSamples {
			hashesExp = append(hashesExp, string(s.hash()))
		}
		sort.Strings(hashesExp)

		a.Equal(t, hashesExp, hashesGot, sym)
	}
}

func Test_Collector_Process_Success_Existing(t *testing.T) {
	defer thInitSampleHasher(hashMD5)()
	c := newCollector()
	// duplicate to simulate adding existing samples
	thCollectorProcessPopulate(c, tfCollectorSamples)
	thCollectorProcessPopulate(c, tfCollectorSamples)
	thCollectorProcessSynchronise(t, c)

	// check if the samples are converted to metrics
	var hashesGot []string
	for h := range c.counters {
		hashesGot = append(hashesGot, h)
	}
	for h := range c.gauges {
		hashesGot = append(hashesGot, h)
	}
	sort.Strings(hashesGot)

	var hashesExp []string
	for _, s := range tfCollectorSamples {
		hashesExp = append(hashesExp, string(s.hash()))
	}
	sort.Strings(hashesExp)

	a.Equal(t, hashesExp, hashesGot)
}

func Test_Collector_Process_Success_Values(t *testing.T) {
	defer thInitSampleHasher(hashMD5)()
	c := newCollector()
	// duplicate to simulate adding existing samples
	thCollectorProcessPopulate(c, tfCollectorSamples)
	thCollectorProcessPopulate(c, tfCollectorSamples)
	thCollectorProcessPopulate(c, tfCollectorSamples)
	thCollectorProcessSynchronise(t, c)

	for _, s := range tfCollectorSamples {
		var mm dto.Metric
		switch s.kind {
		case sampleCounter:
			m := c.counters[string(s.hash())]
			m.Write(&mm)
			// samples were added 3 times
			a.Equal(t, s.value*3, mm.Counter.GetValue())
		case sampleGauge:
			m := c.gauges[string(s.hash())]
			m.Write(&mm)
			a.Equal(t, s.value, mm.Gauge.GetValue())
		}
	}
}

func Test_Collector_Process_Success_HistogramLinear(t *testing.T) {
	s1 := sample{
		name: "name_of_1_metric_seconds", kind: sampleHistogramLinear,
		labels:       map[string]string{"labelA": "labelValueA", "label2": "labelValue2"},
		histogramDef: []string{"8.0", "2.0", "10"},
	}
	s2 := *&s1

	s1.value = 10
	s2.value = 20

	defer thInitSampleHasher(hashMD5)()
	c := newCollector()
	c.ingressCh <- &s1
	c.ingressCh <- &s2

	thCollectorProcessSynchronise(t, c)

	var mm dto.Metric
	m := c.histograms[string(s1.hash())]
	m.Write(&mm)
	a.Equal(t, uint64(2), mm.Histogram.GetSampleCount())
	a.Equal(t, float64(30), mm.Histogram.GetSampleSum())
	if !a.Len(t, mm.Histogram.GetBucket(), 10) {
		t.FailNow()
	}

	// inspect one of the buckets
	b := mm.Histogram.GetBucket()[3]
	a.Equal(t, uint64(1), b.GetCumulativeCount())
	a.Equal(t, float64(14), b.GetUpperBound())
}

func Test_Collector_Collect_NoMetric(t *testing.T) {
	c := newCollector()
	metricCh := make(chan prometheus.Metric, 2048)
	c.Collect(metricCh)

	if !a.Len(t, metricCh, 3) {
		t.FailNow()
	}

	mA := <-metricCh
	a.Equal(t, c.metricAppStart.Desc(), mA.Desc())

	mB := <-metricCh
	a.Equal(t, c.metricAppDuration.Desc(), mB.Desc())
}

func Test_Collector_Collect_MetricFromSamples(t *testing.T) {
	c := newCollector()

	// set-up
	c.counters["c1"] = prometheus.NewCounter(prometheus.CounterOpts{Name: "counter_A", Help: "auto"})
	c.counters["c2"] = prometheus.NewCounter(prometheus.CounterOpts{Name: "counter_B", Help: "auto"})
	c.gauges["g1"] = prometheus.NewGauge(prometheus.GaugeOpts{Name: "gauge_A", Help: "auto"})
	c.gauges["g2"] = prometheus.NewGauge(prometheus.GaugeOpts{Name: "gauge_B", Help: "auto"})
	c.histograms["hl1"] = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "histLinear_A", Help: "auto"})

	expDescMap := make(map[string]prometheus.Desc)
	descHash := func(d *prometheus.Desc) []byte {
		return []byte(d.String())
	}
	addDesc := func(m map[string]prometheus.Desc, me prometheus.Metric) {
		d := me.Desc()
		m[string(descHash(d))] = *d
	}
	addDesc(expDescMap, c.counters["c1"])
	addDesc(expDescMap, c.counters["c2"])
	addDesc(expDescMap, c.gauges["g1"])
	addDesc(expDescMap, c.gauges["g2"])
	addDesc(expDescMap, c.histograms["hl1"])
	addDesc(expDescMap, c.metricAppStart)
	addDesc(expDescMap, c.metricAppDuration)
	addDesc(expDescMap, c.metricQueueLength)

	metricCh := make(chan prometheus.Metric, 2048)

	// call
	c.Collect(metricCh)

	// check
	if !a.Len(t, metricCh, len(expDescMap)) {
		t.FailNow()
	}

	gotDescMap := make(map[string]prometheus.Desc)
	totMetric := len(metricCh)
	for i := 0; i < totMetric; i++ {
		addDesc(gotDescMap, <-metricCh)
	}
	a.Equal(t, expDescMap, gotDescMap)
}
