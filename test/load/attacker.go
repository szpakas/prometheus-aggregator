package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type attacker struct {
	// wID is an id of the worker
	wID          int
	addr         *net.UDPAddr
	attackOrders chan struct{}
	stopOrder    chan struct{}
	waitGroup    *sync.WaitGroup

	counterTotalRequestsSend prometheus.Counter
}

func (a *attacker) Start() {
	// Opening destination-less UDP connection to achieve real fire-and-forget scenario.
	// If we use "Dial" than we will end-up with socket errors if the receiving end does not exists
	// (due to ICMP error messages passed as per RFC1122/4.1.3.3)
	lAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	conn, err := net.ListenUDP("udp4", lAddr)

	if err != nil {
		log.Fatal(err)
	}

	var (
		messageCnt int64 = 0
		outBuffer        = new(bytes.Buffer)
	)

	// -- metrics
	counterRequestsSend := prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem:   "worker",
		Name:        "requests_send",
		Help:        "test help",
		ConstLabels: prometheus.Labels{"worker_id": strconv.Itoa(a.wID)},
	})

	// Increasing of the wait group counter should be the last operation before spawning goroutine,
	// otherwise we could introduce race condition.
	a.waitGroup.Add(1)

	go func() {
		defer conn.Close()
		defer a.waitGroup.Done()

		tAttackStart := time.Now()

	AttackerMainLoop:
		for {
			tS := time.Now()
			select {
			case <-a.attackOrders:
			case <-a.stopOrder:
				log.Printf("#%d => stop order received", a.wID)
				break AttackerMainLoop
			}

			messageCnt += 1

			//counterRequestsSend.Write(sendInWorkerMetric)
			//a.counterTotalRequestsSend.Write(sendInTotalMetric)
			outBuffer.Reset()
			tD_ms := time.Since(tS).Nanoseconds() / 1e6
			s := fmt.Sprintf(`service=loader;workerId=%d
load_requests_total|c|duplicted=true|2
load_requests_total|c|1
load_attack_duration|g|%f
load_requests_duration_ms|hl|390;2;10|labelA=labelValueA|%d`, a.wID, time.Now().Sub(tAttackStart).Seconds(), tD_ms)

			outBuffer.WriteString(s)
			if err != nil {
				log.Println(err)
			}

			_, err = conn.WriteToUDP(outBuffer.Bytes(), a.addr)

			if err != nil {
				log.Println(err)
			} else {
				counterRequestsSend.Inc()
				a.counterTotalRequestsSend.Inc()
			}
		}
	}()
}
