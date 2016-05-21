package main

import (
	"bytes"
	"net"
	"time"

	"github.com/pkg/errors"

	"github.com/prometheus/client_golang/prometheus"
)

type sampleHandler func(samples *sample) error

type server struct {
	sampleHandler sampleHandler
	buf           []byte

	metricRequestsTotal           prometheus.Counter
	metricSamplesTotal            prometheus.Counter
	metricRequestHandlingDuration prometheus.Summary
}

// newServer is factory for UDP server for incoming metrics data
//
// handler is a function of sampleHandler type responsible for dealing with incoming samples
// bs is a UDP buffer size in bytes
func newServer(handler sampleHandler, bs int) *server {
	s := server{
		sampleHandler: handler,
		buf:           make([]byte, bs),
		metricRequestsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "app_ingress_requests_total",
				Help: "Number of request entering server.",
			},
		),
		metricSamplesTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "app_ingress_samples_total",
				Help: "Number of samples entering server.",
			},
		),
		metricRequestHandlingDuration: prometheus.NewSummary(
			prometheus.SummaryOpts{
				Name: "app_ingress_request_handling_duration_ns",
				Help: "Time in ns spent on handling single request.",
			},
		),
	}
	prometheus.MustRegister(s.metricRequestsTotal)
	prometheus.MustRegister(s.metricSamplesTotal)
	prometheus.MustRegister(s.metricRequestHandlingDuration)
	return &s
}

func (s *server) Listen(ip string, port int) error {
	listenAddr := net.UDPAddr{
		Port: port,
		IP:   net.ParseIP(ip),
	}
	conn, err := net.ListenUDP("udp", &listenAddr)
	if err != nil {
		return errors.Wrap(err, "opening server socket failed")
	}

	go func() {
		var (
			reader *bytes.Reader
			tS     time.Time
		)

		for {
			n, _, _ := conn.ReadFromUDP(s.buf)

			tS = time.Now()

			s.metricRequestsTotal.Inc()

			reader = bytes.NewReader(s.buf[:n])

			samples, _ := parseSample(reader)

			s.metricSamplesTotal.Add(float64(len(samples)))

			for _, sample := range samples {
				_ = s.sampleHandler(sample)
			}

			s.metricRequestHandlingDuration.Observe(float64(time.Since(tS).Nanoseconds()))
		}
	}()

	return nil
}
