package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/davecgh/go-spew/spew"
)

// fake to have spew always imported (yew, laziness)
var _ = spew.ConfigState{}

type Result struct {
	Err error
}

func main() {
	flag.Usage = func() {
		appName := path.Base(os.Args[0])
		fmt.Printf("Usage of %s:\n", appName)
		fmt.Printf("  %s [-n 5] [-r 10] [-pprof 8089] target.example:9876\n", appName)
		fmt.Printf("\n")
		flag.PrintDefaults()
	}
	threadsToUse := flag.Int("n", runtime.NumCPU(), "Number of threads to use for attackers")
	attackRatePassed := flag.Int("r", 10, "Rate of the attack (packets per sec)")
	pprofPort := flag.Int("pprof", 8089, "Port for pprof")

	flag.Parse()
	if len(flag.Args()) != 1 {
		log.Fatalln("please pass single target")
	}

	targetAddrString := flag.Args()[0]

	var attackRate float64 = float64(*attackRatePassed)

	counterRequestsSend := prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "main",
		Name:      "requests_send",
		Help:      "test help",
	})

	// -- general info
	log.Printf("procs: %d, threads: %d, rate: %d (req/s)\n", runtime.NumCPU(), *threadsToUse, int(attackRate))

	// extra goroutine for order generator
	runtime.GOMAXPROCS(*threadsToUse + 1)

	targetAddr, err := net.ResolveUDPAddr("udp4", targetAddrString)
	if err != nil {
		log.Fatal(err)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	//results := make(chan *Result)
	ordersToAttack := make(chan struct{}, int(attackRate)) // todo: base in on maxAttackRate
	orderToStop := make(chan struct{})
	var (
		attackers        []*attacker
		workersWaitGroup sync.WaitGroup
	)
	// -- set-up worker (attacker) pool
	for i := 0; i < *threadsToUse; i++ {
		a := &attacker{
			wID:                      i,
			addr:                     targetAddr,
			attackOrders:             ordersToAttack,
			stopOrder:                orderToStop,
			waitGroup:                &workersWaitGroup,
			counterTotalRequestsSend: counterRequestsSend,
		}
		attackers = append(attackers, a)
		go a.Start()
	}

	// -- set-up attack order generator
	workersWaitGroup.Add(1)
	go startOrderer(&workersWaitGroup, &attackRate, ordersToAttack, orderToStop)

	// input via stdin
	stdinReader := bufio.NewReader(os.Stdin)
	stdinChannel := make(chan string)
	go func() {
		for {
			text, _ := stdinReader.ReadString('\n')
			stdinChannel <- text
		}
	}()

	// for pprof
	go func() {
		http.ListenAndServe(fmt.Sprintf(":%d", *pprofPort), http.DefaultServeMux)
	}()

MainLoop:
	for {
		select {
		case stdinReceived := <-stdinChannel:
			log.Printf("Stdin data received: %s", stdinReceived)
			dataIn, err := strconv.Atoi(strings.Trim(stdinReceived, "\n"))
			if err == nil {
				oldAttackRate := attackRate
				attackRate = float64(dataIn)
				log.Printf("Attack rate modified: %d => %d", int(oldAttackRate), int(attackRate))
			} else {
				log.Printf("Attack rate conversion error: %s", err.Error())
			}
		case <-signals:
			log.Println("Signal detected, issuing close order")
			close(orderToStop)
			workersWaitGroup.Wait()

			break MainLoop
		}
	}
}
