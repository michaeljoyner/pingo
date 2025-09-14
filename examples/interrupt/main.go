package main

import (
	"fmt"
	"log"
	"time"

	pingo "github.com/michaeljoyner/pingo/src"
)

func main() {
	dev, err := pingo.New()
	if err != nil {
		log.Fatal(err)
	}
	defer dev.ShutDown()

	irq, err := dev.ReqIRQ(17, pingo.EDGE_BOTH)
	if err != nil {
		fmt.Printf("%v\n", dev)
		log.Fatal(err)
	}

	quit := make(chan struct{})

	detections, err := irq.Listen(quit)

	go func() {
		for edge := range detections {
			fmt.Printf("edge detected: %d", edge)
		}
	}()

	time.Sleep(time.Second * 30)
	quit <- struct{}{}
	close(quit)
}
