package main

import (
	"log"
	"sync"
	"time"
)

func startOrderer(workersWaitGroup *sync.WaitGroup, attackRate *float64, ordersToAttack, orderToStop chan struct{}) {
	defer workersWaitGroup.Done()

	var (
		windowNo        = 0
		windowDone      = 0
		orderIntervalNs float64
		windowStart     = time.Now()
		windowNextStart = time.Now()
	)

OrderGeneratorMainLoop:
	for {
		orderIntervalNs = 1e9 / *attackRate
		nextOrderAt := windowStart.Add(time.Duration(orderIntervalNs * float64(windowDone+1)))

		// -- decide if we need reset of the window
		if !time.Now().Before(windowNextStart) {
			if windowNo > 0 {
				log.Printf("window summary, no.: %d, orders: %d/%d, interval: %d ns", windowNo, windowDone, int(*attackRate), int(orderIntervalNs))
			}
			windowNo += 1
			windowDone = 0
			windowStart = time.Now()
			windowNextStart = windowStart.Add(time.Second) // by default we have one window per second
		}

		// -- inject new orders to attack
		select {
		case ordersToAttack <- struct{}{}:
			windowDone += 1
		default:
			// queue full - should not happen
		}

		// -- wait for tick or interrupt
		select {
		case <-time.After(nextOrderAt.Sub(time.Now())):
		case <-orderToStop:
			log.Print("order generator => stop order received")
			break OrderGeneratorMainLoop
		}
	}
}
