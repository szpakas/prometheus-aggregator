package main

type sampleHasherFunc func(*sample) []byte

// sampleHasher is a hashing function used on samples.
var sampleHasher sampleHasherFunc

type sampleKind string

const (
	sampleUnknown sampleKind = ""

	// sampleCounter represents a counter
	sampleCounter sampleKind = "c"

	// sampleGauge represents a gauge
	sampleGauge sampleKind = "g"

	// sampleHistogramLinear represents histogram with linearly spaced buckets.
	// See Prometheus Go client LinearBuckets for details.
	sampleHistogramLinear sampleKind = "hl"
)

// sample represents single measurement submitted to the system.
// Samples are converted to metrics by collector.
type sample struct {
	// name is used to represent sample. It's used as metric name in export to prometheus.
	name string

	// kind of the sample wen mapped to prometheus metric type
	kind sampleKind

	// labels is a set of string pairs mapped to prometheus LabelPairs type
	labels map[string]string

	// value of the sample
	value float64

	// histogramDef is a set of values used in mapping for the histogram types
	histogramDef []string
}

// hash calculates a hash of the sample so it can be recognized.
// Should take all elements other than value under consideration.
func (s *sample) hash() []byte {
	return sampleHasher(s)
}
