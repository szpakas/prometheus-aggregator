package main

import (
	"errors"
	"io"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// TODO(szpakas): move to config
	ingressQueueSize = 2048
)

var (
	// ErrIngressQueueFull is returned when ingress queue for samples is full.
	// Sample is not queued in such case.
	// Optional retries should be handled on caller side.
	ErrIngressQueueFull = errors.New("collector: ingress queue is full")

	appStartTimestampMetric = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "app_start_timestamp_seconds",
			Help: "Unix timestamp of the app collector start.",
		},
	)
	appDurationSecondsMetric = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "app_duration_seconds",
			Help: "Time in seconds since start of the app.",
		},
	)
)

type collector struct {
	startTime time.Time

	// ingress holds incoming samples for processing
	ingressCh chan *sample

	// sampleParser parses samples represented in transport (text) format and converts it to samples
	sampleParser func(r io.Reader) ([]sample, error)

	counters map[string]prometheus.Counter
	// countersMu protects scraping functions from interfering with processing
	countersMu sync.RWMutex

	gauges   map[string]prometheus.Gauge
	gaugesMu sync.RWMutex

	histograms   map[string]prometheus.Histogram
	histogramsMu sync.RWMutex

	testHookProcessSampleDone func()

	// quitCh is used to signal shutdown request
	quitCh chan struct{}

	// shutdownDownCh is used to signal when shutdown is done
	shutdownDownCh  chan struct{}
	shutdownTimeout time.Duration
}

func newCollector() *collector {
	return &collector{
		ingressCh:                 make(chan *sample, ingressQueueSize),
		counters:                  make(map[string]prometheus.Counter),
		gauges:                    make(map[string]prometheus.Gauge),
		histograms:                make(map[string]prometheus.Histogram),
		testHookProcessSampleDone: func() {},
		quitCh:          make(chan struct{}),
		shutdownDownCh:  make(chan struct{}),
		shutdownTimeout: time.Second,
	}
}

// Collect implements prometheus.Collector.
func (c *collector) Collect(ch chan<- prometheus.Metric) {
	appStartTimestampMetric.Collect(ch)

	appDurationSecondsMetric.Set(time.Now().Sub(c.startTime).Seconds())
	appDurationSecondsMetric.Collect(ch)

	c.countersMu.RLock()
	for _, m := range c.counters {
		m.Collect(ch)
	}
	c.countersMu.RUnlock()

	c.gaugesMu.RLock()
	for _, m := range c.gauges {
		m.Collect(ch)
	}
	c.gaugesMu.RUnlock()

	c.histogramsMu.RLock()
	for _, m := range c.histograms {
		m.Collect(ch)
	}
	c.histogramsMu.RUnlock()
}

// Describe implements prometheus.Collector.
func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	appStartTimestampMetric.Describe(ch)
	appDurationSecondsMetric.Describe(ch)
}

func (c *collector) start() {
	c.startTime = time.Now()

	appStartTimestampMetric.Set(float64(c.startTime.UnixNano()) / 1e9)

	go c.process()
}

func (c *collector) stop() error {
	close(c.quitCh)
	runtime.Gosched()

	select {
	case <-c.shutdownDownCh:
	case <-time.After(c.shutdownTimeout):
		return errors.New("collector: shutdown timed out")
	}

	return nil
}

// Write adds samples to internal queue for processing.
// Will result in ErrIngressQueueFull error if queue is full. The sample is not added to queue in such case.
func (c *collector) Write(s *sample) error {
	select {
	case c.ingressCh <- s:
	default:
		return ErrIngressQueueFull
	}
	return nil
}

func (c *collector) process() {
	var (
		s *sample
		h []byte
	)
	for {
		select {
		case s = <-c.ingressCh:
			h = s.hash()

			switch s.kind {
			case sampleCounter:
				m, found := c.counters[string(h)]
				if !found {
					m = prometheus.NewCounter(
						prometheus.CounterOpts{
							Name:        s.name,
							Help:        "auto",
							ConstLabels: s.labels,
						},
					)
					c.countersMu.Lock()
					c.counters[string(h)] = m
					c.countersMu.Unlock()
				}

				m.Add(s.value)

			case sampleGauge:
				m, found := c.gauges[string(h)]
				if !found {
					m = prometheus.NewGauge(
						prometheus.GaugeOpts{
							Name:        s.name,
							Help:        "auto",
							ConstLabels: s.labels,
						},
					)
					c.gaugesMu.Lock()
					c.gauges[string(h)] = m
					c.gaugesMu.Unlock()
				}

				m.Set(s.value)

			case sampleHistogramLinear:
				m, found := c.histograms[string(h)]
				if !found {
					start, _ := strconv.ParseFloat(s.histogramDef[0], 10)
					width, _ := strconv.ParseFloat(s.histogramDef[1], 10)
					count, _ := strconv.Atoi(s.histogramDef[2])
					m = prometheus.NewHistogram(
						prometheus.HistogramOpts{
							Name:        s.name,
							Help:        "auto",
							ConstLabels: s.labels,
							Buckets:     prometheus.LinearBuckets(start, width, count),
						},
					)
					c.histogramsMu.Lock()
					c.histograms[string(h)] = m
					c.histogramsMu.Unlock()
				}

				m.Observe(s.value)
			}

			c.testHookProcessSampleDone()

		case <-c.quitCh:
			close(c.shutdownDownCh)
			return
		}
	}
}
