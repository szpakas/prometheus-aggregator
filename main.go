package main

import (
	"flag"
	"fmt"
	"net/http"
	"runtime"
	"syscall"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/log"
	"github.com/vrischmann/envconfig"
)

const (
	// ConfigAppPrefix prefixes all ENV values used to config the program.
	ConfigAppPrefix = "APP"
)

type config struct {
	// UdpHost is address on which UDP server is listening
	UDPHost string `envconfig:"default=0.0.0.0"`

	// UdpPort is port number on which UDP server is listening
	UDPPort int `envconfig:"default=8080"`

	// UDPBufferSize is a size of a buffer in bytes used for incoming samples.
	// Sample not fitting in buffer will be partially discarded.
	// Sync buffer size with client.
	UDPBufferSize int `envconfig:"default=4096"`

	// MetricsHost is address on which metric server for prometheus is listening
	MetricsHost string `envconfig:"default=0.0.0.0"`

	// MetricsHost is port number on which metric server for prometheus is listening
	MetricsPort int `envconfig:"default=9090"`

	// LogLevel is a minimal log severity required for the message to be logged.
	// Valid levels: [debug, info, warn, error, fatal, panic].
	LogLevel string `envconfig:"default=info"`

	// MaxProcs limits number of processors used by the app.
	MaxProcs int `envconfig:"default=0"`

	// SampleHasher sets hashing function used with samples.
	// Valid values:
	// - prom: hasher based on prometheus implementation of FNV-1a hash
	// - md5: naive MD5 implementation
	SampleHasher string `envconfig:"default=prom"`
}

func main() {
	// -> config from env
	cfg := &config{}
	if err := envconfig.InitWithPrefix(&cfg, ConfigAppPrefix); err != nil {
		exitOnFatal(err, "init config")
	}

	// convert env config to flag one for prometheus log package
	flag.Set("log.level", cfg.LogLevel)
	flag.Parse()

	log.Debugf("Parsed config from env => %+v", *cfg)

	if cfg.MaxProcs != 0 {
		nGot := runtime.GOMAXPROCS(cfg.MaxProcs)
		log.Debugf("Processor limiting, Req: %d, MaxAvailable: %d, NumCPU: %d", cfg.MaxProcs, nGot, runtime.NumCPU())
	}

	switch cfg.SampleHasher {
	case "prom":
		sampleHasher = hashProm
	case "md5":
		sampleHasher = hashMD5
	default:
		exitOnFatal(errors.New("unknown hashing implementation"), "sampleHasher selection")
	}
	log.Debugf("Sample hasher used: %s", cfg.SampleHasher)

	// TODO(szpakas): attach to signals for graceful shutdown and call c.stop()
	c := newCollector()
	prometheus.MustRegister(c)
	c.start()

	s := newServer(c.Write, cfg.UDPBufferSize)
	log.Infof("Starting ingrees samples server => %s:%d", cfg.UDPHost, cfg.UDPPort)
	if err := s.Listen(cfg.UDPHost, cfg.UDPPort); err != nil {
		exitOnFatal(err, "UDP server init")
	}

	http.Handle("/metrics", prometheus.Handler())

	//prometheus.EnableCollectChecks(true)

	metricsListenOn := fmt.Sprintf("%s:%d", cfg.MetricsHost, cfg.MetricsPort)
	log.Infof("Starting metrics server => %s", metricsListenOn)
	if err := http.ListenAndServe(metricsListenOn, nil); err != nil {
		exitOnFatal(err, "metric server")
	}
}

func exitOnFatal(err error, loc string) {
	log.Fatalf("EXIT on %s: err=%s\n", loc, err)
	syscall.Exit(1)
}
