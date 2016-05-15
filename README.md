# Prometheus aggregator [![Build Status](https://travis-ci.org/szpakas/prometheus-aggregator.svg)](https://travis-ci.org/szpakas/prometheus-aggregator)

[![Apache 2.0 License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://github.com/szpakas/prometheus-aggregator/blob/master/LICENSE) [![Go Report Card](https://goreportcard.com/badge/github.com/szpakas/prometheus-aggregator)](https://goreportcard.com/report/github.com/szpakas/prometheus-aggregator)

`prometheus_aggregator` receives prometheus style samples, aggregates them and than exposes as prometheus metrics.

Project is in very early stage. It's tested but with minimal functionality. There is almost no optimisation done yet and a few known unbound memory allocations.

## Overview

Aggregator was designed as a way to bridge short-lived PHP scripts with [prometheus](https://github.com/prometheus/prometheus).
It extends ideas brought by [statsd_exporter](https://github.com/prometheus/statsd_exporter) by supporting native labeling and histograms.

Short-lived client is shooting samples via UDP toward aggregator server which parses, aggregates and stores them in memory.
The storage is then scraped using standard Prometheus HTTP endpoint (both text and binary exposition formats are supported).

    +----------+            +-------------------------+                        +--------------+
    |  client  |---(UDP)--->|  prometheus_aggregator  |<---(scrape /metrics)---|  Prometheus  |
    +----------+            +-------------------------+                        +--------------+

## Ingress format

Ingress format for samples is a text, line based format with two types of lines:
- shared labels
- sample

Each line should be terminated with single new-line.

### shared labels line

List of labels shared between all samples in the packet. Designed to lower the packet size by removing duplicates.

If present, it must be first line of the packet.
There is only one shared labels line allowed per packet.

    service=srvA1;host=hostA;phpVersion=5.6
    name_of_1_metric_total|c|labelA=labelValueA;label2=labelValue2|12.345
    name_of_2_metric_total|c|56
    name_of_3_metric|g|7.3

###  sample line

    name|type|typeConfig|labels|value

| field | desc               | allowed values |
|-------|--------------------|----------------|
| name  | name of the metric | a-zA-Z0-9_ |
| type  | type of the metric | counter: c<br>gauge: g<br>histogram with linear buckets: hl |
| type config | additional configuration for the type<br>currently used only for histograms | |
| labels | pairs of name and value separated by semicolon (;)<br>field is optional | name: a-zA-Z0-9<br>value: a-zA-Z0-9. |
| value | sample value<br>negative values are not yet supported | 0-9. |

## Metrics

As of now following metrics are supported:
- counter
- gauge
- histogram with linear buckets

### Counters

    name_of_2_metric_total|c|56
    name_of_1_metric_total|c|labelA=labelValueA;label2=labelValue2|12.345

### Gauges

    name_of_3_metric|g|labelA=labelValueA;label2=labelValue2|7.3
    name_of_3_metric|g|17.3
    
### Histograms with linear buckets

Type config values are passed to LinearBuckets(start, width float64, count int)

    name_of_1_metric_seconds|hl|3.3;2.0;5|12.345
    name_of_1_metric_seconds|hl|3.3;2.0;5|labelA=labelValueA;label2=labelValue2|12.345

## Internal architecture

There are two major components: sample server and collector.

### Sample server

Sample server is responsible for listening for the incoming samples via UDP, parsing each packet to samples and handing over to collector for processing.
As of now there is single goroutine responsible for reading and parsing.

### Collector

Collector is responsible for:
- processing of the samples to metrics metrics,
- storing metrics in memory,
- exposing metrics for scraping.

Collector implements prometheus.Collector interface.

New samples are buffered in ingress channel and then picked-up by a processor, converted to metrics and stored.
Processor is implemented as single goroutine.

## Usage

### Building
[govend](https://github.com/govend/govend) is used for vendoring.

```bash
govend -v 

go build
```

### Configuration

Configuration options
```go
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
```

### Running
```bash
# !/usr/bin/env bash

export APP_UDP_HOST="0.0.0.0"
export APP_UDP_PORT="9090"
export APP_UDP_BUFFER_SIZE="2048"
export APP_METRICS_HOST="0.0.0.0"
export APP_METRICS_PORT="8080"
export APP_LOG_LEVEL="DEBUG"

./prometheus-aggregator
```

## Running tests

    $ go test

Dedicated tests for race detection:

    $ go test ./ -run Test_Race_ -race -count 1000 -cpu 1,2,4,8,16

## Using docker

```bash
docker build -t prometheus-aggregator .

docker run -it --rm -p 10901:8080 -p 10902:9090 --name prometheus_aggregator prometheus-aggregator
```
